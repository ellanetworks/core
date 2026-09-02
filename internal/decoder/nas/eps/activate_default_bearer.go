// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	nasie "github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/eps"
)

type ActivateDefaultBearer struct {
	ESMCause                             *utils.EnumField                    `json:"esm_cause,omitempty"`
	EPSQoS                               *EPSQoS                             `json:"eps_qos,omitempty"`
	APNAMBR                              *APNAMBR                            `json:"apn_ambr,omitempty"`
	ProtocolConfigurationOptions         *nasie.ProtocolConfigurationOptions `json:"protocol_configuration_options,omitempty"`
	ExtendedProtocolConfigurationOptions *nasie.ProtocolConfigurationOptions `json:"extended_protocol_configuration_options,omitempty"`
	AccessPointName                      string                              `json:"access_point_name,omitempty"`
	PDNAddress                           *PDNAddress                         `json:"pdn_address,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildActivateDefaultBearer(msg *eps.ActivateDefaultEPSBearerContextRequest) *ActivateDefaultBearer {
	out := &ActivateDefaultBearer{
		AccessPointName: string(msg.AccessPointName),
		PDNAddress:      pdnAddress(msg.PDNAddress),
	}

	if msg.Cause != nil {
		cause := utils.NamedEnum(uint8(*msg.Cause), msg.Cause.Name())
		out.ESMCause = &cause
	}

	out.EPSQoS = epsQoS(msg.EPSQoS)
	out.APNAMBR = apnAMBR(msg.APNAMBR)
	out.ProtocolConfigurationOptions = nasie.ExtendedPCO(msg.ProtocolConfigurationOptions)
	out.ExtendedProtocolConfigurationOptions = nasie.ExtendedPCO(msg.ExtendedProtocolConfigurationOptions)
	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}
