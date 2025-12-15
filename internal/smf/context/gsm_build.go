// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/qos"
	"github.com/ellanetworks/core/internal/smf/util"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"go.uber.org/zap"
)

const (
	DefaultQosRuleID uint8 = 1
)

func BuildGSMPDUSessionEstablishmentAccept(smContext *SMContext, pco *ProtocolConfigurationOptions, pduSessionType uint8) ([]byte, error) {
	if smContext == nil {
		return nil, fmt.Errorf("SM Context is nil")
	}

	if len(smContext.SmPolicyUpdates) == 0 {
		return nil, fmt.Errorf("no SM Policy Update found in SM Context")
	}

	if smContext.SmPolicyUpdates[0].SessRuleUpdate == nil {
		return nil, fmt.Errorf("no Session Rule Update found in SM Policy Update")
	}

	if smContext.SmPolicyUpdates[0].QosFlowUpdate == nil {
		return nil, fmt.Errorf("no Qos Flow Update found in SM Policy Update")
	}

	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionEstablishmentAccept)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionEstablishmentAccept = nasMessage.NewPDUSessionEstablishmentAccept(0x0)
	pDUSessionEstablishmentAccept := m.PDUSessionEstablishmentAccept

	sessRule := smContext.SmPolicyUpdates[0].SessRuleUpdate.ActiveSessRule

	pDUSessionEstablishmentAccept.SetPDUSessionID(uint8(smContext.PDUSessionID))
	pDUSessionEstablishmentAccept.SetMessageType(nas.MsgTypePDUSessionEstablishmentAccept)
	pDUSessionEstablishmentAccept.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	pDUSessionEstablishmentAccept.SetPTI(smContext.Pti)

	if v := smContext.EstAcceptCause5gSMValue; v != 0 {
		pDUSessionEstablishmentAccept.Cause5GSM = nasType.NewCause5GSM(nasMessage.PDUSessionEstablishmentAcceptCause5GSMType)
		pDUSessionEstablishmentAccept.Cause5GSM.SetCauseValue(v)
	}
	pDUSessionEstablishmentAccept.SetPDUSessionType(pduSessionType)

	pDUSessionEstablishmentAccept.SetSSCMode(1)
	ambr, err := util.ModelsToSessionAMBR(sessRule.AuthSessAmbr)
	if err != nil {
		return nil, fmt.Errorf("failed to convert models to SessionAMBR: %v", err)
	}
	pDUSessionEstablishmentAccept.SessionAMBR = ambr
	pDUSessionEstablishmentAccept.SessionAMBR.SetLen(uint8(len(pDUSessionEstablishmentAccept.SessionAMBR.Octet)))

	defaultQFI := smContext.SmPolicyUpdates[0].QosFlowUpdate.Add.QFI

	defQosRule := qos.BuildDefaultQosRule(DefaultQosRuleID, defaultQFI)
	qosRules := qos.QoSRules{
		defQosRule,
	}

	logger.SmfLog.Debug("Built qos rules", zap.Any("QoS Rules", qosRules))

	qosRulesBytes, err := qosRules.MarshalBinary()
	if err != nil {
		return nil, err
	}

	pDUSessionEstablishmentAccept.AuthorizedQosRules.SetLen(uint16(len(qosRulesBytes)))
	pDUSessionEstablishmentAccept.AuthorizedQosRules.SetQosRule(qosRulesBytes)

	if smContext.PDUAddress != nil {
		addr, addrLen := smContext.PDUAddressToNAS(pduSessionType)
		pDUSessionEstablishmentAccept.PDUAddress = nasType.NewPDUAddress(nasMessage.PDUSessionEstablishmentAcceptPDUAddressType)
		pDUSessionEstablishmentAccept.PDUAddress.SetLen(addrLen)
		pDUSessionEstablishmentAccept.PDUAddress.SetPDUSessionTypeValue(pduSessionType)
		pDUSessionEstablishmentAccept.PDUAddress.SetPDUAddressInformation(addr)
	}

	// Get Authorized QoS Flow Descriptions
	authQfd, err := qos.BuildAuthorizedQosFlowDescription(smContext.SmPolicyUpdates[0].QosFlowUpdate.Add)
	if err != nil {
		return nil, fmt.Errorf("failed to build Authorized QoS Flow Descriptions: %v", err)
	}

	// Add Default Qos Flow
	// authQfd.AddDefaultQosFlowDescription(smContext.SmPolicyUpdates[0].SessRuleUpdate.ActiveSessRule)

	pDUSessionEstablishmentAccept.AuthorizedQosFlowDescriptions = nasType.NewAuthorizedQosFlowDescriptions(nasMessage.PDUSessionEstablishmentAcceptAuthorizedQosFlowDescriptionsType)
	pDUSessionEstablishmentAccept.AuthorizedQosFlowDescriptions.SetLen(authQfd.IeLen)
	pDUSessionEstablishmentAccept.SetQoSFlowDescriptions(authQfd.Content)
	// pDUSessionEstablishmentAccept.AuthorizedQosFlowDescriptions.SetLen(6)
	// pDUSessionEstablishmentAccept.SetQoSFlowDescriptions([]uint8{uint8(authDefQos.Var5qi), 0x20, 0x41, 0x01, 0x01, 0x09})

	pDUSessionEstablishmentAccept.SNSSAI = nasType.NewSNSSAI(nasMessage.ULNASTransportSNSSAIType)
	pDUSessionEstablishmentAccept.SNSSAI.SetSST(uint8(smContext.Snssai.Sst))
	pDUSessionEstablishmentAccept.SNSSAI.SetLen(1)

	if smContext.Snssai.Sd != "" {
		byteArray, err := hex.DecodeString(smContext.Snssai.Sd)
		if err != nil {
			return nil, fmt.Errorf("failed to decode sd: %v", err)
		}

		var sd [3]uint8

		copy(sd[:], byteArray)

		pDUSessionEstablishmentAccept.SNSSAI.SetSD(sd)
		pDUSessionEstablishmentAccept.SNSSAI.SetLen(4)
	}

	pDUSessionEstablishmentAccept.DNN = nasType.NewDNN(nasMessage.ULNASTransportDNNType)
	pDUSessionEstablishmentAccept.DNN.SetDNN(smContext.Dnn)

	if pco.DNSIPv4Request || pco.DNSIPv6Request || pco.IPv4LinkMTURequest {
		pDUSessionEstablishmentAccept.ExtendedProtocolConfigurationOptions = nasType.NewExtendedProtocolConfigurationOptions(
			nasMessage.PDUSessionEstablishmentAcceptExtendedProtocolConfigurationOptionsType,
		)
		protocolConfigurationOptions := nasConvert.NewProtocolConfigurationOptions()

		// IPv4 DNS
		if pco.DNSIPv4Request {
			err := protocolConfigurationOptions.AddDNSServerIPv4Address(smContext.DNNInfo.DNS)
			if err != nil {
				logger.SmfLog.Warn("Error while adding DNS IPv4 Addr", zap.Error(err), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
			}
		}

		// IPv6 DNS
		if pco.DNSIPv6Request {
			logger.SmfLog.Warn("IPv6 DNS request is not supported")
		}

		// MTU
		if pco.IPv4LinkMTURequest {
			err := protocolConfigurationOptions.AddIPv4LinkMTU(smContext.DNNInfo.MTU)
			if err != nil {
				logger.SmfLog.Warn("Error while adding MTU", zap.Error(err), zap.String("supi", smContext.Supi), zap.Int32("pduSessionID", smContext.PDUSessionID))
			}
		}

		pcoContents := protocolConfigurationOptions.Marshal()
		pcoContentsLength := len(pcoContents)
		pDUSessionEstablishmentAccept.
			ExtendedProtocolConfigurationOptions.
			SetLen(uint16(pcoContentsLength))
		pDUSessionEstablishmentAccept.
			ExtendedProtocolConfigurationOptions.
			SetExtendedProtocolConfigurationOptionsContents(pcoContents)
	}
	return m.PlainNasEncode()
}

func BuildGSMPDUSessionEstablishmentReject(smContext *SMContext, cause uint8) ([]byte, error) {
	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionEstablishmentReject)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionEstablishmentReject = nasMessage.NewPDUSessionEstablishmentReject(0x0)
	pDUSessionEstablishmentReject := m.PDUSessionEstablishmentReject

	pDUSessionEstablishmentReject.SetMessageType(nas.MsgTypePDUSessionEstablishmentReject)
	pDUSessionEstablishmentReject.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	pDUSessionEstablishmentReject.SetPDUSessionID(uint8(smContext.PDUSessionID))
	pDUSessionEstablishmentReject.SetCauseValue(cause)
	pDUSessionEstablishmentReject.SetPTI(smContext.Pti)

	return m.PlainNasEncode()
}

func BuildGSMPDUSessionReleaseCommand(smContext *SMContext) ([]byte, error) {
	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionReleaseCommand)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionReleaseCommand = nasMessage.NewPDUSessionReleaseCommand(0x0)
	pDUSessionReleaseCommand := m.PDUSessionReleaseCommand

	pDUSessionReleaseCommand.SetMessageType(nas.MsgTypePDUSessionReleaseCommand)
	pDUSessionReleaseCommand.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	pDUSessionReleaseCommand.SetPDUSessionID(uint8(smContext.PDUSessionID))
	pDUSessionReleaseCommand.SetPTI(smContext.Pti)
	pDUSessionReleaseCommand.SetCauseValue(0x0)

	return m.PlainNasEncode()
}
