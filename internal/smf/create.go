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
	"github.com/ellanetworks/core/internal/smf/procedure"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	libngap "github.com/ellanetworks/core/ngap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// establishmentReject answers the UE's establishment request with a 5GSM reject
// carrying cause, and records the attempt as refused.
type establishmentReject func(cause fgs.GSMCause, err error) ([]byte, error)

func requestedPDUSessionType(req *fgs.PDUSessionEstablishmentRequest) fgs.PDUSessionType {
	if req.PDUSessionType != nil {
		return *req.PDUSessionType
	}

	return fgs.PDUSessionTypeIPv4
}

func nasToNgapPDUSessionType(nasType uint8) libngap.PDUSessionType {
	switch nasType {
	case uint8(fgs.PDUSessionTypeIPv6):
		return libngap.PDUSessionTypeIPv6
	case uint8(fgs.PDUSessionTypeIPv4v6):
		return libngap.PDUSessionTypeIPv4v6
	default:
		return libngap.PDUSessionTypeIPv4
	}
}

// parseEstablishmentRequest decodes and polices the UE's message before any
// state is allocated, so a refusal can still answer it. A nil request means the
// message was answered — or, for a reserved PTI, deliberately not — without an
// establishment being attempted.
func parseEstablishmentRequest(pduSessionID uint8, n1Msg []byte) (*fgs.PDUSessionEstablishmentRequest, []byte, error) {
	// A message that will not decode draws a protocol error; a well-formed message
	// of the wrong type is a protocol-state mismatch.
	msg, err := fgs.ParseMessage(n1Msg)
	if err != nil && !nas.SoftOnly(err) {
		rsp, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(fgs.PDUSessionID(pduSessionID), 0, fgs.GSMCauseProtocolErrorUnspecified)
		if buildErr != nil {
			return nil, nil, fmt.Errorf("error decoding NAS message: %v (build reject failed: %v)", err, buildErr)
		}

		return nil, rsp, fmt.Errorf("error decoding NAS message: %v", err)
	}

	// A message type this codec does not model draws a 5GSM STATUS naming that,
	// not a reject of a procedure the UE never started: TS 24.501 §7.4 has the
	// network ignore such a message except to return a STATUS with cause #97,
	// where #98 reports a message the receiver does understand arriving in the
	// wrong state.
	if unknown, isUnknown := msg.(*fgs.UnknownGSMMessage); isUnknown {
		rsp, buildErr := smfNas.BuildGSM5GSMStatus(unknown.PDUSessionID, unknown.PTI,
			fgs.GSMCauseMessageTypeNonExistentOrNotImplemented)
		if buildErr != nil {
			return nil, nil, fmt.Errorf("unimplemented 5GSM message type %#02x (build 5GSM STATUS failed: %v)", uint8(unknown.Type), buildErr)
		}

		return nil, rsp, fmt.Errorf("unimplemented 5GSM message type %#02x", uint8(unknown.Type))
	}

	req, ok := msg.(*fgs.PDUSessionEstablishmentRequest)
	if !ok {
		rsp, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(fgs.PDUSessionID(pduSessionID), 0, fgs.GSMCauseMessageTypeNotCompatibleWithTheProtocolState)
		if buildErr != nil {
			return nil, nil, fmt.Errorf("unexpected NAS message %T (build reject failed: %v)", msg, buildErr)
		}

		return nil, rsp, fmt.Errorf("unexpected NAS message: %T", msg)
	}

	// TS 24.501 §7.3.1: an unassigned PTI yields a 5GSM STATUS (#81); a reserved
	// PTI is ignored — no context and no response.
	switch verdict, cause := smfNas.PolicePTI(fgs.MsgPDUSessionEstablishmentRequest, uint8(req.PTI), func(uint8) bool { return false }); verdict {
	case smfNas.PTIIgnore:
		return nil, nil, nil
	case smfNas.PTIRespondStatus:
		rsp, buildErr := smfNas.BuildGSM5GSMStatus(fgs.PDUSessionID(pduSessionID), req.PTI, cause)
		if buildErr != nil {
			return nil, nil, fmt.Errorf("build 5GSM STATUS failed: %v", buildErr)
		}

		return nil, rsp, nil
	}

	return req, nil, nil
}

// CreateSmContext establishes a new 5G PDU session from the UE's NAS
// establishment request, returning the SM context ref or a NAS reject message.
func (s *SMF) CreateSmContext(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, dnn string, snssai *models.Snssai, requestType fgs.RequestType, n1Msg []byte) (string, []byte, error) {
	ctx, span := tracer.Start(ctx, "smf/create_session",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("ue.supi", supi.String()),
			attribute.Int("smf.pdu_session_id", int(pduSessionID)),
			attribute.String("smf.dnn", dnn),
		),
	)
	defer span.End()

	// TS 24.007 §11.2.3.1b: the UE allocates 1..15.
	if pduSessionID < 1 || pduSessionID > 15 {
		return "", nil, fmt.Errorf("PDU session id %d out of range (1..15)", pduSessionID)
	}

	req, rsp, err := parseEstablishmentRequest(pduSessionID, n1Msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to accept the establishment request")
	}

	if req == nil {
		return "", rsp, err
	}

	reqPTI := req.PTI

	// Record exactly one establishment outcome per attempt; parsing the request is
	// not an establishment attempt, so it precedes this defer.
	var establishmentResult string

	defer func() { recordSessionEstablishmentResult(metrics.RAT5G, establishmentResult) }()

	reject := establishmentReject(func(cause fgs.GSMCause, err error) ([]byte, error) {
		establishmentResult = metrics.ResultReject

		rsp, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(fgs.PDUSessionID(pduSessionID), reqPTI, cause)
		if buildErr != nil {
			return nil, fmt.Errorf("%w (build reject failed: %v)", err, buildErr)
		}

		return rsp, err
	})

	// Ella Core serves no emergency bearer services, so neither emergency request
	// type is served. No emergency PDU session exists for an "existing emergency
	// PDU session" request to name, which TS 24.501 §6.4.1.7 d) refuses with 5GSM
	// #54. An initial emergency request draws the same cause by local choice: the
	// spec assumes a network offering emergency service and gives no refusal.
	switch requestType {
	case fgs.RequestTypeInitialEmergencyRequest, fgs.RequestTypeExistingEmergencyPDUSession:
		rsp, err := reject(fgs.GSMCausePDUSessionDoesNotExist, fmt.Errorf("request type %v is not served", requestType))

		return "", rsp, err
	}

	isTransfer := requestType == fgs.RequestTypeExistingPDUSession

	// An initial request supersedes the session holding the identity
	// (TS 24.501 §6.4.1.7 c); a transfer names that same session
	// (§6.4.1.2 e)2)ii).
	if existing := s.currentPDUSession(supi, pduSessionID); existing != nil && !isTransfer {
		// TS 24.501 §6.4.1.7 c) is unconditional: the SMF shall locally release the
		// session holding the identity and proceed. The identity is correlated
		// across both systems (TS 23.501 §5.17.2.1), so the one released may be a
		// PDN connection, and the MME is told so it does not keep naming it.
		s.handlePduSessionContextReplacement(ctx, existing)
	}

	policy, err := s.GetSessionPolicy(ctx, supi, snssai, dnn)
	if err != nil {
		rsp, err := reject(establishmentRejectCause(err), fmt.Errorf("failed to find subscriber policy: %w", err))

		return "", rsp, err
	}

	if isTransfer {
		ref, rsp, err := s.transferTo5GS(ctx, supi, pduSessionID, dnn, snssai, policy, req, reqPTI, reject)
		if err != nil {
			establishmentResult = metrics.ResultReject
		} else {
			establishmentResult = metrics.ResultAccept
		}

		return ref, rsp, err
	}

	requestedType := requestedPDUSessionType(req)

	negotiatedType, err := s.negotiatePDUSessionType(ctx, uint8(requestedType), policy)
	if err != nil {
		rsp, err := reject(pduSessionTypeRejectCause(uint8(requestedType), policy), fmt.Errorf("PDU session type negotiation failed: %w", err))

		return "", rsp, err
	}

	pco, err := parsePDUSessionRequest(req)
	if err != nil {
		rsp, err := reject(fgs.GSMCauseRequestRejectedUnspecified, fmt.Errorf("parse PDU session request failed: %w", err))

		return "", rsp, err
	}

	sc, _, err := s.establishSession(ctx, SessionRequest{
		Supi:     supi,
		Identity: SessionIdentity{PDUSessionID: pduSessionID},
		Dnn:      dnn,
		Snssai:   snssai,
		Access:   Access5G,
		PDUType:  negotiatedType,
		Policy:   policy,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create SM context")

		cause := fgs.GSMCauseRequestRejectedUnspecified
		if errors.Is(err, errUEAddressAllocation) || errors.Is(err, errSessionIdentityInUse) {
			cause = fgs.GSMCauseInsufficientResources
		}

		rsp, err := reject(cause, fmt.Errorf("failed to create SM Context: %w", err))

		return "", rsp, err
	}

	cause := narrow5GSMCause(uint8(requestedType), sc.PDUSessionType)

	addrs := &smfNas.PDUSessionAddresses{
		PDUSessionType: fgs.PDUSessionType(sc.PDUSessionType),
		IPv4Address:    sc.PDUIPV4Address,
		IPv6IID:        sc.IPv6IID,
	}

	// The UPF session is up, so the establishment counts as an accept even if
	// the N1N2 delivery below fails.
	establishmentResult = metrics.ResultAccept

	if err := s.sendPduSessionEstablishmentAccept(ctx, sc, policy, pco, addrs, uint8(reqPTI), cause, alwaysOnIndication(req.AlwaysOnRequested)); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to send PDU session establishment accept")

		s.abortSession(ctx, sc)

		return "", nil, fmt.Errorf("failed to send pdu session establishment accept n1 message: %v", err)
	}

	return sc.Ref, nil, nil
}

// The UE address, the UPF session and its uplink F-TEID survive the move
// (TS 23.502 §4.11.2.3 step 9), so the accept is built from the session as it
// stands on the access serving it. The downlink switches when the gNB binds
// (§4.3.2.2.1 step 16a NOTE 11).
func (s *SMF) transferTo5GS(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, dnn string, snssai *models.Snssai, policy *Policy, req *fgs.PDUSessionEstablishmentRequest, pti nas.ProcedureTransactionIdentity, reject establishmentReject) (string, []byte, error) {
	transfer := transferRequest{Access: Access5G, Dnn: dnn, Snssai: snssai, Policy: policy}

	sc, err := s.findTransferable(supi, pduSessionID, transfer)
	if err != nil {
		// TS 24.501 §6.4.1.7 d): a request type "existing PDU session" naming a PDU
		// session the SMF has no information about draws #54, whatever the reason it
		// cannot be transferred as described. The UE ignores the back-off timer for
		// #54 and retries with request type "initial request" (§6.4.1.4.3); every
		// other cause holds the DNN down for 12 minutes.
		rsp, err := reject(fgs.GSMCausePDUSessionDoesNotExist, err)

		return "", rsp, err
	}

	pco, err := parsePDUSessionRequest(req)
	if err != nil {
		// The only failure is a PDU session type outside the modelled range.
		rsp, err := reject(fgs.GSMCauseUnknownPDUSessionType, fmt.Errorf("parse PDU session request failed: %w", err))

		return "", rsp, err
	}

	if err := s.prepareTransfer(sc, transfer); err != nil {
		rsp, err := reject(fgs.GSMCauseInsufficientResources, err)

		return "", rsp, err
	}

	sc.mu.Lock()
	sessionType := sc.PDUSessionType
	addrs := &smfNas.PDUSessionAddresses{
		PDUSessionType: fgs.PDUSessionType(sessionType),
		IPv4Address:    sc.PDUIPV4Address,
		IPv6IID:        sc.IPv6IID,
	}
	sc.mu.Unlock()

	// The UE learns which family the transferred session has.
	cause := narrow5GSMCause(uint8(requestedPDUSessionType(req)), sessionType)

	if err := s.sendPduSessionEstablishmentAccept(ctx, sc, policy, pco, addrs, uint8(pti), cause, alwaysOnIndication(req.AlwaysOnRequested)); err != nil {
		// The UE never learns of the 5GS leg, so it stays on the access serving it.
		sc.abandonTransfer()

		return "", nil, fmt.Errorf("failed to send pdu session establishment accept for a transferred session: %v", err)
	}

	return sc.Ref, nil, nil
}

func (s *SMF) handlePduSessionContextReplacement(ctx context.Context, smCtxt *SMContext) {
	var (
		releasedEBI uint8
		releasedPSI uint8
	)

	defer func() {
		if releasedEBI != 0 && s.mme != nil {
			s.mme.SessionReleased(ctx, smCtxt.Supi.IMSI(), releasedEBI, smCtxt.Ref)
		}

		if releasedPSI != 0 && s.amf != nil {
			s.amf.SessionReleased(ctx, smCtxt.Supi, releasedPSI, smCtxt.Ref)
		}
	}()

	if err := smCtxt.procedures.Begin(procedure.Release); err != nil {
		logger.WithTrace(ctx, logger.SmfLog).Warn("skipping supersede of a session with a procedure in flight",
			zap.Error(err), zap.String("smContextRef", smCtxt.Ref))

		return
	}

	defer smCtxt.procedures.End(procedure.Release)

	smCtxt.mu.Lock()
	defer smCtxt.mu.Unlock()

	if smCtxt.IsEPS() {
		releasedEBI = smCtxt.EBI
	} else {
		releasedPSI = smCtxt.PDUSessionID
	}

	// A move the target has not bound leaves it holding the state it installed from
	// the accept it already sent, under the identity it would serve the session by.
	if p := smCtxt.pending; p != nil {
		if p.to == Access4G {
			releasedEBI = p.ebi
		} else {
			releasedPSI = smCtxt.PDUSessionID
		}
	}

	smCtxt.stopProcedureTimer()
	s.RemoveSession(ctx, smCtxt.Ref)
}

// establishmentRejectCause maps a session-policy lookup failure to the 5GSM
// cause of the PDU Session Establishment Reject (TS 24.501 §9.11.4.2): #70 when
// the slice is served but not the DNN, #27 when the DNN is unknown, and the
// generic #31 otherwise.
func establishmentRejectCause(err error) fgs.GSMCause {
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
		if t != fgs.PDUSessionTypeIPv4 && t != fgs.PDUSessionTypeIPv6 && t != fgs.PDUSessionTypeIPv4v6 {
			return nil, fmt.Errorf("requested PDUSessionType is invalid: %d", t)
		}
	}

	pco := &smfNas.ProtocolConfigurationOptions{}

	if req.ExtendedPCO != nil {
		pco.IPv4LinkMTURequest = true

		for _, id := range req.ExtendedPCO.ContainerIDs() {
			switch id {
			case nas.PCOContainerDNSServerIPv6Address:
				pco.DNSIPv6Request = true
			case nas.PCOContainerDNSServerIPv4Address:
				pco.DNSIPv4Request = true
			}
		}
	}

	return pco, nil
}

// alwaysOnIndication answers a UE that asked for an always-on PDU session. The
// element is absent unless the UE asked, and TS 24.501 table 9.11.4.3.1 codes the
// answer as "not allowed" — this core grants none (§6.4.1.3).
func alwaysOnIndication(requested *bool) *bool {
	if requested != nil && *requested {
		notAllowed := false
		return &notAllowed
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
	cause *fgs.GSMCause,
	alwaysOn *bool,
) error {
	ctx, span := tracer.Start(ctx, "smf/send_pdu_session_establishment_accept",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	// A transfer rewrites the access, the slice and the tunnel, so the message
	// content is taken in one critical section. The AMF call below is made
	// outside it.
	smContext.mu.Lock()
	smContext.establishmentPTI = pti

	if smContext.Tunnel == nil || smContext.Tunnel.DataPath == nil || smContext.Tunnel.DataPath.UpLinkTunnel == nil {
		smContext.mu.Unlock()

		return fmt.Errorf("session %q has no uplink tunnel", smContext.Ref)
	}

	var (
		supi         = smContext.Supi
		pduSessionID = smContext.PDUSessionID
		snssai       = smContext.Snssai
		dnn          = smContext.Dnn
		ngapPDUType  = nasToNgapPDUSessionType(smContext.PDUSessionType)
		ul           = smContext.Tunnel.DataPath.UpLinkTunnel
		ulTEID       = ul.TEID
		ulN3IPv4     = ul.N3IPv4
		ulN3IPv6     = ul.N3IPv6
	)

	smContext.mu.Unlock()

	n1Msg, err := smfNas.BuildGSMPDUSessionEstablishmentAccept(&policy.Ambr, &policy.QosData, pduSessionID, pti, snssai, dnn, pco, policy.DNS, policy.MTU, cause, addrs, alwaysOn)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to build PDU session establishment accept")

		return fmt.Errorf("build GSM PDUSessionEstablishmentAccept failed: %v", err)
	}

	n2Msg, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(&policy.Ambr, &policy.QosData, ulTEID, ulN3IPv4, ulN3IPv6, ngapPDUType)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to build PDU session resource setup request transfer")

		return fmt.Errorf("build PDUSessionResourceSetupRequestTransfer failed: %v", err)
	}

	smContext.SetPolicyData(policy)

	err = s.amf.TransferN1N2(ctx, supi, pduSessionID, snssai, n1Msg, n2Msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to transfer N1N2 message")

		return fmt.Errorf("failed to send n1 n2 transfer request: %v", err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Debug("Sent n1 n2 transfer request", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	return nil
}
