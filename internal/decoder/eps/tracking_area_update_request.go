// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/eps"
)

type TrackingAreaUpdateRequest struct {
	NASKeySetIdentifier    uint8                        `json:"nas_key_set_identifier"`
	OldGUTI                *MobileIdentity              `json:"old_guti,omitempty"`
	UENetworkCapability    *UENetworkCapability         `json:"ue_network_capability,omitempty"`
	MSNetworkCapability    *MSNetworkCapability         `json:"ms_network_capability,omitempty"`
	AdditionalGUTI         *MobileIdentity              `json:"additional_guti,omitempty"`
	OldGUTIType            *utils.EnumField             `json:"old_guti_type,omitempty"`
	UEStatus               *UEStatus                    `json:"ue_status,omitempty"`
	EPSBearerContextStatus []EPSBearerContextStatusItem `json:"eps_bearer_context_status,omitempty"`
	UpdateType             utils.EnumField              `json:"update_type"`
	ActiveFlag             bool                         `json:"active_flag"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildTrackingAreaUpdateRequest(msg *eps.TrackingAreaUpdateRequest) *TrackingAreaUpdateRequest {
	out := &TrackingAreaUpdateRequest{
		UpdateType: updateTypeToEnum(msg.EPSUpdateType),
		ActiveFlag: msg.ActiveFlag,
	}

	out.NASKeySetIdentifier = msg.NASKeySetIdentifier.HalfOctet()

	oldGUTI := mobileIdentity(msg.OldGUTI)
	out.OldGUTI = &oldGUTI
	out.UENetworkCapability = nil

	if msg.UENetworkCapability != nil {
		out.UENetworkCapability = ueNetworkCapability(*msg.UENetworkCapability)
	}

	out.MSNetworkCapability = msNetworkCapability(msg.MSNetworkCapability)
	out.UEStatus = ueStatus(msg.UEStatus)
	out.EPSBearerContextStatus = EPSBearerContextStatus(msg.EPSBearerContextStatus)

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
