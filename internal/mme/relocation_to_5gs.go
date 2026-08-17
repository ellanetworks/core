// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme/procedure"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

var (
	ErrNoFiveGSPeer            = errors.New("mme: N26 interworking is not wired")
	ErrRelocationToFiveGSBusy  = errors.New("mme: another procedure holds the UE's key chain")
	ErrNoTransferablePDNs      = errors.New("mme: UE has no PDN connection that can transfer to 5GS")
	ErrNoUEIdentityForRelocate = errors.New("mme: the UE has no IMSI to hand over under")
	ErrFiveGSMobilityBarred    = errors.New("mme: the subscriber may not be moved to 5GS")
)

func (m *MME) PrepareHandoverToFiveGS(ue *UeContext, source *UeConn, target interworking.NGRANIdentity, sourceToTarget []byte, cause *s1ap.Cause) (*interworking.FiveGSRelocationRequest, error) {
	if m.FiveGS == nil {
		return nil, ErrNoFiveGSPeer
	}

	if ue == nil {
		return nil, fmt.Errorf("mme: handover to 5GS without a UE context")
	}

	supi := ue.Supi()
	if supi.IMSI() == "" {
		return nil, ErrNoUEIdentityForRelocate
	}

	if !ue.FiveGSInterworkingAllowed() {
		return nil, ErrFiveGSMobilityBarred
	}

	connections, candidates := TransferablePDNConnections(ue)
	if len(connections) == 0 {
		return nil, ErrNoTransferablePDNs
	}

	id, ok := m.stageRelocationToFiveGS(ue, source, candidates)
	if !ok {
		return nil, ErrRelocationToFiveGSBusy
	}

	if err := ue.advanceNextHop(); err != nil {
		m.ClearRelocationToFiveGS(ue, id)

		return nil, fmt.Errorf("mme: advance the next hop for a handover to 5GS: %w", err)
	}

	req, err := m.buildFiveGSRelocationRequest(ue, connections, target, sourceToTarget, cause)
	if err != nil {
		m.ClearRelocationToFiveGS(ue, id)

		return nil, err
	}

	req.ID = id

	return req, nil
}

func (m *MME) buildFiveGSRelocationRequest(ue *UeContext, connections []interworking.PDNConnection, target interworking.NGRANIdentity, sourceToTarget []byte, cause *s1ap.Cause) (*interworking.FiveGSRelocationRequest, error) {
	security, err := ue.EPSSecurityContextForRelocation()
	if err != nil {
		return nil, err
	}

	ambrUL, ambrDL := ue.AmbrRates()
	if ambrUL.Bps() == 0 || ambrDL.Bps() == 0 {
		return nil, fmt.Errorf("mme: UE has no AMBR")
	}

	return &interworking.FiveGSRelocationRequest{
		SUPI:            ue.Supi(),
		SecurityContext: security,
		PDNConnections:  connections,
		Target:          target,
		SourceToTarget:  sourceToTarget,
		Cause:           handoverRequiredCause(cause),
		UEAMBRUplink:    ambrUL,
		UEAMBRDownlink:  ambrDL,
	}, nil
}

func handoverRequiredCause(cause *s1ap.Cause) s1ap.Cause {
	if cause != nil {
		return *cause
	}

	return causeHandoverUnspecified
}

func TransferablePDNConnections(ue *UeContext) ([]interworking.PDNConnection, []HandoverCandidate) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	connections := make([]interworking.PDNConnection, 0, len(ue.Pdns))
	candidates := make([]HandoverCandidate, 0, len(ue.Pdns))

	for _, p := range ue.Pdns {
		if p.PDUSessionID == 0 || p.Snssai == nil {
			logger.MmeLog.Warn("PDN connection cannot move to 5GS; leaving it behind",
				zap.String("imsi", ue.imsiOrEmpty()), zap.Uint8("ebi", p.Ebi), zap.String("apn", p.Apn))

			candidates = append(candidates, HandoverCandidate{Ebi: p.Ebi, Cause: &causeHandoverInterSystemTriggered})

			continue
		}

		connections = append(connections, interworking.PDNConnection{
			PDUSessionID:      p.PDUSessionID,
			EPSBearerIdentity: p.Ebi,
			APN:               p.Apn,
			Snssai:            *p.Snssai,
		})
		candidates = append(candidates, HandoverCandidate{Ebi: p.Ebi})
	}

	return connections, candidates
}

func (m *MME) RequestRelocationToFiveGS(ctx context.Context, req interworking.FiveGSRelocationRequest) (interworking.FiveGSRelocationResponse, error) {
	if m.FiveGS == nil {
		return interworking.FiveGSRelocationResponse{}, ErrNoFiveGSPeer
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), m.handoverGuardTimeout)
	defer cancel()

	return m.FiveGS.ForwardRelocation(ctx, req)
}

func (m *MME) AbandonHandoverToFiveGS(ctx context.Context, ue *UeContext, id interworking.RelocationID) bool {
	if _, cleared := m.ClearRelocationToFiveGS(ue, id); !cleared {
		logger.From(ctx, logger.MmeLog).Info("handover to 5GS already unwound by whoever cancelled it",
			zap.Uint64("relocation", uint64(id)))

		return false
	}

	if err := m.CancelRelocationToFiveGS(ctx, ue, id); err != nil {
		logger.From(ctx, logger.MmeLog).Info("the 5GS peer had no handover to cancel", zap.Error(err))
	}

	// SessionDropped leaves the release to the relocation owner, so a UE whose
	// PDN connections already moved has nobody left to release it.
	if ue.PDNCount() == 0 {
		m.DeregisterEmptyUE(ctx, ue)
	}

	return true
}

func (m *MME) CancelRelocationToFiveGS(ctx context.Context, ue *UeContext, id interworking.RelocationID) error {
	if m.FiveGS == nil || ue == nil {
		return nil
	}

	return m.FiveGS.RelocationCancel(ctx, ue.Supi(), id)
}

func (m *MME) SuperviseHandoverToFiveGS(ue *UeContext, id interworking.RelocationID) {
	if ue == nil {
		return
	}

	ue.SuperviseKeyChainProc(procedure.S1Handover,
		time.Now().Add(m.handoverGuardTimeout),
		func(cctx context.Context) (procedure.Disposition, error) {
			if !m.AbandonHandoverToFiveGS(cctx, ue, id) {
				return procedure.Release, nil
			}

			logger.From(cctx, logger.MmeLog).Warn("handover to 5GS abandoned: the UE did not arrive in time",
				logger.SUPI(ue.Supi().String()))

			return procedure.Release, nil
		})
}

func (m *MME) RelocationComplete(ctx context.Context, supi etsi.SUPI, id interworking.RelocationID) error {
	ue, ok := m.LookupUeBySupi(supi)
	if !ok {
		return fmt.Errorf("%w: %s", ErrNoRelocationToFiveGS, supi)
	}

	if _, cleared := m.ClearRelocationToFiveGS(ue, id); !cleared {
		logger.From(ctx, logger.MmeLog).Info("the UE reached 5GS on a handover this MME no longer tracks; releasing anyway",
			logger.SUPI(supi.String()), zap.Uint64("relocation", uint64(id)))
	}

	logger.From(ctx, logger.MmeLog).Info("UE handed over to 5GS; releasing its EPS resources",
		logger.SUPI(supi.String()))

	m.ReleaseAllSessions(ctx, ue)

	// The context outlives the command: the eNB answers it with a UE CONTEXT
	// RELEASE COMPLETE, which is the last message of the procedure and must find
	// its connection (TS 36.413 §8.3.3.2, §10.6). Deregistering first is what
	// makes the Complete delete the context rather than drop it to ECM-IDLE.
	ue.TransitionTo(EMMDeregistered)
	m.ReleaseUEContext(ctx, ue, CauseHandoverSuccess)

	return nil
}
