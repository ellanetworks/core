// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pdusession

import (
	ctxt "context"
	"fmt"
	"net"

	amf_producer "github.com/ellanetworks/core/internal/amf/producer"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/pfcp"
	"github.com/ellanetworks/core/internal/smf/qos"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/smf")

func CreateSmContext(ctx ctxt.Context, supi string, pduSessionID uint8, dnn string, snssai *models.Snssai, n1Msg []byte) (string, []byte, error) {
	ctx, span := tracer.Start(ctx, "SMF Create SmContext",
		trace.WithAttributes(
			attribute.String("supi", supi),
			attribute.Int("pduSessionID", int(pduSessionID)),
		),
	)
	defer span.End()

	smContext := context.GetSMContext(context.CanonicalName(supi, pduSessionID))
	if smContext != nil {
		err := handlePduSessionContextReplacement(ctx, smContext)
		if err != nil {
			return "", nil, fmt.Errorf("failed to replace existing context")
		}
	}

	smContext = context.NewSMContext(supi, pduSessionID)

	smContextRef, pco, pduSessionType, dnnInfo, pduAddress, pti, errRsp, err := handlePDUSessionSMContextCreate(ctx, supi, dnn, snssai, n1Msg, smContext)
	if err != nil {
		return "", errRsp, fmt.Errorf("failed to create SM Context: %v", err)
	}

	if errRsp != nil {
		return "", errRsp, nil
	}

	err = sendPFCPRules(ctx, smContext)
	if err != nil {
		err := sendPduSessionEstablishmentReject(ctx, smContext, pti)
		if err != nil {
			return "", nil, fmt.Errorf("failed to send pdu session establishment reject n1 message: %v", err)
		}
		return "", nil, fmt.Errorf("failed to create SM Context: %v", err)
	}

	err = sendPduSessionEstablishmentAccept(ctx, smContext, pco, pduSessionType, dnnInfo, pduAddress, pti)
	if err != nil {
		return "", nil, fmt.Errorf("failed to send pdu session establishment accept n1 message: %v", err)
	}

	return smContextRef, nil, nil
}

func handlePduSessionContextReplacement(ctx ctxt.Context, smCtxt *context.SMContext) error {
	smCtxt.Mutex.Lock()
	defer smCtxt.Mutex.Unlock()

	context.RemoveSMContext(ctx, context.CanonicalName(smCtxt.Supi, smCtxt.PDUSessionID))

	// Check if UPF session set, send release
	if smCtxt.Tunnel != nil {
		err := releaseTunnel(ctx, smCtxt)
		if err != nil {
			logger.SmfLog.Error("release tunnel failed", zap.Error(err), zap.String("supi", smCtxt.Supi), zap.Uint8("pduSessionID", smCtxt.PDUSessionID))
		}
	}

	return nil
}

func handlePDUSessionSMContextCreate(
	ctx ctxt.Context,
	supi string,
	dnn string,
	snssai *models.Snssai,
	n1Msg []byte,
	smContext *context.SMContext,
) (
	string,
	*context.ProtocolConfigurationOptions,
	uint8,
	*context.SnssaiSmfDnnInfo,
	net.IP,
	uint8,
	[]byte,
	error,
) {
	m := nas.NewMessage()

	err := m.GsmMessageDecode(&n1Msg)
	if err != nil {
		return "", nil, 0, nil, nil, 0, nil, fmt.Errorf("error decoding NAS message: %v", err)
	}

	if m.GsmHeader.GetMessageType() != nas.MsgTypePDUSessionEstablishmentRequest {
		return "", nil, 0, nil, nil, 0, nil, fmt.Errorf("error decoding NAS message: %v", err)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	pti := m.PDUSessionEstablishmentRequest.GetPTI()

	smContext.Supi = supi
	smContext.Dnn = dnn
	smContext.Snssai = snssai

	subscriberPolicy, err := context.GetSubscriberPolicy(ctx, smContext.Supi)
	if err != nil {
		rsp, err := context.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GSMRequestRejectedUnspecified)
		if err != nil {
			logger.SmfLog.Error("failed to build PDU Session Establishment Reject message", zap.Error(err), zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))
		}
		return "", nil, 0, nil, nil, 0, rsp, fmt.Errorf("failed to find subscriber policy: %v", err)
	}

	dnnInfo, err := context.RetrieveDnnInformation(ctx, *snssai, dnn)
	if err != nil {
		logger.SmfLog.Warn("error retrieving DNN information", zap.String("SST", fmt.Sprintf("%d", snssai.Sst)), zap.String("SD", snssai.Sd), zap.String("DNN", dnn), zap.Error(err))
		rsp, err := context.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GMMDNNNotSupportedOrNotSubscribedInTheSlice)
		if err != nil {
			logger.SmfLog.Error("failed to build PDU Session Establishment Reject message", zap.Error(err), zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))
		}
		return "", nil, 0, nil, nil, 0, rsp, nil
	}

	smfSelf := context.SMFSelf()

	pduAddress, err := smfSelf.DBInstance.AllocateIP(ctx, smContext.Supi)
	if err != nil {
		rsp, err := context.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GSMInsufficientResources)
		if err != nil {
			logger.SmfLog.Error("failed to build PDU Session Establishment Reject message", zap.Error(err), zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))
		}
		return "", nil, 0, nil, nil, 0, rsp, nil
	}

	logger.SmfLog.Info("Successfully allocated IP address", zap.String("IP", pduAddress.String()), zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

	smContext.PDUSessionID = m.PDUSessionEstablishmentRequest.PDUSessionID.GetPDUSessionID()

	pco, err := handlePDUSessionEstablishmentRequest(m.PDUSessionEstablishmentRequest)
	if err != nil {
		logger.SmfLog.Error("failed to handle PDU Session Establishment Request", zap.Error(err), zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))
		response, err := context.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GSMRequestRejectedUnspecified)
		if err != nil {
			logger.SmfLog.Error("failed to build PDU Session Establishment Reject message", zap.Error(err), zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))
		}
		return "", nil, 0, nil, nil, 0, response, err
	}

	policyUpdates := qos.BuildSmPolicyUpdate(&smContext.SmPolicyData, subscriberPolicy)

	smContext.SmPolicyUpdates = policyUpdates

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

	err = defaultPath.ActivateTunnelAndPDR(smContext, pduAddress, 255)
	if err != nil {
		response, err := context.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GSMRequestRejectedUnspecified)
		if err != nil {
			logger.SmfLog.Error("failed to build PDU Session Establishment Reject message", zap.Error(err), zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))
		}
		return "", nil, 0, nil, nil, 0, response, fmt.Errorf("couldn't activate data path: %v", err)
	}

	allowedSessionType := context.GetAllowedSessionType()

	logger.SmfLog.Info("Successfully created PDU session context", zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

	return context.CanonicalName(smContext.Supi, smContext.PDUSessionID), pco, allowedSessionType, dnnInfo, pduAddress, pti, nil, nil
}

func handlePDUSessionEstablishmentRequest(req *nasMessage.PDUSessionEstablishmentRequest) (*context.ProtocolConfigurationOptions, error) {
	if req.PDUSessionType != nil {
		requestedPDUSessionType := req.PDUSessionType.GetPDUSessionTypeValue()
		if requestedPDUSessionType != nasMessage.PDUSessionTypeIPv4 && requestedPDUSessionType != nasMessage.PDUSessionTypeIPv4IPv6 {
			return nil, fmt.Errorf("requested PDUSessionType is invalid: %d", requestedPDUSessionType)
		}
	}

	pco := &context.ProtocolConfigurationOptions{}

	if req.ExtendedProtocolConfigurationOptions != nil {
		EPCOContents := req.ExtendedProtocolConfigurationOptions.GetExtendedProtocolConfigurationOptionsContents()
		protocolConfigurationOptions := nasConvert.NewProtocolConfigurationOptions()
		unmarshalErr := protocolConfigurationOptions.UnMarshal(EPCOContents)
		if unmarshalErr != nil {
			return nil, fmt.Errorf("parsing PCO failed: %v", unmarshalErr)
		}

		// Send MTU to UE always even if UE does not request it.
		// Preconfiguring MTU request flag.

		pco.IPv4LinkMTURequest = true

		for _, container := range protocolConfigurationOptions.ProtocolOrContainerList {
			switch container.ProtocolOrContainerID {
			case nasMessage.DNSServerIPv6AddressRequestUL:
				pco.DNSIPv6Request = true
			case nasMessage.DNSServerIPv4AddressRequestUL:
				pco.DNSIPv4Request = true
			default:
				continue
			}
		}
	}

	return pco, nil
}

// SendPFCPRules send all datapaths to UPFs
func sendPFCPRules(ctx ctxt.Context, smContext *context.SMContext) error {
	dataPath := smContext.Tunnel.DataPath
	if !dataPath.Activated {
		logger.SmfLog.Debug("DataPath is not activated, skip sending PFCP rules")
		return nil
	}

	curDataPathNode := dataPath.DPNode

	pdrList := make([]*context.PDR, 0, 2)
	farList := make([]*context.FAR, 0, 2)
	qerList := make([]*context.QER, 0, 2)
	urrList := make([]*context.URR, 0, 2)

	if curDataPathNode.UpLinkTunnel != nil && curDataPathNode.UpLinkTunnel.PDR != nil {
		pdrList = append(pdrList, curDataPathNode.UpLinkTunnel.PDR)
		farList = append(farList, curDataPathNode.UpLinkTunnel.PDR.FAR)
		if curDataPathNode.UpLinkTunnel.PDR.QER != nil {
			qerList = append(qerList, curDataPathNode.UpLinkTunnel.PDR.QER)
		}
		if curDataPathNode.UpLinkTunnel.PDR.URR != nil {
			urrList = append(urrList, curDataPathNode.UpLinkTunnel.PDR.URR)
		}
	}

	if curDataPathNode.DownLinkTunnel != nil && curDataPathNode.DownLinkTunnel.PDR != nil {
		pdrList = append(pdrList, curDataPathNode.DownLinkTunnel.PDR)
		farList = append(farList, curDataPathNode.DownLinkTunnel.PDR.FAR)

		if curDataPathNode.DownLinkTunnel.PDR.QER != nil {
			qerList = append(qerList, curDataPathNode.DownLinkTunnel.PDR.QER)
		}
		if curDataPathNode.DownLinkTunnel.PDR.URR != nil {
			urrList = append(urrList, curDataPathNode.DownLinkTunnel.PDR.URR)
		}
	}

	nodeID := curDataPathNode.UPF.NodeID

	sessionContext, exist := smContext.PFCPContext[curDataPathNode.GetNodeIP()]
	if !exist || sessionContext.RemoteSEID == 0 {
		err := pfcp.SendPfcpSessionEstablishmentRequest(ctx, sessionContext.LocalSEID, pdrList, farList, qerList, urrList)
		if err != nil {
			return fmt.Errorf("failed to send PFCP session establishment request: %v", err)
		}

		logger.SmfLog.Info("Sent PFCP session establishment request to upf", zap.String("nodeID", nodeID.String()))

		return nil
	}

	err := pfcp.SendPfcpSessionModificationRequest(ctx, sessionContext.LocalSEID, sessionContext.RemoteSEID, pdrList, farList, qerList)
	if err != nil {
		return fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}

	logger.SmfLog.Info("Sent PFCP session modification request to upf", zap.String("nodeID", nodeID.String()))

	return nil
}

func sendPduSessionEstablishmentReject(ctx ctxt.Context, smContext *context.SMContext, pti uint8) error {
	smNasBuf, err := context.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GSMRequestRejectedUnspecified)
	if err != nil {
		return fmt.Errorf("build GSM PDUSessionEstablishmentReject failed: %v", err)
	}

	err = amf_producer.TransferN1Msg(ctx, smContext.Supi, smNasBuf, smContext.PDUSessionID)
	if err != nil {
		err1 := smContext.CommitSmPolicyDecision(false)
		if err1 != nil {
			return fmt.Errorf("failed to commit sm policy decision: %v", err1)
		}

		return fmt.Errorf("failed to send n1 message: %v", err)
	}

	logger.SmfLog.Debug("Sent n1 message", zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

	err = smContext.CommitSmPolicyDecision(true)
	if err != nil {
		return fmt.Errorf("failed to commit sm policy decision: %v", err)
	}

	return nil
}

func sendPduSessionEstablishmentAccept(
	ctx ctxt.Context,
	smContext *context.SMContext,
	pco *context.ProtocolConfigurationOptions,
	pduSessionType uint8,
	dnnInfo *context.SnssaiSmfDnnInfo,
	pduAddress net.IP,
	pti uint8,
) error {
	n1Msg, err := context.BuildGSMPDUSessionEstablishmentAccept(smContext.SmPolicyUpdates, smContext.PDUSessionID, pti, smContext.Snssai, smContext.Dnn, pco, pduSessionType, dnnInfo, pduAddress)
	if err != nil {
		return fmt.Errorf("build GSM PDUSessionEstablishmentAccept failed: %v", err)
	}

	n2Msg, err := context.BuildPDUSessionResourceSetupRequestTransfer(smContext.SmPolicyUpdates, smContext.SmPolicyData, smContext.Tunnel.DataPath.DPNode)
	if err != nil {
		return fmt.Errorf("build PDUSessionResourceSetupRequestTransfer failed: %v", err)
	}

	n1n2Request := models.N1N2MessageTransferRequest{
		PduSessionID:            smContext.PDUSessionID,
		SNssai:                  smContext.Snssai,
		BinaryDataN1Message:     n1Msg,
		BinaryDataN2Information: n2Msg,
	}

	err = amf_producer.TransferN1N2Message(ctx, smContext.Supi, n1n2Request)
	if err != nil {
		err1 := smContext.CommitSmPolicyDecision(false)
		if err1 != nil {
			return fmt.Errorf("failed to commit sm policy decision: %v", err1)
		}

		return fmt.Errorf("failed to send n1 n2 transfer request: %v", err)
	}

	logger.SmfLog.Debug("Sent n1 n2 transfer request", zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

	err = smContext.CommitSmPolicyDecision(true)
	if err != nil {
		return fmt.Errorf("failed to commit sm policy decision: %v", err)
	}

	return nil
}
