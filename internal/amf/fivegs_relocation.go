// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

var _ interworking.FiveGSPeer = (*AMF)(nil)

var (
	ErrUnknownTargetRAN         = fmt.Errorf("%w: amf: the target NG-RAN node is not connected", interworking.ErrUnknownTarget)
	ErrRelocationFromEPSBusy    = errors.New("amf: a handover from EPS is already in progress for this subscriber")
	ErrNoRelocationFromEPS      = errors.New("amf: no handover from EPS is in progress for this subscriber")
	ErrNoAdmissiblePDNs         = errors.New("amf: no PDN connection could be opened as a PDU session")
	ErrRelocationFromEPSTooLate = fmt.Errorf("%w: amf", interworking.ErrRelocationTooLate)
)

var (
	causeRelocationExpiry    = ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkTNGRelocOverallExpiry}
	causeRelocationCancelled = ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkHandoverCancelled}
)

func NGRANIdentityToNGAP(target interworking.NGRANIdentity) (ngap.GlobalRANNodeID, error) {
	var kind ngap.RANNodeIDKind

	switch target.Kind {
	case interworking.NGRANNodeGNB:
		if target.Bits < 22 || target.Bits > 32 {
			return ngap.GlobalRANNodeID{}, fmt.Errorf("amf: no gNB identity is %d bits wide", target.Bits)
		}

		kind = ngap.RANNodeIDGNB
	case interworking.NGRANNodeNgENB:
		k, ok := map[uint8]ngap.RANNodeIDKind{
			18: ngap.RANNodeIDShortMacroNgENB,
			20: ngap.RANNodeIDMacroNgENB,
			21: ngap.RANNodeIDLongMacroNgENB,
		}[target.Bits]
		if !ok {
			return ngap.GlobalRANNodeID{}, fmt.Errorf("amf: no ng-eNB identity is %d bits wide", target.Bits)
		}

		kind = k
	default:
		return ngap.GlobalRANNodeID{}, fmt.Errorf("amf: unknown NG-RAN node kind %d", target.Kind)
	}

	plmn, err := util.PLMNToNGAP(target.PlmnID)
	if err != nil {
		return ngap.GlobalRANNodeID{}, err
	}

	return ngap.GlobalRANNodeID{
		Kind:         kind,
		PLMNIdentity: plmn,
		Value:        target.ID,
		Bits:         int(target.Bits),
	}, nil
}

func (a *AMF) ForwardRelocation(ctx context.Context, req interworking.FiveGSRelocationRequest) (interworking.FiveGSRelocationResponse, error) {
	var none interworking.FiveGSRelocationResponse

	target, err := NGRANIdentityToNGAP(req.Target)
	if err != nil {
		return none, err
	}

	radio, ok := a.FindConnectedRadioByRanID(util.RANNodeIDToModels(target))
	if !ok {
		return none, fmt.Errorf("%w: %s", ErrUnknownTargetRAN, target.Hex())
	}

	operatorInfo, err := a.OperatorInfo(ctx)
	if err != nil {
		return none, fmt.Errorf("amf: resolve the operator: %w", err)
	}

	if !servesTAI(operatorInfo.Tais, req.Target.SelectedTAI) {
		return none, fmt.Errorf("%w: tracking area %s-%06x is not served", ErrUnknownTargetRAN,
			req.Target.SelectedTAI.PlmnID, req.Target.SelectedTAI.TAC)
	}

	subscriberProfile, err := a.SubscriberProfile(ctx, req.SUPI)
	if err != nil {
		return none, fmt.Errorf("amf: resolve the subscriber profile: %w", err)
	}

	if !subscriberProfile.Allow5G {
		return none, interworking.TargetRefusal{Cause: s1ap.Cause{
			Group: s1ap.CauseGroupRadioNetwork,
			Value: s1ap.CauseRadioNetworkHOTargetNotAllowed,
		}}
	}

	snssaiList := subscriberProfile.AllowedNssai
	if len(snssaiList) == 0 {
		return none, fmt.Errorf("amf: %s is subscribed to no network slice", req.SUPI)
	}

	intOrder, encOrder, err := a.SecurityAlgorithms(ctx)
	if err != nil {
		return none, fmt.Errorf("amf: resolve the NAS security algorithms: %w", err)
	}

	mapped, err := interworking.MapTo5GSOnHandover(req.SecurityContext, intOrder, encOrder)
	if err != nil {
		return none, err
	}

	ue := NewUeContext()
	ue.SetSupi(req.SUPI)
	ue.SetAmbr(&models.Ambr{Uplink: req.UEAMBRUplink, Downlink: req.UEAMBRDownlink})
	ue.AllowedNssai = snssaiList
	ue.SetAllow4G(subscriberProfile.Allow4G)
	ue.AttestS1Mode()
	ue.smf = a.Session

	if err := ue.InstallMappedSecurityContextFromEPS(mapped.Context, MintAuthProofForInterworking()); err != nil {
		return none, err
	}

	ue.TransitionTo(RegistrationInitiated)

	if !a.beginRelocationFromEPS(req.SUPI, req.ID, ue) {
		return none, ErrRelocationFromEPSBusy
	}

	resp, err := a.relocateFromEPS(ctx, ue, radio, req, mapped, operatorInfo, snssaiList)
	if err != nil {
		a.dropRelocationFromEPS(ctx, ue)

		return none, err
	}

	return resp, nil
}

func (a *AMF) relocateFromEPS(
	ctx context.Context,
	ue *UeContext,
	radio *Radio,
	req interworking.FiveGSRelocationRequest,
	mapped interworking.EPSTo5GSHandover,
	operatorInfo *OperatorInfo,
	snssaiList []models.Snssai,
) (interworking.FiveGSRelocationResponse, error) {
	var none interworking.FiveGSRelocationResponse

	sessions, candidates, bearers, err := a.openArrivingSessions(ctx, ue, req.PDNConnections)
	if err != nil {
		return none, err
	}

	nasc, err := mapped.Container.MarshalBinary()
	if err != nil {
		return none, fmt.Errorf("amf: encode the S1 mode to N1 mode NAS transparent container: %w", err)
	}

	targetUe, outcome, ok := a.prepareRelocationFromEPS(ctx, ue, radio, candidates)
	if !ok {
		return none, fmt.Errorf("amf: could not prepare a handover from EPS for %s", ue.Supi())
	}

	logger.From(ctx, logger.AmfLog).Info("Handover Request (EPS to 5GS)",
		logger.SUPI(ue.Supi().String()),
		zap.Uint64("target-amf-ue-id", uint64(targetUe.AmfUeNgapID)),
		zap.Int("pdu-sessions", len(sessions)))

	err = targetUe.SendHandoverRequest(ctx, HandoverRequestOpts{
		HandoverType:         ngap.HandoverTypeEPSToFiveGS,
		UplinkAmbr:           req.UEAMBRUplink,
		DownlinkAmbr:         req.UEAMBRDownlink,
		UESecurityCapability: ue.UESecCap(),
		NCC:                  0,
		NH:                   mapped.Context.TemporaryKgNB[:],
		Cause:                NGAPHandoverCause(req.Cause),
		Sessions:             sessions,
		SourceToTarget:       ngap.SourceToTargetTransparentContainer(req.SourceToTarget),
		SnssaiList:           snssaiList,
		GUAMI:                operatorInfo.Guami,
		NASC:                 nasc,
		NewSecurityContext:   true,
		ServingPLMN:          operatorInfo.Guami.PlmnID,
	})
	if err != nil {
		a.ClearHandover(ue)

		if rerr := a.RemoveUeConn(ctx, targetUe); rerr != nil {
			logger.From(ctx, logger.AmfLog).Error("error removing the target ue after a failed Handover Request", zap.Error(rerr))
		}

		return none, fmt.Errorf("amf: send the inter-system Handover Request: %w", err)
	}

	a.SuperviseHandoverFromEPS(ue, targetUe)

	select {
	case out := <-outcome:
		if out.err != nil {
			return none, out.err
		}

		a.SuperviseHandoverFromEPS(ue, targetUe)

		return interworking.FiveGSRelocationResponse{
			TargetToSource:     out.targetToSource,
			AcceptedEPSBearers: a.dropUnadmittedSessions(ctx, ue, bearers, out.unadmitted),
		}, nil
	case <-ctx.Done():
		a.abandonHandoverFromEPS(context.WithoutCancel(ctx), ue, targetUe, causeRelocationExpiry)

		return none, ctx.Err()
	}
}

func (a *AMF) openArrivingSessions(ctx context.Context, ue *UeContext, conns []interworking.PDNConnection) (ngap.PDUSessionResourceSetupListHOReq, []HandoverCandidate, map[uint8]uint8, error) {
	var (
		sessions   ngap.PDUSessionResourceSetupListHOReq
		candidates []HandoverCandidate
	)

	bearers := make(map[uint8]uint8, len(conns))

	for _, c := range conns {
		snssai := c.Snssai

		ref, n2, err := a.Session.PrepareSmContextFromEPS(ctx, ue.Supi(), c.PDUSessionID, c.EPSBearerIdentity, c.APN, &snssai)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("failed to take over a PDN connection as a PDU session; leaving it behind",
				logger.SUPI(ue.Supi().String()), logger.PDUSessionID(c.PDUSessionID),
				zap.Uint8("ebi", c.EPSBearerIdentity), zap.String("dnn", c.APN), zap.Error(err))

			continue
		}

		item, err := PDUSessionSetupItemHOReq(c.PDUSessionID, &snssai, n2)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Error("could not build the handover request item for an arriving PDN connection",
				logger.PDUSessionID(c.PDUSessionID), zap.Error(err))
			a.releaseArrivingSession(ctx, ref)

			continue
		}

		if err := ue.CreateSmContext(c.PDUSessionID, ref, &snssai, c.APN); err != nil {
			logger.From(ctx, logger.AmfLog).Error("could not route an arriving PDN connection",
				logger.PDUSessionID(c.PDUSessionID), zap.Error(err))
			a.releaseArrivingSession(ctx, ref)

			continue
		}

		ue.SetSmContextPending(c.PDUSessionID)
		ue.SetEPSBearerIdentity(c.PDUSessionID, c.EPSBearerIdentity)

		sessions = append(sessions, item)
		candidates = append(candidates, HandoverCandidate{PDUSessionID: ngap.PDUSessionID(c.PDUSessionID)})
		bearers[c.PDUSessionID] = c.EPSBearerIdentity
	}

	if len(sessions) == 0 {
		return nil, nil, nil, ErrNoAdmissiblePDNs
	}

	return sessions, candidates, bearers, nil
}

func (a *AMF) releaseArrivingSession(ctx context.Context, ref string) {
	if err := a.Session.ReleaseSmContext(ctx, ref); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to unwind a PDU session that could not be handed over",
			zap.String("ref", ref), zap.Error(err))
	}
}

func (a *AMF) dropUnadmittedSessions(ctx context.Context, ue *UeContext, bearers map[uint8]uint8, unadmitted []HandoverCandidate) []uint8 {
	for _, c := range unadmitted {
		pduSessionID := uint8(c.PDUSessionID)

		if sc, ok := ue.SmContextFindByPDUSessionID(pduSessionID); ok {
			a.releaseArrivingSession(ctx, sc.Ref)
			ue.DeleteSmContext(pduSessionID)
		}

		delete(bearers, pduSessionID)
	}

	out := make([]uint8, 0, len(bearers))
	for _, ebi := range bearers {
		out = append(out, ebi)
	}

	slices.Sort(out)

	return out
}

func (a *AMF) SuperviseHandoverFromEPS(ue *UeContext, targetUe *UeConn) {
	ue.SuperviseKeyChainProc(procedure.N2Handover,
		time.Now().Add(a.handoverGuardTimeout),
		func(cctx context.Context) (procedure.Disposition, error) {
			switch a.unwindHandoverFromEPS(cctx, ue, targetUe, causeRelocationExpiry) {
			case handoverCommitting:
				return procedure.Retain, nil
			case handoverNotInProgress:
				return procedure.Release, nil
			}

			logger.From(cctx, logger.AmfLog).Warn("handover from EPS abandoned: the UE did not arrive in time",
				logger.SUPI(ue.Supi().String()))

			return procedure.Release, nil
		})
}

func (a *AMF) abandonHandoverFromEPS(ctx context.Context, ue *UeContext, targetUe *UeConn, cause ngap.Cause) bool {
	return a.unwindHandoverFromEPS(ctx, ue, targetUe, cause) == handoverAbandoned
}

func (a *AMF) unwindHandoverFromEPS(ctx context.Context, ue *UeContext, targetUe *UeConn, cause ngap.Cause) handoverUnwind {
	delivery, unwound := a.claimHandoverUnwind(ue)
	if unwound != handoverAbandoned {
		return unwound
	}

	a.UnbindHandoverTarget(ctx, ue)
	a.dropRelocationFromEPS(ctx, ue)

	settleRelocation(delivery, relocationOutcome{err: ErrRelocationAbandoned})

	if targetUe != nil {
		targetUe.ReleaseAction = UeContextReleaseHandover
		targetUe.SendUEContextReleaseCommand(ctx, cause)
	}

	return handoverAbandoned
}

func (a *AMF) CompleteRelocationFromEPS(ctx context.Context, ue *UeContext) {
	supi := ue.Supi()

	a.mu.Lock()
	held := a.relocatingFromEPS[supi]
	a.mu.Unlock()

	if held == nil || held.ue != ue {
		logger.From(ctx, logger.AmfLog).Warn("completing a handover from EPS this AMF no longer holds",
			logger.SUPI(supi.String()))

		return
	}

	id := held.id

	ue.MarkArrivedFromEPSHandover()
	ue.TransitionTo(Registered)

	if err := a.CommitUEIdentity(ctx, ue, MintAuthProofForInterworking()); err != nil {
		logger.From(ctx, logger.AmfLog).Error("could not index a UE that arrived from EPS",
			logger.SUPI(supi.String()), zap.Error(err))
	}

	a.endRelocationFromEPS(supi, ue)

	logger.From(ctx, logger.AmfLog).Info("handover from EPS complete", logger.SUPI(supi.String()))

	if a.EPS == nil {
		return
	}

	if err := a.EPS.RelocationComplete(ctx, supi, id); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("the EPS peer refused the relocation completion notification",
			logger.SUPI(supi.String()), zap.Error(err))
	}
}

func (a *AMF) RelocationCancel(ctx context.Context, supi etsi.SUPI, id interworking.RelocationID) error {
	a.mu.Lock()

	held, ok := a.relocatingFromEPS[supi]
	if !ok {
		a.mu.Unlock()

		return ErrNoRelocationFromEPS
	}

	if held.id != id {
		a.mu.Unlock()

		return fmt.Errorf("%w: %s is on relocation %d, not %d", ErrNoRelocationFromEPS, supi, held.id, id)
	}

	if !held.prepared {
		held.cancelled = true
		a.mu.Unlock()

		logger.From(ctx, logger.AmfLog).Info("Relocation Cancel", logger.SUPI(supi.String()))

		return nil
	}

	ue := held.ue
	a.mu.Unlock()

	logger.From(ctx, logger.AmfLog).Info("Relocation Cancel", logger.SUPI(supi.String()))

	if !a.abandonHandoverFromEPS(ctx, ue, a.HandoverTarget(ue), causeRelocationCancelled) {
		return fmt.Errorf("%w: %s", ErrRelocationFromEPSTooLate, supi)
	}

	return nil
}

func (ue *UeContext) MarkArrivedFromEPSHandover() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.arrivedFromEPSHandover = true
}

func (ue *UeContext) ArrivedFromEPSHandover() bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.arrivedFromEPSHandover
}

type fromEPSRelocation struct {
	id        interworking.RelocationID
	ue        *UeContext
	prepared  bool
	cancelled bool
}

func (a *AMF) beginRelocationFromEPS(supi etsi.SUPI, id interworking.RelocationID, ue *UeContext) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, busy := a.relocatingFromEPS[supi]; busy {
		return false
	}

	a.relocatingFromEPS[supi] = &fromEPSRelocation{id: id, ue: ue}

	return true
}

func (a *AMF) endRelocationFromEPS(supi etsi.SUPI, ue *UeContext) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.endRelocationFromEPSLocked(supi, ue)
}

// The caller holds a.mu.
func (a *AMF) endRelocationFromEPSLocked(supi etsi.SUPI, ue *UeContext) {
	if held, ok := a.relocatingFromEPS[supi]; ok && held.ue == ue {
		delete(a.relocatingFromEPS, supi)
	}
}

func (a *AMF) dropRelocationFromEPS(ctx context.Context, ue *UeContext) {
	ctx = context.WithoutCancel(ctx)

	ue.releaseSmContexts(ctx)
	a.endRelocationFromEPS(ue.Supi(), ue)
}

func servesTAI(served []models.Tai, tai interworking.FiveGSTAI) bool {
	want := models.Tai{PlmnID: &tai.PlmnID, Tac: fmt.Sprintf("%06x", tai.TAC)}

	return slices.ContainsFunc(served, func(t models.Tai) bool { return t.Equal(want) })
}
