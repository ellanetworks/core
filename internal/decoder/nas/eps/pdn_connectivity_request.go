// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	nasie "github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/eps"
)

type PDNConnectivityRequest struct {
	AccessPointName                      *string                             `json:"access_point_name,omitempty"`
	ProtocolConfigurationOptions         *nasie.ProtocolConfigurationOptions `json:"protocol_configuration_options,omitempty"`
	ExtendedProtocolConfigurationOptions *nasie.ProtocolConfigurationOptions `json:"extended_protocol_configuration_options,omitempty"`
	ESMInformationTransferFlag           *bool                               `json:"esm_information_transfer_flag,omitempty"`
	RequestType                          utils.EnumField                     `json:"request_type"`
	PDNType                              utils.EnumField                     `json:"pdn_type"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildPDNConnectivityRequest(msg *eps.PDNConnectivityRequest) *PDNConnectivityRequest {
	out := &PDNConnectivityRequest{
		RequestType: requestTypeToEnum(uint8(msg.RequestType)),
		PDNType:     pdnTypeToEnum(msg.PDNType),
	}

	if msg.AccessPointName != nil {
		apn := string(*msg.AccessPointName)
		out.AccessPointName = &apn
	}

	out.ProtocolConfigurationOptions = nasie.ExtendedPCO(msg.ProtocolConfigurationOptions)
	out.ExtendedProtocolConfigurationOptions = nasie.ExtendedPCO(msg.ExtendedProtocolConfigurationOptions)
	out.ESMInformationTransferFlag = msg.ESMInformationTransferFlag
	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}
