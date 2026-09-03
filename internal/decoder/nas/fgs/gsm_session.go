// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	nasie "github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/internal/decoder/utils"
	naslib "github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// GSMCauseOnly is the shape of a 5GSM message whose only content beyond the
// shared header is a cause: the session id and transaction are rendered on the
// GSM header, so the cause is all that is left.
type GSMCauseOnly struct {
	Cause5GSM utils.EnumField `json:"cause_5g_s_m"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func gsmCauseOnly(c fgs.GSMCause, unrecognized []naslib.RawIE) *GSMCauseOnly {
	return &GSMCauseOnly{Cause5GSM: cause5GSMToString(c), UnrecognizedIEs: utils.RawIEs(unrecognized)}
}

// GSMOptionalCause is a 5GSM message whose cause the sender may omit.
type GSMOptionalCause struct {
	Cause5GSM *utils.EnumField `json:"cause_5g_s_m,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func gsmOptionalCause(c *fgs.GSMCause, unrecognized []naslib.RawIE) *GSMOptionalCause {
	out := &GSMOptionalCause{UnrecognizedIEs: utils.RawIEs(unrecognized)}

	if c != nil {
		cause := cause5GSMToString(*c)
		out.Cause5GSM = &cause
	}

	return out
}

// PDUSessionModificationRequest is the UE asking to change a live session: new
// QoS rules or flows, and the mapped EPS bearers that follow it to 4G
// (TS 24.501 §8.3.7).
type PDUSessionModificationRequest struct {
	Capability5GSM                       *Capability5GSM                     `json:"capability_5gsm,omitempty"`
	Cause5GSM                            *utils.EnumField                    `json:"cause_5g_s_m,omitempty"`
	AlwaysonPDUSessionRequested          *bool                               `json:"alwayson_pdu_session_requested,omitempty"`
	RequestedQosRules                    []QosRule                           `json:"requested_qos_rules,omitempty"`
	RequestedQosFlowDescriptions         []QoSFlowDescription                `json:"requested_qos_flow_descriptions,omitempty"`
	ExtendedProtocolConfigurationOptions *nasie.ProtocolConfigurationOptions `json:"extended_protocol_configuration_options,omitempty"`
	MappedEPSBearerContexts              []MappedEPSBearerContext            `json:"mapped_eps_bearer_contexts,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildPDUSessionModificationRequest(msg *fgs.PDUSessionModificationRequest) *PDUSessionModificationRequest {
	out := &PDUSessionModificationRequest{
		AlwaysonPDUSessionRequested:          msg.AlwaysOnRequested,
		RequestedQosRules:                    QosRulesFromNAS(msg.RequestedQoSRules),
		RequestedQosFlowDescriptions:         QosFlowDescriptionsFromNAS(msg.RequestedQoSFlows),
		ExtendedProtocolConfigurationOptions: nasie.ExtendedPCO(msg.ExtendedPCO),
	}

	if msg.GSMCapability != nil {
		out.Capability5GSM = &Capability5GSM{
			RqoS:   b2u(msg.GSMCapability.RqoS),
			MH6PDU: b2u(msg.GSMCapability.MH6PDU),
		}
	}

	if msg.Cause != nil {
		cause := cause5GSMToString(*msg.Cause)
		out.Cause5GSM = &cause
	}

	if msg.MappedEPSBearerContexts != nil {
		out.MappedEPSBearerContexts = MappedEPSBearerContextsFromNAS(msg.MappedEPSBearerContexts)
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

// PDUSessionModificationCommand is the network's answer, carrying the QoS the
// session actually gets (TS 24.501 §8.3.9).
type PDUSessionModificationCommand struct {
	SessionAMBR                          *SessionAMBR                        `json:"session_ambr,omitempty"`
	MappedEPSBearerContexts              []MappedEPSBearerContext            `json:"mapped_eps_bearer_contexts,omitempty"`
	AuthorizedQosFlowDescriptions        []QoSFlowDescription                `json:"authorized_qos_flow_descriptions,omitempty"`
	ExtendedProtocolConfigurationOptions *nasie.ProtocolConfigurationOptions `json:"extended_protocol_configuration_options,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildPDUSessionModificationCommand(msg *fgs.PDUSessionModificationCommand) *PDUSessionModificationCommand {
	out := &PDUSessionModificationCommand{
		AuthorizedQosFlowDescriptions:        QosFlowDescriptionsFromNAS(msg.QoSFlowDescriptions),
		ExtendedProtocolConfigurationOptions: nasie.ExtendedPCO(msg.ExtendedPCO),
	}

	if msg.SessionAMBR != nil {
		ambr := buildSessionAMBR(*msg.SessionAMBR)
		out.SessionAMBR = &ambr
	}

	if msg.MappedEPSBearerContexts != nil {
		out.MappedEPSBearerContexts = MappedEPSBearerContextsFromNAS(msg.MappedEPSBearerContexts)
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

// PDUSessionModificationReject refuses the modification, with the back-off timer
// that tells the UE how long to wait (TS 24.501 §8.3.8).
type PDUSessionModificationReject struct {
	Cause5GSM                            utils.EnumField                     `json:"cause_5g_s_m"`
	BackoffTimer                         *GPRSTimer3Value                    `json:"backoff_timer,omitempty"`
	ExtendedProtocolConfigurationOptions *nasie.ProtocolConfigurationOptions `json:"extended_protocol_configuration_options,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildPDUSessionModificationReject(msg *fgs.PDUSessionModificationReject) *PDUSessionModificationReject {
	out := &PDUSessionModificationReject{
		Cause5GSM:                            cause5GSMToString(msg.Cause),
		BackoffTimer:                         gprsTimer3(msg.BackoffTimer),
		ExtendedProtocolConfigurationOptions: nasie.ExtendedPCO(msg.ExtendedPCO),
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

type PDUSessionModificationComplete struct {
	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildPDUSessionModificationComplete(msg *fgs.PDUSessionModificationComplete) *PDUSessionModificationComplete {
	return &PDUSessionModificationComplete{UnrecognizedIEs: utils.RawIEs(msg.Unrecognized)}
}
