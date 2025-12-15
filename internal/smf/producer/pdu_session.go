// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	ctxt "context"
	"fmt"

	amf_producer "github.com/ellanetworks/core/internal/amf/producer"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/pfcp"
	"github.com/ellanetworks/core/internal/smf/qos"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/smf/pdu")

func HandlePduSessionContextReplacement(ctx ctxt.Context, smCtxt *context.SMContext) error {
	smCtxt.Mutex.Lock()
	defer smCtxt.Mutex.Unlock()

	context.RemoveSMContext(ctx, context.CanonicalName(smCtxt.Supi, smCtxt.PDUSessionID))

	// Check if UPF session set, send release
	if smCtxt.Tunnel != nil {
		err := releaseTunnel(ctx, smCtxt)
		if err != nil {
			logger.SmfLog.Error("release tunnel failed", zap.Error(err), zap.String("supi", smCtxt.Supi), zap.Int32("pduSessionID", smCtxt.PDUSessionID))
		}
	}

	return nil
}

func HandlePDUSessionSMContextCreate(ctx ctxt.Context, request models.PostSmContextsRequest, smContext *context.SMContext) (string, *context.ProtocolConfigurationOptions, uint8, uint8, *context.SnssaiSmfDnnInfo, *models.PostSmContextsErrorResponse, error) {
	ctx, span := tracer.Start(ctx, "SMF Handle PDU Session SM Context Create")
	defer span.End()
	span.SetAttributes(
		attribute.String("supi", smContext.Supi),
		attribute.Int("pduSessionID", int(smContext.PDUSessionID)),
	)

	m := nas.NewMessage()

	err := m.GsmMessageDecode(&request.BinaryDataN1SmMessage)
	if err != nil {
		errRsp := &models.PostSmContextsErrorResponse{}
		return "", nil, 0, 0, nil, errRsp, fmt.Errorf("error decoding NAS message: %v", err)
	}

	if m.GsmHeader.GetMessageType() != nas.MsgTypePDUSessionEstablishmentRequest {
		errRsp := &models.PostSmContextsErrorResponse{}
		return "", nil, 0, 0, nil, errRsp, fmt.Errorf("error decoding NAS message: %v", err)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	smContext.SetCreateData(request.JSONData)

	subscriberPolicy, err := context.GetSubscriberPolicy(ctx, smContext.Supi)
	if err != nil {
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMRequestRejectedUnspecified)
		return "", nil, 0, 0, nil, response, fmt.Errorf("failed to find subscriber policy: %v", err)
	}

	dnnInfo, err := context.RetrieveDnnInformation(ctx, *request.JSONData.SNssai, request.JSONData.Dnn)
	if err != nil {
		logger.SmfLog.Warn("error retrieving DNN information", zap.String("SST", fmt.Sprintf("%d", request.JSONData.SNssai.Sst)), zap.String("SD", request.JSONData.SNssai.Sd), zap.String("DNN", request.JSONData.Dnn), zap.Error(err))
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GMMDNNNotSupportedOrNotSubscribedInTheSlice)
		return "", nil, 0, 0, nil, response, nil
	}

	// IP Allocation
	smfSelf := context.SMFSelf()
	ip, err := smfSelf.DBInstance.AllocateIP(ctx, smContext.Supi)
	if err != nil {
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMInsufficientResources)
		return "", nil, 0, 0, nil, response, nil
	}

	logger.SmfLog.Info("Successfully allocated IP address", zap.String("IP", ip.String()), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))

	smContext.PDUAddress = ip

	allowedSessionType := context.GetAllowedSessionType()

	pco, pduSessionType, estAcceptCause5gSMValue, err := smContext.HandlePDUSessionEstablishmentRequest(allowedSessionType, m.PDUSessionEstablishmentRequest)
	if err != nil {
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMRequestRejectedUnspecified)
		return "", nil, 0, 0, nil, response, err
	}

	policyUpdates := qos.BuildSmPolicyUpdate(&smContext.SmPolicyData, subscriberPolicy)

	smContext.SmPolicyUpdates = append(smContext.SmPolicyUpdates, policyUpdates)

	defaultPath := &context.DataPath{
		DPNode: &context.DataPathNode{
			UpLinkTunnel:   &context.GTPTunnel{},
			DownLinkTunnel: &context.GTPTunnel{},
			UPF:            smfSelf.UPF,
		},
	}

	smContext.Tunnel = &context.UPTunnel{
		DataPath: defaultPath,
	}

	err = defaultPath.ActivateTunnelAndPDR(smContext, 255)
	if err != nil {
		response := smContext.GeneratePDUSessionEstablishmentReject(nasMessage.Cause5GSMRequestRejectedUnspecified)
		return "", nil, 0, 0, nil, response, fmt.Errorf("couldn't activate data path: %v", err)
	}

	logger.SmfLog.Info("Successfully created PDU session context", zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))

	return context.CanonicalName(smContext.Supi, smContext.PDUSessionID), pco, pduSessionType, estAcceptCause5gSMValue, dnnInfo, nil, nil
}

func HandlePDUSessionSMContextUpdate(ctx ctxt.Context, request models.UpdateSmContextRequest, smContext *context.SMContext) (*models.UpdateSmContextResponse, error) {
	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	pfcpAction := &pfcpAction{}
	var response models.UpdateSmContextResponse
	response.JSONData = new(models.SmContextUpdatedData)

	err := HandleUpdateN1Msg(ctx, request, smContext, &response, pfcpAction)
	if err != nil {
		return nil, fmt.Errorf("error handling N1 message: %v", err)
	}

	pfcpParam := &pfcpParam{
		pdrList: []*context.PDR{},
		farList: []*context.FAR{},
		qerList: []*context.QER{},
	}

	// UP Cnx State handling
	if err := HandleUpCnxState(request, smContext, &response, pfcpAction, pfcpParam); err != nil {
		return nil, fmt.Errorf("error handling UP connection state: %v", err)
	}

	// N2 Msg Handling
	if err := HandleUpdateN2Msg(ctx, request, smContext, &response, pfcpAction, pfcpParam); err != nil {
		return nil, fmt.Errorf("error handling N2 message: %v", err)
	}

	// Ho state handling
	if err := HandleUpdateHoState(request, smContext, &response); err != nil {
		return nil, fmt.Errorf("error handling HO state: %v", err)
	}

	// Cause handling
	if err := HandleUpdateCause(request, smContext, &response, pfcpAction); err != nil {
		return nil, fmt.Errorf("error handling cause: %v", err)
	}

	// Initiate PFCP Release
	if pfcpAction.sendPfcpDelete {
		err := releaseTunnel(ctx, smContext)
		if err != nil {
			return nil, fmt.Errorf("failed to release tunnel: %v", err)
		}
	} else if pfcpAction.sendPfcpModify {
		dataPath := smContext.Tunnel.DataPath
		ANUPF := dataPath.DPNode

		sessionContext, exist := smContext.PFCPContext[ANUPF.UPF.NodeID.String()]
		if !exist {
			return nil, fmt.Errorf("pfcp session context not found for upf: %s", ANUPF.UPF.NodeID.String())
		}

		err := pfcp.SendPfcpSessionModificationRequest(ctx, sessionContext.LocalSEID, sessionContext.RemoteSEID, pfcpParam.pdrList, pfcpParam.farList, pfcpParam.qerList)
		if err != nil {
			return nil, fmt.Errorf("failed to send PFCP session modification request: %v", err)
		}

		logger.SmfLog.Info("Sent PFCP session modification request", zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
	}

	return &response, nil
}

func HandlePDUSessionSMContextRelease(ctx ctxt.Context, smContext *context.SMContext) error {
	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	err := smContext.ReleaseUeIPAddr(ctx)
	if err != nil {
		logger.SmfLog.Error("release UE IP address failed", zap.Error(err), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
	}

	// Release User-plane
	err = releaseTunnel(ctx, smContext)
	if err != nil {
		context.RemoveSMContext(ctx, context.CanonicalName(smContext.Supi, smContext.PDUSessionID))
		return fmt.Errorf("release tunnel failed: %v", err)
	}

	context.RemoveSMContext(ctx, context.CanonicalName(smContext.Supi, smContext.PDUSessionID))
	return nil
}

func releaseTunnel(ctx ctxt.Context, smContext *context.SMContext) error {
	if smContext.Tunnel == nil {
		return fmt.Errorf("tunnel not found")
	}

	smContext.Tunnel.DataPath.DeactivateTunnelAndPDR(smContext)

	err := pfcp.SendPfcpSessionDeletionRequest(ctx, smContext.Tunnel.DataPath.DPNode.UPF.NodeID, smContext)
	if err != nil {
		return fmt.Errorf("send PFCP session deletion request failed: %v", err)
	}

	smContext.Tunnel = nil

	return nil
}

func SendPduSessN1N2Transfer(ctx ctxt.Context, smContext *context.SMContext, pco *context.ProtocolConfigurationOptions, pduSessionType uint8, estAcceptCause5gSMValue uint8, dnnInfo *context.SnssaiSmfDnnInfo, success bool) error {
	n1n2Request := models.N1N2MessageTransferRequest{}

	n1n2Request.JSONData = &models.N1N2MessageTransferReqData{PduSessionID: smContext.PDUSessionID}

	if success {
		smNasBuf, err := context.BuildGSMPDUSessionEstablishmentAccept(smContext, pco, pduSessionType, estAcceptCause5gSMValue, dnnInfo)
		if err != nil {
			return fmt.Errorf("build GSM PDUSessionEstablishmentAccept failed: %v", err)
		}

		n1n2Request.BinaryDataN1Message = smNasBuf

		n2Pdu, err := context.BuildPDUSessionResourceSetupRequestTransfer(smContext)
		if err != nil {
			return fmt.Errorf("build PDUSessionResourceSetupRequestTransfer failed: %v", err)
		}

		n1n2Request.BinaryDataN2Information = n2Pdu
		n1n2Request.JSONData = &models.N1N2MessageTransferReqData{
			PduSessionID: smContext.PDUSessionID,
			NgapIeType:   models.N2SmInfoTypePduResSetupReq,
			SNssai:       smContext.Snssai,
		}
	} else {
		smNasBuf, err := context.BuildGSMPDUSessionEstablishmentReject(smContext, nasMessage.Cause5GSMRequestRejectedUnspecified)
		if err != nil {
			return fmt.Errorf("build GSM PDUSessionEstablishmentReject failed: %v", err)
		}

		n1n2Request.BinaryDataN1Message = smNasBuf
	}

	err := amf_producer.N1N2MessageTransferProcedure(ctx, smContext.Supi, n1n2Request)
	if err != nil {
		err1 := smContext.CommitSmPolicyDecision(false)
		if err1 != nil {
			return fmt.Errorf("failed to commit sm policy decision: %v", err1)
		}

		return fmt.Errorf("failed to send n1 n2 transfer request: %v", err)
	}

	logger.SmfLog.Debug("Sent n1 n2 transfer request", zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))

	err = smContext.CommitSmPolicyDecision(true)
	if err != nil {
		return fmt.Errorf("failed to commit sm policy decision: %v", err)
	}

	return nil
}
