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

// PDUSessionAddresses holds the address information for a PDU session.
// For IPv4-only, only IPv4Address is set. For IPv6-only, only IPv6IID is set.
// For IPv4v6, both are set.
type PDUSessionAddresses struct {
	PDUSessionType uint8   // nasMessage.PDUSessionTypeIPv4/IPv6/IPv4IPv6
	IPv4Address    net.IP  // 4-byte IPv4 address (nil for IPv6-only)
	IPv6IID        [8]byte // Interface Identifier (zero for IPv4-only)
}

func BuildGSMPDUSessionEstablishmentAccept(
	ambr *models.Ambr,
	qosData *models.QosData,
	pduSessionID uint8,
	pti uint8,
	snssai *models.Snssai,
	dnn string,
	pco *ProtocolConfigurationOptions,
	dns net.IP,
	mtu uint16,
	cause uint8,
	addrs *PDUSessionAddresses,
) ([]byte, error) {
	pduSessionType := nasMessage.PDUSessionTypeIPv4
	if addrs != nil {
		pduSessionType = addrs.PDUSessionType
	}

	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionEstablishmentAccept)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionEstablishmentAccept = nasMessage.NewPDUSessionEstablishmentAccept(0x0)
	m.PDUSessionEstablishmentAccept.SetPDUSessionID(pduSessionID)
	m.PDUSessionEstablishmentAccept.SetMessageType(nas.MsgTypePDUSessionEstablishmentAccept)
	m.PDUSessionEstablishmentAccept.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionEstablishmentAccept.SetPTI(pti)
	m.SetPDUSessionType(pduSessionType)
	m.PDUSessionEstablishmentAccept.SetSSCMode(1)

	if cause != 0 {
		m.PDUSessionEstablishmentAccept.Cause5GSM = nasType.NewCause5GSM(nasMessage.PDUSessionEstablishmentAcceptCause5GSMType)
		m.PDUSessionEstablishmentAccept.SetCauseValue(cause)
	}

	sessAmbr, err := modelsToSessionAMBR(ambr)
	if err != nil {
		return nil, fmt.Errorf("failed to convert models to SessionAMBR: %v", err)
	}

	m.PDUSessionEstablishmentAccept.SessionAMBR = sessAmbr
	m.PDUSessionEstablishmentAccept.SessionAMBR.SetLen(uint8(len(m.PDUSessionEstablishmentAccept.SessionAMBR.Octet)))

	qosRules := QoSRules{
		BuildDefaultQosRule(DefaultQosRuleID, qosData.QFI),
	}

	qosRulesBytes, err := qosRules.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal QoS rules: %v", err)
	}

	m.PDUSessionEstablishmentAccept.AuthorizedQosRules.SetLen(uint16(len(qosRulesBytes)))
	m.PDUSessionEstablishmentAccept.SetQosRule(qosRulesBytes)

	if addrs != nil {
		addr, addrLen := pduAddressToNAS(addrs)
		m.PDUAddress = nasType.NewPDUAddress(nasMessage.PDUSessionEstablishmentAcceptPDUAddressType)
		m.PDUAddress.SetLen(addrLen)
		m.PDUSessionEstablishmentAccept.SetPDUSessionTypeValue(pduSessionType)
		m.SetPDUAddressInformation(addr)
	}

	// Get Authorized QoS Flow Descriptions
	authQfd, err := BuildAuthorizedQosFlowDescription(qosData)
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
			err := protocolConfigurationOptions.AddDNSServerIPv4Address(dns)
			if err != nil {
				logger.SmfLog.Warn("Error while adding DNS IPv4 Addr", zap.Error(err), logger.PDUSessionID(pduSessionID))
			}
		}

		// IPv6 DNS
		if pco.DNSIPv6Request {
			err := protocolConfigurationOptions.AddDNSServerIPv6Address(dns)
			if err != nil {
				logger.SmfLog.Warn("Error while adding DNS IPv6 Addr", zap.Error(err), logger.PDUSessionID(pduSessionID))
			}
		}

		// MTU
		if pco.IPv4LinkMTURequest {
			err := protocolConfigurationOptions.AddIPv4LinkMTU(mtu)
			if err != nil {
				logger.SmfLog.Warn("Error while adding MTU", zap.Error(err), logger.PDUSessionID(pduSessionID))
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

// pduAddressToNAS encodes the PDU address for the NAS wire format.
// Per 3GPP TS 24.501 §9.11.4.10:
//   - IPv4:   1 byte type + 4 bytes IPv4 address  → len 5
//   - IPv6:   1 byte type + 8 bytes IID            → len 9
//   - IPv4v6: 1 byte type + 8 bytes IID + 4 bytes IPv4 → len 13
func pduAddressToNAS(addrs *PDUSessionAddresses) ([12]byte, uint8) {
	var addr [12]byte

	switch addrs.PDUSessionType {
	case nasMessage.PDUSessionTypeIPv4:
		copy(addr[:], addrs.IPv4Address.To4())
		return addr, 4 + 1
	case nasMessage.PDUSessionTypeIPv6:
		copy(addr[:8], addrs.IPv6IID[:])
		return addr, 8 + 1
	case nasMessage.PDUSessionTypeIPv4IPv6:
		copy(addr[:8], addrs.IPv6IID[:])
		copy(addr[8:12], addrs.IPv4Address.To4())

		return addr, 12 + 1
	default:
		return addr, 0
	}
}
