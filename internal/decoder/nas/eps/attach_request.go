// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	nasie "github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/eps"
)

type AttachRequest struct {
	NASKeySetIdentifier     utils.KeySetIdentifier     `json:"nas_key_set_identifier"`
	UENetworkCapability     *nasie.UENetworkCapability `json:"ue_network_capability,omitempty"`
	MSNetworkCapability     *MSNetworkCapability       `json:"ms_network_capability,omitempty"`
	AdditionalGUTI          *MobileIdentity            `json:"additional_guti,omitempty"`
	OldGUTIType             *utils.EnumField           `json:"old_guti_type,omitempty"`
	TMSIStatus              *bool                      `json:"tmsi_status,omitempty"`
	DeviceProperties        *bool                      `json:"device_properties,omitempty"`
	MSNetworkFeatureSupport *bool                      `json:"ms_network_feature_support,omitempty"`
	UEStatus                *UEStatus                  `json:"ue_status,omitempty"`
	AdditionalUpdateType    *AdditionalUpdateType      `json:"additional_update_type,omitempty"`
	AttachType              utils.EnumField            `json:"attach_type"`
	MobileIdentity          MobileIdentity             `json:"mobile_identity"`
	ESMContainer            *ESMMessage                `json:"esm_container,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildAttachRequest(msg *eps.AttachRequest) *AttachRequest {
	out := &AttachRequest{
		AttachType:     attachTypeToEnum(msg.EPSAttachType),
		MobileIdentity: mobileIdentity(msg.EPSMobileIdentity),
		ESMContainer:   decodeESMContainer(msg.ESMMessageContainer),
	}

	out.NASKeySetIdentifier = utils.NewKeySetIdentifier(msg.NASKeySetIdentifier)
	out.UENetworkCapability = nasie.UENetworkCapabilityFrom(msg.UENetworkCapability)
	out.MSNetworkCapability = msNetworkCapability(msg.MSNetworkCapability)
	out.TMSIStatus = msg.TMSIStatus
	out.DeviceProperties = msg.DeviceProperties
	out.MSNetworkFeatureSupport = msg.MSNetworkFeatureSupport
	out.UEStatus = ueStatus(msg.UEStatus)
	out.AdditionalUpdateType = additionalUpdateType(msg.AdditionalUpdateType)

	if msg.AdditionalGUTI != nil {
		id := mobileIdentity(*msg.AdditionalGUTI)
		out.AdditionalGUTI = &id
	}

	if msg.OldGUTIType != nil {
		gt := utils.NamedEnum(uint8(*msg.OldGUTIType), msg.OldGUTIType.String())
		out.OldGUTIType = &gt
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}
