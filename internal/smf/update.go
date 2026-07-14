// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// UpdateResult carries the N1/N2 messages produced by an SM context update.
type UpdateResult struct {
	ReleaseN2 bool   // true when N2 info signals PDU session resource release
	N1Msg     []byte // NAS message for the UE (may be nil)
	N2Msg     []byte // NGAP transfer for the RAN (may be nil)
}

// UpdateSmContextN1Msg handles a NAS N1 message update (e.g. PDU session release request).
func (s *SMF) UpdateSmContextN1Msg(ctx context.Context, smContextRef string, n1Msg []byte) (*UpdateResult, error) {
	ctx, span := tracer.Start(ctx, "smf/update_sm_context_n1_msg",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	if smContextRef == "" {
		return nil, fmt.Errorf("SM Context reference is missing")
	}

	smContext := s.GetSession(smContextRef)
	if smContext == nil {
		return nil, fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	rsp, err := s.handleUpdateN1Msg(ctx, n1Msg, smContext)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to handle N1 message")

		return nil, fmt.Errorf("error handling N1 message: %v", err)
	}

	return rsp, nil
}

func (s *SMF) handleUpdateN1Msg(ctx context.Context, n1Msg []byte, smContext *SMContext) (*UpdateResult, error) {
	if n1Msg == nil {
		return nil, nil
	}

	m := nas.NewMessage()

	// The PTI is octet 3 of every 5GSM message (TS 24.501 §9.6); capture it
	// before GsmMessageDecode advances the slice.
	raw := n1Msg

	if err := m.GsmMessageDecode(&n1Msg); err != nil {
		return nil, fmt.Errorf("error decoding N1SmMessage: %v", err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Debug("Update SM Context Request N1SmMessage", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	msgType := m.GsmHeader.GetMessageType()

	if len(raw) < 3 {
		return nil, fmt.Errorf("5GSM message too short to contain a PTI")
	}

	pti := raw[2]

	switch verdict, cause := smfNas.PolicePTI(msgType, pti, smContext.IsPTIInUse); verdict {
	case smfNas.PTIIgnore:
		logger.WithTrace(ctx, logger.SmfLog).Info("ignoring 5GSM message with reserved PTI", zap.Uint8("MessageType", msgType), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		return nil, nil
	case smfNas.PTIRespondStatus:
		n1SmMsg, err := smfNas.BuildGSM5GSMStatus(smContext.PDUSessionID, pti, cause)
		if err != nil {
			return nil, fmt.Errorf("build GSM 5GSM STATUS failed: %v", err)
		}

		return &UpdateResult{N1Msg: n1SmMsg}, nil
	}

	switch msgType {
	case nas.MsgTypePDUSessionReleaseRequest:
		// A UE-requested release runs as a network-requested release
		// (TS 24.501 §6.4.3.3 → §6.3.3): the UE-allocated PTI is carried on the
		// Release Command and held until the matching Release Complete; T3592
		// retransmits the command meanwhile.
		logger.WithTrace(ctx, logger.SmfLog).Info("N1 Msg PDU Session Release Request received", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

		if err := s.startRelease(ctx, smContext, pti, nasMessage.Cause5GSMRegularDeactivation); err != nil {
			return nil, fmt.Errorf("start PDU session release: %w", err)
		}

		return nil, nil

	case nas.MsgTypePDUSessionModificationRequest:
		logger.WithTrace(ctx, logger.SmfLog).Info("N1 Msg PDU Session Modification Request received; rejecting", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

		// The UE cannot set its own QoS; the authorized QoS is network-determined
		// and not modifiable on UE request, so the request is rejected
		// (TS 24.501 clause 6.4.2.4).
		n1SmMsg, err := smfNas.BuildGSMPDUSessionModificationReject(smContext.PDUSessionID, pti, nasMessage.Cause5GSMRequestRejectedUnspecified)
		if err != nil {
			return nil, fmt.Errorf("build GSM PDUSessionModificationReject failed: %v", err)
		}

		return &UpdateResult{N1Msg: n1SmMsg}, nil

	case nas.MsgTypePDUSessionReleaseComplete:
		// Release acknowledged; stop T3592 and tear down the user plane, held active
		// through the release window (TS 24.501 §6.3.3.3).
		logger.WithTrace(ctx, logger.SmfLog).Info("N1 Msg PDU Session Release Complete received", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		smContext.stopProcedureTimer()
		smContext.ClearPTIInUse(pti)
		s.teardownAndRemove(ctx, smContext)

		return nil, nil

	case nas.MsgTypePDUSessionModificationComplete:
		// The UE accepted the modification; stop T3591 and commit the new policy
		// (TS 24.501 §6.3.2.2, "consider the PDU session as modified").
		logger.WithTrace(ctx, logger.SmfLog).Info("N1 Msg PDU Session Modification Complete received", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		smContext.stopProcedureTimer()
		smContext.ClearPTIInUse(pti)

		if smContext.pendingPolicy != nil {
			smContext.PolicyData = smContext.pendingPolicy
			smContext.pendingPolicy = nil
		}

		return nil, nil

	case nas.MsgTypePDUSessionModificationCommandReject:
		// The UE rejected the modification; stop T3591 and discard the pending policy,
		// keeping the previous configuration (TS 24.501 §6.3.2.4, §6.3.2.5).
		logger.WithTrace(ctx, logger.SmfLog).Warn("N1 Msg PDU Session Modification Command Reject received", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		smContext.stopProcedureTimer()
		smContext.ClearPTIInUse(pti)
		smContext.pendingPolicy = nil

		return nil, nil

	case nas.MsgTypeStatus5GSM:
		s.handle5GSMStatus(ctx, smContext, pti, m.Status5GSM.Cause5GSM.GetCauseValue())

		return nil, nil

	default:
		logger.WithTrace(ctx, logger.SmfLog).Warn("N1 Msg type not supported in SM Context Update", zap.Uint8("MessageType", msgType), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		return nil, nil
	}
}

// UpdateSmContextN2InfoPduResSetupRsp handles the N2 PDUSession Resource Setup Response.
func (s *SMF) UpdateSmContextN2InfoPduResSetupRsp(ctx context.Context, smContextRef string, n2Data []byte) error {
	ctx, span := tracer.Start(ctx, "smf/update_sm_context_pdu_resource_setup_response",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	if smContextRef == "" {
		span.RecordError(fmt.Errorf("SM Context reference is missing"))
		span.SetStatus(codes.Error, "SM Context reference is missing")

		return fmt.Errorf("SM Context reference is missing")
	}

	smContext := s.GetSession(smContextRef)
	if smContext == nil {
		span.RecordError(fmt.Errorf("sm context not found"))
		span.SetStatus(codes.Error, "sm context not found")

		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if smContext.Tunnel == nil || smContext.Tunnel.DataPath == nil {
		span.RecordError(fmt.Errorf("session already released"))
		span.SetStatus(codes.Error, "session already released")

		return fmt.Errorf("session already released")
	}

	pdrList, farList, err := handleUpdateN2MsgPDUResourceSetupResp(n2Data, smContext)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to handle N2 message")

		return fmt.Errorf("error handling N2 message: %v", err)
	}

	if smContext.PFCPContext == nil {
		span.RecordError(fmt.Errorf("pfcp session context not found"))
		span.SetStatus(codes.Error, "pfcp session context not found")

		return fmt.Errorf("pfcp session context not found")
	}

	if err := s.upf.ModifySession(ctx, BuildModifyRequest(
		smContext.PFCPContext.RemoteSEID,
		"",
		pdrList, farList, nil,
	)); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to modify PFCP session")

		return fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}

	s.registerIPv6SessionIfNeeded(ctx, smContext)

	logger.SmfLog.Info("Sent PFCP session modification request", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	return nil
}

func handleUpdateN2MsgPDUResourceSetupResp(binaryDataN2SmInformation []byte, smContext *SMContext) ([]*PDR, []*FAR, error) {
	logger.SmfLog.Debug("received n2 sm info type", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	var pdrList []*PDR

	var farList []*FAR

	if smContext.Tunnel.DataPath.Activated {
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ApplyAction = models.ApplyAction{Forw: true}
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ForwardingParameters = &models.ForwardingParameters{}

		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.State = RuleUpdate
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.State = RuleUpdate

		pdrList = append(pdrList, smContext.Tunnel.DataPath.DownLinkTunnel.PDR)
		farList = append(farList, smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR)

		// Initial PDR creation set the UL OuterHeaderRemoval before the gNB IP was
		// known; mark it for update so the corrected value reaches the UPF.
		smContext.Tunnel.DataPath.UpLinkTunnel.PDR.State = RuleUpdate
		pdrList = append(pdrList, smContext.Tunnel.DataPath.UpLinkTunnel.PDR)
	}

	if err := handlePDUSessionResourceSetupResponseTransfer(binaryDataN2SmInformation, smContext); err != nil {
		return nil, nil, fmt.Errorf("handle PDUSessionResourceSetupResponseTransfer failed: %v", err)
	}

	return pdrList, farList, nil
}

func anchorFromGTPTunnel(t *ngapType.GTPTunnel) AnchorBinding {
	ipv4, ipv6 := ngap.ParseTransportLayerAddress(t.TransportLayerAddress.Value)

	return AnchorBinding{
		TEID: binary.BigEndian.Uint32(t.GTPTEID.Value),
		IPv4: ipv4,
		IPv6: ipv6,
	}
}

func handlePDUSessionResourceSetupResponseTransfer(b []byte, smContext *SMContext) error {
	resourceSetupResponseTransfer := ngapType.PDUSessionResourceSetupResponseTransfer{}

	if err := aper.UnmarshalWithParams(b, &resourceSetupResponseTransfer, "valueExt"); err != nil {
		return fmt.Errorf("failed to unmarshall resource setup response transfer: %s", err.Error())
	}

	tnl := resourceSetupResponseTransfer.DLQosFlowPerTNLInformation.UPTransportLayerInformation
	if tnl.Present != ngapType.UPTransportLayerInformationPresentGTPTunnel {
		return fmt.Errorf("expected qos flow per tnl information up transport layer information present to be gtp tunnel")
	}

	smContext.bindAccessTunnel(anchorFromGTPTunnel(tnl.GTPTunnel))

	return nil
}

// UpdateSmContextN2InfoPduResSetupFail handles a PDUSession Resource Setup failure.
func (s *SMF) UpdateSmContextN2InfoPduResSetupFail(ctx context.Context, smContextRef string, n2Data []byte) error {
	_, span := tracer.Start(ctx, "smf/update_sm_context_pdu_resource_setup_fail",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	if smContextRef == "" {
		span.RecordError(fmt.Errorf("SM Context reference is missing"))
		span.SetStatus(codes.Error, "SM Context reference is missing")

		return fmt.Errorf("SM Context reference is missing")
	}

	smContext := s.GetSession(smContextRef)
	if smContext == nil {
		span.RecordError(fmt.Errorf("sm context not found"))
		span.SetStatus(codes.Error, "sm context not found")

		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	return handlePDUSessionResourceSetupUnsuccessfulTransfer(n2Data)
}

func handlePDUSessionResourceSetupUnsuccessfulTransfer(b []byte) error {
	resourceSetupUnsuccessfulTransfer := ngapType.PDUSessionResourceSetupUnsuccessfulTransfer{}

	if err := aper.UnmarshalWithParams(b, &resourceSetupUnsuccessfulTransfer, "valueExt"); err != nil {
		return fmt.Errorf("failed to unmarshall resource setup unsuccessful transfer: %s", err.Error())
	}

	switch resourceSetupUnsuccessfulTransfer.Cause.Present {
	case ngapType.CausePresentRadioNetwork:
		logger.SmfLog.Warn("PDU Session Resource Setup Unsuccessful by RadioNetwork", logger.Cause(radioNetworkCauseString(resourceSetupUnsuccessfulTransfer.Cause.RadioNetwork.Value)))
	case ngapType.CausePresentTransport:
		logger.SmfLog.Warn("PDU Session Resource Setup Unsuccessful by Transport", logger.Cause(transportCauseString(resourceSetupUnsuccessfulTransfer.Cause.Transport.Value)))
	case ngapType.CausePresentNas:
		logger.SmfLog.Warn("PDU Session Resource Setup Unsuccessful by NAS", logger.Cause(nasCauseString(resourceSetupUnsuccessfulTransfer.Cause.Nas.Value)))
	case ngapType.CausePresentProtocol:
		logger.SmfLog.Warn("PDU Session Resource Setup Unsuccessful by Protocol", logger.Cause(protocolCauseString(resourceSetupUnsuccessfulTransfer.Cause.Protocol.Value)))
	case ngapType.CausePresentMisc:
		logger.SmfLog.Warn("PDU Session Resource Setup Unsuccessful by Misc", logger.Cause(miscCauseString(resourceSetupUnsuccessfulTransfer.Cause.Misc.Value)))
	case ngapType.CausePresentChoiceExtensions:
		logger.SmfLog.Warn("PDU Session Resource Setup Unsuccessful by ChoiceExtensions", zap.Any("Cause", resourceSetupUnsuccessfulTransfer.Cause.ChoiceExtensions))
	}

	return nil
}

// UpdateSmContextN2InfoPduResRelRsp handles the final N2 PDU Session Resource Release Response.
func (s *SMF) UpdateSmContextN2InfoPduResRelRsp(ctx context.Context, smContextRef string) error {
	ctx, span := tracer.Start(ctx, "smf/update_sm_context_pdu_resource_release_response",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	if smContextRef == "" {
		span.RecordError(fmt.Errorf("SM Context reference is missing"))
		span.SetStatus(codes.Error, "SM Context reference is missing")

		return fmt.Errorf("SM Context reference is missing")
	}

	smContext := s.GetSession(smContextRef)
	if smContext == nil {
		// Session already removed (e.g. by slice-mismatch release); return nil to
		// keep the response idempotent.
		logger.SmfLog.Info("SM context already removed, skipping",
			zap.String("smContextRef", smContextRef))

		return nil
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	// N2 release complete; stop T3592 (TS 24.501 §6.3.3).
	smContext.stopProcedureTimer()

	if smContext.PDUSessionReleaseDueToDupPduID {
		// Duplicate-PDU-ID release: the tunnel was already torn down when the
		// duplicate was detected (UpdateSmContextCauseDuplicatePDUSessionID).
		smContext.PDUSessionReleaseDueToDupPduID = false
		s.RemoveSession(ctx, smContext.Ref)
	} else {
		// Network-requested release: tear down the user plane, held active through
		// the release window (TS 23.502 §4.3.4).
		s.teardownAndRemove(ctx, smContext)
	}

	return nil
}

// UpdateSmContextCauseDuplicatePDUSessionID handles duplicate PDU session ID by releasing
// the existing session and building a release command for the radio.
func (s *SMF) UpdateSmContextCauseDuplicatePDUSessionID(ctx context.Context, smContextRef string) ([]byte, error) {
	ctx, span := tracer.Start(ctx, "smf/update_sm_context_cause_duplicate_pdu_session_id",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	if smContextRef == "" {
		span.RecordError(fmt.Errorf("SM Context reference is missing"))
		span.SetStatus(codes.Error, "SM Context reference is missing")

		return nil, fmt.Errorf("SM Context reference is missing")
	}

	smContext := s.GetSession(smContextRef)
	if smContext == nil {
		span.RecordError(fmt.Errorf("sm context not found"))
		span.SetStatus(codes.Error, "sm context not found")

		return nil, fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	smContext.PDUSessionReleaseDueToDupPduID = true

	n2Rsp, err := ngap.BuildPDUSessionResourceReleaseCommandTransfer()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to build PDU session resource release command transfer")

		return nil, fmt.Errorf("build PDUSession Resource Release Command Transfer Error: %v", err)
	}

	if err := s.releaseTunnel(ctx, smContext); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to release tunnel")

		return nil, fmt.Errorf("failed to release tunnel: %v", err)
	}

	return n2Rsp, nil
}

// registerIPv6SessionIfNeeded registers the IPv6 session with the UPF's RA
// responder if the session has a delegated IPv6 prefix and the gNB's tunnel
// endpoint is known. Must be called with smContext.Mutex held.
func (s *SMF) registerIPv6SessionIfNeeded(ctx context.Context, smContext *SMContext) {
	if smContext.PDUIPV6Prefix == nil || smContext.Tunnel == nil {
		return
	}

	anInfo := smContext.Tunnel.ANInformation
	if anInfo.TEID == 0 {
		return
	}

	var gnbIP netip.Addr

	if anInfo.IPv6Address != nil {
		if addr, ok := netip.AddrFromSlice(anInfo.IPv6Address.To16()); ok {
			gnbIP = addr
		}
	}

	if !gnbIP.IsValid() && anInfo.IPv4Address != nil {
		if addr, ok := netip.AddrFromSlice(anInfo.IPv4Address.To4()); ok {
			gnbIP = addr
		}
	}

	if !gnbIP.IsValid() {
		return
	}

	prefixAddr, ok := netip.AddrFromSlice(smContext.PDUIPV6Prefix.To16())
	if !ok {
		return
	}

	var qfi uint8
	if smContext.PolicyData != nil {
		qfi = smContext.PolicyData.QosData.QFI
	}

	var mtu uint32
	if smContext.PolicyData != nil {
		mtu = uint32(smContext.PolicyData.MTU)
	}

	reg := &models.IPv6SessionRegistration{
		UplinkTEID:   smContext.Tunnel.DataPath.UpLinkTunnel.TEID,
		DownlinkTEID: anInfo.TEID,
		GnbN3Addr:    gnbIP,
		Prefix:       netip.PrefixFrom(prefixAddr, 64),
		MTU:          mtu,
		QFI:          qfi,
		// A 4G S1-U bearer carries the RA PSC-less; 5G N3 carries it in the PDU
		// Session Container. The encap follows the access.
		S1U: !smContext.Access.usesPSC(),
	}

	if err := s.upf.RegisterIPv6Session(ctx, reg); err != nil {
		logger.SmfLog.Warn("failed to register IPv6 session for RA",
			zap.Error(err),
			logger.SUPI(smContext.Supi.String()),
			logger.PDUSessionID(smContext.PDUSessionID),
		)
	}
}
