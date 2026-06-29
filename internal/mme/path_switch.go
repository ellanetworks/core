// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
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
// target eNB (TS 36.413): it switches the S1-U downlink to the new eNB,
// advances the {NH, NCC} key chain, moves the UE's S1 association, and replies
// with PATH SWITCH REQUEST ACKNOWLEDGE — or a FAILURE when the path cannot be
// switched. value is the initiatingMessage open-type payload; conn is the target
// eNB's association the request arrived on.
func (m *MME) handlePathSwitchRequest(ctx context.Context, conn NasWriter, value []byte) {
	req, err := s1ap.ParsePathSwitchRequest(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Path Switch Request", zap.Error(err))
		return
	}

	// TS 36.413: a to-be-switched list repeating an E-RAB ID is an
	// abnormal condition the MME rejects.
	if id, dup := duplicateERABID(req.ERABToBeSwitchedDL); dup {
		logger.MmeLog.Warn("Path Switch Request with a duplicate E-RAB ID",
			zap.Uint32("source-mme-ue-id", uint32(req.SourceMMEUES1APID)), zap.Uint8("e-rab-id", uint8(id)))
		m.sendPathSwitchFailure(conn, req, causeMultipleERABInstances)

		return
	}

	ue, ok := m.LookupUe(req.SourceMMEUES1APID)
	if !ok {
		logger.MmeLog.Warn("Path Switch Request for unknown UE",
			zap.Uint32("source-mme-ue-id", uint32(req.SourceMMEUES1APID)))
		m.sendPathSwitchFailure(conn, req, causeUnknownMMEUES1APID)

		return
	}

	if !ue.Secured() || !ue.HasKASME() {
		logger.MmeLog.Warn("Path Switch Request for a UE without a security context",
			zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)))
		m.sendPathSwitchFailure(conn, req, causePathSwitchNoSecurity)

		return
	}

	// Claim the {NH, NCC} chain — refusing if a Path Switch or S1 handover is
	// concurrently advancing it, which would derive the same NH for two targets. The
	// claim is held until commit so a handover cannot start in the unlocked
	// derive/switch window below.
	curNH, curNCC, mmeID, ok := m.beginPathSwitch(ue)
	if !ok {
		logger.MmeLog.Warn("Path Switch Request while the key chain is being advanced",
			zap.Uint32("mme-ue-id", uint32(mmeID)))
		m.sendPathSwitchFailure(conn, req, causePathSwitchUPFailure)

		return
	}

	defer m.clearKeyChainBusy(ue)

	// Compute the next NH before any user-plane change so a derivation error leaves
	// the UE on the source eNB cleanly; the chain is committed only after at least
	// one E-RAB is switched (TS 33.401 — no rollback once advanced).
	newNH, err := m.advancePathSwitchNH(ue, curNH)
	if err != nil {
		logger.MmeLog.Error("failed to advance NH for Path Switch", zap.Error(err))
		m.sendPathSwitchFailure(conn, req, causePathSwitchUPFailure)

		return
	}

	// Switch the downlink of every E-RAB in the list to the endpoint it carries.
	switched := m.switchPathBearers(ctx, ue, mmeID, req.ERABToBeSwitchedDL)
	if switched == 0 {
		// TS 36.413: the UP path was switched for no E-RAB.
		logger.MmeLog.Warn("Path Switch Request switched no E-RAB",
			zap.Uint32("mme-ue-id", uint32(mmeID)))
		m.sendPathSwitchFailure(conn, req, causePathSwitchUPFailure)

		return
	}

	replayCaps := m.pathSwitchSecurityCapabilities(ue, req.UESecurityCapabilities)

	// UP switch succeeded: move the S1 association to the target eNB and commit the
	// advanced {NH, NCC}. NCC is a 3-bit chaining counter (TS 33.401). The UE may
	// have been released during the unlocked switch above (its source association
	// dropping), so the commit is gated on the connection still being present.
	ncc, ok := m.commitPathSwitch(ue, conn, req.ENBUES1APID, newNH, curNCC)
	if !ok {
		logger.MmeLog.Warn("Path Switch Request: UE released during the user-plane switch",
			zap.Uint32("mme-ue-id", uint32(mmeID)))
		m.sendPathSwitchFailure(conn, req, causePathSwitchUPFailure)

		return
	}

	ack := &s1ap.PathSwitchRequestAcknowledge{
		MMEUES1APID:            mmeID,
		ENBUES1APID:            req.ENBUES1APID,
		SecurityContext:        s1ap.SecurityContext{NextHopChainingCount: ncc, NextHopParameter: s1ap.SecurityKey(newNH)},
		UESecurityCapabilities: replayCaps,
	}

	b, err := ack.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal Path Switch Request Acknowledge", zap.Error(err))
		return
	}

	logger.MmeLog.Info("Path Switch Request",
		zap.Uint32("mme-ue-id", uint32(mmeID)),
		zap.Uint32("enb-ue-id", uint32(req.ENBUES1APID)),
		zap.Int("e-rabs-switched", switched),
		zap.Uint8("ncc", ncc))
	m.SendS1AP(ctx, ue, S1APProcedurePathSwitchRequestAck, b)
}

// switchPathBearers points the downlink of each E-RAB in the to-be-switched list
// at the target eNB endpoint it carries, and returns how many were switched. Each
// E-RAB is matched to its PDN connection by EPS bearer identity and switched
// independently with one ModifyEPSSession call. An E-RAB the MME cannot resolve
// to a PDN connection, or whose endpoint is malformed or fails to switch, is
// logged and skipped — not silently dropped — and counts as not switched
// (TS 36.413).
func (m *MME) switchPathBearers(ctx context.Context, ue *UeContext, mmeID s1ap.MMEUES1APID, items []s1ap.ERABToBeSwitchedDLItem) int {
	switched := 0

	for _, erab := range items {
		p := m.LookupPDN(ue, uint8(erab.ERABID))
		if p == nil {
			logger.MmeLog.Warn("Path Switch Request lists an unknown E-RAB; not switched",
				zap.Uint32("mme-ue-id", uint32(mmeID)), zap.Uint8("e-rab-id", uint8(erab.ERABID)))

			continue
		}

		addr, ok := enbTransportAddress(erab.TransportLayerAddress)
		if !ok {
			logger.MmeLog.Warn("Path Switch Request E-RAB has an invalid eNB transport address; not switched",
				zap.Uint32("mme-ue-id", uint32(mmeID)), zap.Uint8("e-rab-id", uint8(erab.ERABID)))

			continue
		}

		fteid := models.FTEID{TEID: uint32(erab.GTPTEID), Addr: addr}

		if err := m.Session.ModifyEPSSession(ctx, ue.IMSI(), p.Ebi, fteid); err != nil {
			logger.MmeLog.Error("failed to switch an EPS session downlink to the target eNB",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", uint8(erab.ERABID)), zap.Error(err))

			continue
		}

		p.EnbFTEID = fteid
		switched++

		logger.MmeLog.Debug("Path Switch: E-RAB downlink switched",
			zap.Uint32("mme-ue-id", uint32(mmeID)),
			zap.Uint8("e-rab-id", uint8(erab.ERABID)),
			zap.String("enb-s1u", addr.String()))
	}

	return switched
}

// sendPathSwitchFailure sends a PATH SWITCH REQUEST FAILURE on the association the
// request arrived on (TS 36.413). The UE keeps its source-eNB context.
func (m *MME) sendPathSwitchFailure(conn NasWriter, req *s1ap.PathSwitchRequest, cause s1ap.Cause) {
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

	if _, err := conn.WriteMsg(b, &sctp.SndRcvInfo{PPID: s1apWirePPID, Stream: s1apStreamUE}); err != nil {
		logger.MmeLog.Error("failed to send Path Switch Request Failure", zap.Error(err))
		return
	}

	// A Path Switch Failure can be sent before the UE is resolved; use a fresh root.
	m.LogOutboundS1AP(context.Background(), conn, S1APProcedurePathSwitchRequestFailure, b)
}

// pathSwitchSecurityCapabilities compares the UE security capabilities the target
// eNB reported against the MME's stored values (TS 36.413, TS 33.401).
// On a mismatch it returns the stored values to replay in the
// Acknowledge so the eNB corrects its context; on a match (or when the stored
// capabilities cannot be parsed) it returns nil and the IE is omitted. The stored
// values are never overwritten with the received ones.
func (m *MME) pathSwitchSecurityCapabilities(ue *UeContext, received s1ap.UESecurityCapabilities) *s1ap.UESecurityCapabilities {
	uecap, err := eps.ParseUENetworkCapability(ue.UeNetCap)
	if err != nil {
		return nil
	}

	stored := S1apSecurityCapabilities(uecap)

	if received == stored {
		return nil
	}

	logger.MmeLog.Warn("UE security capabilities reported by target eNB differ from stored; replaying stored values",
		zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)),
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
