// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package nas

import (
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
)

// BuildPDUSessionModificationCommand constructs a NAS PDU Session Modification
// Command message (TS 24.501 clause 8.3.9). It includes:
//   - Session-AMBR IE (when ambr is non-nil, clause 8.3.9.3)
//   - Authorized QoS flow descriptions IE (when qosData is non-nil, clause 8.3.9.8)
//   - Extended PCO with DNS server address(es) (when dns is non-nil, clause 6.3.2)
//
// At least one of ambr, qosData, or dns must be non-nil.
func BuildPDUSessionModificationCommand(pduSessionID uint8, ambr *models.Ambr, qosData *models.QosData, dns net.IP) ([]byte, error) {
	if ambr == nil && qosData == nil && dns == nil {
		return nil, fmt.Errorf("at least one of ambr, qosData, or dns must be provided")
	}

	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionModificationCommand)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionModificationCommand = nasMessage.NewPDUSessionModificationCommand(0x0)
	m.PDUSessionModificationCommand.SetPDUSessionID(pduSessionID)
	m.PDUSessionModificationCommand.SetMessageType(nas.MsgTypePDUSessionModificationCommand)
	m.PDUSessionModificationCommand.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)

	// Session-AMBR IE (TS 24.501 clause 8.3.9.3)
	if ambr != nil {
		sessAmbr, err := ModelsToSessionAMBR(ambr)
		if err != nil {
			return nil, fmt.Errorf("convert AMBR: %v", err)
		}

		sessAmbr.SetIei(nasMessage.PDUSessionModificationCommandSessionAMBRType)
		m.PDUSessionModificationCommand.SessionAMBR = &sessAmbr
		m.PDUSessionModificationCommand.SessionAMBR.SetLen(uint8(len(m.PDUSessionModificationCommand.SessionAMBR.Octet)))
	}

	// Authorized QoS flow descriptions IE (TS 24.501 clause 8.3.9.8)
	if qosData != nil {
		authQfd, err := BuildModifyQosFlowDescription(qosData)
		if err != nil {
			return nil, fmt.Errorf("build QoS flow descriptions: %v", err)
		}

		m.PDUSessionModificationCommand.AuthorizedQosFlowDescriptions = nasType.NewAuthorizedQosFlowDescriptions(nasMessage.PDUSessionModificationCommandAuthorizedQosFlowDescriptionsType)
		m.PDUSessionModificationCommand.AuthorizedQosFlowDescriptions.SetLen(authQfd.IeLen)
		m.PDUSessionModificationCommand.SetQoSFlowDescriptions(authQfd.Content)
	}

	// Extended PCO with DNS server address(es) (TS 24.501 clause 6.3.2)
	if dns != nil {
		m.PDUSessionModificationCommand.ExtendedProtocolConfigurationOptions = nasType.NewExtendedProtocolConfigurationOptions(
			nasMessage.PDUSessionModificationCommandExtendedProtocolConfigurationOptionsType,
		)
		protocolConfigurationOptions := nasConvert.NewProtocolConfigurationOptions()

		if dns.To4() != nil {
			if err := protocolConfigurationOptions.AddDNSServerIPv4Address(dns); err != nil {
				return nil, fmt.Errorf("encode DNS IPv4 address in PCO: %w", err)
			}
		} else {
			if err := protocolConfigurationOptions.AddDNSServerIPv6Address(dns); err != nil {
				return nil, fmt.Errorf("encode DNS IPv6 address in PCO: %w", err)
			}
		}

		pcoContents := protocolConfigurationOptions.Marshal()
		pcoContentsLength := len(pcoContents)
		m.PDUSessionModificationCommand.ExtendedProtocolConfigurationOptions.SetLen(uint16(pcoContentsLength))
		m.PDUSessionModificationCommand.SetExtendedProtocolConfigurationOptionsContents(pcoContents)
	}

	return m.PlainNasEncode()
}
