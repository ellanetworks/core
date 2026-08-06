// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// esmRequestHeaderCause validates the ESM header of a UE-requested message not
// associated with a bearer context — a PDN Connectivity or PDN Disconnect Request
// (TS 24.301 §7.3). The PTI must be assigned (1..254) and the header EPS bearer
// identity must be 0 ("no EPS bearer identity assigned"). It returns the reject
// cause #81 or #43, or 0 when the header is valid.
func esmRequestHeaderCause(pti, headerEBI uint8) eps.ESMCause {
	if pti == 0 || pti == 255 {
		return eps.ESMCauseInvalidPTIValue
	}

	if headerEBI != 0 {
		return eps.ESMCauseInvalidEPSBearerIdentity
	}

	return 0
}

// handlePDNConnectivityRequest opens an additional PDN connection (a second default
// bearer to the APN the UE names) for an already EMM-REGISTERED UE, authorised
// against the subscriber's profile (TS 24.301 §6.5.1; TS 23.401 §5.10.2).
func handlePDNConnectivityRequest(ctx context.Context, m *mme.MME, ue *mme.UeContext, req *eps.PDNConnectivityRequest) nasreply.Disposition {
	pti := req.PTI

	if cause := esmRequestHeaderCause(uint8(pti), uint8(req.EPSBearerIdentity)); cause != 0 {
		logger.From(ctx, logger.MmeLog).Info("PDN connectivity rejected: invalid ESM header",
			zap.String("imsi", ue.IMSI()), zap.Uint8("pti", uint8(pti)),
			zap.Uint8("header-ebi", uint8(req.EPSBearerIdentity)), zap.Stringer("esm-cause", cause))
		rejectPDNConnectivity(ctx, ue, uint8(pti), cause)

		return nasreply.Handled()
	}

	if ue.EMMState() != mme.EMMRegistered || !ue.Connected() {
		rejectPDNConnectivity(ctx, ue, uint8(pti), eps.ESMCauseRequestRejectedUnspecified)
		return nasreply.Handled()
	}

	apn := ""

	if req.AccessPointName != nil {
		apn = string(*req.AccessPointName)
	}

	if apn == "" {
		rejectPDNConnectivity(ctx, ue, uint8(pti), eps.ESMCauseMissingOrUnknownAPN)
		return nasreply.Handled()
	}

	if m.FindPDNByAPN(ue, apn) != nil {
		logger.From(ctx, logger.MmeLog).Info("PDN connectivity rejected: APN already connected",
			zap.String("imsi", ue.IMSI()), zap.String("apn", apn))
		rejectPDNConnectivity(ctx, ue, uint8(pti), eps.ESMCauseMultiplePDNNotAllowed)

		return nasreply.Handled()
	}

	qos, err := mme.ResolveQoSByAPN(ctx, m, ue.IMSI(), apn)
	if errors.Is(err, mme.ErrUnknownAPN) {
		logger.From(ctx, logger.MmeLog).Info("PDN connectivity rejected: APN not in subscriber profile",
			zap.String("imsi", ue.IMSI()), zap.String("apn", apn))
		rejectPDNConnectivity(ctx, ue, uint8(pti), eps.ESMCauseMissingOrUnknownAPN)

		return nasreply.Handled()
	}

	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to resolve QoS for additional PDN", zap.String("apn", apn), zap.Error(err))
		rejectPDNConnectivity(ctx, ue, uint8(pti), eps.ESMCauseRequestRejectedUnspecified)

		return nasreply.Handled()
	}

	p := m.AddPDN(ue)
	if p == nil {
		logger.From(ctx, logger.MmeLog).Info("PDN connectivity rejected: no free EPS bearer identity",
			zap.String("imsi", ue.IMSI()))
		rejectPDNConnectivity(ctx, ue, uint8(pti), eps.ESMCauseMaxEPSBearersReached)

		return nasreply.Handled()
	}

	bearer, err := m.Session.CreateEPSSession(ctx, models.EPSBearerRequest{
		IMSI:              ue.IMSI(),
		EPSBearerIdentity: p.Ebi,
		PDUSessionID:      requestedPDUSessionID(req),
		RequestType:       req.RequestType,
		Snssai:            &qos.Snssai,
		PolicyID:          qos.PolicyID,
		APN:               qos.APN,
		AMBRUplink:        qos.SessAmbrUL,
		AMBRDownlink:      qos.SessAmbrDL,
		IPv4Pool:          qos.IPv4Pool,
		IPv6Pool:          qos.IPv6Pool,
		DNS:               qos.DNS,
		MTU:               qos.MTU,
		RequestedPDNType:  uint8(req.PDNType),
	})
	if err != nil {
		logger.From(ctx, logger.MmeLog).Info("PDN connectivity rejected: session setup failed",
			zap.String("imsi", ue.IMSI()), zap.String("apn", apn), zap.Error(err))
		m.DropPDN(ue, p.Ebi)
		rejectPDNConnectivity(ctx, ue, uint8(pti), sessionSetupESMCause(bearer))

		return nasreply.Handled()
	}

	m.FillBearer(ue, p, qos, bearer)

	operator, err := m.Operator(ctx)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to read operator configuration", zap.Error(err))
		m.ReleasePDN(ctx, ue, p)

		return nasreply.Handled()
	}

	esm, err := buildActivateDefaultESM(p, qos, uint8(pti), operator.PLMN())
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to build Activate Default EPS Bearer Context Request", zap.Error(err))
		m.ReleasePDN(ctx, ue, p)

		return nasreply.Handled()
	}

	naspdu, err := ue.ProtectDownlink(esm, eps.SHTIntegrityProtectedCiphered)
	if err != nil {
		mme.ReportProtectFailure(ctx, ue.Conn(), "Activate Default EPS Bearer Context Request", err)
		m.ReleasePDN(ctx, ue, p)

		return nasreply.Handled()
	}

	logger.From(ctx, logger.MmeLog).Info("opening additional PDN connection",
		zap.String("imsi", ue.IMSI()), zap.String("apn", apn), zap.Uint8("ebi", p.Ebi))
	sendERABSetup(ctx, m, ue, p, qos, naspdu)

	return nasreply.Handled()
}

// sendERABSetup asks the eNB to set up the radio leg of a new PDN connection,
// carrying the ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST to the UE (TS 36.413
// §8.2.1).
func sendERABSetup(ctx context.Context, m *mme.MME, ue *mme.UeContext, p *mme.PdnConnection, qos *mme.EpsQoS, naspdu []byte) {
	sgwTLA, err := models.EncodeTransportLayerAddress(p.SgwFTEID.Addr, p.SgwN3IPv6)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to encode S-GW transport layer address", zap.Error(err))
		m.ReleasePDN(ctx, ue, p)

		return
	}

	ambr := s1ap.UEAggregateMaximumBitRate{DL: s1ap.BitRate(qos.AMBRDL.Bps()), UL: s1ap.BitRate(qos.AMBRUL.Bps())}

	reqMsg := &s1ap.ERABSetupRequest{
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

	if err := ue.Conn().SendERABSetup(ctx, reqMsg); err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to send E-RAB Setup Request", zap.Error(err))
		m.ReleasePDN(ctx, ue, p)

		return
	}

	// The S1AP send chokepoint swallows write failures, so without T3485 supervision a
	// failed or unanswered activation would leak the committed PDN and its S-GW session.
	// The guard retransmits, then releases the PDN once the budget is spent
	// (TS 24.301 §6.4.1.6).
	m.ArmESMGuardAbortOnly(ue, p, "Activate Default EPS Bearer Context Request", naspdu, func() {
		m.ReleasePDN(context.Background(), ue, p)
	})
}

// handlePDNDisconnectRequest releases one of a UE's PDN connections at its request
// (TS 24.301 §6.5.2; TS 23.401 §5.10.3). The UE names the PDN by its default
// bearer's Linked EPS Bearer Identity. The last PDN connection cannot be
// disconnected this way — the UE detaches instead (ESM cause #49).
func handlePDNDisconnectRequest(ctx context.Context, m *mme.MME, ue *mme.UeContext, req *eps.PDNDisconnectRequest) nasreply.Disposition {
	pti := req.PTI

	if cause := esmRequestHeaderCause(uint8(pti), uint8(req.EPSBearerIdentity)); cause != 0 {
		logger.From(ctx, logger.MmeLog).Info("PDN disconnect rejected: invalid ESM header",
			zap.String("imsi", ue.IMSI()), zap.Uint8("pti", uint8(pti)),
			zap.Uint8("header-ebi", uint8(req.EPSBearerIdentity)), zap.Stringer("esm-cause", cause))
		rejectPDNDisconnect(ctx, ue, uint8(pti), cause)

		return nasreply.Handled()
	}

	p := m.LookupPDN(ue, uint8(req.LinkedEPSBearerIdentity))
	if p == nil {
		logger.From(ctx, logger.MmeLog).Info("PDN disconnect rejected: unknown linked EPS bearer",
			zap.String("imsi", ue.IMSI()), zap.Uint8("linked-ebi", uint8(req.LinkedEPSBearerIdentity)))
		rejectPDNDisconnect(ctx, ue, uint8(pti), eps.ESMCauseRequestRejectedUnspecified)

		return nasreply.Handled()
	}

	numPDNs := ue.PDNCount()

	if numPDNs <= 1 {
		logger.From(ctx, logger.MmeLog).Info("PDN disconnect rejected: last PDN connection",
			zap.String("imsi", ue.IMSI()), zap.Uint8("linked-ebi", uint8(req.LinkedEPSBearerIdentity)))
		rejectPDNDisconnect(ctx, ue, uint8(pti), eps.ESMCauseLastPDNDisconnectionNotAllow)

		return nasreply.Handled()
	}

	logger.From(ctx, logger.MmeLog).Info("disconnecting PDN connection",
		zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn), zap.Uint8("ebi", p.Ebi))
	m.DisconnectBearer(ctx, ue, p, eps.ESMCauseRegularDeactivation, uint8(pti))

	return nasreply.Handled()
}

// rejectPDNDisconnect refuses a PDN DISCONNECT REQUEST with an ESM cause
// (TS 24.301 §6.5.2.4).
func rejectPDNDisconnect(ctx context.Context, ue *mme.UeContext, pti uint8, cause eps.ESMCause) {
	ue.Conn().SendDownlinkProtected(ctx, &eps.PDNDisconnectReject{
		PTI:   nas.ProcedureTransactionIdentity(pti),
		Cause: cause,
	})
}

// handleActivateDefaultBearerAccept confirms an additional PDN connection once the UE
// accepts the network's ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST (TS 24.301
// §6.4.1).
func handleActivateDefaultBearerAccept(m *mme.MME, ue *mme.UeContext, accept *eps.ActivateDefaultEPSBearerContextAccept) nasreply.Disposition {
	p := m.LookupPDN(ue, uint8(accept.EPSBearerIdentity))
	if p == nil {
		logger.MmeLog.Warn("Activate Default Accept for an unknown EPS bearer",
			zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", uint8(accept.EPSBearerIdentity)))

		return nasreply.Silent(nasreply.ReasonNoContext)
	}

	m.StopESMGuard(p)

	logger.MmeLog.Info("additional PDN connection active",
		zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn), zap.Uint8("ebi", p.Ebi))

	return nasreply.Handled()
}

// handleActivateDefaultBearerReject releases an additional PDN connection the UE
// refused (TS 24.301 §6.4.1.5).
func handleActivateDefaultBearerReject(ctx context.Context, m *mme.MME, ue *mme.UeContext, rej *eps.ActivateDefaultEPSBearerContextReject) nasreply.Disposition {
	if p := m.LookupPDN(ue, uint8(rej.EPSBearerIdentity)); p != nil {
		logger.MmeLog.Info("UE rejected an additional PDN connection; releasing it",
			zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", p.Ebi), zap.Stringer("esm-cause", rej.Cause))
		m.StopESMGuard(p)
		m.ReleasePDN(ctx, ue, p)
	}

	return nasreply.Handled()
}

// The anchor names a cause only when the UE can act on it, such as #54 for a
// transfer of a PDN connection it does not hold (TS 24.301 §6.5.1.6 b).
func sessionSetupESMCause(bearer models.EPSBearer) eps.ESMCause {
	if bearer.ESMCause != 0 {
		return bearer.ESMCause
	}

	return eps.ESMCauseRequestRejectedUnspecified
}

// rejectPDNConnectivity refuses a PDN CONNECTIVITY REQUEST with an ESM cause
// (TS 24.301 §6.5.1.4).
func rejectPDNConnectivity(ctx context.Context, ue *mme.UeContext, pti uint8, cause eps.ESMCause) {
	ue.Conn().SendDownlinkProtected(ctx, &eps.PDNConnectivityReject{
		PTI:   nas.ProcedureTransactionIdentity(pti),
		Cause: cause,
	})
}
