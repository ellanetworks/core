// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package smf

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// netipToIP converts a netip.Addr to a net.IP for use with existing NAS/NGAP code.
func netipToIP(addr netip.Addr) net.IP {
	if addr.Is4() {
		b := addr.As4()
		return net.IP(b[:])
	}

	b := addr.As16()

	return net.IP(b[:])
}

// nasToNgapPDUSessionType maps a NAS PDU session type value to the NGAP
// equivalent used in PDUSessionResourceSetupRequestTransfer.
func nasToNgapPDUSessionType(nasType uint8) aper.Enumerated {
	switch nasType {
	case nasMessage.PDUSessionTypeIPv6:
		return ngapType.PDUSessionTypePresentIpv6
	case nasMessage.PDUSessionTypeIPv4IPv6:
		return ngapType.PDUSessionTypePresentIpv4v6
	default:
		return ngapType.PDUSessionTypePresentIpv4
	}
}

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
			s.releaseAllocatedAddresses(ctx, smContext)

			if smContext.Tunnel != nil {
				if err := s.releaseTunnel(ctx, smContext); err != nil {
					logger.WithTrace(ctx, logger.SmfLog).Error("release tunnel failed during cleanup", zap.Error(err))
				}
			}
			smContext.Mutex.Unlock()
			s.RemoveSession(ctx, smContext.CanonicalName())
		}
	}()

	pco, addrs, pti, policy, cause, errRsp, err := s.handlePDUSessionSMContextCreate(ctx, m, smContext)
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

	smContext.SetPolicyData(policy)

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

	err = s.sendPduSessionEstablishmentAccept(ctx, smContext, policy, pco, addrs, pti, cause)
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
	*smfNas.PDUSessionAddresses,
	uint8,
	*Policy,
	uint8,
	[]byte,
	error,
) {
	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	pti := m.PDUSessionEstablishmentRequest.GetPTI()

	policy, err := s.GetSessionPolicy(ctx, smContext.Supi, smContext.Snssai, smContext.Dnn)
	if err != nil {
		PDUSessionEstablishmentAttempts.WithLabelValues("reject").Inc()

		cause := nasMessage.Cause5GSMRequestRejectedUnspecified
		if errors.Is(err, ErrDNNNotFound) {
			cause = nasMessage.Cause5GMMDNNNotSupportedOrNotSubscribedInTheSlice
		}

		rsp, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, cause)
		if buildErr != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to build PDU Session Establishment Reject message", zap.Error(buildErr), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		}

		return nil, nil, 0, nil, 0, rsp, fmt.Errorf("failed to find subscriber policy: %v", err)
	}

	// Determine the requested PDU session type.
	requestedType := nasMessage.PDUSessionTypeIPv4 // default
	if m.PDUSessionType != nil {
		requestedType = m.PDUSessionEstablishmentRequest.GetPDUSessionTypeValue()
	}

	// Negotiate the PDU session type based on what the UE requested and
	// what pools are available on the data network.
	negotiatedType, err := s.negotiatePDUSessionType(ctx, requestedType, policy)
	if err != nil {
		PDUSessionEstablishmentAttempts.WithLabelValues("reject").Inc()

		// Determine the appropriate reject cause based on which pools are available.
		cause := nasMessage.Cause5GSMInsufficientResources
		if policy.IPPool != "" && policy.IPv6Pool == "" {
			cause = nasMessage.Cause5GSMPDUSessionTypeIPv4OnlyAllowed
		} else if policy.IPPool == "" && policy.IPv6Pool != "" {
			cause = nasMessage.Cause5GSMPDUSessionTypeIPv6OnlyAllowed
		}

		rsp, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, cause)
		if buildErr != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to build PDU Session Establishment Reject message", zap.Error(buildErr), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		}

		return nil, nil, 0, nil, 0, rsp, fmt.Errorf("PDU session type negotiation failed: %v", err)
	}

	smContext.PDUSessionType = negotiatedType

	// Compute the 5GSM cause for the PDU SESSION ESTABLISHMENT ACCEPT message.
	// Per TS 24.501 §6.4.1.3, when the UE requests IPv4v6 but the network
	// supports only a single stack, the SMF shall include the appropriate
	// cause (#50 or #51) in the ACCEPT message.
	var cause uint8

	if requestedType == nasMessage.PDUSessionTypeIPv4IPv6 && negotiatedType != nasMessage.PDUSessionTypeIPv4IPv6 {
		if negotiatedType == nasMessage.PDUSessionTypeIPv4 {
			cause = nasMessage.Cause5GSMPDUSessionTypeIPv4OnlyAllowed
		} else {
			cause = nasMessage.Cause5GSMPDUSessionTypeIPv6OnlyAllowed
		}
	}

	// Allocate addresses based on negotiated type.
	addrs := &smfNas.PDUSessionAddresses{PDUSessionType: negotiatedType}

	// The UE IP used for downlink PDR keying — for IPv4 or IPv4v6 this is
	// the IPv4 address; for IPv6-only it's the /64 prefix base.
	var dlPdrIP netip.Addr

	// IPv4 allocation (for IPv4 and IPv4v6).
	if negotiatedType == nasMessage.PDUSessionTypeIPv4 || negotiatedType == nasMessage.PDUSessionTypeIPv4IPv6 {
		ipv4Addr, allocErr := s.store.AllocateIP(ctx, smContext.Supi.IMSI(), smContext.Dnn, smContext.PDUSessionID)
		if allocErr != nil {
			PDUSessionEstablishmentAttempts.WithLabelValues("reject").Inc()

			rsp, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GSMInsufficientResources)
			if buildErr != nil {
				logger.WithTrace(ctx, logger.SmfLog).Error("failed to build PDU Session Establishment Reject message", zap.Error(buildErr), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
			}

			return nil, nil, 0, nil, 0, rsp, fmt.Errorf("failed to allocate IPv4 address: %v", allocErr)
		}

		smContext.PDUAddress = netipToIP(ipv4Addr)
		addrs.IPv4Address = smContext.PDUAddress
		dlPdrIP = ipv4Addr

		logger.WithTrace(ctx, logger.SmfLog).Info("Allocated IPv4 address", logger.IPAddress(ipv4Addr.String()), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
	}

	// IPv6 allocation (for IPv6 and IPv4v6).
	if negotiatedType == nasMessage.PDUSessionTypeIPv6 || negotiatedType == nasMessage.PDUSessionTypeIPv4IPv6 {
		ipv6Prefix, allocErr := s.store.AllocateIPv6(ctx, smContext.Supi.IMSI(), smContext.Dnn, smContext.PDUSessionID)
		if allocErr != nil {
			// Roll back IPv4 if dual-stack.
			if smContext.PDUAddress != nil {
				if _, releaseErr := s.store.ReleaseIP(ctx, smContext.Supi.IMSI(), smContext.Dnn, smContext.PDUSessionID); releaseErr != nil {
					logger.WithTrace(ctx, logger.SmfLog).Error("failed to release IPv4 after IPv6 allocation error", zap.Error(releaseErr))
				}

				smContext.PDUAddress = nil
			}

			PDUSessionEstablishmentAttempts.WithLabelValues("reject").Inc()

			rsp, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GSMInsufficientResources)
			if buildErr != nil {
				logger.WithTrace(ctx, logger.SmfLog).Error("failed to build PDU Session Establishment Reject message", zap.Error(buildErr), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
			}

			return nil, nil, 0, nil, 0, rsp, fmt.Errorf("failed to allocate IPv6 prefix: %v", allocErr)
		}

		smContext.PDUAddressIPv6 = netipToIP(ipv6Prefix)

		iid, iidErr := s.assignIID(smContext.Dnn)
		if iidErr != nil {
			// Roll back allocations.
			if _, releaseErr := s.store.ReleaseIPv6(ctx, smContext.Supi.IMSI(), smContext.Dnn, smContext.PDUSessionID); releaseErr != nil {
				logger.WithTrace(ctx, logger.SmfLog).Error("failed to release IPv6 after IID generation error", zap.Error(releaseErr))
			}

			if smContext.PDUAddress != nil {
				if _, releaseErr := s.store.ReleaseIP(ctx, smContext.Supi.IMSI(), smContext.Dnn, smContext.PDUSessionID); releaseErr != nil {
					logger.WithTrace(ctx, logger.SmfLog).Error("failed to release IPv4 after IID generation error", zap.Error(releaseErr))
				}
			}

			return nil, nil, 0, nil, 0, nil, fmt.Errorf("failed to generate IID: %v", iidErr)
		}

		smContext.IPv6IID = iid
		addrs.IPv6IID = iid

		// For IPv6-only, the downlink PDR is keyed on the /64 prefix base.
		if negotiatedType == nasMessage.PDUSessionTypeIPv6 {
			dlPdrIP = ipv6Prefix
		}

		logger.WithTrace(ctx, logger.SmfLog).Info("Allocated IPv6 prefix", logger.IPv6Prefix(ipv6Prefix.String()), logger.IPv6IID(fmt.Sprintf("%x", iid)), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
	}

	smContext.PDUSessionID = m.PDUSessionEstablishmentRequest.GetPDUSessionID()

	pco, err := parsePDUSessionRequest(m.PDUSessionEstablishmentRequest)
	if err != nil {
		logger.WithTrace(ctx, logger.SmfLog).Error("failed to handle PDU Session Establishment Request", zap.Error(err), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

		s.releaseAllocatedAddresses(ctx, smContext)

		PDUSessionEstablishmentAttempts.WithLabelValues("reject").Inc()

		response, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GSMRequestRejectedUnspecified)
		if buildErr != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to build PDU Session Establishment Reject message", zap.Error(buildErr), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		}

		return nil, nil, 0, nil, 0, response, err
	}

	defaultPath := &DataPath{
		UpLinkTunnel:   &GTPTunnel{},
		DownLinkTunnel: &GTPTunnel{},
	}

	smContext.Tunnel = &UPTunnel{
		DataPath: defaultPath,
	}

	err = defaultPath.ActivateTunnelAndPDR(s, smContext, policy, dlPdrIP)
	if err != nil {
		s.releaseAllocatedAddresses(ctx, smContext)

		PDUSessionEstablishmentAttempts.WithLabelValues("reject").Inc()

		response, buildErr := smfNas.BuildGSMPDUSessionEstablishmentReject(smContext.PDUSessionID, pti, nasMessage.Cause5GSMRequestRejectedUnspecified)
		if buildErr != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to build PDU Session Establishment Reject message", zap.Error(buildErr), logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))
		}

		return nil, nil, 0, nil, 0, response, fmt.Errorf("couldn't activate data path: %v", err)
	}

	logger.WithTrace(ctx, logger.SmfLog).Info("Successfully created PDU session context", logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	return pco, addrs, pti, policy, cause, nil, nil
}

// negotiatePDUSessionType resolves the final PDU session type based on
// what the UE requested and what pools are available. For single-stack
// requests the UE's choice is honored only if the corresponding pool exists;
// otherwise the request is rejected. For dual-stack (IPv4v6) requests the
// type is downgraded to whichever single stack is available.
func (s *SMF) negotiatePDUSessionType(_ context.Context, requested uint8, policy *Policy) (uint8, error) {
	hasIPv4 := policy.IPPool != ""
	hasIPv6 := policy.IPv6Pool != ""

	switch requested {
	case nasMessage.PDUSessionTypeIPv4:
		if hasIPv4 {
			return nasMessage.PDUSessionTypeIPv4, nil
		}

		return 0, fmt.Errorf("no IPv4 pool available for DNN")

	case nasMessage.PDUSessionTypeIPv6:
		if hasIPv6 {
			return nasMessage.PDUSessionTypeIPv6, nil
		}

		return 0, fmt.Errorf("no IPv6 pool available for DNN")

	case nasMessage.PDUSessionTypeIPv4IPv6:
		if hasIPv4 && hasIPv6 {
			return nasMessage.PDUSessionTypeIPv4IPv6, nil
		}

		if hasIPv4 {
			return nasMessage.PDUSessionTypeIPv4, nil
		}

		if hasIPv6 {
			return nasMessage.PDUSessionTypeIPv6, nil
		}

		return 0, fmt.Errorf("no IP pool available for DNN")

	default:
		return 0, fmt.Errorf("unsupported PDU session type: %d", requested)
	}
}

// releaseAllocatedAddresses releases any IP addresses that were allocated
// during session creation. Used on error paths.
func (s *SMF) releaseAllocatedAddresses(ctx context.Context, smContext *SMContext) {
	if smContext.PDUAddress != nil {
		if _, err := s.store.ReleaseIP(ctx, smContext.Supi.IMSI(), smContext.Dnn, smContext.PDUSessionID); err != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to release IPv4 address", zap.Error(err))
		}

		smContext.PDUAddress = nil
	}

	if smContext.PDUAddressIPv6 != nil {
		if _, err := s.store.ReleaseIPv6(ctx, smContext.Supi.IMSI(), smContext.Dnn, smContext.PDUSessionID); err != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to release IPv6 address", zap.Error(err))
		}

		smContext.PDUAddressIPv6 = nil
	}
}

func parsePDUSessionRequest(req *nasMessage.PDUSessionEstablishmentRequest) (*smfNas.ProtocolConfigurationOptions, error) {
	if req.PDUSessionType != nil {
		requestedPDUSessionType := req.GetPDUSessionTypeValue()
		if requestedPDUSessionType != nasMessage.PDUSessionTypeIPv4 &&
			requestedPDUSessionType != nasMessage.PDUSessionTypeIPv6 &&
			requestedPDUSessionType != nasMessage.PDUSessionTypeIPv4IPv6 {
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

	pdrList := make([]*PDR, 0, 3)
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

	if dataPath.SecondPDR != nil {
		pdrList = append(pdrList, dataPath.SecondPDR)

		if dataPath.SecondPDR.QER != nil {
			qerList = append(qerList, dataPath.SecondPDR.QER)
		}

		if dataPath.SecondPDR.URR != nil {
			urrList = append(urrList, dataPath.SecondPDR.URR)
		}
	}

	if smContext.PFCPContext == nil {
		span.RecordError(fmt.Errorf("PFCP context not initialized"))
		span.SetStatus(codes.Error, "PFCP context not initialized")

		return fmt.Errorf("PFCP context not initialized")
	}

	var policyID string
	if smContext.PolicyData != nil {
		policyID = smContext.PolicyData.PolicyID
	}

	if smContext.PFCPContext.RemoteSEID == 0 {
		req := BuildEstablishRequest(
			smContext.PFCPContext.LocalSEID,
			smContext.Supi.IMSI(),
			policyID,
			pdrList, farList, qerList, urrList,
		)

		resp, err := s.upf.EstablishSession(ctx, req)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to establish PFCP session")

			return fmt.Errorf("failed to send PFCP session establishment request: %v", err)
		}

		smContext.PFCPContext.RemoteSEID = resp.RemoteSEID

		for _, cp := range resp.CreatedPDRs {
			if cp.TEID != 0 {
				smContext.Tunnel.DataPath.UpLinkTunnel.TEID = cp.TEID
				smContext.Tunnel.DataPath.UpLinkTunnel.N3IPv4 = cp.N3IPv4
				smContext.Tunnel.DataPath.UpLinkTunnel.N3IPv6 = cp.N3IPv6

				break
			}
		}

		if dataPath.DownLinkTunnel != nil && dataPath.DownLinkTunnel.PDR != nil {
			dataPath.DownLinkTunnel.PDR.State = RuleCreate
		}

		if dataPath.SecondPDR != nil {
			dataPath.SecondPDR.State = RuleCreate
		}

		return nil
	}

	err := s.upf.ModifySession(ctx, BuildModifyRequest(
		smContext.PFCPContext.RemoteSEID,
		policyID,
		pdrList, farList, qerList,
	))
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
	addrs *smfNas.PDUSessionAddresses,
	pti uint8,
	cause uint8,
) error {
	ctx, span := tracer.Start(ctx, "smf/send_pdu_session_establishment_accept",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	PDUSessionEstablishmentAttempts.WithLabelValues("accept").Inc()

	n1Msg, err := smfNas.BuildGSMPDUSessionEstablishmentAccept(&policy.Ambr, &policy.QosData, smContext.PDUSessionID, pti, smContext.Snssai, smContext.Dnn, pco, policy.DNS, policy.MTU, cause, addrs)
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
