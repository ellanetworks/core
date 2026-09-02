// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"encoding/hex"

	nasie "github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// ESMCauseOnly is the shape of every ESM message whose only content beyond the
// shared header is a cause: the bearer identity and transaction are rendered on
// the ESM header, so the cause is all that is left.
type ESMCauseOnly struct {
	ESMCause utils.EnumField `json:"esm_cause"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func esmCauseOnly(c eps.ESMCause, unrecognized []nas.RawIE) *ESMCauseOnly {
	return &ESMCauseOnly{ESMCause: esmCauseToEnum(c), UnrecognizedIEs: utils.RawIEs(unrecognized)}
}

type ModifyEPSBearerContextRequest struct {
	NewEPSQoS                            *EPSQoS                             `json:"new_eps_qos,omitempty"`
	APNAMBR                              *APNAMBR                            `json:"apn_ambr,omitempty"`
	ProtocolConfigurationOptions         *nasie.ProtocolConfigurationOptions `json:"protocol_configuration_options,omitempty"`
	ExtendedProtocolConfigurationOptions *nasie.ProtocolConfigurationOptions `json:"extended_protocol_configuration_options,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildModifyEPSBearerContextRequest(msg *eps.ModifyEPSBearerContextRequest) *ModifyEPSBearerContextRequest {
	out := &ModifyEPSBearerContextRequest{
		APNAMBR:                              apnAMBR(msg.APNAMBR),
		ProtocolConfigurationOptions:         nasie.ExtendedPCO(msg.ProtocolConfigurationOptions),
		ExtendedProtocolConfigurationOptions: nasie.ExtendedPCO(msg.ExtendedProtocolConfigurationOptions),
	}

	if msg.NewEPSQoS != nil {
		out.NewEPSQoS = epsQoS(*msg.NewEPSQoS)
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

type ModifyEPSBearerContextAccept struct {
	ProtocolConfigurationOptions         *nasie.ProtocolConfigurationOptions `json:"protocol_configuration_options,omitempty"`
	ExtendedProtocolConfigurationOptions *nasie.ProtocolConfigurationOptions `json:"extended_protocol_configuration_options,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildModifyEPSBearerContextAccept(msg *eps.ModifyEPSBearerContextAccept) *ModifyEPSBearerContextAccept {
	out := &ModifyEPSBearerContextAccept{
		ProtocolConfigurationOptions:         nasie.ExtendedPCO(msg.ProtocolConfigurationOptions),
		ExtendedProtocolConfigurationOptions: nasie.ExtendedPCO(msg.ExtendedProtocolConfigurationOptions),
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

// BearerResourceAllocationRequest is the UE asking for resources for a traffic
// flow it describes (TS 24.301 §8.3.8). The traffic flow aggregate is a TFT,
// which the codec keeps as raw bytes.
type BearerResourceAllocationRequest struct {
	LinkedEPSBearerIdentity uint8   `json:"linked_eps_bearer_identity"`
	TrafficFlowAggregate    string  `json:"traffic_flow_aggregate,omitempty"`
	RequiredTrafficFlowQoS  *EPSQoS `json:"required_traffic_flow_qos,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildBearerResourceAllocationRequest(msg *eps.BearerResourceAllocationRequest) *BearerResourceAllocationRequest {
	out := &BearerResourceAllocationRequest{
		LinkedEPSBearerIdentity: uint8(msg.LinkedEPSBearerIdentity),
		RequiredTrafficFlowQoS:  epsQoS(msg.RequiredTrafficFlowQoS),
	}

	if len(msg.TrafficFlowAggregate) > 0 {
		out.TrafficFlowAggregate = hex.EncodeToString(msg.TrafficFlowAggregate)
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

// BearerResourceModificationRequest is the UE asking to change or drop the
// resources bound to one packet filter (TS 24.301 §8.3.10).
type BearerResourceModificationRequest struct {
	EPSBearerIdentityForPacketFilter uint8            `json:"eps_bearer_identity_for_packet_filter"`
	TrafficFlowAggregate             string           `json:"traffic_flow_aggregate,omitempty"`
	RequiredTrafficFlowQoS           *EPSQoS          `json:"required_traffic_flow_qos,omitempty"`
	ESMCause                         *utils.EnumField `json:"esm_cause,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildBearerResourceModificationRequest(msg *eps.BearerResourceModificationRequest) *BearerResourceModificationRequest {
	out := &BearerResourceModificationRequest{
		EPSBearerIdentityForPacketFilter: uint8(msg.EPSBearerIdentityForPacketFilter),
	}

	if len(msg.TrafficFlowAggregate) > 0 {
		out.TrafficFlowAggregate = hex.EncodeToString(msg.TrafficFlowAggregate)
	}

	if msg.RequiredTrafficFlowQoS != nil {
		out.RequiredTrafficFlowQoS = epsQoS(*msg.RequiredTrafficFlowQoS)
	}

	if msg.Cause != nil {
		c := esmCauseToEnum(*msg.Cause)
		out.ESMCause = &c
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}
