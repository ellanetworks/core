// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/qos"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"go.uber.org/zap"
)

const (
	DefaultQosRuleID uint8 = 1
)

type ProtocolConfigurationOptions struct {
	DNSIPv4Request     bool
	DNSIPv6Request     bool
	IPv4LinkMTURequest bool
}

func BuildGSMPDUSessionEstablishmentAccept(
	smPolicyUpdates *qos.PolicyUpdate,
	pduSessionID uint8,
	pti uint8,
	snssai *models.Snssai,
	dnn string,
	pco *ProtocolConfigurationOptions,
	pduSessionType uint8,
	dNNInfo *SnssaiSmfDnnInfo,
	pduAddress net.IP,
) ([]byte, error) {
	if smPolicyUpdates == nil {
		return nil, fmt.Errorf("no SM Policy Update found in SM Context")
	}

	if smPolicyUpdates.SessRuleUpdate == nil {
		return nil, fmt.Errorf("no Session Rule Update found in SM Policy Update")
	}

	if smPolicyUpdates.QosFlowUpdate == nil {
		return nil, fmt.Errorf("no Qos Flow Update found in SM Policy Update")
	}

	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionEstablishmentAccept)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionEstablishmentAccept = nasMessage.NewPDUSessionEstablishmentAccept(0x0)
	pDUSessionEstablishmentAccept := m.PDUSessionEstablishmentAccept

	sessRule := smPolicyUpdates.SessRuleUpdate.ActiveSessRule

	pDUSessionEstablishmentAccept.SetPDUSessionID(pduSessionID)
	pDUSessionEstablishmentAccept.SetMessageType(nas.MsgTypePDUSessionEstablishmentAccept)
	pDUSessionEstablishmentAccept.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	pDUSessionEstablishmentAccept.SetPTI(pti)

	pDUSessionEstablishmentAccept.SetPDUSessionType(pduSessionType)

	pDUSessionEstablishmentAccept.SetSSCMode(1)

	ambr, err := modelsToSessionAMBR(sessRule.AuthSessAmbr)
	if err != nil {
		return nil, fmt.Errorf("failed to convert models to SessionAMBR: %v", err)
	}

	pDUSessionEstablishmentAccept.SessionAMBR = ambr
	pDUSessionEstablishmentAccept.SessionAMBR.SetLen(uint8(len(pDUSessionEstablishmentAccept.SessionAMBR.Octet)))

	defaultQFI := smPolicyUpdates.QosFlowUpdate.Add.QFI

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

	if pduAddress != nil {
		addr, addrLen := PDUAddressToNAS(pduAddress, pduSessionType)
		pDUSessionEstablishmentAccept.PDUAddress = nasType.NewPDUAddress(nasMessage.PDUSessionEstablishmentAcceptPDUAddressType)
		pDUSessionEstablishmentAccept.PDUAddress.SetLen(addrLen)
		pDUSessionEstablishmentAccept.PDUAddress.SetPDUSessionTypeValue(pduSessionType)
		pDUSessionEstablishmentAccept.PDUAddress.SetPDUAddressInformation(addr)
	}

	// Get Authorized QoS Flow Descriptions
	authQfd, err := qos.BuildAuthorizedQosFlowDescription(smPolicyUpdates.QosFlowUpdate.Add)
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
	pDUSessionEstablishmentAccept.SNSSAI.SetSST(uint8(snssai.Sst))
	pDUSessionEstablishmentAccept.SNSSAI.SetLen(1)

	if snssai.Sd != "" {
		byteArray, err := hex.DecodeString(snssai.Sd)
		if err != nil {
			return nil, fmt.Errorf("failed to decode sd: %v", err)
		}

		var sd [3]uint8

		copy(sd[:], byteArray)

		pDUSessionEstablishmentAccept.SNSSAI.SetSD(sd)
		pDUSessionEstablishmentAccept.SNSSAI.SetLen(4)
	}

	pDUSessionEstablishmentAccept.DNN = nasType.NewDNN(nasMessage.ULNASTransportDNNType)
	pDUSessionEstablishmentAccept.DNN.SetDNN(dnn)

	if pco.DNSIPv4Request || pco.DNSIPv6Request || pco.IPv4LinkMTURequest {
		pDUSessionEstablishmentAccept.ExtendedProtocolConfigurationOptions = nasType.NewExtendedProtocolConfigurationOptions(
			nasMessage.PDUSessionEstablishmentAcceptExtendedProtocolConfigurationOptionsType,
		)
		protocolConfigurationOptions := nasConvert.NewProtocolConfigurationOptions()

		// IPv4 DNS
		if pco.DNSIPv4Request {
			err := protocolConfigurationOptions.AddDNSServerIPv4Address(dNNInfo.DNS)
			if err != nil {
				logger.SmfLog.Warn("Error while adding DNS IPv4 Addr", zap.Error(err), zap.Uint8("pduSessionID", pduSessionID))
			}
		}

		// IPv6 DNS
		if pco.DNSIPv6Request {
			logger.SmfLog.Warn("IPv6 DNS request is not supported")
		}

		// MTU
		if pco.IPv4LinkMTURequest {
			err := protocolConfigurationOptions.AddIPv4LinkMTU(dNNInfo.MTU)
			if err != nil {
				logger.SmfLog.Warn("Error while adding MTU", zap.Error(err), zap.Uint8("pduSessionID", pduSessionID))
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

func BuildGSMPDUSessionEstablishmentReject(pduSessionID uint8, pti uint8, cause uint8) ([]byte, error) {
	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionEstablishmentReject)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionEstablishmentReject = nasMessage.NewPDUSessionEstablishmentReject(0x0)
	pDUSessionEstablishmentReject := m.PDUSessionEstablishmentReject

	pDUSessionEstablishmentReject.SetMessageType(nas.MsgTypePDUSessionEstablishmentReject)
	pDUSessionEstablishmentReject.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	pDUSessionEstablishmentReject.SetPDUSessionID(pduSessionID)
	pDUSessionEstablishmentReject.SetCauseValue(cause)
	pDUSessionEstablishmentReject.SetPTI(pti)

	return m.PlainNasEncode()
}

func BuildGSMPDUSessionReleaseCommand(pduSessionID uint8, pti uint8) ([]byte, error) {
	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionReleaseCommand)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionReleaseCommand = nasMessage.NewPDUSessionReleaseCommand(0x0)
	pDUSessionReleaseCommand := m.PDUSessionReleaseCommand

	pDUSessionReleaseCommand.SetMessageType(nas.MsgTypePDUSessionReleaseCommand)
	pDUSessionReleaseCommand.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	pDUSessionReleaseCommand.SetPDUSessionID(pduSessionID)
	pDUSessionReleaseCommand.SetPTI(pti)
	pDUSessionReleaseCommand.SetCauseValue(0x0)

	return m.PlainNasEncode()
}

func modelsToSessionAMBR(ambr *models.Ambr) (nasType.SessionAMBR, error) {
	var sessAmbr nasType.SessionAMBR
	uplink := strings.Split(ambr.Uplink, " ")
	if bitRate, err := strconv.ParseUint(uplink[0], 10, 16); err != nil {
		return sessAmbr, fmt.Errorf("failed to parse uplink bitrate: %v", err)
	} else {
		var bitRateBytes [2]byte
		binary.BigEndian.PutUint16(bitRateBytes[:], uint16(bitRate))
		sessAmbr.SetSessionAMBRForUplink(bitRateBytes)
	}
	sessAmbr.SetUnitForSessionAMBRForUplink(strToAMBRUnit(uplink[1]))

	downlink := strings.Split(ambr.Downlink, " ")
	if bitRate, err := strconv.ParseUint(downlink[0], 10, 16); err != nil {
		return sessAmbr, fmt.Errorf("failed to parse downlink bitrate: %v", err)
	} else {
		var bitRateBytes [2]byte
		binary.BigEndian.PutUint16(bitRateBytes[:], uint16(bitRate))
		sessAmbr.SetSessionAMBRForDownlink(bitRateBytes)
	}
	sessAmbr.SetUnitForSessionAMBRForDownlink(strToAMBRUnit(downlink[1]))
	return sessAmbr, nil
}

func strToAMBRUnit(unit string) uint8 {
	switch unit {
	case "bps":
		return nasMessage.SessionAMBRUnitNotUsed
	case "Kbps":
		return nasMessage.SessionAMBRUnit1Kbps
	case "Mbps":
		return nasMessage.SessionAMBRUnit1Mbps
	case "Gbps":
		return nasMessage.SessionAMBRUnit1Gbps
	case "Tbps":
		return nasMessage.SessionAMBRUnit1Tbps
	case "Pbps":
		return nasMessage.SessionAMBRUnit1Pbps
	}
	return nasMessage.SessionAMBRUnitNotUsed
}
