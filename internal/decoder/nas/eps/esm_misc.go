// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	nasie "github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/eps"
)

type ESMInformationRequest struct {
	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildESMInformationRequest(msg *eps.ESMInformationRequest) *ESMInformationRequest {
	return &ESMInformationRequest{UnrecognizedIEs: utils.RawIEs(msg.Unrecognized)}
}

type ESMInformationResponse struct {
	AccessPointName                      *string                             `json:"access_point_name,omitempty"`
	ProtocolConfigurationOptions         *nasie.ProtocolConfigurationOptions `json:"protocol_configuration_options,omitempty"`
	ExtendedProtocolConfigurationOptions *nasie.ProtocolConfigurationOptions `json:"extended_protocol_configuration_options,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildESMInformationResponse(msg *eps.ESMInformationResponse) *ESMInformationResponse {
	out := &ESMInformationResponse{}

	if msg.AccessPointName != nil {
		apn := string(*msg.AccessPointName)
		out.AccessPointName = &apn
	}

	out.ProtocolConfigurationOptions = nasie.ExtendedPCO(msg.ProtocolConfigurationOptions)
	out.ExtendedProtocolConfigurationOptions = nasie.ExtendedPCO(msg.ExtendedProtocolConfigurationOptions)
	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

type ActivateDefaultBearerAccept struct {
	ProtocolConfigurationOptions         *nasie.ProtocolConfigurationOptions `json:"protocol_configuration_options,omitempty"`
	ExtendedProtocolConfigurationOptions *nasie.ProtocolConfigurationOptions `json:"extended_protocol_configuration_options,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildActivateDefaultBearerAccept(msg *eps.ActivateDefaultEPSBearerContextAccept) *ActivateDefaultBearerAccept {
	out := &ActivateDefaultBearerAccept{}

	out.ProtocolConfigurationOptions = nasie.ExtendedPCO(msg.ProtocolConfigurationOptions)
	out.ExtendedProtocolConfigurationOptions = nasie.ExtendedPCO(msg.ExtendedProtocolConfigurationOptions)
	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

type PDNDisconnectRequest struct {
	LinkedEPSBearerIdentity uint8 `json:"linked_eps_bearer_identity"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildPDNDisconnectRequest(msg *eps.PDNDisconnectRequest) *PDNDisconnectRequest {
	out := &PDNDisconnectRequest{LinkedEPSBearerIdentity: uint8(msg.LinkedEPSBearerIdentity)}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

type DeactivateBearerAccept struct {
	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildDeactivateBearerAccept(msg *eps.DeactivateEPSBearerContextAccept) *DeactivateBearerAccept {
	return &DeactivateBearerAccept{UnrecognizedIEs: utils.RawIEs(msg.Unrecognized)}
}
