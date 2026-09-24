// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// S1AP causes the MME returns in a PATH SWITCH REQUEST FAILURE (TS 36.413).
var (
	causeUnknownMMEUES1APID    = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkUnknownMMEUES1APID}
	causeMultipleERABInstances = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkMultipleERABIDInstances}
	causePathSwitchNoSecurity  = s1ap.Cause{Group: s1ap.CauseGroupNAS, Value: s1ap.CauseNASAuthenticationFailure}
	causePathSwitchUPFailure   = s1ap.Cause{Group: s1ap.CauseGroupTransport, Value: s1ap.CauseTransportResourceUnavailable}
)

// handlePathSwitchRequest handles an X2-handover PATH SWITCH REQUEST from the
// target eNB, advancing the {NH, NCC} key chain and switching the UE's S1-U
// downlink, or replying with FAILURE (TS 36.413).
func handlePathSwitchRequest(ctx context.Context, m *mme.MME, radio *mme.Radio, value []byte) {
	req, err := s1ap.ParsePathSwitchRequest(value)
	if err != nil {
		rejectWithFailure(ctx, m, radio.Conn, s1ap.ProcPathSwitchRequest, err,
			func(cause s1ap.Cause, diag *s1ap.CriticalityDiagnostics) ([]byte, error) {
				mmeID, enbID := rejectedUEIDs(err)

				return (&s1ap.PathSwitchRequestFailure{
					MMEUES1APID: mmeID, ENBUES1APID: enbID, Cause: &cause, CriticalityDiagnostics: diag,
				}).Marshal()
			}, mme.S1APProcedurePathSwitchRequestFailure)

		return
	}

	reportDiagnostics(ctx, m, radio.Conn, s1ap.ProcPathSwitchRequest, s1ap.TriggeringInitiatingMessage, ueAssociated(req.SourceMMEUES1APID, req.ENBUES1APID), req.Diagnostics())

	// TS 36.413: a to-be-switched list repeating an E-RAB ID is an
	// abnormal condition the MME rejects.
	if id, dup := duplicateERABID(req.ERABToBeSwitchedDL); dup {
		logger.From(ctx, logger.MmeLog).Warn("Path Switch Request with a duplicate E-RAB ID",
			zap.Uint32("source_mme_ue_s1ap_id", uint32(req.SourceMMEUES1APID)), logger.ERABID(uint8(id)))
		sendPathSwitchFailure(ctx, m, radio.Conn, req, causeMultipleERABInstances)

		return
	}

	ue, ok := m.LookupUe(req.SourceMMEUES1APID)
	if !ok {
		logger.From(ctx, logger.MmeLog).Warn("Path Switch Request for unknown UE",
			zap.Uint32("source_mme_ue_s1ap_id", uint32(req.SourceMMEUES1APID)))
		sendPathSwitchFailure(ctx, m, radio.Conn, req, causeUnknownMMEUES1APID)

		return
	}

	ue.TouchLastSeen()

	// Nil in ECM-IDLE, and a concurrent detach can nil it at any point.
	if c := ue.Conn(); c != nil {
		ctx = logger.Into(ctx, c.LogFields()...)
	}

	if !ue.Secured() || !ue.HasKASME() {
		logger.From(ctx, logger.MmeLog).Warn("Path Switch Request for a UE without a security context")
		sendPathSwitchFailure(ctx, m, radio.Conn, req, causePathSwitchNoSecurity)

		return
	}

	// Claim the {NH, NCC} chain, refusing if a Path Switch or S1 handover is
	// concurrently advancing it (deriving the same NH for two targets). Held until
	// commit so a handover cannot start in the unlocked derive/switch window below.
	curNH, curNCC, mmeID, ok := m.BeginPathSwitch(ue)
	if !ok {
		logger.From(ctx, logger.MmeLog).Warn("Path Switch Request while the key chain is being advanced",
			logger.MMEUeS1apID(uint32(mmeID)))
		sendPathSwitchFailure(ctx, m, radio.Conn, req, causePathSwitchUPFailure)

		return
	}

	defer m.ClearKeyChainBusy(ue)

	// Compute the next NH before any user-plane change so a derivation error leaves
	// the UE on the source eNB cleanly; the chain is committed only after at least
	// one E-RAB is switched (TS 33.401 — no rollback once advanced).
	newNH, err := m.AdvancePathSwitchNH(ue, curNH)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to advance NH for Path Switch", zap.Error(err))
		sendPathSwitchFailure(ctx, m, radio.Conn, req, causePathSwitchUPFailure)

		return
	}

	present, undecodable := pathSwitchBearers(ctx, mmeID, req.ERABToBeSwitchedDL)

	result := m.ReconcileBearersToRAN(ctx, ue, mme.RANBearers{
		Present:       present,
		Rejected:      undecodable,
		Authoritative: true,
	})

	released := releasedERABItems(append(result.Failed, undecodable...))

	if len(result.Applied) == 0 {
		logger.From(ctx, logger.MmeLog).Warn("Path Switch Request switched no E-RAB",
			logger.MMEUeS1apID(uint32(mmeID)))

		m.DetachUEAfterPathSwitchFailure(ctx, ue)

		sendPathSwitchFailure(ctx, m, radio.Conn, req, causePathSwitchUPFailure)

		return
	}

	replayCaps := pathSwitchSecurityCapabilities(ctx, ue, req.UESecurityCapabilities)

	ncc, ok := m.CommitPathSwitch(ue, radio.Conn, req.ENBUES1APID, newNH, curNCC)
	if !ok {
		logger.From(ctx, logger.MmeLog).Warn("Path Switch Request: UE released during the user-plane switch",
			logger.MMEUeS1apID(uint32(mmeID)))
		sendPathSwitchFailure(ctx, m, radio.Conn, req, causePathSwitchUPFailure)

		return
	}

	ueConn := ue.Conn()
	if ueConn == nil {
		logger.From(ctx, logger.MmeLog).Warn("Path Switch Request: UE released immediately after the path switch",
			logger.MMEUeS1apID(uint32(mmeID)))

		return
	}

	if req.EUTRANCGI != nil && req.TAI != nil {
		ueConn.UpdateLocation(*req.EUTRANCGI, *req.TAI)
	}

	var ambr *s1ap.UEAggregateMaximumBitRate

	if ul, dl := ue.AmbrRates(); ul.Bps() != 0 || dl.Bps() != 0 {
		ambr = new(handoverUEAMBR(ue))
	}

	ack := &s1ap.PathSwitchRequestAcknowledge{
		SecurityContext:           s1ap.SecurityContext{NextHopChainingCount: ncc, NextHopParameter: s1ap.SecurityKey(newNH)},
		UEAggregateMaximumBitRate: ambr,
		UESecurityCapabilities:    replayCaps,
		ERABToBeReleased:          released,
	}

	logger.From(ctx, logger.MmeLog).Info("Path Switch Request",
		logger.MMEUeS1apID(uint32(mmeID)),
		logger.ENBUeS1apID(uint32(req.ENBUES1APID)),
		zap.Int("e_rabs_switched", len(result.Applied)),
		zap.Int("e_rabs_released", len(result.Released)),
		zap.Uint8("ncc", ncc))

	if err := ueConn.SendPathSwitchAcknowledge(ctx, ack); err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to send Path Switch Request Acknowledge", zap.Error(err))
	}
}

func pathSwitchBearers(ctx context.Context, mmeID s1ap.MMEUES1APID, items []s1ap.ERABToBeSwitchedDLItem) (present []mme.RANBearer, undecodable []uint8) {
	present = make([]mme.RANBearer, 0, len(items))

	for _, erab := range items {
		addr, ok := enbTransportAddress(erab.TransportLayerAddress)
		if !ok {
			logger.From(ctx, logger.MmeLog).Warn("Path Switch Request E-RAB has an invalid eNB transport address; not switched",
				logger.MMEUeS1apID(uint32(mmeID)), logger.ERABID(uint8(erab.ERABID)))

			undecodable = append(undecodable, uint8(erab.ERABID))

			continue
		}

		present = append(present, mme.RANBearer{
			Ebi:      uint8(erab.ERABID),
			EnbFTEID: models.FTEID{TEID: uint32(erab.GTPTEID), Addr: addr},
		})
	}

	return present, undecodable
}

func releasedERABItems(failed []uint8) []s1ap.ERABItem {
	if len(failed) == 0 {
		return nil
	}

	cause := s1ap.Cause{Group: s1ap.CauseGroupTransport, Value: s1ap.CauseTransportResourceUnavailable}

	items := make([]s1ap.ERABItem, 0, len(failed))
	for _, ebi := range failed {
		items = append(items, s1ap.ERABItem{ERABID: s1ap.ERABID(ebi), Cause: cause})
	}

	return items
}

// sendPathSwitchFailure sends a PATH SWITCH REQUEST FAILURE on the association the
// request arrived on (TS 36.413). The UE keeps its source-eNB context.
func sendPathSwitchFailure(ctx context.Context, m *mme.MME, conn mme.S1APWriter, req *s1ap.PathSwitchRequest, cause s1ap.Cause) {
	fail := &s1ap.PathSwitchRequestFailure{
		MMEUES1APID: s1ap.Ptr(req.SourceMMEUES1APID),
		ENBUES1APID: s1ap.Ptr(req.ENBUES1APID),
		Cause:       s1ap.Ptr(cause),
	}

	b, err := fail.Marshal()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to marshal Path Switch Request Failure", zap.Error(err))
		return
	}

	_ = m.SendToRadio(ctx, conn, mme.S1APProcedurePathSwitchRequestFailure, b)
}

// pathSwitchSecurityCapabilities compares the UE security capabilities the target
// eNB reported against the MME's stored values, returning the stored values to
// replay in the Acknowledge on a mismatch so the eNB corrects its context, or nil
// (IE omitted) otherwise (TS 36.413, TS 33.401). The stored values are never
// overwritten with the received ones.
func pathSwitchSecurityCapabilities(ctx context.Context, ue *mme.UeContext, received *s1ap.UESecurityCapabilities) *s1ap.UESecurityCapabilities {
	uecap := ue.UeNetCap()

	stored := mme.S1apSecurityCapabilities(uecap)

	if received == nil || *received == stored {
		return nil
	}

	logger.From(ctx, logger.MmeLog).Warn("UE security capabilities reported by target eNB differ from stored; replaying stored values",
		zap.Uint16("received_eea", received.EncryptionAlgorithms),
		zap.Uint16("received_eia", received.IntegrityProtectionAlgorithms),
		zap.Uint16("stored_eea", stored.EncryptionAlgorithms),
		zap.Uint16("stored_eia", stored.IntegrityProtectionAlgorithms))

	return &stored
}

// duplicateERABID reports the first E-RAB ID that appears more than once in the
// to-be-switched list (TS 36.413).
func duplicateERABID(items []s1ap.ERABToBeSwitchedDLItem) (s1ap.ERABID, bool) {
	seen := make(map[s1ap.ERABID]struct{}, len(items))

	for _, it := range items {
		if _, ok := seen[it.ERABID]; ok {
			return it.ERABID, true
		}

		seen[it.ERABID] = struct{}{}
	}

	return 0, false
}
