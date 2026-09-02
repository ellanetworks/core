// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/eps"
)

type AttachAccept struct {
	TAIList               []TAI                  `json:"tai_list,omitempty"`
	NetworkFeatureSupport *NetworkFeatureSupport `json:"network_feature_support,omitempty"`
	AttachResult          utils.EnumField        `json:"attach_result"`
	T3412                 uint8                  `json:"t3412"`
	GUTI                  *MobileIdentity        `json:"guti,omitempty"`
	EMMCause              *uint8                 `json:"emm_cause,omitempty"`
	ESMContainer          *ESMMessage            `json:"esm_container,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildAttachAccept(msg *eps.AttachAccept) *AttachAccept {
	out := &AttachAccept{
		AttachResult: attachResultToEnum(msg.EPSAttachResult),
		T3412:        timerOctet(msg.T3412),
		EMMCause:     emmCauseValue(msg.Cause),
		ESMContainer: decodeESMContainer(msg.ESMMessageContainer),
	}
	if msg.GUTI != nil {
		id := mobileIdentity(*msg.GUTI)
		out.GUTI = &id
	}

	out.TAIList = taiList(msg.TAIList)
	out.NetworkFeatureSupport = networkFeatureSupport(msg.NetworkFeatureSupport)
	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}
