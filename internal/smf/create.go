// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package smf

import (
	"context"
	"fmt"
	"net"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// CreateSmContext creates a new PDU session. It decodes the NAS message, retrieves the
// subscriber policy and DNN info, allocates an IP, creates the data path, sends
// PFCP rules to the UPF, and delivers the accept/reject to the AMF.
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

	// Decode the NAS message before making any state changes so we can
	// validate prerequisites and build a proper reject if needed.
	m := nas.NewMessage()

	if err := m.GsmMessageDecode(&n1Msg); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to decode NAS message")

		return "", nil, fmt.Errorf("error decoding NAS message: %v", err)
	}

	if m.GsmHeader.GetMessageType() != nas.MsgTypePDUSessionEstablishmentRequest {
		return "", nil, fmt.Errorf("unexpected NAS message type: %d", m.GsmHeader.GetMessageType())
	}

	pti := m.PDUSessionEstablishmentRequest.GetPTI()

	// Validate that the requested DNN exists before replacing any existing
	// session. Without this guard a request for a non-existent DNN (e.g.
	// "ims") destroys the active session sharing this PDU session ID,
	// releasing its IP for re-allocation to another subscriber.
	if _, err := s.GetDataNetwork(ctx, snssai, dnn); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "DNN not available")
		logger.WithTrace(ctx, logger.SmfLog).Warn("PDU session rejected: DNN not available",
			logger.DNN(dnn), logger.SUPI(supi.String()), zap.Error(err))

		PDUSessionEstablishmentAttempts.WithLabelValues("reject").Inc()

		rsp, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(pduSessionID, pti, nasMessage.Cause5GMMDNNNotSupportedOrNotSubscribedInTheSlice)
		if buildErr != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to build PDU Session Establishment Reject", zap.Error(buildErr))
		}

		return "", rsp, fmt.Errorf("DNN %q not available: %v", dnn, err)
	}

	smContext := s.GetSession(CanonicalName(supi, pduSessionID))
	if smContext != nil {
		s.handlePduSessionContextReplacement(ctx, smContext)
	}

	smContext = s.NewSession(supi, pduSessionID, dnn, snssai)

	// Clean up the session on any failure path to avoid leaking IPs and IDs.
	success := false

	defer func() {
		if !success {
			smContext.Mutex.Lock()
			if smContext.Tunnel != nil {
				if err := s.releaseTunnel(ctx, smContext); err != nil {
					logger.WithTrace(ctx, logger.SmfLog).Error("release tunnel failed during cleanup", zap.Error(err))
				}
			}
			smContext.Mutex.Unlock()
			s.RemoveSession(ctx, smContext.CanonicalName())
		}
	}()

	pco, dnnInfo, pduAddress, _, policy, errRsp, err := s.handlePDUSessionSMContextCreate(ctx, m, smContext)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create SM context")

		return "", errRsp, fmt.Errorf("failed to create SM Context: %v", err)
	}

	if errRsp != nil {
		return "", errRsp, nil
	}

	span.AddEvent("nas_decoded")
	span.AddEvent("policy_retrieved")
	span.AddEvent("ip_allocated", trace.WithAttributes(attribute.String("ip", pduAddress.String())))

	err = s.sendPFCPRules(ctx, smContext)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to send PFCP rules")

		sendErr := s.sendPduSessionEstablishmentReject(ctx, smContext, pti)
		if sendErr != nil {
			return "", nil, fmt.Errorf("failed to send pdu session establishment reject n1 message: %v", sendErr)
		}

		return "", nil, fmt.Errorf("failed to create SM Context: %v", err)
	}

	span.AddEvent("pfcp_rules_sent")

	err = s.sendPduSessionEstablishmentAccept(ctx, smContext, policy, pco, dnnInfo, pduAddress, pti)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to send PDU session establishment accept")

		return "", nil, fmt.Errorf("failed to send pdu session establishment accept n1 message: %v", err)
	}

	span.AddEvent("session_accepted")

	success = true

	return smContext.CanonicalName(), nil, nil
}

func (s *SMF) handlePduSessionContextReplacement(ctx context.Context, smCtxt *SMContext) {
	smCtxt.Mutex.Lock()
	defer smCtxt.Mutex.Unlock()

	s.RemoveSession(ctx, smCtxt.CanonicalName())

	if smCtxt.Tunnel != nil {
		err := s.releaseTunnel(ctx, smCtxt)
		if err != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("release tunnel failed", zap.Error(err), logger.SUPI(smCtxt.Supi.String()), logger.PDUSessionID(smCtxt.PDUSessionID))
		}
	}
}

func (s *SMF) handlePDUSessionSMContextCreate(
	ctx context.Context,
	m *nas.Message,
	smContext *SMContext,
) (
	*smfNas.ProtocolConfigurationOptions,
	*DataNetworkInfo,
	net.IP,
	uint8,
	*Policy,
	[]byte,
	error,
) {
	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	pti := m.PDUSessionEstablishmentRequest.GetPTI()

	policy, err := s.GetSubscriberPolicy(ctx, smContext.Supi)
	if err != nil {
		PDUSessionEstablishmentAttempts.WithLabelValues("reject").Inc()

		rsp, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GSMRequestRejectedUnspecified)
		if buildErr != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to build PDU Session Establishment Reject message", zap.Error(buildErr), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		}

		return nil, nil, nil, 0, nil, rsp, fmt.Errorf("failed to find subscriber policy: %v", err)
	}

	dnnInfo, err := s.GetDataNetwork(ctx, smContext.Snssai, smContext.Dnn)
	if err != nil {
		logger.WithTrace(ctx, logger.SmfLog).Warn("error retrieving DNN information", logger.SST(uint8(smContext.Snssai.Sst)), logger.SD(smContext.Snssai.Sd), logger.DNN(smContext.Dnn), zap.Error(err))

		PDUSessionEstablishmentAttempts.WithLabelValues("reject").Inc()

		rsp, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GMMDNNNotSupportedOrNotSubscribedInTheSlice)
		if buildErr != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to build PDU Session Establishment Reject message", zap.Error(buildErr), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		}

		return nil, nil, nil, 0, nil, rsp, fmt.Errorf("failed to retrieve DNN information: %v", err)
	}

	pduAddress, err := s.store.AllocateIP(ctx, smContext.Supi.IMSI())
	if err != nil {
		PDUSessionEstablishmentAttempts.WithLabelValues("reject").Inc()

		rsp, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GSMInsufficientResources)
		if buildErr != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to build PDU Session Establishment Reject message", zap.Error(buildErr), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		}

		return nil, nil, nil, 0, nil, rsp, fmt.Errorf("failed to allocate IP address: %v", err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Info("Successfully allocated IP address", logger.IPAddress(pduAddress.String()), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	smContext.PDUAddress = pduAddress
	smContext.PDUSessionID = m.PDUSessionEstablishmentRequest.GetPDUSessionID()

	pco, err := parsePDUSessionRequest(m.PDUSessionEstablishmentRequest)
	if err != nil {
		logger.WithTrace(ctx, logger.SmfLog).Error("failed to handle PDU Session Establishment Request", zap.Error(err), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

		if releaseErr := s.store.ReleaseIP(ctx, smContext.Supi.IMSI(), pduAddress); releaseErr != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to release IP after session create error", zap.Error(releaseErr))
		}

		PDUSessionEstablishmentAttempts.WithLabelValues("reject").Inc()

		response, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GSMRequestRejectedUnspecified)
		if buildErr != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to build PDU Session Establishment Reject message", zap.Error(buildErr), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		}

		return nil, nil, nil, 0, nil, response, err
	}

	defaultPath := &DataPath{
		UpLinkTunnel:   &GTPTunnel{},
		DownLinkTunnel: &GTPTunnel{},
	}

	smContext.Tunnel = &UPTunnel{
		DataPath: defaultPath,
	}

	err = defaultPath.ActivateTunnelAndPDR(s, smContext, policy, pduAddress)
	if err != nil {
		if releaseErr := s.store.ReleaseIP(ctx, smContext.Supi.IMSI(), pduAddress); releaseErr != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to release IP after session create error", zap.Error(releaseErr))
		}

		PDUSessionEstablishmentAttempts.WithLabelValues("reject").Inc()

		response, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GSMRequestRejectedUnspecified)
		if buildErr != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to build PDU Session Establishment Reject message", zap.Error(buildErr), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		}

		return nil, nil, nil, 0, nil, response, fmt.Errorf("couldn't activate data path: %v", err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Info("Successfully created PDU session context", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	return pco, dnnInfo, pduAddress, pti, policy, nil, nil
}

func parsePDUSessionRequest(req *nasMessage.PDUSessionEstablishmentRequest) (*smfNas.ProtocolConfigurationOptions, error) {
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

func (s *SMF) sendPFCPRules(ctx context.Context, smContext *SMContext) error {
	ctx, span := tracer.Start(ctx, "smf/send_pfcp_rules",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	dataPath := smContext.Tunnel.DataPath
	if !dataPath.Activated {
		logger.WithTrace(ctx, logger.SmfLog).Debug("DataPath is not activated, skip sending PFCP rules")
		return nil
	}

	pdrList := make([]*PDR, 0, 2)
	farList := make([]*FAR, 0, 2)
	qerList := make([]*QER, 0, 2)
	urrList := make([]*URR, 0, 2)

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

	if smContext.PFCPContext == nil {
		span.RecordError(fmt.Errorf("PFCP context not initialized"))
		span.SetStatus(codes.Error, "PFCP context not initialized")

		return fmt.Errorf("PFCP context not initialized")
	}

	if smContext.PFCPContext.RemoteSEID == 0 {
		result, err := s.upf.EstablishSession(ctx, &PFCPEstablishmentRequest{
			NodeID:    s.nodeID,
			LocalSEID: smContext.PFCPContext.LocalSEID,
			PDRs:      pdrList,
			FARs:      farList,
			QERs:      qerList,
			URRs:      urrList,
			SUPI:      smContext.Supi.IMSI(),
		})
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to establish PFCP session")

			return fmt.Errorf("failed to send PFCP session establishment request: %v", err)
		}

		smContext.PFCPContext.RemoteSEID = result.RemoteSEID
		smContext.Tunnel.DataPath.UpLinkTunnel.TEID = result.TEID
		smContext.Tunnel.DataPath.UpLinkTunnel.N3IP = result.N3IP

		return nil
	}

	err := s.upf.ModifySession(ctx, &PFCPModificationRequest{
		LocalSEID:  smContext.PFCPContext.LocalSEID,
		RemoteSEID: smContext.PFCPContext.RemoteSEID,
		PDRs:       pdrList,
		FARs:       farList,
		QERs:       qerList,
		URRs:       urrList,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to modify PFCP session")

		return fmt.Errorf("failed to send PFCP session modification request: %v", err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Info("Sent PFCP session modification request to upf")

	return nil
}

func (s *SMF) sendPduSessionEstablishmentReject(ctx context.Context, smContext *SMContext, pti uint8) error {
	ctx, span := tracer.Start(ctx, "smf/send_pdu_session_establishment_reject",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	PDUSessionEstablishmentAttempts.WithLabelValues("reject").Inc()

	smNasBuf, err := smfNas.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GSMRequestRejectedUnspecified)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to build PDU session establishment reject")

		return fmt.Errorf("build GSM PDUSessionEstablishmentReject failed: %v", err)
	}

	err = s.amf.TransferN1(ctx, smContext.Supi, smNasBuf, smContext.PDUSessionID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to transfer N1 message")

		return fmt.Errorf("failed to send n1 message: %v", err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Debug("Sent n1 message", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	return nil
}

func (s *SMF) sendPduSessionEstablishmentAccept(
	ctx context.Context,
	smContext *SMContext,
	policy *Policy,
	pco *smfNas.ProtocolConfigurationOptions,
	dnnInfo *DataNetworkInfo,
	pduAddress net.IP,
	pti uint8,
) error {
	ctx, span := tracer.Start(ctx, "smf/send_pdu_session_establishment_accept",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	PDUSessionEstablishmentAttempts.WithLabelValues("accept").Inc()

	n1Msg, err := smfNas.BuildGSMPDUSessionEstablishmentAccept(&policy.Ambr, &policy.QosData, smContext.PDUSessionID, pti, smContext.Snssai, smContext.Dnn, pco, dnnInfo.DNS, dnnInfo.MTU, pduAddress)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to build PDU session establishment accept")

		return fmt.Errorf("build GSM PDUSessionEstablishmentAccept failed: %v", err)
	}

	n2Msg, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(&policy.Ambr, &policy.QosData, smContext.Tunnel.DataPath.UpLinkTunnel.TEID, smContext.Tunnel.DataPath.UpLinkTunnel.N3IP)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to build PDU session resource setup request transfer")

		return fmt.Errorf("build PDUSessionResourceSetupRequestTransfer failed: %v", err)
	}

	err = s.amf.TransferN1N2(ctx, smContext.Supi, smContext.PDUSessionID, smContext.Snssai, n1Msg, n2Msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to transfer N1N2 message")

		return fmt.Errorf("failed to send n1 n2 transfer request: %v", err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Debug("Sent n1 n2 transfer request", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	smContext.SetPolicyData(policy)

	return nil
}
