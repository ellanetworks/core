// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pdusession

import (
	"context"
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/amf/producer"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	smfContext "github.com/ellanetworks/core/internal/smf/context"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/ellanetworks/core/internal/smf/pfcp"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/smf")

func CreateSmContext(ctx context.Context, supi string, pduSessionID uint8, dnn string, snssai *models.Snssai, n1Msg []byte) (string, []byte, error) {
	ctx, span := tracer.Start(ctx, "SMF create session",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("ue.supi", supi),
			attribute.Int("smf.pdu_session_id", int(pduSessionID)),
			attribute.String("smf.dnn", dnn),
		),
	)
	defer span.End()

	smf := smfContext.SMFSelf()

	smContext := smf.GetSMContext(smfContext.CanonicalName(supi, pduSessionID))
	if smContext != nil {
		err := handlePduSessionContextReplacement(ctx, smf, smContext)
		if err != nil {
			return "", nil, fmt.Errorf("failed to replace existing context")
		}
	}

	smContext = smf.NewSMContext(supi, pduSessionID, dnn, snssai)

	pco, dnnInfo, pduAddress, pti, smPolicyUpdates, errRsp, err := handlePDUSessionSMContextCreate(ctx, smf, n1Msg, smContext)
	if err != nil {
		return "", errRsp, fmt.Errorf("failed to create SM Context: %v", err)
	}

	if errRsp != nil {
		return "", errRsp, nil
	}

	err = sendPFCPRules(ctx, smf.CPNodeID, smContext)
	if err != nil {
		err := sendPduSessionEstablishmentReject(ctx, smContext, smPolicyUpdates, pti)
		if err != nil {
			return "", nil, fmt.Errorf("failed to send pdu session establishment reject n1 message: %v", err)
		}

		return "", nil, fmt.Errorf("failed to create SM Context: %v", err)
	}

	err = sendPduSessionEstablishmentAccept(ctx, smContext, smPolicyUpdates, pco, dnnInfo, pduAddress, pti)
	if err != nil {
		return "", nil, fmt.Errorf("failed to send pdu session establishment accept n1 message: %v", err)
	}

	return smContext.CanonicalName(), nil, nil
}

func handlePduSessionContextReplacement(ctx context.Context, smf *smfContext.SMF, smCtxt *smfContext.SMContext) error {
	smCtxt.Mutex.Lock()
	defer smCtxt.Mutex.Unlock()

	smf.RemoveSMContext(ctx, smCtxt.CanonicalName())

	// Check if UPF session set, send release
	if smCtxt.Tunnel != nil {
		err := releaseTunnel(ctx, smf, smCtxt)
		if err != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("release tunnel failed", zap.Error(err), zap.String("supi", smCtxt.Supi), zap.Uint8("pduSessionID", smCtxt.PDUSessionID))
		}
	}

	return nil
}

func handlePDUSessionSMContextCreate(
	ctx context.Context,
	smf *smfContext.SMF,
	n1Msg []byte,
	smContext *smfContext.SMContext,
) (
	*smfNas.ProtocolConfigurationOptions,
	*smfContext.SnssaiSmfDnnInfo,
	net.IP,
	uint8,
	*models.SmPolicyData,
	[]byte,
	error,
) {
	m := nas.NewMessage()

	err := m.GsmMessageDecode(&n1Msg)
	if err != nil {
		return nil, nil, nil, 0, nil, nil, fmt.Errorf("error decoding NAS message: %v", err)
	}

	if m.GsmHeader.GetMessageType() != nas.MsgTypePDUSessionEstablishmentRequest {
		return nil, nil, nil, 0, nil, nil, fmt.Errorf("error decoding NAS message: %v", err)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	pti := m.PDUSessionEstablishmentRequest.GetPTI()

	subscriberPolicy, err := smf.GetSubscriberPolicy(ctx, smContext.Supi)
	if err != nil {
		PDUSessionEstablishmentAttempts.WithLabelValues("reject").Inc()

		rsp, err := smfNas.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GSMRequestRejectedUnspecified)
		if err != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to build PDU Session Establishment Reject message", zap.Error(err), zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))
		}

		return nil, nil, nil, 0, nil, rsp, fmt.Errorf("failed to find subscriber policy: %v", err)
	}

	dnnInfo, err := smf.RetrieveDnnInformation(ctx, smContext.Snssai, smContext.Dnn)
	if err != nil {
		logger.WithTrace(ctx, logger.SmfLog).Warn("error retrieving DNN information", zap.String("SST", fmt.Sprintf("%d", smContext.Snssai.Sst)), zap.String("SD", smContext.Snssai.Sd), zap.String("DNN", smContext.Dnn), zap.Error(err))

		PDUSessionEstablishmentAttempts.WithLabelValues("reject").Inc()

		rsp, err1 := smfNas.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GMMDNNNotSupportedOrNotSubscribedInTheSlice)
		if err1 != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to build PDU Session Establishment Reject message", zap.Error(err), zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))
		}

		return nil, nil, nil, 0, nil, rsp, fmt.Errorf("failed to retrieve DNN information: %v", err)
	}

	pduAddress, err := smf.DBInstance.AllocateIP(ctx, smContext.Supi)
	if err != nil {
		PDUSessionEstablishmentAttempts.WithLabelValues("reject").Inc()

		rsp, err1 := smfNas.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GSMInsufficientResources)
		if err1 != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to build PDU Session Establishment Reject message", zap.Error(err), zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))
		}

		return nil, nil, nil, 0, nil, rsp, fmt.Errorf("failed to allocate IP address: %v", err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Info("Successfully allocated IP address", zap.String("IP", pduAddress.String()), zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

	smContext.PDUSessionID = m.PDUSessionEstablishmentRequest.GetPDUSessionID()

	pco, err := handlePDUSessionEstablishmentRequest(m.PDUSessionEstablishmentRequest)
	if err != nil {
		logger.WithTrace(ctx, logger.SmfLog).Error("failed to handle PDU Session Establishment Request", zap.Error(err), zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

		PDUSessionEstablishmentAttempts.WithLabelValues("reject").Inc()

		response, err := smfNas.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GSMRequestRejectedUnspecified)
		if err != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to build PDU Session Establishment Reject message", zap.Error(err), zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))
		}

		return nil, nil, nil, 0, nil, response, err
	}

	defaultPath := &smfContext.DataPath{
		UpLinkTunnel:   &smfContext.GTPTunnel{},
		DownLinkTunnel: &smfContext.GTPTunnel{},
	}

	smContext.Tunnel = &smfContext.UPTunnel{
		DataPath: defaultPath,
	}

	err = defaultPath.ActivateTunnelAndPDR(smf, smContext, subscriberPolicy, pduAddress)
	if err != nil {
		PDUSessionEstablishmentAttempts.WithLabelValues("reject").Inc()

		response, err := smfNas.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GSMRequestRejectedUnspecified)
		if err != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to build PDU Session Establishment Reject message", zap.Error(err), zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))
		}

		return nil, nil, nil, 0, nil, response, fmt.Errorf("couldn't activate data path: %v", err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Info("Successfully created PDU session context", zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

	return pco, dnnInfo, pduAddress, pti, subscriberPolicy, nil, nil
}

func handlePDUSessionEstablishmentRequest(req *nasMessage.PDUSessionEstablishmentRequest) (*smfNas.ProtocolConfigurationOptions, error) {
	if req.PDUSessionType != nil {
		requestedPDUSessionType := req.GetPDUSessionTypeValue()
		if requestedPDUSessionType != nasMessage.PDUSessionTypeIPv4 && requestedPDUSessionType != nasMessage.PDUSessionTypeIPv4IPv6 {
			return nil, fmt.Errorf("requested PDUSessionType is invalid: %d", requestedPDUSessionType)
		}
	}

	pco := &smfNas.ProtocolConfigurationOptions{}

	if req.ExtendedProtocolConfigurationOptions != nil {
		EPCOContents := req.GetExtendedProtocolConfigurationOptionsContents()
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
func sendPFCPRules(ctx context.Context, cpNodeID net.IP, smContext *smfContext.SMContext) error {
	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	dataPath := smContext.Tunnel.DataPath
	if !dataPath.Activated {
		logger.WithTrace(ctx, logger.SmfLog).Debug("DataPath is not activated, skip sending PFCP rules")
		return nil
	}

	pdrList := make([]*smfContext.PDR, 0, 2)
	farList := make([]*smfContext.FAR, 0, 2)
	qerList := make([]*smfContext.QER, 0, 2)
	urrList := make([]*smfContext.URR, 0, 2)

	if dataPath.UpLinkTunnel != nil && dataPath.UpLinkTunnel.PDR != nil {
		pdrList = append(pdrList, dataPath.UpLinkTunnel.PDR)

		farList = append(farList, dataPath.UpLinkTunnel.PDR.FAR)
		if dataPath.UpLinkTunnel.PDR.QER != nil {
			qerList = append(qerList, dataPath.UpLinkTunnel.PDR.QER)
		}

		if dataPath.UpLinkTunnel.PDR.URR != nil {
			urrList = append(urrList, dataPath.UpLinkTunnel.PDR.URR)
		}
	}

	if dataPath.DownLinkTunnel != nil && dataPath.DownLinkTunnel.PDR != nil {
		pdrList = append(pdrList, dataPath.DownLinkTunnel.PDR)
		farList = append(farList, dataPath.DownLinkTunnel.PDR.FAR)

		if dataPath.DownLinkTunnel.PDR.QER != nil {
			qerList = append(qerList, dataPath.DownLinkTunnel.PDR.QER)
		}

		if dataPath.DownLinkTunnel.PDR.URR != nil {
			urrList = append(urrList, dataPath.DownLinkTunnel.PDR.URR)
		}
	}

	if smContext.PFCPContext == nil || smContext.PFCPContext.RemoteSEID == 0 {
		result, err := pfcp.SendPfcpSessionEstablishmentRequest(ctx, cpNodeID, smContext.PFCPContext.LocalSEID, pdrList, farList, qerList, urrList)
		if err != nil {
			return fmt.Errorf("failed to send PFCP session establishment request: %v", err)
		}

		smContext.PFCPContext.RemoteSEID = result.RemoteSEID
		smContext.Tunnel.DataPath.UpLinkTunnel.TEID = result.TEID
		smContext.Tunnel.DataPath.UpLinkTunnel.N3IP = result.N3IP

		return nil
	}

	err := pfcp.SendPfcpSessionModificationRequest(ctx, cpNodeID, smContext.PFCPContext.LocalSEID, smContext.PFCPContext.RemoteSEID, pdrList, farList, qerList)
	if err != nil {
		return fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Info("Sent PFCP session modification request to upf")

	return nil
}

func sendPduSessionEstablishmentReject(ctx context.Context, smContext *smfContext.SMContext, smPolicyUpdates *models.SmPolicyData, pti uint8) error {
	PDUSessionEstablishmentAttempts.WithLabelValues("reject").Inc()

	smNasBuf, err := smfNas.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GSMRequestRejectedUnspecified)
	if err != nil {
		return fmt.Errorf("build GSM PDUSessionEstablishmentReject failed: %v", err)
	}

	err = producer.TransferN1Msg(ctx, smContext.Supi, smNasBuf, smContext.PDUSessionID)
	if err != nil {
		return fmt.Errorf("failed to send n1 message: %v", err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Debug("Sent n1 message", zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

	smContext.SetSMPolicyData(smPolicyUpdates)

	return nil
}

func sendPduSessionEstablishmentAccept(
	ctx context.Context,
	smContext *smfContext.SMContext,
	smPolicyUpdates *models.SmPolicyData,
	pco *smfNas.ProtocolConfigurationOptions,
	dnnInfo *smfContext.SnssaiSmfDnnInfo,
	pduAddress net.IP,
	pti uint8,
) error {
	PDUSessionEstablishmentAttempts.WithLabelValues("accept").Inc()

	n1Msg, err := smfNas.BuildGSMPDUSessionEstablishmentAccept(smPolicyUpdates, smContext.PDUSessionID, pti, smContext.Snssai, smContext.Dnn, pco, dnnInfo, pduAddress)
	if err != nil {
		return fmt.Errorf("build GSM PDUSessionEstablishmentAccept failed: %v", err)
	}

	n2Msg, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(smPolicyUpdates, smContext.Tunnel.DataPath.UpLinkTunnel.TEID, smContext.Tunnel.DataPath.UpLinkTunnel.N3IP)
	if err != nil {
		return fmt.Errorf("build PDUSessionResourceSetupRequestTransfer failed: %v", err)
	}

	n1n2Request := models.N1N2MessageTransferRequest{
		PduSessionID:            smContext.PDUSessionID,
		SNssai:                  smContext.Snssai,
		BinaryDataN1Message:     n1Msg,
		BinaryDataN2Information: n2Msg,
	}

	err = producer.TransferN1N2Message(ctx, smContext.Supi, n1n2Request)
	if err != nil {
		return fmt.Errorf("failed to send n1 n2 transfer request: %v", err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Debug("Sent n1 n2 transfer request", zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))

	smContext.SetSMPolicyData(smPolicyUpdates)

	return nil
}
