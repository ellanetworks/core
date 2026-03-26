// Copyright 2024 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package smf

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas"
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

	rsp, sendPfcpDelete, err := s.handleUpdateN1Msg(ctx, n1Msg, smContext)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to handle N1 message")

		return nil, fmt.Errorf("error handling N1 message: %v", err)
	}

	if sendPfcpDelete {
		if err := s.releaseTunnel(ctx, smContext); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to release tunnel")

			return nil, fmt.Errorf("failed to release tunnel: %v", err)
		}
	}

	return rsp, nil
}

func (s *SMF) handleUpdateN1Msg(ctx context.Context, n1Msg []byte, smContext *SMContext) (*UpdateResult, bool, error) {
	if n1Msg == nil {
		return nil, false, nil
	}

	m := nas.NewMessage()

	if err := m.GsmMessageDecode(&n1Msg); err != nil {
		return nil, false, fmt.Errorf("error decoding N1SmMessage: %v", err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Debug("Update SM Context Request N1SmMessage", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	switch m.GsmHeader.GetMessageType() {
	case nas.MsgTypePDUSessionReleaseRequest:
		logger.WithTrace(ctx, logger.SmfLog).Info("N1 Msg PDU Session Release Request received", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

		if smContext.PDUAddress != nil {
			if err := s.store.ReleaseIP(ctx, smContext.Supi.IMSI(), smContext.PDUAddress); err != nil {
				return nil, false, fmt.Errorf("failed to release UE IP Addr: %v", err)
			}
		}

		pti := m.PDUSessionReleaseRequest.GetPTI()

		n1SmMsg, err := smfNas.BuildGSMPDUSessionReleaseCommand(smContext.PDUSessionID, pti)
		if err != nil {
			return nil, false, fmt.Errorf("build GSM PDUSessionReleaseCommand failed: %v", err)
		}

		n2SmMsg, err := ngap.BuildPDUSessionResourceReleaseCommandTransfer()
		if err != nil {
			return nil, false, fmt.Errorf("build PDUSession Resource Release Command Transfer Error: %v", err)
		}

		sendPfcpDelete := smContext.Tunnel != nil

		response := &UpdateResult{
			N1Msg:     n1SmMsg,
			ReleaseN2: true,
			N2Msg:     n2SmMsg,
		}

		return response, sendPfcpDelete, nil

	default:
		logger.WithTrace(ctx, logger.SmfLog).Warn("N1 Msg type not supported in SM Context Update", zap.Uint8("MessageType", m.GsmHeader.GetMessageType()), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		return nil, false, nil
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

	if err := s.upf.ModifySession(ctx, &PFCPModificationRequest{
		LocalSEID:  smContext.PFCPContext.LocalSEID,
		RemoteSEID: smContext.PFCPContext.RemoteSEID,
		PDRs:       pdrList,
		FARs:       farList,
	}); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to modify PFCP session")

		return fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}

	logger.SmfLog.Info("Sent PFCP session modification request", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	return nil
}

func handleUpdateN2MsgPDUResourceSetupResp(binaryDataN2SmInformation []byte, smContext *SMContext) ([]*PDR, []*FAR, error) {
	logger.SmfLog.Debug("received n2 sm info type", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	var pdrList []*PDR

	var farList []*FAR

	if smContext.Tunnel.DataPath.Activated {
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ApplyAction = ApplyAction{Forw: true}
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ForwardingParameters = &ForwardingParameters{
			DestinationInterface: DestinationInterface{
				InterfaceValue: DestinationInterfaceAccess,
			},
			NetworkInstance: smContext.Dnn,
		}

		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.State = RuleUpdate
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.State = RuleUpdate

		pdrList = append(pdrList, smContext.Tunnel.DataPath.DownLinkTunnel.PDR)
		farList = append(farList, smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR)
	}

	if err := handlePDUSessionResourceSetupResponseTransfer(binaryDataN2SmInformation, smContext); err != nil {
		return nil, nil, fmt.Errorf("handle PDUSessionResourceSetupResponseTransfer failed: %v", err)
	}

	return pdrList, farList, nil
}

func handlePDUSessionResourceSetupResponseTransfer(b []byte, smContext *SMContext) error {
	resourceSetupResponseTransfer := ngapType.PDUSessionResourceSetupResponseTransfer{}

	if err := aper.UnmarshalWithParams(b, &resourceSetupResponseTransfer, "valueExt"); err != nil {
		return fmt.Errorf("failed to unmarshall resource setup response transfer: %s", err.Error())
	}

	QosFlowPerTNLInformation := resourceSetupResponseTransfer.DLQosFlowPerTNLInformation

	if QosFlowPerTNLInformation.UPTransportLayerInformation.Present != ngapType.UPTransportLayerInformationPresentGTPTunnel {
		return fmt.Errorf("expected qos flow per tnl information up transport layer information present to be gtp tunnel")
	}

	gtpTunnel := QosFlowPerTNLInformation.UPTransportLayerInformation.GTPTunnel

	teid := binary.BigEndian.Uint32(gtpTunnel.GTPTEID.Value)

	smContext.Tunnel.ANInformation.IPAddress = gtpTunnel.TransportLayerAddress.Value.Bytes
	smContext.Tunnel.ANInformation.TEID = teid

	if smContext.Tunnel.DataPath.Activated {
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ForwardingParameters.OuterHeaderCreation = &OuterHeaderCreation{
			OuterHeaderCreationDescription: OuterHeaderCreationGtpUUdpIpv4,
			TeID:                           teid,
			IPv4Address:                    smContext.Tunnel.ANInformation.IPAddress.To4(),
		}
	}

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
		span.RecordError(fmt.Errorf("sm context not found"))
		span.SetStatus(codes.Error, "sm context not found")

		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if smContext.PDUSessionReleaseDueToDupPduID {
		smContext.PDUSessionReleaseDueToDupPduID = false
		s.RemoveSession(ctx, smContext.CanonicalName())
	} else {
		// N1 release path already called ReleaseIP; just remove from pool.
		s.removeSessionUnlocked(ctx, smContext.CanonicalName())
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

// UpdateSmContextN2HandoverPreparing handles the handover-required N2 message
// and returns a PDUSession Resource Setup Request Transfer for the target radio.
func (s *SMF) UpdateSmContextN2HandoverPreparing(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	_, span := tracer.Start(ctx, "smf/update_sm_context_n2_handover_preparing",
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

	if err := handleHandoverRequiredTransfer(n2Data); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to handle handover required transfer")

		return nil, fmt.Errorf("handle HandoverRequiredTransfer failed: %v", err)
	}

	n2Rsp, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(&smContext.PolicyData.Ambr, &smContext.PolicyData.QosData, smContext.Tunnel.DataPath.UpLinkTunnel.TEID, smContext.Tunnel.DataPath.UpLinkTunnel.N3IP)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to build PDU session resource setup request transfer")

		return nil, fmt.Errorf("build PDUSession Resource Setup Request Transfer Error: %v", err)
	}

	return n2Rsp, nil
}

func handleHandoverRequiredTransfer(b []byte) error {
	handoverRequiredTransfer := ngapType.HandoverRequiredTransfer{}

	if err := aper.UnmarshalWithParams(b, &handoverRequiredTransfer, "valueExt"); err != nil {
		return fmt.Errorf("failed to unmarshall handover required transfer: %s", err.Error())
	}

	return nil
}

// UpdateSmContextN2HandoverPrepared handles the handover request acknowledge
// from the target radio and returns a Handover Command Transfer.
func (s *SMF) UpdateSmContextN2HandoverPrepared(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	_, span := tracer.Start(ctx, "smf/update_sm_context_n2_handover_prepared",
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

	if err := handleHandoverRequestAcknowledgeTransfer(n2Data, smContext); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to handle handover request acknowledge transfer")

		return nil, fmt.Errorf("handle HandoverRequestAcknowledgeTransfer failed: %v", err)
	}

	n2Rsp, err := ngap.BuildHandoverCommandTransfer(smContext.Tunnel.DataPath.UpLinkTunnel.TEID, smContext.Tunnel.DataPath.UpLinkTunnel.N3IP)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to build handover command transfer")

		return nil, fmt.Errorf("build Handover Command Transfer Error: %v", err)
	}

	return n2Rsp, nil
}

func handleHandoverRequestAcknowledgeTransfer(b []byte, smContext *SMContext) error {
	handoverRequestAcknowledgeTransfer := ngapType.HandoverRequestAcknowledgeTransfer{}

	if err := aper.UnmarshalWithParams(b, &handoverRequestAcknowledgeTransfer, "valueExt"); err != nil {
		return fmt.Errorf("failed to unmarshall handover request acknowledge transfer: %s", err.Error())
	}

	DLNGUUPTNLInformation := handoverRequestAcknowledgeTransfer.DLNGUUPTNLInformation
	GTPTunnel := DLNGUUPTNLInformation.GTPTunnel

	teid := binary.BigEndian.Uint32(GTPTunnel.GTPTEID.Value)

	if smContext.Tunnel.DataPath.Activated {
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ForwardingParameters.OuterHeaderCreation = &OuterHeaderCreation{
			OuterHeaderCreationDescription: OuterHeaderCreationGtpUUdpIpv4,
			TeID:                           teid,
			IPv4Address:                    GTPTunnel.TransportLayerAddress.Value.Bytes,
		}
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.State = RuleUpdate
	}

	return nil
}

// UpdateSmContextXnHandoverPathSwitchReq handles an Xn handover path-switch request.
func (s *SMF) UpdateSmContextXnHandoverPathSwitchReq(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	ctx, span := tracer.Start(ctx, "smf/update_sm_context_handover_path_switch_request",
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

	pdrList, farList, n2buf, err := handleUpdateN2MsgXnHandoverPathSwitchReq(n2Data, smContext)
	if err != nil {
		return nil, fmt.Errorf("error handling N2 message: %v", err)
	}

	if smContext.PFCPContext == nil {
		return nil, fmt.Errorf("pfcp session context not found for upf")
	}

	if err := s.upf.ModifySession(ctx, &PFCPModificationRequest{
		LocalSEID:  smContext.PFCPContext.LocalSEID,
		RemoteSEID: smContext.PFCPContext.RemoteSEID,
		PDRs:       pdrList,
		FARs:       farList,
	}); err != nil {
		return nil, fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}

	logger.SmfLog.Info("Sent PFCP session modification request", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	return n2buf, nil
}

func handleUpdateN2MsgXnHandoverPathSwitchReq(n2Data []byte, smContext *SMContext) ([]*PDR, []*FAR, []byte, error) {
	logger.SmfLog.Debug("handle Path Switch Request", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	if err := handlePathSwitchRequestTransfer(n2Data, smContext); err != nil {
		return nil, nil, nil, fmt.Errorf("handle PathSwitchRequestTransfer failed: %v", err)
	}

	n2Buf, err := ngap.BuildPathSwitchRequestAcknowledgeTransfer(smContext.Tunnel.DataPath.UpLinkTunnel.TEID, smContext.Tunnel.DataPath.UpLinkTunnel.N3IP)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("build Path Switch Transfer Error: %v", err)
	}

	var pdrList []*PDR

	var farList []*FAR

	if smContext.Tunnel.DataPath.Activated {
		pdrList = append(pdrList, smContext.Tunnel.DataPath.DownLinkTunnel.PDR)
		farList = append(farList, smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR)
	}

	return pdrList, farList, n2Buf, nil
}

func handlePathSwitchRequestTransfer(b []byte, smContext *SMContext) error {
	pathSwitchRequestTransfer := ngapType.PathSwitchRequestTransfer{}

	if err := aper.UnmarshalWithParams(b, &pathSwitchRequestTransfer, "valueExt"); err != nil {
		return err
	}

	if pathSwitchRequestTransfer.DLNGUUPTNLInformation.Present != ngapType.UPTransportLayerInformationPresentGTPTunnel {
		return errors.New("pathSwitchRequestTransfer.DLNGUUPTNLInformation.Present")
	}

	gtpTunnel := pathSwitchRequestTransfer.DLNGUUPTNLInformation.GTPTunnel

	teid := binary.BigEndian.Uint32(gtpTunnel.GTPTEID.Value)

	smContext.Tunnel.ANInformation.IPAddress = gtpTunnel.TransportLayerAddress.Value.Bytes
	smContext.Tunnel.ANInformation.TEID = teid

	if smContext.Tunnel.DataPath.Activated {
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ForwardingParameters.OuterHeaderCreation = &OuterHeaderCreation{
			OuterHeaderCreationDescription: OuterHeaderCreationGtpUUdpIpv4,
			TeID:                           teid,
			IPv4Address:                    gtpTunnel.TransportLayerAddress.Value.Bytes,
		}
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.State = RuleUpdate
		smContext.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ForwardingParameters.PFCPSMReqFlags = &PFCPSMReqFlags{Sndem: true}
	}

	return nil
}

// UpdateSmContextHandoverFailed handles a path switch failure.
func (s *SMF) UpdateSmContextHandoverFailed(ctx context.Context, smContextRef string, n2Data []byte) error {
	_, span := tracer.Start(ctx, "smf/update_sm_context_handover_failed",
		trace.WithAttributes(attribute.String("smf.smContextRef", smContextRef)),
	)
	defer span.End()

	if smContextRef == "" {
		return fmt.Errorf("SM Context reference is missing")
	}

	smContext := s.GetSession(smContextRef)
	if smContext == nil {
		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	return handlePathSwitchRequestSetupFailedTransfer(n2Data)
}

func handlePathSwitchRequestSetupFailedTransfer(b []byte) error {
	pathSwitchRequestSetupFailedTransfer := ngapType.PathSwitchRequestSetupFailedTransfer{}

	if err := aper.UnmarshalWithParams(b, &pathSwitchRequestSetupFailedTransfer, "valueExt"); err != nil {
		return fmt.Errorf("failed to unmarshall path switch request setup failed transfer: %s", err.Error())
	}

	return nil
}

// --- NGAP cause string helpers (moved from pdusession) ---

func radioNetworkCauseString(cause aper.Enumerated) string {
	switch cause {
	case ngapType.CauseRadioNetworkPresentUnspecified:
		return "unspecified"
	case ngapType.CauseRadioNetworkPresentTxnrelocoverallExpiry:
		return "txNRelocOverallExpiry"
	case ngapType.CauseRadioNetworkPresentSuccessfulHandover:
		return "successfulHandover"
	case ngapType.CauseRadioNetworkPresentReleaseDueToNgranGeneratedReason:
		return "releaseDueToNgranGeneratedReason"
	case ngapType.CauseRadioNetworkPresentReleaseDueTo5gcGeneratedReason:
		return "releaseDueTo5gcGeneratedReason"
	case ngapType.CauseRadioNetworkPresentHandoverCancelled:
		return "handoverCancelled"
	case ngapType.CauseRadioNetworkPresentPartialHandover:
		return "partialHandover"
	case ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem:
		return "hoFailureInTarget5GCNgranNodeOrTargetSystem"
	case ngapType.CauseRadioNetworkPresentHoTargetNotAllowed:
		return "hoTargetNotAllowed"
	case ngapType.CauseRadioNetworkPresentTngrelocoverallExpiry:
		return "tnGRelocOverallExpiry"
	case ngapType.CauseRadioNetworkPresentTngrelocprepExpiry:
		return "tnGRelocPrepExpiry"
	case ngapType.CauseRadioNetworkPresentCellNotAvailable:
		return "cellNotAvailable"
	case ngapType.CauseRadioNetworkPresentUnknownTargetID:
		return "unknownTargetID"
	case ngapType.CauseRadioNetworkPresentNoRadioResourcesAvailableInTargetCell:
		return "noRadioResourcesAvailableInTargetCell"
	case ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID:
		return "unknownLocalUENGAPID"
	case ngapType.CauseRadioNetworkPresentInconsistentRemoteUENGAPID:
		return "inconsistentRemoteUENGAPID"
	case ngapType.CauseRadioNetworkPresentHandoverDesirableForRadioReason:
		return "handoverDesirableForRadioReason"
	case ngapType.CauseRadioNetworkPresentTimeCriticalHandover:
		return "timeCriticalHandover"
	case ngapType.CauseRadioNetworkPresentResourceOptimisationHandover:
		return "resourceOptimisationHandover"
	case ngapType.CauseRadioNetworkPresentReduceLoadInServingCell:
		return "reduceLoadInServingCell"
	case ngapType.CauseRadioNetworkPresentUserInactivity:
		return "userInactivity"
	case ngapType.CauseRadioNetworkPresentRadioConnectionWithUeLost:
		return "radioConnectionWithUeLost"
	case ngapType.CauseRadioNetworkPresentRadioResourcesNotAvailable:
		return "radioResourcesNotAvailable"
	case ngapType.CauseRadioNetworkPresentInvalidQosCombination:
		return "invalidQosCombination"
	case ngapType.CauseRadioNetworkPresentFailureInRadioInterfaceProcedure:
		return "failureInRadioInterfaceProcedure"
	case ngapType.CauseRadioNetworkPresentInteractionWithOtherProcedure:
		return "interactionWithOtherProcedure"
	case ngapType.CauseRadioNetworkPresentUnknownPDUSessionID:
		return "unknownPDUSessionID"
	case ngapType.CauseRadioNetworkPresentUnkownQosFlowID:
		return "unkownQosFlowID"
	case ngapType.CauseRadioNetworkPresentMultiplePDUSessionIDInstances:
		return "multiplePDUSessionIDInstances"
	case ngapType.CauseRadioNetworkPresentMultipleQosFlowIDInstances:
		return "multipleQosFlowIDInstances"
	case ngapType.CauseRadioNetworkPresentEncryptionAndOrIntegrityProtectionAlgorithmsNotSupported:
		return "encryptionAndOrIntegrityProtectionAlgorithmsNotSupported"
	case ngapType.CauseRadioNetworkPresentNgIntraSystemHandoverTriggered:
		return "ngIntraSystemHandoverTriggered"
	case ngapType.CauseRadioNetworkPresentNgInterSystemHandoverTriggered:
		return "ngInterSystemHandoverTriggered"
	case ngapType.CauseRadioNetworkPresentXnHandoverTriggered:
		return "xnHandoverTriggered"
	case ngapType.CauseRadioNetworkPresentNotSupported5QIValue:
		return "notSupported5QIValue"
	case ngapType.CauseRadioNetworkPresentUeContextTransfer:
		return "ueContextTransfer"
	case ngapType.CauseRadioNetworkPresentImsVoiceEpsFallbackOrRatFallbackTriggered:
		return "imsVoiceEpsFallbackOrRatFallbackTriggered"
	case ngapType.CauseRadioNetworkPresentUpIntegrityProtectionNotPossible:
		return "upIntegrityProtectionNotPossible"
	case ngapType.CauseRadioNetworkPresentUpConfidentialityProtectionNotPossible:
		return "upConfidentialityProtectionNotPossible"
	case ngapType.CauseRadioNetworkPresentSliceNotSupported:
		return "sliceNotSupported"
	case ngapType.CauseRadioNetworkPresentUeInRrcInactiveStateNotReachable:
		return "ueInRrcInactiveStateNotReachable"
	case ngapType.CauseRadioNetworkPresentRedirection:
		return "redirection"
	case ngapType.CauseRadioNetworkPresentResourcesNotAvailableForTheSlice:
		return "resourcesNotAvailableForTheSlice"
	case ngapType.CauseRadioNetworkPresentUeMaxIntegrityProtectedDataRateReason:
		return "ueMaxIntegrityProtectedDataRateReason"
	case ngapType.CauseRadioNetworkPresentReleaseDueToCnDetectedMobility:
		return "releaseDueToCnDetectedMobility"
	case ngapType.CauseRadioNetworkPresentN26InterfaceNotAvailable:
		return "n26InterfaceNotAvailable"
	case ngapType.CauseRadioNetworkPresentReleaseDueToPreEmption:
		return "releaseDueToPreEmption"
	}

	return "unknown"
}

func transportCauseString(cause aper.Enumerated) string {
	switch cause {
	case ngapType.CauseTransportPresentTransportResourceUnavailable:
		return "transportResourceUnavailable"
	case ngapType.CauseTransportPresentUnspecified:
		return "unspecified"
	}

	return "unknown"
}

func nasCauseString(cause aper.Enumerated) string {
	switch cause {
	case ngapType.CauseNasPresentNormalRelease:
		return "normalRelease"
	case ngapType.CauseNasPresentAuthenticationFailure:
		return "authenticationFailure"
	case ngapType.CauseNasPresentDeregister:
		return "deregister"
	case ngapType.CauseNasPresentUnspecified:
		return "unspecified"
	}

	return "unknown"
}

func protocolCauseString(cause aper.Enumerated) string {
	switch cause {
	case ngapType.CauseProtocolPresentTransferSyntaxError:
		return "transferSyntaxError"
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorReject:
		return "abstractSyntaxErrorReject"
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorIgnoreAndNotify:
		return "abstractSyntaxErrorIgnoreAndNotify"
	case ngapType.CauseProtocolPresentMessageNotCompatibleWithReceiverState:
		return "messageNotCompatibleWithReceiverState"
	case ngapType.CauseProtocolPresentSemanticError:
		return "semanticError"
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorFalselyConstructedMessage:
		return "abstractSyntaxErrorFalselyConstructedMessage"
	case ngapType.CauseProtocolPresentUnspecified:
		return "unspecified"
	}

	return "unknown"
}

func miscCauseString(cause aper.Enumerated) string {
	switch cause {
	case ngapType.CauseMiscPresentControlProcessingOverload:
		return "controlProcessingOverload"
	case ngapType.CauseMiscPresentNotEnoughUserPlaneProcessingResources:
		return "notEnoughUserPlaneProcessingResources"
	case ngapType.CauseMiscPresentHardwareFailure:
		return "hardwareFailure"
	case ngapType.CauseMiscPresentOmIntervention:
		return "omIntervention"
	case ngapType.CauseMiscPresentUnknownPLMN:
		return "unknownPLMN"
	case ngapType.CauseMiscPresentUnspecified:
		return "unspecified"
	}

	return "unknown"
}
