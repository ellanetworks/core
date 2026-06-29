// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// S1AP causes the MME returns during S1 handover (TS 36.413 §9.2.1.3,
// CauseRadioNetwork enumeration).
var (
	causeSuccessfulHandover     = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 2}  // successful-handover
	causeHOFailureInTarget      = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 6}  // ho-failure-in-target-EPC-eNB-or-target-system
	causeHOTargetNotAllowed     = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 7}  // ho-target-not-allowed
	causeUnknownTargetID        = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 11} // unknown-targetID
	causeHandoverNoSecurity     = s1ap.Cause{Group: s1ap.CauseGroupNAS, Value: 1}           // authentication-failure
	causeHandoverPrepUnspecific = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0}  // unspecified
)

// handleHandoverRequired starts an S1 handover preparation toward the target eNB,
// or replies HANDOVER PREPARATION FAILURE (TS 36.413 §8.4.1). conn is the source.
func (m *MME) handleHandoverRequired(ctx context.Context, conn NasWriter, value []byte) {
	req, err := s1ap.ParseHandoverRequired(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Handover Required", zap.Error(err))
		return
	}

	ue, ok := m.resolveUE(conn, req.MMEUES1APID, req.ENBUES1APID)
	if !ok {
		return
	}

	if req.HandoverType != s1ap.HandoverTypeIntraLTE {
		logger.MmeLog.Warn("Handover Required for an unsupported handover type",
			zap.Uint32("mme-ue-id", uint32(req.MMEUES1APID)), zap.Uint8("handover-type", uint8(req.HandoverType)))
		m.sendHandoverPreparationFailure(ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeHOTargetNotAllowed)

		return
	}

	if !ue.Secured() || !ue.HasKASME() {
		logger.MmeLog.Warn("Handover Required for a UE without a security context",
			zap.Uint32("mme-ue-id", uint32(req.MMEUES1APID)))
		m.sendHandoverPreparationFailure(ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeHandoverNoSecurity)

		return
	}

	target, ok := m.findENBByGlobalID(req.TargetID.TargeteNBID.GlobalENBID)
	if !ok {
		logger.MmeLog.Warn("Handover Required for an unknown target eNB",
			zap.Uint32("mme-ue-id", uint32(req.MMEUES1APID)), zap.String("target-enb", enbID(req.TargetID.TargeteNBID.GlobalENBID)))
		m.sendHandoverPreparationFailure(ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeUnknownTargetID)

		return
	}

	if target == conn {
		logger.MmeLog.Warn("Handover Required targets the source eNB",
			zap.Uint32("mme-ue-id", uint32(req.MMEUES1APID)))
		m.sendHandoverPreparationFailure(ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeHOTargetNotAllowed)

		return
	}

	bearers, ok := m.handoverBearers(ue)
	if !ok {
		m.sendHandoverPreparationFailure(ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeHandoverPrepUnspecific)
		return
	}

	targetMMEID, newNH, newNCC, ok := m.prepareHandover(ue, target, req.MMEUES1APID)
	if !ok {
		m.sendHandoverPreparationFailure(ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeHandoverPrepUnspecific)
		return
	}

	hoReq := &s1ap.HandoverRequest{
		MMEUES1APID:            targetMMEID,
		HandoverType:           s1ap.HandoverTypeIntraLTE,
		Cause:                  req.Cause,
		UEAMBR:                 m.handoverUEAMBR(ue),
		ERABToBeSetup:          bearers,
		SourceToTarget:         req.SourceToTarget,
		UESecurityCapabilities: m.handoverSecurityCapabilities(ue),
		SecurityContext:        s1ap.SecurityContext{NextHopChainingCount: newNCC, NextHopParameter: s1ap.SecurityKey(newNH)},
	}

	b, err := hoReq.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal Handover Request", zap.Error(err))
		m.clearHandover(ue)
		m.sendHandoverPreparationFailure(ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeHandoverPrepUnspecific)

		return
	}

	logger.MmeLog.Info("Handover Request",
		zap.Uint32("target-mme-ue-id", uint32(targetMMEID)),
		zap.String("target-enb", enbID(req.TargetID.TargeteNBID.GlobalENBID)),
		zap.Int("e-rabs", len(bearers)))
	m.SendS1APConn(ctx, target, S1APProcedureHandoverRequest, b)
}

// handleHandoverRequestAcknowledge records the target's downlink endpoints and
// sends a HANDOVER COMMAND to the source, or fails the handover when no usable
// bearer was admitted (TS 36.413 §8.4.2). conn is the target; the acknowledge
// carries the target's MME-UE-S1AP-ID.
func (m *MME) handleHandoverRequestAcknowledge(ctx context.Context, conn NasWriter, value []byte) {
	ack, err := s1ap.ParseHandoverRequestAcknowledge(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Handover Request Acknowledge", zap.Error(err))
		return
	}

	ue, ok := m.LookupUe(ack.MMEUES1APID)
	if !ok {
		// No UE for this target id; release the context the ack just created.
		m.sendUEContextRelease(ctx, conn, ack.MMEUES1APID, ack.ENBUES1APID)
		return
	}

	if !m.matchAndSetTargetENB(ue, ack.MMEUES1APID, ack.ENBUES1APID, conn) {
		logger.MmeLog.Warn("Handover Request Acknowledge with no matching preparation",
			zap.Uint32("target-mme-ue-id", uint32(ack.MMEUES1APID)))
		m.sendUEContextRelease(ctx, conn, ack.MMEUES1APID, ack.ENBUES1APID)

		return
	}

	admitted := make([]admittedERAB, 0, len(ack.ERABAdmitted))

	for _, it := range ack.ERABAdmitted {
		addr, ok := enbTransportAddress(it.TransportLayerAddress)
		if !ok {
			logger.MmeLog.Warn("Handover Request Acknowledge E-RAB has an invalid target address; treating as failed",
				zap.Uint32("target-mme-ue-id", uint32(ack.MMEUES1APID)), zap.Uint8("e-rab-id", uint8(it.ERABID)))

			continue
		}

		admitted = append(admitted, admittedERAB{ebi: uint8(it.ERABID), enbFTEID: models.FTEID{TEID: uint32(it.GTPTEID), Addr: addr}})
	}

	// Each E-RAB is a PDN's default bearer, so a rejected one releases its PDN
	// (TS 23.401 §5.5.1.2.2 step 15).
	releaseEBIs := failedHandoverEBIs(ack, admitted)

	if len(admitted) == 0 {
		// No default bearer admitted: the handover is rejected (TS 23.401 §5.5.1.2.3).
		logger.MmeLog.Warn("Handover Request Acknowledge admitted no E-RAB; rejecting handover",
			zap.Uint32("target-mme-ue-id", uint32(ack.MMEUES1APID)))
		m.sendUEContextRelease(ctx, conn, ack.MMEUES1APID, ack.ENBUES1APID)
		m.failHandoverToSource(ctx, ue, causeHOFailureInTarget)

		return
	}

	sourceConn, sourceMMEID, sourceENBID, ok := m.commitHandoverPrepared(ue, ack.MMEUES1APID, conn, admitted, releaseEBIs)
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
	m.SendS1APConn(ctx, sourceConn, S1APProcedureHandoverCommand, b)
}

// handleHandoverFailure fails the preparation toward the source when the target
// could not admit the handover, leaving the UE on the source (TS 36.413 §8.4.2.3).
// conn is the target; the failure carries the target's MME-UE-S1AP-ID.
func (m *MME) handleHandoverFailure(ctx context.Context, conn NasWriter, value []byte) {
	fail, err := s1ap.ParseHandoverFailure(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Handover Failure", zap.Error(err))
		return
	}

	ue, ok := m.LookupUe(fail.MMEUES1APID)
	if !ok {
		return
	}

	if !m.handoverTargetMatches(ue, fail.MMEUES1APID, conn) {
		return
	}

	logger.MmeLog.Info("Handover Failure", zap.Uint32("target-mme-ue-id", uint32(fail.MMEUES1APID)))
	m.failHandoverToSource(ctx, ue, causeHOFailureInTarget)
}

// handleENBStatusTransfer relays the source's status container to the target as an
// MME STATUS TRANSFER (TS 36.413 §8.4.6/§8.4.7). Optional: the source may omit it,
// so it never gates completion. conn is the source.
func (m *MME) handleENBStatusTransfer(ctx context.Context, conn NasWriter, value []byte) {
	st, err := s1ap.ParseENBStatusTransfer(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode eNB Status Transfer", zap.Error(err))
		return
	}

	ue, ok := m.resolveUE(conn, st.MMEUES1APID, st.ENBUES1APID)
	if !ok {
		return
	}

	targetConn, targetMMEID, targetENBID, ok := m.handoverStatusTarget(ue)
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

	m.SendS1APConn(ctx, targetConn, S1APProcedureMMEStatusTransfer, b)
}

// handleHandoverNotify completes the handover once the UE reaches the target: it
// switches the user plane, commits the {NH, NCC} chain, moves the active S1
// connection to the target, and releases the source by its own MME-UE-S1AP-ID
// (TS 36.413 §8.4.3, TS 23.401 §5.5.1.2.2 steps 13-19). conn is the target.
func (m *MME) handleHandoverNotify(ctx context.Context, conn NasWriter, value []byte) {
	notify, err := s1ap.ParseHandoverNotify(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Handover Notify", zap.Error(err))
		return
	}

	ue, ok := m.LookupUe(notify.MMEUES1APID)
	if !ok {
		return
	}

	admitted, releaseEBIs, ok := m.beginHandoverCommit(ue, conn, notify.ENBUES1APID)
	if !ok {
		logger.MmeLog.Warn("Handover Notify with no matching prepared handover", zap.Uint32("target-mme-ue-id", uint32(notify.MMEUES1APID)))

		return
	}

	// Switch the downlink only at notify (TS 23.401 §5.5.1.2.2 step 15).
	for _, a := range admitted {
		p := m.LookupPDN(ue, a.ebi)
		if p == nil {
			continue
		}

		if err := m.Session.ModifyEPSSession(ctx, ue.IMSI(), a.ebi, a.enbFTEID); err != nil {
			logger.MmeLog.Error("failed to switch an EPS session downlink to the target eNB",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", a.ebi), zap.Error(err))

			continue
		}

		ue.mu.Lock()
		p.EnbFTEID = a.enbFTEID
		ue.mu.Unlock()
	}

	// Release the PDN connections whose default bearer the target rejected.
	for _, ebi := range releaseEBIs {
		if err := m.Session.ReleaseEPSSession(ctx, ue.IMSI(), ebi); err != nil {
			logger.MmeLog.Error("failed to release a rejected PDN connection after handover",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", ebi), zap.Error(err))
		}

		m.DropPDN(ue, ebi)
	}

	m.ensureDefaultPDN(ue, admitted)

	sourceConn, sourceMMEID, sourceENBID, targetMMEID, ok := m.finishHandoverCommit(ue, conn, notify.ENBUES1APID)
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

	m.sendUEContextRelease(ctx, sourceConn, sourceMMEID, sourceENBID)
}

// handleHandoverCancel releases any prepared target resources and acknowledges,
// leaving the UE on the source (TS 36.413 §8.4.5). conn is the source.
func (m *MME) handleHandoverCancel(ctx context.Context, conn NasWriter, value []byte) {
	cancel, err := s1ap.ParseHandoverCancel(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Handover Cancel", zap.Error(err))
		return
	}

	ue, ok := m.resolveUE(conn, cancel.MMEUES1APID, cancel.ENBUES1APID)
	if !ok {
		return
	}

	if releaseConn, releaseMMEID, releaseENBID, has := m.cancelHandover(ue); has {
		m.sendUEContextRelease(ctx, releaseConn, releaseMMEID, releaseENBID)
	}

	ack := &s1ap.HandoverCancelAcknowledge{MMEUES1APID: cancel.MMEUES1APID, ENBUES1APID: cancel.ENBUES1APID}

	b, err := ack.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal Handover Cancel Acknowledge", zap.Error(err))
		return
	}

	logger.MmeLog.Info("Handover Cancel", zap.Uint32("mme-ue-id", uint32(cancel.MMEUES1APID)))
	m.SendS1APConn(ctx, conn, S1APProcedureHandoverCancelAcknowledge, b)
}

// sendHandoverPreparationFailure sends a HANDOVER PREPARATION FAILURE on the
// source association (TS 36.413 §8.4.1.3).
func (m *MME) sendHandoverPreparationFailure(ctx context.Context, conn NasWriter, mmeUEID s1ap.MMEUES1APID, enbUEID s1ap.ENBUES1APID, cause s1ap.Cause) {
	fail := &s1ap.HandoverPreparationFailure{MMEUES1APID: mmeUEID, ENBUES1APID: enbUEID, Cause: cause}

	b, err := fail.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal Handover Preparation Failure", zap.Error(err))
		return
	}

	m.SendS1APConn(ctx, conn, S1APProcedureHandoverPreparationFailure, b)
}

// sendUEContextRelease sends a UE Context Release Command for a handover
// association by its own MME-UE-S1AP-ID (TS 36.413 §8.4): the source after notify,
// or a rejected/superseded target. The connection is removed when its Release
// Complete arrives (releaseDetachedConn) or when its association drops.
func (m *MME) sendUEContextRelease(ctx context.Context, conn NasWriter, mmeUEID s1ap.MMEUES1APID, enbUEID s1ap.ENBUES1APID) {
	cmd := &s1ap.UEContextReleaseCommand{
		UES1APIDs: s1ap.UES1APIDs{MMEUES1APID: mmeUEID, ENBUES1APID: enbUEID, Pair: true},
		Cause:     causeSuccessfulHandover,
	}

	b, err := cmd.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal handover UE Context Release Command", zap.Error(err))
		return
	}

	logger.MmeLog.Info("UE Context Release Command (handover)", zap.Uint32("mme-ue-id", uint32(mmeUEID)))
	m.SendS1APConn(ctx, conn, S1APProcedureUEContextReleaseCommand, b)
}

// ensureDefaultPDN promotes the lowest surviving admitted PDN to the UE's default
// when the attach-default PDN was released during a partial-admission handover, so
// a registered UE always retains a default PDN connection (its EPS last-resort
// connectivity, TS 23.401). A no-op when a default still exists or no admitted PDN
// survives.
func (m *MME) ensureDefaultPDN(ue *UeContext, admitted []admittedERAB) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.DefaultEBI != 0 {
		return
	}

	lowest := uint8(0)

	for _, a := range admitted {
		if _, ok := ue.Pdns[a.ebi]; !ok {
			continue
		}

		if lowest == 0 || a.ebi < lowest {
			lowest = a.ebi
		}
	}

	if lowest != 0 {
		ue.DefaultEBI = lowest
	}
}

// handoverBearers snapshots the UE's PDN connections into the E-RABs To Be Setup
// list of a HANDOVER REQUEST (TS 36.413 §9.1.5.4): each bearer's serving GW S1-U
// uplink endpoint and QoS. It returns false when the UE has no usable bearer.
func (m *MME) handoverBearers(ue *UeContext) ([]s1ap.ERABToBeSetupItemHOReq, bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	bearers := make([]s1ap.ERABToBeSetupItemHOReq, 0, len(ue.Pdns))

	for _, p := range ue.Pdns {
		sgwTLA, err := models.EncodeTransportLayerAddress(p.SgwFTEID.Addr, p.SgwN3IPv6)
		if err != nil {
			logger.MmeLog.Error("failed to encode S-GW transport layer address for handover",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", p.Ebi), zap.Error(err))

			continue
		}

		bearers = append(bearers, s1ap.ERABToBeSetupItemHOReq{
			ERABID:                s1ap.ERABID(p.Ebi),
			TransportLayerAddress: s1ap.TransportLayerAddress(sgwTLA),
			GTPTEID:               s1ap.GTPTEID(p.SgwFTEID.TEID),
			QoS: s1ap.ERABLevelQoSParameters{
				QCI: s1ap.QCI(p.Qci),
				ARP: s1ap.AllocationAndRetentionPriority{
					PriorityLevel:           p.Arp,
					PreemptionCapability:    s1ap.PreemptionShallNotTrigger,
					PreemptionVulnerability: s1ap.PreemptionNotPreemptable,
				},
			},
		})
	}

	return bearers, len(bearers) > 0
}

// handoverUEAMBR builds the UE Aggregate Maximum Bit Rate IE for a HANDOVER
// REQUEST from the UE's stored profile UE-AMBR.
func (m *MME) handoverUEAMBR(ue *UeContext) s1ap.UEAggregateMaximumBitRate {
	return s1ap.UEAggregateMaximumBitRate{
		DL: s1ap.BitRate(BitRateToBps(ue.AmbrDownlink)),
		UL: s1ap.BitRate(BitRateToBps(ue.AmbrUplink)),
	}
}

// handoverSecurityCapabilities encodes the UE's stored security capabilities for a
// HANDOVER REQUEST.
func (m *MME) handoverSecurityCapabilities(ue *UeContext) s1ap.UESecurityCapabilities {
	uecap, err := eps.ParseUENetworkCapability(ue.UeNetCap)
	if err != nil {
		return s1ap.UESecurityCapabilities{}
	}

	return S1apSecurityCapabilities(uecap)
}

// failedHandoverEBIs returns the EPS bearer identities the target eNB did not
// admit: those it explicitly failed plus any the source offered that are missing
// from the admitted set.
func failedHandoverEBIs(ack *s1ap.HandoverRequestAcknowledge, admitted []admittedERAB) []uint8 {
	admittedSet := make(map[uint8]struct{}, len(admitted))
	for _, a := range admitted {
		admittedSet[a.ebi] = struct{}{}
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
