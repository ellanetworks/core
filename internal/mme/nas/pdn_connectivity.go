// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"errors"
	"fmt"

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
func handlePDNConnectivityRequest(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, req *eps.PDNConnectivityRequest) nasreply.Disposition {
	pti := req.PTI

	if cause := esmRequestHeaderCause(uint8(pti), uint8(req.EPSBearerIdentity)); cause != 0 {
		logger.From(ctx, logger.MmeLog).Info("PDN connectivity rejected: invalid ESM header",
			zap.String("imsi", ue.IMSI()), zap.Uint8("pti", uint8(pti)),
			zap.Uint8("header-ebi", uint8(req.EPSBearerIdentity)), zap.Stringer("esm-cause", cause))
		rejectPDNConnectivity(ctx, ueConn, uint8(pti), cause)

		return nasreply.Handled()
	}

	if ue.EMMState() != mme.EMMRegistered || !ue.Connected() {
		rejectPDNConnectivity(ctx, ueConn, uint8(pti), eps.ESMCauseRequestRejectedUnspecified)
		return nasreply.Handled()
	}

	apn := ""

	if req.AccessPointName != nil {
		apn = string(*req.AccessPointName)
	}

	if cause, refused := requestTypeRefusal(req.RequestType); refused {
		logger.From(ctx, logger.MmeLog).Info("PDN connectivity rejected: request type not served",
			zap.String("imsi", ue.IMSI()), zap.Stringer("request-type", req.RequestType))
		rejectPDNConnectivity(ctx, ueConn, uint8(pti), cause)

		return nasreply.Handled()
	}

	ue.RequestedPDUSessionID = pduSessionIDFromPCOs(req.ProtocolConfigurationOptions, req.ExtendedProtocolConfigurationOptions)
	ue.RequestedProtocolOpts, _ = protocolOptionsFromPCOs(req.ProtocolConfigurationOptions, req.ExtendedProtocolConfigurationOptions)
	ue.RequestedType = req.RequestType

	if req.ESMInformationTransferFlag != nil && *req.ESMInformationTransferFlag {
		ue.RequestedAPN = apn
		ue.AwaitESMInformation(uint8(pti), &mme.PendingPDNConnectivity{
			PTI:     uint8(pti),
			PDNType: uint8(req.PDNType),
		})

		// T3489's final expiry outlives this request's context.
		requestESMInformation(ctx, ue, ueConn, func(abortedPTI uint8) {
			rejectPDNConnectivity(context.Background(), ueConn, abortedPTI, eps.ESMCauseESMInformationNotReceived)
		})

		return nasreply.Handled()
	}

	if apn == "" {
		rejectPDNConnectivity(ctx, ueConn, uint8(pti), eps.ESMCauseMissingOrUnknownAPN)
		return nasreply.Handled()
	}

	return openPDNConnection(ctx, m, ue, ueConn, apn, uint8(pti), req.PDNType)
}

func openPDNConnection(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, apn string, ptiValue uint8, pdnType eps.PDNType) nasreply.Disposition {
	pti := nas.ProcedureTransactionIdentity(ptiValue)

	// Re-checked: a resumed request arrives after a detach or S1 release may have
	// intervened.
	if ue.EMMState() != mme.EMMRegistered || !ue.Connected() {
		rejectPDNConnectivity(ctx, ueConn, ptiValue, eps.ESMCauseRequestRejectedUnspecified)
		return nasreply.Handled()
	}

	if apn == "" {
		rejectPDNConnectivity(ctx, ueConn, ptiValue, eps.ESMCauseMissingOrUnknownAPN)
		return nasreply.Handled()
	}

	if m.FindPDNByAPN(ue, apn) != nil {
		logger.From(ctx, logger.MmeLog).Info("PDN connectivity rejected: APN already connected",
			zap.String("imsi", ue.IMSI()), zap.String("apn", apn))
		rejectPDNConnectivity(ctx, ueConn, uint8(pti), eps.ESMCauseMultiplePDNNotAllowed)

		return nasreply.Handled()
	}

	qos, err := mme.ResolveQoSByAPN(ctx, m, ue.IMSI(), apn)
	if errors.Is(err, mme.ErrUnknownAPN) {
		logger.From(ctx, logger.MmeLog).Info("PDN connectivity rejected: APN not in subscriber profile",
			zap.String("imsi", ue.IMSI()), zap.String("apn", apn))
		rejectPDNConnectivity(ctx, ueConn, uint8(pti), eps.ESMCauseMissingOrUnknownAPN)

		return nasreply.Handled()
	}

	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to resolve QoS for additional PDN", zap.String("apn", apn), zap.Error(err))
		rejectPDNConnectivity(ctx, ueConn, uint8(pti), eps.ESMCauseRequestRejectedUnspecified)

		return nasreply.Handled()
	}

	p := m.AddPDN(ue)
	if p == nil {
		logger.From(ctx, logger.MmeLog).Info("PDN connectivity rejected: no free EPS bearer identity",
			zap.String("imsi", ue.IMSI()))
		rejectPDNConnectivity(ctx, ueConn, uint8(pti), eps.ESMCauseMaxEPSBearersReached)

		return nasreply.Handled()
	}

	bearer, err := m.Session.CreateEPSSession(ctx, models.EPSBearerRequest{
		IMSI:              ue.IMSI(),
		EPSBearerIdentity: p.Ebi,
		PDUSessionID:      ue.RequestedPDUSessionID,
		Snssai:            qos.Snssai,
		PolicyID:          qos.PolicyID,
		APN:               qos.APN,
		AMBRUplink:        qos.SessAmbrUL,
		AMBRDownlink:      qos.SessAmbrDL,
		IPv4Pool:          qos.IPv4Pool,
		IPv6Pool:          qos.IPv6Pool,
		DNS:               qos.DNS,
		MTU:               qos.MTU,
		RequestedPDNType:  uint8(pdnType),
		RequestType:       ue.RequestedType,
	})
	if err != nil {
		logger.From(ctx, logger.MmeLog).Info("PDN connectivity rejected: session setup failed",
			zap.String("imsi", ue.IMSI()), zap.String("apn", apn), zap.Error(err))
		m.DropPDN(ue, p.Ebi)
		rejectPDNConnectivity(ctx, ueConn, uint8(pti), attachBearerRejectCause(ue.RequestedType, err))

		return nasreply.Handled()
	}

	p = m.FillBearer(ue, p, qos, bearer)

	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to resolve the serving PLMN", zap.Error(err))
		m.ReleasePDN(ctx, ue, p)

		return nasreply.Handled()
	}

	esm, err := buildActivateDefaultESM(p, qos, uint8(pti), plmn, ue.UsesEPCO(p), ue.RequestedProtocolOpts)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to build Activate Default EPS Bearer Context Request", zap.Error(err))
		m.ReleasePDN(ctx, ue, p)

		return nasreply.Handled()
	}

	req, err := buildERABSetup(p, qos)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to build E-RAB Setup Request", zap.Error(err))
		m.ReleasePDN(ctx, ue, p)

		return nasreply.Handled()
	}

	logger.From(ctx, logger.MmeLog).Info("opening additional PDN connection",
		zap.String("imsi", ue.IMSI()), zap.String("apn", apn), zap.Uint8("ebi", p.Ebi))

	var writeErr error

	if err := ueConn.SendProtected(esm, eps.SHTIntegrityProtectedCiphered, func(wire []byte) error {
		req.ERABToBeSetup[0].NASPDU = s1ap.NASPDU(wire)
		writeErr = ueConn.SendERABSetup(ctx, req)

		return writeErr
	}); err != nil {
		if writeErr == nil {
			mme.ReportProtectFailure(ctx, ueConn, "Activate Default EPS Bearer Context Request", err)
		} else {
			logger.From(ctx, logger.MmeLog).Error("failed to send E-RAB Setup Request", zap.Error(writeErr))
		}

		m.ReleasePDN(ctx, ue, p)

		return nasreply.Handled()
	}

	m.ArmESMGuardAbortOnly(ue, p, "Activate Default EPS Bearer Context Request", esm, eps.SHTIntegrityProtectedCiphered, func() {
		m.ReleasePDN(context.Background(), ue, p)
	})

	return nasreply.Handled()
}

func resumePDNConnectivity(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, pending *mme.PendingPDNConnectivity) {
	openPDNConnection(ctx, m, ue, ueConn, ue.RequestedAPN, pending.PTI, eps.PDNType(pending.PDNType))
}

func buildERABSetup(p *mme.PdnConnection, qos *mme.EpsQoS) (*s1ap.ERABSetupRequest, error) {
	sgwTLA, err := models.EncodeTransportLayerAddress(p.SgwFTEID.Addr, p.SgwN3IPv6)
	if err != nil {
		return nil, fmt.Errorf("failed to encode S-GW transport layer address: %w", err)
	}

	ambr := s1ap.UEAggregateMaximumBitRate{DL: s1ap.BitRate(qos.AMBRDL.Bps()), UL: s1ap.BitRate(qos.AMBRUL.Bps())}

	return &s1ap.ERABSetupRequest{
		UEAggregateMaximumBitRate: &ambr,
		ERABToBeSetup: []s1ap.ERABToBeSetupItemBearerSUReq{{
			ERABID: s1ap.ERABID(p.Ebi),
			QoS: s1ap.ERABLevelQoSParameters{
				QCI: s1ap.QCI(qos.QCI),
				ARP: mme.BearerARP(qos.ARP),
			},
			TransportLayerAddress: s1ap.TransportLayerAddress(sgwTLA),
			GTPTEID:               s1ap.GTPTEID(p.SgwFTEID.TEID),
		}},
	}, nil
}

// handlePDNDisconnectRequest releases one of a UE's PDN connections at its request
// (TS 24.301 §6.5.2; TS 23.401 §5.10.3). The UE names the PDN by its default
// bearer's Linked EPS Bearer Identity. The last PDN connection cannot be
// disconnected this way — the UE detaches instead (ESM cause #49).
func handlePDNDisconnectRequest(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, req *eps.PDNDisconnectRequest) nasreply.Disposition {
	pti := req.PTI

	if cause := esmRequestHeaderCause(uint8(pti), uint8(req.EPSBearerIdentity)); cause != 0 {
		logger.From(ctx, logger.MmeLog).Info("PDN disconnect rejected: invalid ESM header",
			zap.String("imsi", ue.IMSI()), zap.Uint8("pti", uint8(pti)),
			zap.Uint8("header-ebi", uint8(req.EPSBearerIdentity)), zap.Stringer("esm-cause", cause))
		rejectPDNDisconnect(ctx, ueConn, uint8(pti), cause)

		return nasreply.Handled()
	}

	p := m.LookupPDN(ue, uint8(req.LinkedEPSBearerIdentity))
	if p == nil {
		logger.From(ctx, logger.MmeLog).Info("PDN disconnect rejected: unknown linked EPS bearer",
			zap.String("imsi", ue.IMSI()), zap.Uint8("linked-ebi", uint8(req.LinkedEPSBearerIdentity)))
		rejectPDNDisconnect(ctx, ueConn, uint8(pti), eps.ESMCauseRequestRejectedUnspecified)

		return nasreply.Handled()
	}

	numPDNs := ue.PDNCount()

	if numPDNs <= 1 {
		logger.From(ctx, logger.MmeLog).Info("PDN disconnect rejected: last PDN connection",
			zap.String("imsi", ue.IMSI()), zap.Uint8("linked-ebi", uint8(req.LinkedEPSBearerIdentity)))
		rejectPDNDisconnect(ctx, ueConn, uint8(pti), eps.ESMCauseLastPDNDisconnectionNotAllow)

		return nasreply.Handled()
	}

	logger.From(ctx, logger.MmeLog).Info("disconnecting PDN connection",
		zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn), zap.Uint8("ebi", p.Ebi))
	m.DisconnectBearer(ctx, ue, p, eps.ESMCauseRegularDeactivation, uint8(pti))

	return nasreply.Handled()
}

// rejectPDNDisconnect refuses a PDN DISCONNECT REQUEST with an ESM cause
// (TS 24.301 §6.5.2.4).
func rejectPDNDisconnect(ctx context.Context, ueConn *mme.UeConn, pti uint8, cause eps.ESMCause) {
	ueConn.SendDownlinkProtected(ctx, &eps.PDNDisconnectReject{
		PTI:   nas.ProcedureTransactionIdentity(pti),
		Cause: cause,
	})
}

// handleActivateDefaultBearerAccept confirms a PDN connection once the UE
// accepts the network's ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST (TS 24.301
// §6.4.1).
func handleActivateDefaultBearerAccept(ctx context.Context, m *mme.MME, ue *mme.UeContext, accept *eps.ActivateDefaultEPSBearerContextAccept) nasreply.Disposition {
	p := m.LookupPDN(ue, uint8(accept.EPSBearerIdentity))
	if p == nil {
		logger.From(ctx, logger.MmeLog).Warn("Activate Default Accept for an unknown EPS bearer",
			zap.String("imsi", ue.IMSI()), zap.Uint8("ebi", uint8(accept.EPSBearerIdentity)))

		return nasreply.Silent(nasreply.ReasonNoContext)
	}

	m.StopESMGuard(p)

	if cause, ok := fiveGSMCauseFromPCOs(accept.ProtocolConfigurationOptions, accept.ExtendedProtocolConfigurationOptions); ok {
		logger.From(ctx, logger.MmeLog).Warn("UE discarded the mapped 5GS QoS parameters of the PDN connection",
			zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn), zap.Uint8("5gsm-cause", cause))
	}

	logger.From(ctx, logger.MmeLog).Info("PDN connection active",
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

// rejectPDNConnectivity refuses a PDN CONNECTIVITY REQUEST with an ESM cause
// (TS 24.301 §6.5.1.4).
func rejectPDNConnectivity(ctx context.Context, ueConn *mme.UeConn, pti uint8, cause eps.ESMCause) {
	ueConn.SendDownlinkProtected(ctx, &eps.PDNConnectivityReject{
		PTI:   nas.ProcedureTransactionIdentity(pti),
		Cause: cause,
	})
}
