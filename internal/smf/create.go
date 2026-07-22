// SPDX-FileCopyrightText: Ella Networks Inc.
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/models"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func nasToNgapPDUSessionType(nasType uint8) aper.Enumerated {
	switch nasType {
	case fgs.PDUSessionTypeIPv6:
		return ngapType.PDUSessionTypePresentIpv6
	case fgs.PDUSessionTypeIPv4IPv6:
		return ngapType.PDUSessionTypePresentIpv4v6
	default:
		return ngapType.PDUSessionTypePresentIpv4
	}
}

// CreateSmContext establishes a new 5G PDU session from the UE's NAS
// establishment request, returning the SM context ref or a NAS reject message.
func (s *SMF) CreateSmContext(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, dnn string, snssai *models.Snssai, n1Msg []byte) (string, []byte, error) {
	ctx, span := tracer.Start(ctx, "smf/create_session",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("ue.supi", supi.String()),
			attribute.Int("smf.pdu_session_id", int(pduSessionID)),
			attribute.String("smf.dnn", dnn),
		),
	)
	defer span.End()

	// Decode before any state changes so a failure can still build a reject. A
	// bad discriminator/length peeks as a decode error; a well-formed message of
	// the wrong type is a protocol-state mismatch.
	msgType, err := fgs.PeekGSMMessageType(n1Msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to decode NAS message")

		rsp, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(pduSessionID, 0, fgs.GSMCauseProtocolErrorUnspecified)
		if buildErr != nil {
			return "", nil, fmt.Errorf("error decoding NAS message: %v (build reject failed: %v)", err, buildErr)
		}

		return "", rsp, fmt.Errorf("error decoding NAS message: %v", err)
	}

	if msgType != fgs.MsgPDUSessionEstablishmentRequest {
		rsp, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(pduSessionID, 0, fgs.GSMCauseMessageTypeNotCompatibleWithTheProtocolState)
		if buildErr != nil {
			return "", nil, fmt.Errorf("unexpected NAS message type %d (build reject failed: %v)", msgType, buildErr)
		}

		return "", rsp, fmt.Errorf("unexpected NAS message type: %d", msgType)
	}

	req, err := fgs.ParsePDUSessionEstablishmentRequest(n1Msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to decode NAS message")

		rsp, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(pduSessionID, 0, fgs.GSMCauseProtocolErrorUnspecified)
		if buildErr != nil {
			return "", nil, fmt.Errorf("error decoding NAS message: %v (build reject failed: %v)", err, buildErr)
		}

		return "", rsp, fmt.Errorf("error decoding NAS message: %v", err)
	}

	// Police the PTI before allocating any state (TS 24.501 §7.3.1): an
	// unassigned PTI yields a 5GSM STATUS (#81); a reserved PTI is ignored —
	// no context and no response.
	reqPTI := req.PTI

	switch verdict, cause := smfNas.PolicePTI(fgs.MsgPDUSessionEstablishmentRequest, reqPTI, func(uint8) bool { return false }); verdict {
	case smfNas.PTIIgnore:
		return "", nil, nil
	case smfNas.PTIRespondStatus:
		rsp, buildErr := smfNas.BuildGSM5GSMStatus(pduSessionID, reqPTI, cause)
		if buildErr != nil {
			return "", nil, fmt.Errorf("build 5GSM STATUS failed: %v", buildErr)
		}

		return "", rsp, nil
	}

	// Record exactly one establishment outcome per attempt; the returns above are
	// not establishment attempts, so they precede this defer.
	var establishmentResult string

	defer func() { recordSessionEstablishmentResult(metrics.RAT5G, establishmentResult) }()

	if existing := s.currentSession(supi, pduSessionID); existing != nil {
		s.handlePduSessionContextReplacement(ctx, existing)
	}

	policy, err := s.GetSessionPolicy(ctx, supi, snssai, dnn)
	if err != nil {
		establishmentResult = metrics.ResultReject

		rsp, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(pduSessionID, reqPTI, establishmentRejectCause(err))
		if buildErr != nil {
			return "", nil, fmt.Errorf("failed to find subscriber policy: %v (build reject failed: %v)", err, buildErr)
		}

		return "", rsp, fmt.Errorf("failed to find subscriber policy: %v", err)
	}

	requestedType := fgs.PDUSessionTypeIPv4
	if req.PDUSessionType != nil {
		requestedType = *req.PDUSessionType
	}

	negotiatedType, err := s.negotiatePDUSessionType(ctx, requestedType, policy)
	if err != nil {
		establishmentResult = metrics.ResultReject

		rsp, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(pduSessionID, reqPTI, pduSessionTypeRejectCause(requestedType, policy))
		if buildErr != nil {
			return "", nil, fmt.Errorf("PDU session type negotiation failed: %v (build reject failed: %v)", err, buildErr)
		}

		return "", rsp, fmt.Errorf("PDU session type negotiation failed: %v", err)
	}

	pco, err := parsePDUSessionRequest(req)
	if err != nil {
		establishmentResult = metrics.ResultReject

		rsp, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(pduSessionID, reqPTI, fgs.GSMCauseRequestRejectedUnspecified)
		if buildErr != nil {
			return "", nil, fmt.Errorf("parse PDU session request failed: %v (build reject failed: %v)", err, buildErr)
		}

		return "", rsp, fmt.Errorf("parse PDU session request failed: %v", err)
	}

	sc, _, err := s.establishSession(ctx, SessionRequest{
		Supi:    supi,
		Key:     pduSessionID,
		Dnn:     dnn,
		Snssai:  snssai,
		Access:  Access5G,
		PDUType: negotiatedType,
		Policy:  policy,
	})
	if err != nil {
		establishmentResult = metrics.ResultReject

		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create SM context")

		cause := fgs.GSMCauseRequestRejectedUnspecified
		if errors.Is(err, errUEAddressAllocation) {
			cause = fgs.GSMCauseInsufficientResources
		}

		rsp, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(pduSessionID, reqPTI, cause)
		if buildErr != nil {
			return "", nil, fmt.Errorf("failed to create SM Context: %v (build reject failed: %v)", err, buildErr)
		}

		return "", rsp, fmt.Errorf("failed to create SM Context: %v", err)
	}

	// IPv4v6 narrowed to a single family is signalled in the accept with 5GSM
	// cause #50/#51 (TS 24.501 §6.4.1.3).
	var cause uint8

	switch narrowPDUType(requestedType, sc.PDUSessionType) {
	case narrowIPv4Only:
		cause = fgs.GSMCausePDUSessionTypeIPv4OnlyAllowed
	case narrowIPv6Only:
		cause = fgs.GSMCausePDUSessionTypeIPv6OnlyAllowed
	}

	addrs := &smfNas.PDUSessionAddresses{
		PDUSessionType: sc.PDUSessionType,
		IPv4Address:    sc.PDUIPV4Address,
		IPv6IID:        sc.IPv6IID,
	}

	// The PFCP session is up, so the establishment counts as an accept even if
	// the N1N2 delivery below fails.
	establishmentResult = metrics.ResultAccept

	alwaysOnRequested := req.AlwaysOnRequested

	if err := s.sendPduSessionEstablishmentAccept(ctx, sc, policy, pco, addrs, reqPTI, cause, alwaysOnIndication(alwaysOnRequested)); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to send PDU session establishment accept")

		s.abortSession(ctx, sc)

		return "", nil, fmt.Errorf("failed to send pdu session establishment accept n1 message: %v", err)
	}

	return sc.Ref, nil, nil
}

func (s *SMF) handlePduSessionContextReplacement(ctx context.Context, smCtxt *SMContext) {
	smCtxt.Mutex.Lock()
	defer smCtxt.Mutex.Unlock()

	// Stop the superseded context's outstanding procedure retransmission.
	smCtxt.stopProcedureTimer()

	s.RemoveSession(ctx, smCtxt.Ref)

	if smCtxt.Tunnel != nil {
		err := s.releaseTunnel(ctx, smCtxt)
		if err != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("release tunnel failed", zap.Error(err), logger.SUPI(smCtxt.Supi.String()), logger.PDUSessionID(smCtxt.PDUSessionID))
		}
	}
}

// establishmentRejectCause maps a session-policy lookup failure to the 5GSM
// cause of the PDU Session Establishment Reject (TS 24.501 §9.11.4.2): #70 when
// the slice is served but not the DNN, #27 when the DNN is unknown, and the
// generic #31 otherwise.
func establishmentRejectCause(err error) uint8 {
	switch {
	case errors.Is(err, ErrDNNNotInSlice):
		return fgs.GSMCauseMissingOrUnknownDNNInASlice
	case errors.Is(err, ErrDNNNotFound):
		return fgs.GSMCauseMissingOrUnknownDNN
	default:
		return fgs.GSMCauseRequestRejectedUnspecified
	}
}

func parsePDUSessionRequest(req *fgs.PDUSessionEstablishmentRequest) (*smfNas.ProtocolConfigurationOptions, error) {
	if req.PDUSessionType != nil {
		t := *req.PDUSessionType
		if t != fgs.PDUSessionTypeIPv4 && t != fgs.PDUSessionTypeIPv6 && t != fgs.PDUSessionTypeIPv4IPv6 {
			return nil, fmt.Errorf("requested PDUSessionType is invalid: %d", t)
		}
	}

	pco := &smfNas.ProtocolConfigurationOptions{}

	if req.ExtendedPCO != nil {
		ids, err := fgs.ParsePCOContainerIDs(req.ExtendedPCO)
		if err != nil {
			return nil, fmt.Errorf("parsing PCO failed: %v", err)
		}

		pco.IPv4LinkMTURequest = true

		for _, id := range ids {
			switch id {
			case fgs.PCOContainerDNSServerIPv6Address:
				pco.DNSIPv6Request = true
			case fgs.PCOContainerDNSServerIPv4Address:
				pco.DNSIPv4Request = true
			}
		}
	}

	return pco, nil
}

// alwaysOnIndication resolves the Always-on PDU session indication for an
// Establishment Accept (TS 24.501 §6.4.1): "not allowed" (APSI 0) when the UE
// requested an always-on session, or omitted (nil) otherwise. The "required"
// value (§6.4.1 a) is not produced because no PDU session is established as
// always-on.
func alwaysOnIndication(requested bool) *uint8 {
	if requested {
		v := uint8(0) // "Always-on PDU session not allowed"
		return &v
	}

	return nil
}

func (s *SMF) sendPduSessionEstablishmentAccept(
	ctx context.Context,
	smContext *SMContext,
	policy *Policy,
	pco *smfNas.ProtocolConfigurationOptions,
	addrs *smfNas.PDUSessionAddresses,
	pti uint8,
	cause uint8,
	alwaysOn *uint8,
) error {
	ctx, span := tracer.Start(ctx, "smf/send_pdu_session_establishment_accept",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	smContext.Mutex.Lock()
	smContext.establishmentPTI = pti
	smContext.Mutex.Unlock()

	n1Msg, err := smfNas.BuildGSMPDUSessionEstablishmentAccept(&policy.Ambr, &policy.QosData, smContext.PDUSessionID, pti, smContext.Snssai, smContext.Dnn, pco, policy.DNS, policy.MTU, cause, addrs, alwaysOn)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to build PDU session establishment accept")

		return fmt.Errorf("build GSM PDUSessionEstablishmentAccept failed: %v", err)
	}

	ngapPDUType := nasToNgapPDUSessionType(smContext.PDUSessionType)

	n2Msg, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(&policy.Ambr, &policy.QosData, smContext.Tunnel.DataPath.UpLinkTunnel.TEID, smContext.Tunnel.DataPath.UpLinkTunnel.N3IPv4, smContext.Tunnel.DataPath.UpLinkTunnel.N3IPv6, ngapPDUType)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to build PDU session resource setup request transfer")

		return fmt.Errorf("build PDUSessionResourceSetupRequestTransfer failed: %v", err)
	}

	smContext.SetPolicyData(policy)

	err = s.amf.TransferN1N2(ctx, smContext.Supi, smContext.PDUSessionID, smContext.Snssai, n1Msg, n2Msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to transfer N1N2 message")

		return fmt.Errorf("failed to send n1 n2 transfer request: %v", err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Debug("Sent n1 n2 transfer request", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	return nil
}
