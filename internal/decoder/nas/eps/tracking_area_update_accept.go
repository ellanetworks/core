// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	nasie "github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/eps"
)

type TrackingAreaUpdateAccept struct {
	TAIList                []TAI                              `json:"tai_list,omitempty"`
	NetworkFeatureSupport  *NetworkFeatureSupport             `json:"network_feature_support,omitempty"`
	EPSBearerContextStatus []nasie.EPSBearerContextStatusItem `json:"eps_bearer_context_status,omitempty"`
	UpdateResult           utils.EnumField                    `json:"update_result"`
	GUTI                   *MobileIdentity                    `json:"guti,omitempty"`
	EMMCause               *uint8                             `json:"emm_cause,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildTrackingAreaUpdateAccept(msg *eps.TrackingAreaUpdateAccept) *TrackingAreaUpdateAccept {
	out := &TrackingAreaUpdateAccept{
		UpdateResult: updateResultToEnum(msg.EPSUpdateResult),
		EMMCause:     emmCauseValue(msg.Cause),
	}
	if msg.GUTI != nil {
		id := mobileIdentity(*msg.GUTI)
		out.GUTI = &id
	}

	if msg.TAIList != nil {
		out.TAIList = taiList(*msg.TAIList)
	}

	out.NetworkFeatureSupport = networkFeatureSupport(msg.NetworkFeatureSupport)
	out.EPSBearerContextStatus = nasie.EPSBearerContextStatus(msg.EPSBearerContextStatus)
	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}
