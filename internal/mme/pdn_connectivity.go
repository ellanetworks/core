// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// ESM cause values for PDN connectivity / disconnect (TS 24.301 §9.9.4.4).
const (
	esmCauseUnknownAPN                  uint8 = 27 // missing or unknown APN
	esmCauseRequestRejectedUnspecified  uint8 = 31
	esmCauseRegularDeactivation         uint8 = 36
	esmCauseInvalidEPSBearerIdentity    uint8 = 43
	esmCauseLastPDNDisconnectNotAllowed uint8 = 49
	esmCauseMultiplePDNForAPNNotAllowed uint8 = 55
	esmCauseMaxEPSBearersReached        uint8 = 65
	esmCauseInvalidPTIValue             uint8 = 81
)

// esmRequestHeaderCause validates the ESM header of a UE-requested message that
// is not associated with a bearer context — a PDN Connectivity or PDN Disconnect
// Request (TS 24.301 §7.3). The procedure transaction identity must be assigned
// (1..254, not the unassigned 0 or reserved 255), and the header EPS bearer
// identity must be "no EPS bearer identity assigned" (0). It returns the ESM
// cause to reject with — #81 "invalid PTI value" or #43 "invalid EPS bearer
// identity" — or 0 when the header is valid.
func esmRequestHeaderCause(pti, headerEBI uint8) uint8 {
	if pti == 0 || pti == 255 {
		return esmCauseInvalidPTIValue
	}

	if headerEBI != 0 {
		return esmCauseInvalidEPSBearerIdentity
	}

	return 0
}

// handlePDNConnectivityRequest opens an additional PDN connection (a second default
// bearer to another APN) for a UE that is already EMM-REGISTERED (TS 24.301
// §6.5.1; TS 23.401 §5.10.2). The UE names the data network in the Access Point
// Name IE; the MME authorises it against the subscriber's profile, allocates an
// EPS bearer identity, asks the anchor for a session, and sets up the radio leg
// with an E-RAB SETUP REQUEST carrying the ACTIVATE DEFAULT EPS BEARER CONTEXT
// REQUEST.
func (m *MME) handlePDNConnectivityRequest(ctx context.Context, ue *UeContext, plain []byte) {
	req, err := eps.ParsePDNConnectivityRequest(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode PDN Connectivity Request", zap.Error(err))
		return
	}

	pti := req.ProcedureTransactionIdentity

	if cause := esmRequestHeaderCause(pti, req.EPSBearerIdentity); cause != 0 {
		logger.MmeLog.Info("PDN connectivity rejected: invalid ESM header",
			zap.String("imsi", ue.IMSI()), zap.Uint8("pti", pti),
			zap.Uint8("header-ebi", req.EPSBearerIdentity), zap.Uint8("esm-cause", cause))
		m.rejectPDNConnectivity(ctx, ue, pti, cause)

		return
	}

	if ue.emmState.load() != EMMRegistered || !ue.connected() {
		m.rejectPDNConnectivity(ctx, ue, pti, esmCauseRequestRejectedUnspecified)
		return
	}

	apn := ""
	if len(req.AccessPointName) > 0 {
		if apn, err = eps.ParseAPN(req.AccessPointName); err != nil {
			logger.MmeLog.Warn("failed to decode APN in PDN Connectivity Request", zap.Error(err))
			m.rejectPDNConnectivity(ctx, ue, pti, esmCauseUnknownAPN)

			return
		}
	}

	if apn == "" {
		m.rejectPDNConnectivity(ctx, ue, pti, esmCauseUnknownAPN)
		return
	}

	if m.findPDNByAPN(ue, apn) != nil {
		logger.MmeLog.Info("PDN connectivity rejected: APN already connected",
			zap.String("imsi", ue.IMSI()), zap.String("apn", apn))
		m.rejectPDNConnectivity(ctx, ue, pti, esmCauseMultiplePDNForAPNNotAllowed)

		return
	}

	qos, err := m.resolveQoSByAPN(ctx, ue.IMSI(), apn)
	if errors.Is(err, ErrUnknownAPN) {
		logger.MmeLog.Info("PDN connectivity rejected: APN not in subscriber profile",
			zap.String("imsi", ue.IMSI()), zap.String("apn", apn))
		m.rejectPDNConnectivity(ctx, ue, pti, esmCauseUnknownAPN)

		return
	}

	if err != nil {
		logger.MmeLog.Warn("failed to resolve QoS for additional PDN", zap.String("apn", apn), zap.Error(err))
		m.rejectPDNConnectivity(ctx, ue, pti, esmCauseRequestRejectedUnspecified)

		return
	}

	p := m.addPDN(ue)
	if p == nil {
		logger.MmeLog.Info("PDN connectivity rejected: no free EPS bearer identity",
			zap.String("imsi", ue.IMSI()))
		m.rejectPDNConnectivity(ctx, ue, pti, esmCauseMaxEPSBearersReached)

		return
	}

	bearer, err := m.session.CreateEPSSession(ctx, models.EPSBearerRequest{
		IMSI:              ue.IMSI(),
		EPSBearerIdentity: p.ebi,
		PolicyID:          qos.PolicyID,
		APN:               qos.APN,
		AMBRUplink:        qos.SessAmbrULStr,
		AMBRDownlink:      qos.SessAmbrDLStr,
		IPv4Pool:          qos.IPv4Pool,
		IPv6Pool:          qos.IPv6Pool,
		DNS:               qos.DNS,
		MTU:               qos.MTU,
		RequestedPDNType:  req.PDNType,
	})
	if err != nil {
		logger.MmeLog.Info("PDN connectivity rejected: session setup failed",
			zap.String("imsi", ue.IMSI()), zap.String("apn", apn), zap.Error(err))
		m.dropPDN(ue, p.ebi)
		m.rejectPDNConnectivity(ctx, ue, pti, esmCauseRequestRejectedUnspecified)

		return
	}

	p.apn = qos.APN
	p.dnConfig = qos.dnFingerprint()
	p.sessAmbrDLBps = bitRateToBps(qos.SessAmbrDLStr)
	p.sessAmbrULBps = bitRateToBps(qos.SessAmbrULStr)
	p.qci = qos.QCI
	p.arp = qos.ARP
	p.pdnType = bearer.PDNType
	p.ueIP = bearer.IPv4
	p.ueIPv6Prefix = bearer.IPv6Prefix
	p.ueIPv6IID = bearer.IPv6IID
	p.dns = bearer.DNS
	p.esmCause = bearer.ESMCause
	p.sgwFTEID = bearer.SGW
	p.sgwN3IPv6 = bearer.SGWN3IPv6

	esm, err := buildActivateDefaultESM(p, qos, pti)
	if err != nil {
		logger.MmeLog.Error("failed to build Activate Default EPS Bearer Context Request", zap.Error(err))
		m.releasePDN(ue, p)

		return
	}

	naspdu, err := ue.protectDownlink(esm, eps.SHTIntegrityProtectedCiphered)
	if err != nil {
		logger.MmeLog.Error("failed to protect Activate Default EPS Bearer Context Request", zap.Error(err))
		m.releasePDN(ue, p)

		return
	}

	logger.MmeLog.Info("opening additional PDN connection",
		zap.String("imsi", ue.IMSI()), zap.String("apn", apn), zap.Uint8("ebi", p.ebi))
	m.sendERABSetup(ctx, ue, p, qos, naspdu)
}

// sendERABSetup asks the eNB to set up the radio leg of a new PDN connection,
// carrying the ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST to the UE (TS 36.413
// §8.2.1).
func (m *MME) sendERABSetup(ctx context.Context, ue *UeContext, p *pdnConnection, qos *epsQoS, naspdu []byte) {
	sgwTLA, err := models.EncodeTransportLayerAddress(p.sgwFTEID.Addr, p.sgwN3IPv6)
	if err != nil {
		logger.MmeLog.Error("failed to encode S-GW transport layer address", zap.Error(err))
		m.releasePDN(ue, p)

		return
	}

	ambr := s1ap.UEAggregateMaximumBitRate{DL: s1ap.BitRate(qos.AMBRDL), UL: s1ap.BitRate(qos.AMBRUL)}

	reqMsg := &s1ap.ERABSetupRequest{
		MMEUES1APID:               ue.s1.MMEUES1APID,
		ENBUES1APID:               ue.s1.ENBUES1APID,
		UEAggregateMaximumBitRate: &ambr,
		ERABToBeSetup: []s1ap.ERABToBeSetupItemBearerSUReq{{
			ERABID: s1ap.ERABID(p.ebi),
			QoS: s1ap.ERABLevelQoSParameters{
				QCI: s1ap.QCI(qos.QCI),
				ARP: s1ap.AllocationAndRetentionPriority{
					PriorityLevel:           qos.ARP,
					PreemptionCapability:    s1ap.PreemptionShallNotTrigger,
					PreemptionVulnerability: s1ap.PreemptionNotPreemptable,
				},
			},
			TransportLayerAddress: s1ap.TransportLayerAddress(sgwTLA),
			GTPTEID:               s1ap.GTPTEID(p.sgwFTEID.TEID),
			NASPDU:                s1ap.NASPDU(naspdu),
		}},
	}

	b, err := reqMsg.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal E-RAB Setup Request", zap.Error(err))
		m.releasePDN(ue, p)

		return
	}

	m.sendS1AP(ctx, ue, S1APProcedureERABSetupRequest, b)
}

// handleERABSetupResponse processes the eNB's answer to an E-RAB SETUP REQUEST
// (TS 36.413 §8.2.1): it records the eNB S1-U endpoint of each established E-RAB
// on the anchor session, and releases any E-RAB the eNB failed to set up.
func (m *MME) handleERABSetupResponse(conn nasWriter, value []byte) {
	msg, err := s1ap.ParseERABSetupResponse(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode E-RAB Setup Response", zap.Error(err))
		return
	}

	ue, ok := m.resolveUE(conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	for _, erab := range msg.ERABSetup {
		p := m.lookupPDN(ue, uint8(erab.ERABID))
		if p == nil {
			logger.MmeLog.Warn("E-RAB Setup Response for an unknown E-RAB",
				zap.Uint32("mme-ue-id", uint32(ue.s1.MMEUES1APID)), zap.Uint8("e-rab-id", uint8(erab.ERABID)))

			continue
		}

		enbAddr, ok := enbTransportAddress(erab.TransportLayerAddress)
		if !ok {
			logger.MmeLog.Warn("E-RAB Setup Response with an invalid eNB transport address",
				zap.Uint32("mme-ue-id", uint32(ue.s1.MMEUES1APID)), zap.Uint8("e-rab-id", uint8(erab.ERABID)))

			continue
		}

		p.enbFTEID = models.FTEID{TEID: uint32(erab.GTPTEID), Addr: enbAddr}

		if err := m.session.ModifyEPSSession(context.Background(), ue.IMSI(), p.ebi, p.enbFTEID); err != nil {
			logger.MmeLog.Error("failed to set the eNB F-TEID on the additional EPS session",
				zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", p.ebi), zap.Error(err))

			continue
		}

		logger.MmeLog.Info("additional PDN connection radio leg established",
			zap.String("imsi", ue.IMSI()), zap.String("apn", p.apn), zap.Uint8("ebi", p.ebi),
			zap.String("enb-s1u", enbAddr.String()))
	}

	for _, erab := range msg.ERABFailedToSetup {
		if p := m.lookupPDN(ue, uint8(erab.ERABID)); p != nil {
			logger.MmeLog.Warn("eNB failed to set up an additional E-RAB; releasing the PDN connection",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", uint8(erab.ERABID)))
			m.releasePDN(ue, p)
		}
	}
}

// handlePDNDisconnectRequest releases one of a UE's PDN connections at its request
// (TS 24.301 §6.5.2; TS 23.401 §5.10.3). The UE names the PDN by its default
// bearer's Linked EPS Bearer Identity. The last PDN connection cannot be
// disconnected this way — the UE detaches instead (ESM cause #49). The bearer is
// torn down with a DEACTIVATE EPS BEARER CONTEXT REQUEST; the session is released
// when the UE accepts.
func (m *MME) handlePDNDisconnectRequest(ctx context.Context, ue *UeContext, plain []byte) {
	req, err := eps.ParsePDNDisconnectRequest(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode PDN Disconnect Request", zap.Error(err))
		return
	}

	pti := req.ProcedureTransactionIdentity

	if cause := esmRequestHeaderCause(pti, req.EPSBearerIdentity); cause != 0 {
		logger.MmeLog.Info("PDN disconnect rejected: invalid ESM header",
			zap.String("imsi", ue.IMSI()), zap.Uint8("pti", pti),
			zap.Uint8("header-ebi", req.EPSBearerIdentity), zap.Uint8("esm-cause", cause))
		m.rejectPDNDisconnect(ctx, ue, pti, cause)

		return
	}

	p := m.lookupPDN(ue, req.LinkedEPSBearerIdentity)
	if p == nil {
		logger.MmeLog.Info("PDN disconnect rejected: unknown linked EPS bearer",
			zap.String("imsi", ue.IMSI()), zap.Uint8("linked-ebi", req.LinkedEPSBearerIdentity))
		m.rejectPDNDisconnect(ctx, ue, pti, esmCauseRequestRejectedUnspecified)

		return
	}

	ue.mu.Lock()
	numPDNs := len(ue.pdns)
	ue.mu.Unlock()

	if numPDNs <= 1 {
		logger.MmeLog.Info("PDN disconnect rejected: last PDN connection",
			zap.String("imsi", ue.IMSI()), zap.Uint8("linked-ebi", req.LinkedEPSBearerIdentity))
		m.rejectPDNDisconnect(ctx, ue, pti, esmCauseLastPDNDisconnectNotAllowed)

		return
	}

	logger.MmeLog.Info("disconnecting PDN connection",
		zap.String("imsi", ue.IMSI()), zap.String("apn", p.apn), zap.Uint8("ebi", p.ebi))
	m.disconnectBearer(ctx, ue, p, esmCauseRegularDeactivation, pti)
}

// rejectPDNDisconnect refuses a PDN DISCONNECT REQUEST with an ESM cause
// (TS 24.301 §6.5.2.4).
func (m *MME) rejectPDNDisconnect(ctx context.Context, ue *UeContext, pti, cause uint8) {
	m.sendDownlinkProtected(ctx, ue, &eps.PDNDisconnectReject{
		ProcedureTransactionIdentity: pti,
		ESMCause:                     cause,
	})
}

// handleActivateDefaultBearerAccept confirms an additional PDN connection once the UE
// accepts the network's ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST (TS 24.301
// §6.4.1).
func (m *MME) handleActivateDefaultBearerAccept(ue *UeContext, plain []byte) {
	accept, err := eps.ParseActivateDefaultEPSBearerContextAccept(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Activate Default EPS Bearer Context Accept", zap.Error(err))
		return
	}

	p := m.lookupPDN(ue, accept.EPSBearerIdentity)
	if p == nil {
		logger.MmeLog.Warn("Activate Default Accept for an unknown EPS bearer",
			zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", accept.EPSBearerIdentity))

		return
	}

	logger.MmeLog.Info("additional PDN connection active",
		zap.String("imsi", ue.IMSI()), zap.String("apn", p.apn), zap.Uint8("ebi", p.ebi))
}

// handleActivateDefaultBearerReject releases an additional PDN connection the UE
// refused (TS 24.301 §6.4.1.5).
func (m *MME) handleActivateDefaultBearerReject(ue *UeContext, plain []byte) {
	reject, err := eps.ParseActivateDefaultEPSBearerContextReject(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Activate Default EPS Bearer Context Reject", zap.Error(err))
		return
	}

	if p := m.lookupPDN(ue, reject.EPSBearerIdentity); p != nil {
		logger.MmeLog.Info("UE rejected an additional PDN connection; releasing it",
			zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", p.ebi), zap.Uint8("esm-cause", reject.ESMCause))
		m.releasePDN(ue, p)
	}
}

// rejectPDNConnectivity refuses a PDN CONNECTIVITY REQUEST with an ESM cause
// (TS 24.301 §6.5.1.4).
func (m *MME) rejectPDNConnectivity(ctx context.Context, ue *UeContext, pti, cause uint8) {
	m.sendDownlinkProtected(ctx, ue, &eps.PDNConnectivityReject{
		ProcedureTransactionIdentity: pti,
		ESMCause:                     cause,
	})
}

// releasePDN tears down a PDN connection's anchor session and removes it from the
// UE, freeing its EPS bearer identity.
func (m *MME) releasePDN(ue *UeContext, p *pdnConnection) {
	if err := m.session.ReleaseEPSSession(context.Background(), ue.IMSI(), p.ebi); err != nil {
		logger.MmeLog.Warn("failed to release PDN connection session",
			zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", p.ebi), zap.Error(err))
	}

	ue.mu.Lock()
	delete(ue.pdns, p.ebi)

	if ue.defaultEBI == p.ebi {
		ue.defaultEBI = 0
	}

	ue.mu.Unlock()
}

// releaseAllSessions releases every PDN connection's anchor session and clears
// them from the UE. Used when the whole UE context is torn down (detach).
func (m *MME) releaseAllSessions(ue *UeContext) {
	for _, p := range m.takeAllPDNs(ue) {
		if err := m.session.ReleaseEPSSession(context.Background(), ue.IMSI(), p.ebi); err != nil {
			logger.MmeLog.Warn("failed to release PDN connection session",
				zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", p.ebi), zap.Error(err))
		}
	}
}

// deactivateAllSessions buffers every PDN connection's downlink so data for the
// idle UE triggers paging (TS 23.401), without releasing the sessions.
func (m *MME) deactivateAllSessions(ue *UeContext) {
	for _, p := range m.snapshotPDNs(ue) {
		if err := m.session.DeactivateEPSSession(context.Background(), ue.IMSI(), p.ebi); err != nil {
			logger.MmeLog.Warn("failed to deactivate PDN connection session for paging",
				zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", p.ebi), zap.Error(err))
		}
	}
}

// takeAllPDNs detaches and returns every PDN connection from the UE under the
// lock, so the caller can release the sessions without holding it.
func (m *MME) takeAllPDNs(ue *UeContext) []*pdnConnection {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	out := make([]*pdnConnection, 0, len(ue.pdns))
	for _, p := range ue.pdns {
		out = append(out, p)
	}

	ue.pdns = nil
	ue.defaultEBI = 0

	return out
}
