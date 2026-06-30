// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
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

// esmRequestHeaderCause validates the ESM header of a UE-requested message not
// associated with a bearer context — a PDN Connectivity or PDN Disconnect Request
// (TS 24.301 §7.3). The PTI must be assigned (1..254) and the header EPS bearer
// identity must be 0 ("no EPS bearer identity assigned"). It returns the reject
// cause #81 or #43, or 0 when the header is valid.
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
// bearer to the APN the UE names) for an already EMM-REGISTERED UE, authorised
// against the subscriber's profile (TS 24.301 §6.5.1; TS 23.401 §5.10.2).
func handlePDNConnectivityRequest(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte) {
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
		rejectPDNConnectivity(m, ctx, ue, pti, cause)

		return
	}

	if ue.EMMState() != mme.EMMRegistered || !ue.Connected() {
		rejectPDNConnectivity(m, ctx, ue, pti, esmCauseRequestRejectedUnspecified)
		return
	}

	apn := ""
	if len(req.AccessPointName) > 0 {
		if apn, err = eps.ParseAPN(req.AccessPointName); err != nil {
			logger.MmeLog.Warn("failed to decode APN in PDN Connectivity Request", zap.Error(err))
			rejectPDNConnectivity(m, ctx, ue, pti, esmCauseUnknownAPN)

			return
		}
	}

	if apn == "" {
		rejectPDNConnectivity(m, ctx, ue, pti, esmCauseUnknownAPN)
		return
	}

	if m.FindPDNByAPN(ue, apn) != nil {
		logger.MmeLog.Info("PDN connectivity rejected: APN already connected",
			zap.String("imsi", ue.IMSI()), zap.String("apn", apn))
		rejectPDNConnectivity(m, ctx, ue, pti, esmCauseMultiplePDNForAPNNotAllowed)

		return
	}

	qos, err := mme.ResolveQoSByAPN(m, ctx, ue.IMSI(), apn)
	if errors.Is(err, mme.ErrUnknownAPN) {
		logger.MmeLog.Info("PDN connectivity rejected: APN not in subscriber profile",
			zap.String("imsi", ue.IMSI()), zap.String("apn", apn))
		rejectPDNConnectivity(m, ctx, ue, pti, esmCauseUnknownAPN)

		return
	}

	if err != nil {
		logger.MmeLog.Warn("failed to resolve QoS for additional PDN", zap.String("apn", apn), zap.Error(err))
		rejectPDNConnectivity(m, ctx, ue, pti, esmCauseRequestRejectedUnspecified)

		return
	}

	p := m.AddPDN(ue)
	if p == nil {
		logger.MmeLog.Info("PDN connectivity rejected: no free EPS bearer identity",
			zap.String("imsi", ue.IMSI()))
		rejectPDNConnectivity(m, ctx, ue, pti, esmCauseMaxEPSBearersReached)

		return
	}

	bearer, err := m.Session.CreateEPSSession(ctx, models.EPSBearerRequest{
		IMSI:              ue.IMSI(),
		EPSBearerIdentity: p.Ebi,
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
		m.DropPDN(ue, p.Ebi)
		rejectPDNConnectivity(m, ctx, ue, pti, esmCauseRequestRejectedUnspecified)

		return
	}

	p.Apn = qos.APN
	p.DnConfig = qos.DnFingerprint()
	p.SessAmbrDLBps = mme.BitRateToBps(qos.SessAmbrDLStr)
	p.SessAmbrULBps = mme.BitRateToBps(qos.SessAmbrULStr)
	p.Qci = qos.QCI
	p.Arp = qos.ARP
	p.PdnType = bearer.PDNType
	p.UeIP = bearer.IPv4
	p.UeIPv6Prefix = bearer.IPv6Prefix
	p.UeIPv6IID = bearer.IPv6IID
	p.Dns = bearer.DNS
	p.EsmCause = bearer.ESMCause
	p.SgwFTEID = bearer.SGW
	p.SgwN3IPv6 = bearer.SGWN3IPv6

	esm, err := buildActivateDefaultESM(p, qos, pti)
	if err != nil {
		logger.MmeLog.Error("failed to build Activate Default EPS Bearer Context Request", zap.Error(err))
		m.ReleasePDN(ue, p)

		return
	}

	naspdu, err := ue.ProtectDownlink(esm, eps.SHTIntegrityProtectedCiphered)
	if err != nil {
		logger.MmeLog.Error("failed to protect Activate Default EPS Bearer Context Request", zap.Error(err))
		m.ReleasePDN(ue, p)

		return
	}

	logger.MmeLog.Info("opening additional PDN connection",
		zap.String("imsi", ue.IMSI()), zap.String("apn", apn), zap.Uint8("ebi", p.Ebi))
	sendERABSetup(m, ctx, ue, p, qos, naspdu)
}

// sendERABSetup asks the eNB to set up the radio leg of a new PDN connection,
// carrying the ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST to the UE (TS 36.413
// §8.2.1).
func sendERABSetup(m *mme.MME, ctx context.Context, ue *mme.UeContext, p *mme.PdnConnection, qos *mme.EpsQoS, naspdu []byte) {
	sgwTLA, err := models.EncodeTransportLayerAddress(p.SgwFTEID.Addr, p.SgwN3IPv6)
	if err != nil {
		logger.MmeLog.Error("failed to encode S-GW transport layer address", zap.Error(err))
		m.ReleasePDN(ue, p)

		return
	}

	ambr := s1ap.UEAggregateMaximumBitRate{DL: s1ap.BitRate(qos.AMBRDL), UL: s1ap.BitRate(qos.AMBRUL)}

	reqMsg := &s1ap.ERABSetupRequest{
		MMEUES1APID:               ue.S1.MMEUES1APID,
		ENBUES1APID:               ue.S1.ENBUES1APID,
		UEAggregateMaximumBitRate: &ambr,
		ERABToBeSetup: []s1ap.ERABToBeSetupItemBearerSUReq{{
			ERABID: s1ap.ERABID(p.Ebi),
			QoS: s1ap.ERABLevelQoSParameters{
				QCI: s1ap.QCI(qos.QCI),
				ARP: s1ap.AllocationAndRetentionPriority{
					PriorityLevel:           qos.ARP,
					PreemptionCapability:    s1ap.PreemptionShallNotTrigger,
					PreemptionVulnerability: s1ap.PreemptionNotPreemptable,
				},
			},
			TransportLayerAddress: s1ap.TransportLayerAddress(sgwTLA),
			GTPTEID:               s1ap.GTPTEID(p.SgwFTEID.TEID),
			NASPDU:                s1ap.NASPDU(naspdu),
		}},
	}

	b, err := reqMsg.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal E-RAB Setup Request", zap.Error(err))
		m.ReleasePDN(ue, p)

		return
	}

	m.SendS1AP(ctx, ue, mme.S1APProcedureERABSetupRequest, b)
}

// handlePDNDisconnectRequest releases one of a UE's PDN connections at its request
// (TS 24.301 §6.5.2; TS 23.401 §5.10.3). The UE names the PDN by its default
// bearer's Linked EPS Bearer Identity. The last PDN connection cannot be
// disconnected this way — the UE detaches instead (ESM cause #49). The bearer is
// torn down with a DEACTIVATE EPS BEARER CONTEXT REQUEST; the session is released
// when the UE accepts.
func handlePDNDisconnectRequest(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte) {
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
		rejectPDNDisconnect(m, ctx, ue, pti, cause)

		return
	}

	p := m.LookupPDN(ue, req.LinkedEPSBearerIdentity)
	if p == nil {
		logger.MmeLog.Info("PDN disconnect rejected: unknown linked EPS bearer",
			zap.String("imsi", ue.IMSI()), zap.Uint8("linked-ebi", req.LinkedEPSBearerIdentity))
		rejectPDNDisconnect(m, ctx, ue, pti, esmCauseRequestRejectedUnspecified)

		return
	}

	numPDNs := ue.PDNCount()

	if numPDNs <= 1 {
		logger.MmeLog.Info("PDN disconnect rejected: last PDN connection",
			zap.String("imsi", ue.IMSI()), zap.Uint8("linked-ebi", req.LinkedEPSBearerIdentity))
		rejectPDNDisconnect(m, ctx, ue, pti, esmCauseLastPDNDisconnectNotAllowed)

		return
	}

	logger.MmeLog.Info("disconnecting PDN connection",
		zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn), zap.Uint8("ebi", p.Ebi))
	m.DisconnectBearer(ctx, ue, p, esmCauseRegularDeactivation, pti)
}

// rejectPDNDisconnect refuses a PDN DISCONNECT REQUEST with an ESM cause
// (TS 24.301 §6.5.2.4).
func rejectPDNDisconnect(m *mme.MME, ctx context.Context, ue *mme.UeContext, pti, cause uint8) {
	m.SendDownlinkProtected(ctx, ue, &eps.PDNDisconnectReject{
		ProcedureTransactionIdentity: pti,
		ESMCause:                     cause,
	})
}

// handleActivateDefaultBearerAccept confirms an additional PDN connection once the UE
// accepts the network's ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST (TS 24.301
// §6.4.1).
func handleActivateDefaultBearerAccept(m *mme.MME, ue *mme.UeContext, plain []byte) {
	accept, err := eps.ParseActivateDefaultEPSBearerContextAccept(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Activate Default EPS Bearer Context Accept", zap.Error(err))
		return
	}

	p := m.LookupPDN(ue, accept.EPSBearerIdentity)
	if p == nil {
		logger.MmeLog.Warn("Activate Default Accept for an unknown EPS bearer",
			zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", accept.EPSBearerIdentity))

		return
	}

	logger.MmeLog.Info("additional PDN connection active",
		zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn), zap.Uint8("ebi", p.Ebi))
}

// handleActivateDefaultBearerReject releases an additional PDN connection the UE
// refused (TS 24.301 §6.4.1.5).
func handleActivateDefaultBearerReject(m *mme.MME, ue *mme.UeContext, plain []byte) {
	reject, err := eps.ParseActivateDefaultEPSBearerContextReject(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Activate Default EPS Bearer Context Reject", zap.Error(err))
		return
	}

	if p := m.LookupPDN(ue, reject.EPSBearerIdentity); p != nil {
		logger.MmeLog.Info("UE rejected an additional PDN connection; releasing it",
			zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", p.Ebi), zap.Uint8("esm-cause", reject.ESMCause))
		m.ReleasePDN(ue, p)
	}
}

// rejectPDNConnectivity refuses a PDN CONNECTIVITY REQUEST with an ESM cause
// (TS 24.301 §6.5.1.4).
func rejectPDNConnectivity(m *mme.MME, ctx context.Context, ue *mme.UeContext, pti, cause uint8) {
	m.SendDownlinkProtected(ctx, ue, &eps.PDNConnectivityReject{
		ProcedureTransactionIdentity: pti,
		ESMCause:                     cause,
	})
}
