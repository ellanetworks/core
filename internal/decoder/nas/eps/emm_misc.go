// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/eps"
)

type AttachComplete struct {
	ESMContainer *ESMMessage `json:"esm_container,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildAttachComplete(msg *eps.AttachComplete) *AttachComplete {
	out := &AttachComplete{ESMContainer: decodeESMContainer(msg.ESMMessageContainer)}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

type TrackingAreaUpdateReject struct {
	EMMCause utils.EnumField `json:"emm_cause"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildTrackingAreaUpdateReject(msg *eps.TrackingAreaUpdateReject) *TrackingAreaUpdateReject {
	out := &TrackingAreaUpdateReject{EMMCause: emmCauseToEnum(msg.Cause)}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

type SecurityModeComplete struct {
	IMEISV                      *string `json:"imeisv,omitempty"`
	ReplayedNASMessageContainer *string `json:"replayed_nas_message_container,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildSecurityModeComplete(msg *eps.SecurityModeComplete) *SecurityModeComplete {
	out := &SecurityModeComplete{}

	if msg.IMEISV != nil {
		s := msg.IMEISV.String()
		out.IMEISV = &s
	}

	if len(msg.ReplayedNASMessageContainer) > 0 {
		s := hex.EncodeToString(msg.ReplayedNASMessageContainer)
		out.ReplayedNASMessageContainer = &s
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

type GUTIReallocationCommand struct {
	GUTI MobileIdentity `json:"guti"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildGUTIReallocationCommand(msg *eps.GUTIReallocationCommand) *GUTIReallocationCommand {
	out := &GUTIReallocationCommand{GUTI: mobileIdentity(msg.GUTI)}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

type GUTIReallocationComplete struct {
	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildGUTIReallocationComplete(msg *eps.GUTIReallocationComplete) *GUTIReallocationComplete {
	return &GUTIReallocationComplete{UnrecognizedIEs: utils.RawIEs(msg.Unrecognized)}
}
