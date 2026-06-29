// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// S1AP causes the MME returns during S1 handover (TS 36.413 §9.2.1.3,
// CauseRadioNetwork enumeration).
var (
	causeHOFailureInTarget      = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 6}  // ho-failure-in-target-EPC-eNB-or-target-system
	causeHOTargetNotAllowed     = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 7}  // ho-target-not-allowed
	causeUnknownTargetID        = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 11} // unknown-targetID
	causeHandoverNoSecurity     = s1ap.Cause{Group: s1ap.CauseGroupNAS, Value: 1}           // authentication-failure
	causeHandoverPrepUnspecific = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0}  // unspecified
)

// handleHandoverRequired starts an S1 handover preparation toward the target eNB,
// or replies HANDOVER PREPARATION FAILURE (TS 36.413 §8.4.1). conn is the source.
func handleHandoverRequired(m *mme.MME, ctx context.Context, conn mme.NasWriter, value []byte) {
	req, err := s1ap.ParseHandoverRequired(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Handover Required", zap.Error(err))
		return
	}

	ue, ok := resolveUE(m, conn, req.MMEUES1APID, req.ENBUES1APID)
	if !ok {
		return
	}

	if req.HandoverType != s1ap.HandoverTypeIntraLTE {
		logger.MmeLog.Warn("Handover Required for an unsupported handover type",
			zap.Uint32("mme-ue-id", uint32(req.MMEUES1APID)), zap.Uint8("handover-type", uint8(req.HandoverType)))
		mme.SendHandoverPreparationFailure(m, ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeHOTargetNotAllowed)

		return
	}

	if !ue.Secured() || !ue.HasKASME() {
		logger.MmeLog.Warn("Handover Required for a UE without a security context",
			zap.Uint32("mme-ue-id", uint32(req.MMEUES1APID)))
		mme.SendHandoverPreparationFailure(m, ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeHandoverNoSecurity)

		return
	}

	target, ok := m.FindENBByGlobalID(req.TargetID.TargeteNBID.GlobalENBID)
	if !ok {
		logger.MmeLog.Warn("Handover Required for an unknown target eNB",
			zap.Uint32("mme-ue-id", uint32(req.MMEUES1APID)), zap.String("target-enb", mme.ENBID(req.TargetID.TargeteNBID.GlobalENBID)))
		mme.SendHandoverPreparationFailure(m, ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeUnknownTargetID)

		return
	}

	if target == conn {
		logger.MmeLog.Warn("Handover Required targets the source eNB",
			zap.Uint32("mme-ue-id", uint32(req.MMEUES1APID)))
		mme.SendHandoverPreparationFailure(m, ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeHOTargetNotAllowed)

		return
	}

	bearers, ok := mme.HandoverBearers(ue)
	if !ok {
		mme.SendHandoverPreparationFailure(m, ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeHandoverPrepUnspecific)
		return
	}

	targetMMEID, newNH, newNCC, ok := m.PrepareHandover(ue, target, req.MMEUES1APID)
	if !ok {
		mme.SendHandoverPreparationFailure(m, ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeHandoverPrepUnspecific)
		return
	}

	hoReq := &s1ap.HandoverRequest{
		MMEUES1APID:            targetMMEID,
		HandoverType:           s1ap.HandoverTypeIntraLTE,
		Cause:                  req.Cause,
		UEAMBR:                 handoverUEAMBR(ue),
		ERABToBeSetup:          bearers,
		SourceToTarget:         req.SourceToTarget,
		UESecurityCapabilities: handoverSecurityCapabilities(ue),
		SecurityContext:        s1ap.SecurityContext{NextHopChainingCount: newNCC, NextHopParameter: s1ap.SecurityKey(newNH)},
	}

	b, err := hoReq.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal Handover Request", zap.Error(err))
		m.ClearHandover(ue)
		mme.SendHandoverPreparationFailure(m, ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeHandoverPrepUnspecific)

		return
	}

	logger.MmeLog.Info("Handover Request",
		zap.Uint32("target-mme-ue-id", uint32(targetMMEID)),
		zap.String("target-enb", mme.ENBID(req.TargetID.TargeteNBID.GlobalENBID)),
		zap.Int("e-rabs", len(bearers)))
	m.SendS1APConn(ctx, target, mme.S1APProcedureHandoverRequest, b)
}

// handleHandoverRequestAcknowledge records the target's downlink endpoints and
// sends a HANDOVER COMMAND to the source, or fails the handover when no usable
// bearer was admitted (TS 36.413 §8.4.2). conn is the target; the acknowledge
// carries the target's MME-UE-S1AP-ID.
func handleHandoverRequestAcknowledge(m *mme.MME, ctx context.Context, conn mme.NasWriter, value []byte) {
	ack, err := s1ap.ParseHandoverRequestAcknowledge(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Handover Request Acknowledge", zap.Error(err))
		return
	}

	ue, ok := m.LookupUe(ack.MMEUES1APID)
	if !ok {
		// No UE for this target id; release the context the ack just created.
		mme.SendUEContextRelease(m, ctx, conn, ack.MMEUES1APID, ack.ENBUES1APID)
		return
	}

	if !m.MatchAndSetTargetENB(ue, ack.MMEUES1APID, ack.ENBUES1APID, conn) {
		logger.MmeLog.Warn("Handover Request Acknowledge with no matching preparation",
			zap.Uint32("target-mme-ue-id", uint32(ack.MMEUES1APID)))
		mme.SendUEContextRelease(m, ctx, conn, ack.MMEUES1APID, ack.ENBUES1APID)

		return
	}

	admitted := make([]mme.AdmittedERAB, 0, len(ack.ERABAdmitted))

	for _, it := range ack.ERABAdmitted {
		addr, ok := enbTransportAddress(it.TransportLayerAddress)
		if !ok {
			logger.MmeLog.Warn("Handover Request Acknowledge E-RAB has an invalid target address; treating as failed",
				zap.Uint32("target-mme-ue-id", uint32(ack.MMEUES1APID)), zap.Uint8("e-rab-id", uint8(it.ERABID)))

			continue
		}

		admitted = append(admitted, mme.AdmittedERAB{Ebi: uint8(it.ERABID), EnbFTEID: models.FTEID{TEID: uint32(it.GTPTEID), Addr: addr}})
	}

	// Each E-RAB is a PDN's default bearer, so a rejected one releases its PDN
	// (TS 23.401 §5.5.1.2.2 step 15).
	releaseEBIs := failedHandoverEBIs(ack, admitted)

	if len(admitted) == 0 {
		// No default bearer admitted: the handover is rejected (TS 23.401 §5.5.1.2.3).
		logger.MmeLog.Warn("Handover Request Acknowledge admitted no E-RAB; rejecting handover",
			zap.Uint32("target-mme-ue-id", uint32(ack.MMEUES1APID)))
		mme.SendUEContextRelease(m, ctx, conn, ack.MMEUES1APID, ack.ENBUES1APID)
		m.FailHandoverToSource(ctx, ue, causeHOFailureInTarget)

		return
	}

	sourceConn, sourceMMEID, sourceENBID, ok := m.CommitHandoverPrepared(ue, ack.MMEUES1APID, conn, admitted, releaseEBIs)
	if !ok {
		return
	}

	cmd := &s1ap.HandoverCommand{
		MMEUES1APID:    sourceMMEID,
		ENBUES1APID:    sourceENBID,
		HandoverType:   s1ap.HandoverTypeIntraLTE,
		ERABToRelease:  releaseItems(releaseEBIs),
		TargetToSource: ack.TargetToSource,
	}

	b, err := cmd.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal Handover Command", zap.Error(err))
		return
	}

	logger.MmeLog.Info("Handover Command",
		zap.Uint32("mme-ue-id", uint32(sourceMMEID)),
		zap.Int("admitted", len(admitted)),
		zap.Int("released", len(releaseEBIs)))
	m.SendS1APConn(ctx, sourceConn, mme.S1APProcedureHandoverCommand, b)
}

// handleHandoverFailure fails the preparation toward the source when the target
// could not admit the handover, leaving the UE on the source (TS 36.413 §8.4.2.3).
// conn is the target; the failure carries the target's MME-UE-S1AP-ID.
func handleHandoverFailure(m *mme.MME, ctx context.Context, conn mme.NasWriter, value []byte) {
	fail, err := s1ap.ParseHandoverFailure(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Handover Failure", zap.Error(err))
		return
	}

	ue, ok := m.LookupUe(fail.MMEUES1APID)
	if !ok {
		return
	}

	if !m.HandoverTargetMatches(ue, fail.MMEUES1APID, conn) {
		return
	}

	logger.MmeLog.Info("Handover Failure", zap.Uint32("target-mme-ue-id", uint32(fail.MMEUES1APID)))
	m.FailHandoverToSource(ctx, ue, causeHOFailureInTarget)
}

// handleENBStatusTransfer relays the source's status container to the target as an
// MME STATUS TRANSFER (TS 36.413 §8.4.6/§8.4.7). Optional: the source may omit it,
// so it never gates completion. conn is the source.
func handleENBStatusTransfer(m *mme.MME, ctx context.Context, conn mme.NasWriter, value []byte) {
	st, err := s1ap.ParseENBStatusTransfer(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode eNB Status Transfer", zap.Error(err))
		return
	}

	ue, ok := resolveUE(m, conn, st.MMEUES1APID, st.ENBUES1APID)
	if !ok {
		return
	}

	targetConn, targetMMEID, targetENBID, ok := m.HandoverStatusTarget(ue)
	if !ok {
		logger.MmeLog.Warn("eNB Status Transfer with no handover in progress", zap.Uint32("mme-ue-id", uint32(st.MMEUES1APID)))

		return
	}

	mst := &s1ap.MMEStatusTransfer{MMEUES1APID: targetMMEID, ENBUES1APID: targetENBID, Container: st.Container}

	b, err := mst.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal MME Status Transfer", zap.Error(err))
		return
	}

	m.SendS1APConn(ctx, targetConn, mme.S1APProcedureMMEStatusTransfer, b)
}

// handleHandoverNotify completes the handover once the UE reaches the target: it
// switches the user plane, commits the {NH, NCC} chain, moves the active S1
// connection to the target, and releases the source by its own MME-UE-S1AP-ID
// (TS 36.413 §8.4.3, TS 23.401 §5.5.1.2.2 steps 13-19). conn is the target.
func handleHandoverNotify(m *mme.MME, ctx context.Context, conn mme.NasWriter, value []byte) {
	notify, err := s1ap.ParseHandoverNotify(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Handover Notify", zap.Error(err))
		return
	}

	ue, ok := m.LookupUe(notify.MMEUES1APID)
	if !ok {
		return
	}

	admitted, releaseEBIs, ok := m.BeginHandoverCommit(ue, conn, notify.ENBUES1APID)
	if !ok {
		logger.MmeLog.Warn("Handover Notify with no matching prepared handover", zap.Uint32("target-mme-ue-id", uint32(notify.MMEUES1APID)))

		return
	}

	// Switch the downlink only at notify (TS 23.401 §5.5.1.2.2 step 15).
	for _, a := range admitted {
		p := m.LookupPDN(ue, a.Ebi)
		if p == nil {
			continue
		}

		if err := m.Session.ModifyEPSSession(ctx, ue.IMSI(), a.Ebi, a.EnbFTEID); err != nil {
			logger.MmeLog.Error("failed to switch an EPS session downlink to the target eNB",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", a.Ebi), zap.Error(err))

			continue
		}

		m.SetPDNEnbFTEID(ue, p, a.EnbFTEID)
	}

	// Release the PDN connections whose default bearer the target rejected.
	for _, ebi := range releaseEBIs {
		if err := m.Session.ReleaseEPSSession(ctx, ue.IMSI(), ebi); err != nil {
			logger.MmeLog.Error("failed to release a rejected PDN connection after handover",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", ebi), zap.Error(err))
		}

		m.DropPDN(ue, ebi)
	}

	mme.EnsureDefaultPDN(ue, admitted)

	sourceConn, sourceMMEID, sourceENBID, targetMMEID, ok := m.FinishHandoverCommit(ue, conn, notify.ENBUES1APID)
	if !ok {
		// A concurrent release (e.g. the source association dropping) tore the UE
		// down during the unlocked user-plane switch above and cleared the handover;
		// it is moot, so leave the UE released.
		logger.MmeLog.Warn("Handover Notify: UE released during the user-plane switch",
			zap.Uint32("target-mme-ue-id", uint32(notify.MMEUES1APID)))

		return
	}

	if notify.TAI.TAC != 0 {
		ue.TouchLastSeen()
	}

	logger.MmeLog.Info("Handover Notify",
		zap.Uint32("target-mme-ue-id", uint32(targetMMEID)),
		zap.Uint32("target-enb-ue-id", uint32(notify.ENBUES1APID)))

	mme.SendUEContextRelease(m, ctx, sourceConn, sourceMMEID, sourceENBID)
}

// handleHandoverCancel releases any prepared target resources and acknowledges,
// leaving the UE on the source (TS 36.413 §8.4.5). conn is the source.
func handleHandoverCancel(m *mme.MME, ctx context.Context, conn mme.NasWriter, value []byte) {
	cancel, err := s1ap.ParseHandoverCancel(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Handover Cancel", zap.Error(err))
		return
	}

	ue, ok := resolveUE(m, conn, cancel.MMEUES1APID, cancel.ENBUES1APID)
	if !ok {
		return
	}

	if releaseConn, releaseMMEID, releaseENBID, has := m.CancelHandover(ue); has {
		mme.SendUEContextRelease(m, ctx, releaseConn, releaseMMEID, releaseENBID)
	}

	ack := &s1ap.HandoverCancelAcknowledge{MMEUES1APID: cancel.MMEUES1APID, ENBUES1APID: cancel.ENBUES1APID}

	b, err := ack.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal Handover Cancel Acknowledge", zap.Error(err))
		return
	}

	logger.MmeLog.Info("Handover Cancel", zap.Uint32("mme-ue-id", uint32(cancel.MMEUES1APID)))
	m.SendS1APConn(ctx, conn, mme.S1APProcedureHandoverCancelAcknowledge, b)
}

// mme.SendHandoverPreparationFailure sends a HANDOVER PREPARATION FAILURE on the
// source association (TS 36.413 §8.4.1.3).

// mme.SendUEContextRelease sends a UE Context Release Command for a handover
// association by its own MME-UE-S1AP-ID (TS 36.413 §8.4): the source after notify,
// or a rejected/superseded target. The connection is removed when its Release
// Complete arrives (ReleaseDetachedConn) or when its association drops.

// handoverUEAMBR builds the UE Aggregate Maximum Bit Rate IE for a HANDOVER
// REQUEST from the UE's stored profile UE-AMBR.
func handoverUEAMBR(ue *mme.UeContext) s1ap.UEAggregateMaximumBitRate {
	return s1ap.UEAggregateMaximumBitRate{
		DL: s1ap.BitRate(mme.BitRateToBps(ue.AmbrDownlink)),
		UL: s1ap.BitRate(mme.BitRateToBps(ue.AmbrUplink)),
	}
}

// handoverSecurityCapabilities encodes the UE's stored security capabilities for a
// HANDOVER REQUEST.
func handoverSecurityCapabilities(ue *mme.UeContext) s1ap.UESecurityCapabilities {
	uecap, err := eps.ParseUENetworkCapability(ue.UeNetCap)
	if err != nil {
		return s1ap.UESecurityCapabilities{}
	}

	return mme.S1apSecurityCapabilities(uecap)
}

// failedHandoverEBIs returns the EPS bearer identities the target eNB did not
// admit: those it explicitly failed plus any the source offered that are missing
// from the admitted set.
func failedHandoverEBIs(ack *s1ap.HandoverRequestAcknowledge, admitted []mme.AdmittedERAB) []uint8 {
	admittedSet := make(map[uint8]struct{}, len(admitted))
	for _, a := range admitted {
		admittedSet[a.Ebi] = struct{}{}
	}

	seen := make(map[uint8]struct{})

	var out []uint8

	add := func(ebi uint8) {
		if _, ok := admittedSet[ebi]; ok {
			return
		}

		if _, ok := seen[ebi]; ok {
			return
		}

		seen[ebi] = struct{}{}
		out = append(out, ebi)
	}

	for _, it := range ack.ERABFailedToSetup {
		add(uint8(it.ERABID))
	}

	return out
}

// releaseItems renders EPS bearer identities as the E-RABs to Release List of a
// HANDOVER COMMAND (TS 36.413 §9.1.5.2).
func releaseItems(ebis []uint8) []s1ap.ERABItem {
	if len(ebis) == 0 {
		return nil
	}

	out := make([]s1ap.ERABItem, 0, len(ebis))
	for _, ebi := range ebis {
		out = append(out, s1ap.ERABItem{ERABID: s1ap.ERABID(ebi), Cause: causeHOFailureInTarget})
	}

	return out
}
