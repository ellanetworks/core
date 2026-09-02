// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/hex"
	"fmt"
	"net"
	"strings"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

type AMBR struct {
	Value uint64 `json:"value"`
	Unit  string `json:"unit"`
}

type SessionAMBR struct {
	Uplink   AMBR `json:"uplink"`
	Downlink AMBR `json:"downlink"`
}

type PDUSessionEstablishmentAccept struct {
	SelectedSSCMode                      uint8                                 `json:"selected_ssc_mode"`
	SelectedPDUSessionType               utils.EnumField                       `json:"selected_pdu_session_type"`
	AuthorizedQosRules                   []QosRule                             `json:"authorized_qos_rules"`
	SessionAMBR                          SessionAMBR                           `json:"session_ambr"`
	Cause5GSM                            *utils.EnumField                      `json:"cause_5g_s_m,omitempty"`
	PDUAddress                           *string                               `json:"pdu_address,omitempty"`
	SNSSAI                               *SNSSAI                               `json:"snssai,omitempty"`
	AuthorizedQosFlowDescriptions        []QoSFlowDescription                  `json:"authorized_qos_flow_descriptions,omitempty"`
	ExtendedProtocolConfigurationOptions *ExtendedProtocolConfigurationOptions `json:"extended_protocol_configuration_options,omitempty"`
	DNN                                  *string                               `json:"dnn,omitempty"`

	MappedEPSBearerContexts []MappedEPSBearerContext `json:"mapped_eps_bearer_contexts,omitempty"`

	RQTimerValue                 *GPRSTimer2Value `json:"rq_timer_value,omitempty"`
	AlwaysonPDUSessionIndication *bool            `json:"alwayson_pdu_session_indication,omitempty"`
	EAPMessage                   *RawOctets       `json:"eap_message,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildPDUSessionEstablishmentAccept(msg *fgs.PDUSessionEstablishmentAccept) *PDUSessionEstablishmentAccept {
	estAcc := &PDUSessionEstablishmentAccept{
		SelectedSSCMode:        uint8(msg.SSCMode),
		SelectedPDUSessionType: buildPDUSessionType(uint8(msg.PDUSessionType)),
		AuthorizedQosRules:     QosRulesFromNAS(msg.QoSRules),
		SessionAMBR:            buildSessionAMBR(msg.SessionAMBR),
	}

	if msg.Cause != nil {
		cause := cause5GSMToString(*msg.Cause)
		estAcc.Cause5GSM = &cause
	}

	if a := msg.PDUAddress; a != nil {
		address := net.IPv4(a.IPv4[0], a.IPv4[1], a.IPv4[2], a.IPv4[3]).String()
		estAcc.PDUAddress = &address
	}

	if s := msg.SNSSAI; s != nil {
		snssai := SNSSAI{SST: int32(s.SST)}
		if s.SD != nil {
			sd := strings.ToUpper(hex.EncodeToString(s.SD[:]))
			snssai.SD = &sd
		}

		estAcc.SNSSAI = &snssai
	}

	if msg.MappedEPSBearerContexts != nil {
		estAcc.MappedEPSBearerContexts = MappedEPSBearerContextsFromNAS(msg.MappedEPSBearerContexts)
	}

	if msg.QoSFlowDescriptions != nil {
		estAcc.AuthorizedQosFlowDescriptions = QosFlowDescriptionsFromNAS(msg.QoSFlowDescriptions)
	}

	if msg.ExtendedPCO != nil {
		estAcc.ExtendedProtocolConfigurationOptions = extendedPCOFromNAS(*msg.ExtendedPCO)
	}

	if msg.DNN != nil {
		name := string(*msg.DNN)
		estAcc.DNN = &name
	}

	estAcc.RQTimerValue = gprsTimer2(msg.RQTimer)
	estAcc.AlwaysonPDUSessionIndication = msg.AlwaysOn
	estAcc.EAPMessage = rawOctets(msg.EAP)
	estAcc.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return estAcc
}

func buildSessionAMBR(ambr fgs.SessionAMBR) SessionAMBR {
	// Uplink and downlink each carry their own unit octet (TS 24.501 §9.11.4.14).
	return SessionAMBR{
		Uplink:   AMBR{Value: uint64(ambr.Uplink), Unit: ambrUnitToString(uint8(ambr.UplinkUnit))},
		Downlink: AMBR{Value: uint64(ambr.Downlink), Unit: ambrUnitToString(uint8(ambr.DownlinkUnit))},
	}
}

// Session-AMBR unit values (TS 24.501 §9.11.4.14).
func ambrUnitToString(unit uint8) string {
	switch unit {
	case 0x00:
		// Table 9.11.4.14.1 NOTE: unit 0 is interpreted as multiples of 1 Kbps.
		return "Kbps"
	case 0x01:
		return "Kbps"
	case 0x06:
		return "Mbps"
	case 0x0B:
		return "Gbps"
	case 0x10:
		return "Tbps"
	case 0x15:
		return "Pbps"
	default:
		return fmt.Sprintf("Unknown(%d)", unit)
	}
}

func cause5GSMToString(cause fgs.GSMCause) utils.EnumField {
	return utils.NamedEnum(uint8(cause), cause.Name())
}
