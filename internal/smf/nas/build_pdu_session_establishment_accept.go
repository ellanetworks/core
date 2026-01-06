// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package nas

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"go.uber.org/zap"
)

const (
	DefaultQosRuleID      uint8 = 1
	AllowedPDUSessionType       = nasMessage.PDUSessionTypeIPv4
)

type ProtocolConfigurationOptions struct {
	DNSIPv4Request     bool
	DNSIPv6Request     bool
	IPv4LinkMTURequest bool
}

func BuildGSMPDUSessionEstablishmentAccept(
	smPolicyData *models.SmPolicyData,
	pduSessionID uint8,
	pti uint8,
	snssai *models.Snssai,
	dnn string,
	pco *ProtocolConfigurationOptions,
	dNNInfo *context.SnssaiSmfDnnInfo,
	pduAddress net.IP,
) ([]byte, error) {
	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionEstablishmentAccept)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionEstablishmentAccept = nasMessage.NewPDUSessionEstablishmentAccept(0x0)
	m.PDUSessionEstablishmentAccept.SetPDUSessionID(pduSessionID)
	m.PDUSessionEstablishmentAccept.SetMessageType(nas.MsgTypePDUSessionEstablishmentAccept)
	m.PDUSessionEstablishmentAccept.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionEstablishmentAccept.SetPTI(pti)
	m.SetPDUSessionType(AllowedPDUSessionType)
	m.PDUSessionEstablishmentAccept.SetSSCMode(1)

	ambr, err := modelsToSessionAMBR(smPolicyData.Ambr)
	if err != nil {
		return nil, fmt.Errorf("failed to convert models to SessionAMBR: %v", err)
	}

	m.PDUSessionEstablishmentAccept.SessionAMBR = ambr
	m.PDUSessionEstablishmentAccept.SessionAMBR.SetLen(uint8(len(m.PDUSessionEstablishmentAccept.SessionAMBR.Octet)))

	qosRules := QoSRules{
		BuildDefaultQosRule(DefaultQosRuleID, smPolicyData.QosData.QFI),
	}

	qosRulesBytes, err := qosRules.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal QoS rules: %v", err)
	}

	m.PDUSessionEstablishmentAccept.AuthorizedQosRules.SetLen(uint16(len(qosRulesBytes)))
	m.PDUSessionEstablishmentAccept.SetQosRule(qosRulesBytes)

	if pduAddress != nil {
		addr, addrLen := pduAddressToNAS(pduAddress, AllowedPDUSessionType)
		m.PDUAddress = nasType.NewPDUAddress(nasMessage.PDUSessionEstablishmentAcceptPDUAddressType)
		m.PDUAddress.SetLen(addrLen)
		m.PDUSessionEstablishmentAccept.SetPDUSessionTypeValue(AllowedPDUSessionType)
		m.SetPDUAddressInformation(addr)
	}

	// Get Authorized QoS Flow Descriptions
	authQfd, err := BuildAuthorizedQosFlowDescription(smPolicyData.QosData)
	if err != nil {
		return nil, fmt.Errorf("failed to build Authorized QoS Flow Descriptions: %v", err)
	}

	// Add Default Qos Flow
	// authQfd.AddDefaultQosFlowDescription(smContext.SmPolicyUpdates[0].SessRuleUpdate.ActiveSessRule)

	m.PDUSessionEstablishmentAccept.AuthorizedQosFlowDescriptions = nasType.NewAuthorizedQosFlowDescriptions(nasMessage.PDUSessionEstablishmentAcceptAuthorizedQosFlowDescriptionsType)
	m.PDUSessionEstablishmentAccept.AuthorizedQosFlowDescriptions.SetLen(authQfd.IeLen)
	m.PDUSessionEstablishmentAccept.SetQoSFlowDescriptions(authQfd.Content)
	// pDUSessionEstablishmentAccept.AuthorizedQosFlowDescriptions.SetLen(6)
	// pDUSessionEstablishmentAccept.SetQoSFlowDescriptions([]uint8{uint8(authDefQos.Var5qi), 0x20, 0x41, 0x01, 0x01, 0x09})

	m.PDUSessionEstablishmentAccept.SNSSAI = nasType.NewSNSSAI(nasMessage.ULNASTransportSNSSAIType)
	m.PDUSessionEstablishmentAccept.SetSST(uint8(snssai.Sst))
	m.PDUSessionEstablishmentAccept.SNSSAI.SetLen(1)

	if snssai.Sd != "" {
		byteArray, err := hex.DecodeString(snssai.Sd)
		if err != nil {
			return nil, fmt.Errorf("failed to decode sd: %v", err)
		}

		var sd [3]uint8

		copy(sd[:], byteArray)

		m.PDUSessionEstablishmentAccept.SetSD(sd)
		m.PDUSessionEstablishmentAccept.SNSSAI.SetLen(4)
	}

	m.PDUSessionEstablishmentAccept.DNN = nasType.NewDNN(nasMessage.ULNASTransportDNNType)
	m.PDUSessionEstablishmentAccept.SetDNN(dnn)

	if pco.DNSIPv4Request || pco.DNSIPv6Request || pco.IPv4LinkMTURequest {
		m.PDUSessionEstablishmentAccept.ExtendedProtocolConfigurationOptions = nasType.NewExtendedProtocolConfigurationOptions(
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
		m.PDUSessionEstablishmentAccept.ExtendedProtocolConfigurationOptions.SetLen(uint16(pcoContentsLength))
		m.PDUSessionEstablishmentAccept.SetExtendedProtocolConfigurationOptionsContents(pcoContents)
	}

	return m.PlainNasEncode()
}

func modelsToSessionAMBR(ambr *models.Ambr) (nasType.SessionAMBR, error) {
	var sessAmbr nasType.SessionAMBR

	uplink := strings.Split(ambr.Uplink, " ")

	uplinkBitRate, err := strconv.ParseUint(uplink[0], 10, 16)
	if err != nil {
		return sessAmbr, fmt.Errorf("failed to parse uplink bitrate: %v", err)
	}

	var uplinkBitRateBytes [2]byte
	binary.BigEndian.PutUint16(uplinkBitRateBytes[:], uint16(uplinkBitRate))
	sessAmbr.SetSessionAMBRForUplink(uplinkBitRateBytes)
	sessAmbr.SetUnitForSessionAMBRForUplink(strToAMBRUnit(uplink[1]))

	downlink := strings.Split(ambr.Downlink, " ")

	downlinkBitRate, err := strconv.ParseUint(downlink[0], 10, 16)
	if err != nil {
		return sessAmbr, fmt.Errorf("failed to parse downlink bitrate: %v", err)
	}

	var downlinkBitRateBytes [2]byte
	binary.BigEndian.PutUint16(downlinkBitRateBytes[:], uint16(downlinkBitRate))
	sessAmbr.SetSessionAMBRForDownlink(downlinkBitRateBytes)

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

func pduAddressToNAS(pduAddress net.IP, pduSessionType uint8) ([12]byte, uint8) {
	var addr [12]byte

	copy(addr[:], pduAddress)

	switch pduSessionType {
	case nasMessage.PDUSessionTypeIPv4:
		return addr, 4 + 1
	case nasMessage.PDUSessionTypeIPv4IPv6:
		return addr, 12 + 1
	default:
		return addr, 0
	}
}
