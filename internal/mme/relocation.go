// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

var (
	ErrUnknownTargetENB     = fmt.Errorf("%w: mme: the target eNB is not connected", interworking.ErrUnknownTarget)
	ErrNoRelocatablePDN     = errors.New("mme: no PDN connection could be opened for the relocated UE")
	ErrRelocationInProgress = errors.New("mme: a handover to EPS is already in progress for this subscriber")
	ErrNoRelocation         = errors.New("mme: no handover to EPS is in progress for this subscriber")
)

func (m *MME) ForwardRelocation(ctx context.Context, req interworking.ForwardRelocationRequest) (interworking.ForwardRelocationResponse, error) {
	var none interworking.ForwardRelocationResponse

	supi, err := etsi.NewSUPIFromIMSI(req.IMSI)
	if err != nil {
		return none, fmt.Errorf("mme: forward relocation for %q: %w", req.IMSI, err)
	}

	globalENBID, err := targetGlobalENBID(req.Target)
	if err != nil {
		return none, err
	}

	target, ok := m.FindRadioByGlobalENBID(globalENBID)
	if !ok {
		return none, fmt.Errorf("%w: %s", ErrUnknownTargetENB, ENBID(globalENBID))
	}

	ue := NewUeContext()
	m.SetIMSI(ue, req.IMSI)

	if err := ue.InstallRelocatedSecurityContext(req.SecurityContext, MintAuthProofForInterworking()); err != nil {
		return none, err
	}

	ue.mu.Lock()
	ue.Ambr = &models.Ambr{Uplink: req.UEAMBRUplink, Downlink: req.UEAMBRDownlink}
	ue.mu.Unlock()

	ue.TransitionTo(EMMRegistrationInitiated)

	if !m.beginRelocation(supi, ue) {
		return none, ErrRelocationInProgress
	}

	resp, err := m.relocate(ctx, ue, target, ENBID(globalENBID), req)
	if err != nil {
		m.dropRelocation(ctx, ue)

		return none, err
	}

	return resp, nil
}

func (m *MME) dropRelocation(ctx context.Context, ue *UeContext) {
	m.ReleaseAllSessions(ctx, ue)
	m.RemoveUe(ue)

	if supi, err := etsi.NewSUPIFromIMSI(ue.IMSI()); err == nil {
		m.endRelocation(supi, ue)
	}
}

func (m *MME) relocate(ctx context.Context, ue *UeContext, target *Radio, targetID string, req interworking.ForwardRelocationRequest) (interworking.ForwardRelocationResponse, error) {
	var none interworking.ForwardRelocationResponse

	accepted, err := m.openRelocatedPDNs(ctx, ue, req.PDNConnections)
	if err != nil {
		return none, err
	}

	bearers, candidates, ok := HandoverBearers(ue)
	if !ok {
		return none, ErrNoRelocatablePDN
	}

	restriction, err := m.handoverRestrictionList(ctx)
	if err != nil {
		return none, err
	}

	targetMMEID, outcome, ok := m.prepareRelocation(ue, target.Conn, candidates)
	if !ok {
		return none, fmt.Errorf("mme: could not prepare a handover to eNB %s", targetID)
	}

	hoReq := &s1ap.HandoverRequest{
		MMEUES1APID:  targetMMEID,
		HandoverType: s1ap.HandoverTypeFiveGSToEPS,
		Cause:        s1ap.Ptr(causeHandoverCNReason),
		UEAMBR: s1ap.UEAggregateMaximumBitRate{
			DL: s1ap.BitRate(req.UEAMBRDownlink.Bps()),
			UL: s1ap.BitRate(req.UEAMBRUplink.Bps()),
		},
		ERABToBeSetup:           bearers,
		SourceToTarget:          s1ap.TransparentContainer(req.SourceToTarget),
		UESecurityCapabilities:  S1apSecurityCapabilities(ue.UeNetCap()),
		HandoverRestrictionList: restriction,
		SecurityContext: s1ap.SecurityContext{
			NextHopChainingCount: req.SecurityContext.NCC,
			NextHopParameter:     s1ap.SecurityKey(req.SecurityContext.NH),
		},
	}

	b, err := hoReq.Marshal()
	if err != nil {
		m.ClearHandover(ue)

		return none, fmt.Errorf("mme: marshal inter-system Handover Request: %w", err)
	}

	logger.From(ctx, logger.MmeLog).Info("Handover Request (5GS to EPS)",
		zap.String("imsi", ue.IMSI()),
		zap.Uint32("target-mme-ue-id", uint32(targetMMEID)),
		zap.String("target-enb", targetID),
		zap.Int("e-rabs", len(bearers)))
	m.SendToRadio(ctx, target.Conn, S1APProcedureHandoverRequest, b)
	m.SuperviseHandover(ue)

	select {
	case out := <-outcome:
		if out.err != nil {
			return none, out.err
		}

		return interworking.ForwardRelocationResponse{
			TargetToSource:      out.targetToSource,
			AcceptedPDUSessions: m.dropUnadmittedPDNs(ctx, ue, accepted, out.unadmitted),
		}, nil
	case <-ctx.Done():
		m.abandonHandover(ue)

		return none, ctx.Err()
	}
}

func (m *MME) openRelocatedPDNs(ctx context.Context, ue *UeContext, conns []interworking.PDNConnection) (map[uint8]uint8, error) {
	accepted := make(map[uint8]uint8, len(conns))

	for _, c := range conns {
		qos, err := ResolveQoSByAPN(ctx, m, ue.IMSI(), c.APN)
		if err != nil {
			logger.From(ctx, logger.MmeLog).Warn("relocated PDN connection has no QoS in the subscriber profile; leaving it behind",
				zap.String("imsi", ue.IMSI()), zap.String("apn", c.APN), zap.Error(err))

			continue
		}

		if !qos.Allow4G {
			logger.From(ctx, logger.MmeLog).Warn("relocated PDN connection is not allowed on 4G; leaving it behind",
				zap.String("imsi", ue.IMSI()), zap.String("apn", c.APN))

			continue
		}

		bearer, err := m.Session.CreateEPSSession(ctx, models.EPSBearerRequest{
			IMSI:              ue.IMSI(),
			EPSBearerIdentity: c.EPSBearerIdentity,
			PDUSessionID:      c.PDUSessionID,
			Snssai:            qos.Snssai,
			PolicyID:          qos.PolicyID,
			APN:               qos.APN,
			AMBRUplink:        qos.SessAmbrUL,
			AMBRDownlink:      qos.SessAmbrDL,
			IPv4Pool:          qos.IPv4Pool,
			IPv6Pool:          qos.IPv6Pool,
			DNS:               qos.DNS,
			MTU:               qos.MTU,
			RequestType:       eps.RequestTypeHandover,
		})
		if err != nil {
			logger.From(ctx, logger.MmeLog).Warn("failed to take over a PDU session as a PDN connection; leaving it behind",
				zap.String("imsi", ue.IMSI()), zap.String("apn", c.APN),
				zap.Uint8("ebi", c.EPSBearerIdentity), zap.Error(err))

			continue
		}

		m.publishRelocatedPDN(ue, c.EPSBearerIdentity, qos, bearer)

		accepted[c.EPSBearerIdentity] = c.PDUSessionID
	}

	if len(accepted) == 0 {
		return nil, ErrNoRelocatablePDN
	}

	return accepted, nil
}

func (m *MME) publishRelocatedPDN(ue *UeContext, ebi uint8, qos *EpsQoS, bearer models.EPSBearer) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.publishPDNLocked(ebi, qos, bearer).Transferred = true
}

func (m *MME) dropUnadmittedPDNs(ctx context.Context, ue *UeContext, accepted map[uint8]uint8, unadmitted []HandoverCandidate) []uint8 {
	for _, c := range unadmitted {
		if p := m.LookupPDN(ue, c.Ebi); p != nil {
			m.ReleasePDN(ctx, ue, p)
		}

		delete(accepted, c.Ebi)
	}

	out := make([]uint8, 0, len(accepted))
	for _, pduSessionID := range accepted {
		out = append(out, pduSessionID)
	}

	slices.Sort(out)

	return out
}

func (m *MME) CompleteRelocation(ctx context.Context, ue *UeContext) {
	imsi := ue.IMSI()

	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("relocated UE has no usable identity", zap.Error(err))
		return
	}

	ue.TransitionTo(EMMRegistered)
	m.CommitUEIdentity(ctx, ue, MintAuthProofForInterworking())
	m.endRelocation(supi, ue)

	logger.From(ctx, logger.MmeLog).Info("handover from 5GS complete", zap.String("imsi", imsi))

	if m.FiveGS == nil {
		return
	}

	if err := m.FiveGS.RelocationComplete(ctx, imsi); err != nil {
		logger.From(ctx, logger.MmeLog).Warn("the 5GS peer refused the relocation completion notification",
			zap.String("imsi", imsi), zap.Error(err))
	}
}

func (m *MME) RelocationCancel(ctx context.Context, imsi string) error {
	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return fmt.Errorf("mme: relocation cancel for %q: %w", imsi, err)
	}

	m.mu.Lock()
	ue, ok := m.relocating[supi]
	m.mu.Unlock()

	if !ok {
		return ErrNoRelocation
	}

	logger.From(ctx, logger.MmeLog).Info("Relocation Cancel", zap.String("imsi", imsi))

	m.abandonHandover(ue)
	m.dropRelocation(ctx, ue)

	return nil
}

func (m *MME) beginRelocation(supi etsi.SUPI, ue *UeContext) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, busy := m.relocating[supi]; busy {
		return false
	}

	m.relocating[supi] = ue

	return true
}

func (m *MME) endRelocation(supi etsi.SUPI, ue *UeContext) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if held, ok := m.relocating[supi]; ok && held == ue {
		delete(m.relocating, supi)
	}
}

func (m *MME) handoverRestrictionList(ctx context.Context) (*s1ap.HandoverRestrictionList, error) {
	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		return nil, fmt.Errorf("mme: resolve the serving PLMN: %w", err)
	}

	encoded, err := EncodePLMN(plmn)
	if err != nil {
		return nil, err
	}

	return &s1ap.HandoverRestrictionList{ServingPLMN: encoded}, nil
}

func targetGlobalENBID(target interworking.ENBIdentity) (s1ap.GlobalENBID, error) {
	kind, ok := map[uint8]s1ap.ENBIDKind{
		18: s1ap.ENBIDShortMacro,
		20: s1ap.ENBIDMacro,
		21: s1ap.ENBIDLongMacro,
		28: s1ap.ENBIDHome,
	}[target.Bits]
	if !ok {
		return s1ap.GlobalENBID{}, fmt.Errorf("mme: no eNB identity is %d bits wide", target.Bits)
	}

	plmn, err := EncodePLMN(target.PlmnID)
	if err != nil {
		return s1ap.GlobalENBID{}, err
	}

	return s1ap.GlobalENBID{
		PLMNIdentity: plmn,
		ENBID:        s1ap.ENBID{Kind: kind, Value: target.ID},
	}, nil
}
