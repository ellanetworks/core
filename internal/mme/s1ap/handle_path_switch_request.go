// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/nas/eps"
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
func handlePathSwitchRequest(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	req, err := s1ap.ParsePathSwitchRequest(value)
	if err != nil {
		handleParseError(m, radio.Conn, s1ap.ProcPathSwitchRequest, err)
		return
	}

	// TS 36.413: a to-be-switched list repeating an E-RAB ID is an
	// abnormal condition the MME rejects.
	if id, dup := duplicateERABID(req.ERABToBeSwitchedDL); dup {
		logger.From(ctx, logger.MmeLog).Warn("Path Switch Request with a duplicate E-RAB ID",
			zap.Uint32("source-mme-ue-id", uint32(req.SourceMMEUES1APID)), zap.Uint8("e-rab-id", uint8(id)))
		sendPathSwitchFailure(m, radio.Conn, req, causeMultipleERABInstances)

		return
	}

	ue, ok := m.LookupUe(req.SourceMMEUES1APID)
	if !ok {
		logger.From(ctx, logger.MmeLog).Warn("Path Switch Request for unknown UE",
			zap.Uint32("source-mme-ue-id", uint32(req.SourceMMEUES1APID)))
		sendPathSwitchFailure(m, radio.Conn, req, causeUnknownMMEUES1APID)

		return
	}

	ue.TouchLastSeen()

	if !ue.Secured() || !ue.HasKASME() {
		logger.From(ctx, ue.Conn().Log).Warn("Path Switch Request for a UE without a security context")
		sendPathSwitchFailure(m, radio.Conn, req, causePathSwitchNoSecurity)

		return
	}

	// Claim the {NH, NCC} chain, refusing if a Path Switch or S1 handover is
	// concurrently advancing it (deriving the same NH for two targets). Held until
	// commit so a handover cannot start in the unlocked derive/switch window below.
	curNH, curNCC, mmeID, ok := m.BeginPathSwitch(ue)
	if !ok {
		logger.From(ctx, logger.MmeLog).Warn("Path Switch Request while the key chain is being advanced",
			zap.Uint32("mme-ue-id", uint32(mmeID)))
		sendPathSwitchFailure(m, radio.Conn, req, causePathSwitchUPFailure)

		return
	}

	defer m.ClearKeyChainBusy(ue)

	// Compute the next NH before any user-plane change so a derivation error leaves
	// the UE on the source eNB cleanly; the chain is committed only after at least
	// one E-RAB is switched (TS 33.401 — no rollback once advanced).
	newNH, err := m.AdvancePathSwitchNH(ue, curNH)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to advance NH for Path Switch", zap.Error(err))
		sendPathSwitchFailure(m, radio.Conn, req, causePathSwitchUPFailure)

		return
	}

	switched, released := switchPathBearers(m, ctx, ue, mmeID, req.ERABToBeSwitchedDL)
	if switched == 0 {
		logger.From(ctx, logger.MmeLog).Warn("Path Switch Request switched no E-RAB",
			zap.Uint32("mme-ue-id", uint32(mmeID)))
		sendPathSwitchFailure(m, radio.Conn, req, causePathSwitchUPFailure)

		return
	}

	replayCaps := pathSwitchSecurityCapabilities(ue, req.UESecurityCapabilities)

	// The UE may have been released during the unlocked switch above, so the commit is
	// gated on the connection still being present. NCC is a 3-bit chaining counter
	// (TS 33.401).
	ncc, ok := m.CommitPathSwitch(ue, radio.Conn, req.ENBUES1APID, newNH, curNCC)
	if !ok {
		logger.From(ctx, logger.MmeLog).Warn("Path Switch Request: UE released during the user-plane switch",
			zap.Uint32("mme-ue-id", uint32(mmeID)))
		sendPathSwitchFailure(m, radio.Conn, req, causePathSwitchUPFailure)

		return
	}

	ack := &s1ap.PathSwitchRequestAcknowledge{
		SecurityContext:        s1ap.SecurityContext{NextHopChainingCount: ncc, NextHopParameter: s1ap.SecurityKey(newNH)},
		UESecurityCapabilities: replayCaps,
		ERABToBeReleased:       released,
	}

	logger.From(ctx, logger.MmeLog).Info("Path Switch Request",
		zap.Uint32("mme-ue-id", uint32(mmeID)),
		zap.Uint32("enb-ue-id", uint32(req.ENBUES1APID)),
		zap.Int("e-rabs-switched", switched),
		zap.Uint8("ncc", ncc))

	if err := ue.Conn().SendPathSwitchAcknowledge(ctx, ack); err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to send Path Switch Request Acknowledge", zap.Error(err))
	}
}

// switchPathBearers switches the downlink of each listed E-RAB to the target eNB.
// It returns the number switched and the E-RABs whose UP path it could not switch —
// these must be reported in the PATH SWITCH REQUEST ACKNOWLEDGE E-RAB To Be Released
// List so the eNB releases their data radio bearers (TS 36.413 §8.4.4.2); otherwise
// the eNB keeps a radio bearer for an E-RAB with no downlink path (black-holed DL).
func switchPathBearers(m *mme.MME, ctx context.Context, ue *mme.UeContext, mmeID s1ap.MMEUES1APID, items []s1ap.ERABToBeSwitchedDLItem) (int, []s1ap.ERABItem) {
	switched := 0

	relCause := s1ap.Cause{Group: s1ap.CauseGroupTransport, Value: s1ap.CauseTransportResourceUnavailable}

	var released []s1ap.ERABItem

	for _, erab := range items {
		p := m.LookupPDN(ue, uint8(erab.ERABID))
		if p == nil {
			logger.From(ctx, logger.MmeLog).Warn("Path Switch Request lists an unknown E-RAB; not switched",
				zap.Uint32("mme-ue-id", uint32(mmeID)), zap.Uint8("e-rab-id", uint8(erab.ERABID)))

			released = append(released, s1ap.ERABItem{ERABID: erab.ERABID, Cause: relCause})

			continue
		}

		addr, ok := enbTransportAddress(erab.TransportLayerAddress)
		if !ok {
			logger.From(ctx, logger.MmeLog).Warn("Path Switch Request E-RAB has an invalid eNB transport address; not switched",
				zap.Uint32("mme-ue-id", uint32(mmeID)), zap.Uint8("e-rab-id", uint8(erab.ERABID)))

			released = append(released, s1ap.ERABItem{ERABID: erab.ERABID, Cause: relCause})

			continue
		}

		fteid := models.FTEID{TEID: uint32(erab.GTPTEID), Addr: addr}

		if err := m.Session.ModifyEPSSession(ctx, ue.IMSI(), p.Ebi, fteid); err != nil {
			logger.From(ctx, logger.MmeLog).Error("failed to switch an EPS session downlink to the target eNB",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", uint8(erab.ERABID)), zap.Error(err))

			released = append(released, s1ap.ERABItem{ERABID: erab.ERABID, Cause: relCause})

			continue
		}

		p.EnbFTEID = fteid
		switched++

		logger.From(ctx, logger.MmeLog).Debug("Path Switch: E-RAB downlink switched",
			zap.Uint32("mme-ue-id", uint32(mmeID)),
			zap.Uint8("e-rab-id", uint8(erab.ERABID)),
			zap.String("enb-s1u", addr.String()))
	}

	return switched, released
}

// sendPathSwitchFailure sends a PATH SWITCH REQUEST FAILURE on the association the
// request arrived on (TS 36.413). The UE keeps its source-eNB context.
func sendPathSwitchFailure(m *mme.MME, conn mme.S1APWriter, req *s1ap.PathSwitchRequest, cause s1ap.Cause) {
	fail := &s1ap.PathSwitchRequestFailure{
		MMEUES1APID: req.SourceMMEUES1APID,
		ENBUES1APID: req.ENBUES1APID,
		Cause:       cause,
	}

	b, err := fail.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal Path Switch Request Failure", zap.Error(err))
		return
	}

	if _, err := conn.WriteMsg(b, &sctp.SndRcvInfo{PPID: mme.S1apWirePPID, Stream: mme.S1apStreamUE}); err != nil {
		logger.MmeLog.Error("failed to send Path Switch Request Failure", zap.Error(err))
		return
	}

	// A Path Switch Failure can be sent before the UE is resolved; use a fresh root.
	m.LogOutboundS1AP(context.Background(), conn, mme.S1APProcedurePathSwitchRequestFailure, b)
}

// pathSwitchSecurityCapabilities compares the UE security capabilities the target
// eNB reported against the MME's stored values, returning the stored values to
// replay in the Acknowledge on a mismatch so the eNB corrects its context, or nil
// (IE omitted) otherwise (TS 36.413, TS 33.401). The stored values are never
// overwritten with the received ones.
func pathSwitchSecurityCapabilities(ue *mme.UeContext, received s1ap.UESecurityCapabilities) *s1ap.UESecurityCapabilities {
	uecap, err := eps.ParseUENetworkCapability(ue.UeNetCap)
	if err != nil {
		return nil
	}

	stored := mme.S1apSecurityCapabilities(uecap)

	if received == stored {
		return nil
	}

	ue.Conn().Log.Warn("UE security capabilities reported by target eNB differ from stored; replaying stored values",
		zap.Uint16("received-eea", received.EncryptionAlgorithms),
		zap.Uint16("received-eia", received.IntegrityProtectionAlgorithms),
		zap.Uint16("stored-eea", stored.EncryptionAlgorithms),
		zap.Uint16("stored-eia", stored.IntegrityProtectionAlgorithms))

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
