// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/eps"
)

type DetachRequest struct {
	NASKeySetIdentifier uint8          `json:"nas_key_set_identifier"`
	MobileIdentity      MobileIdentity `json:"mobile_identity"`
	SwitchOff           bool           `json:"switch_off"`
	DetachType          uint8          `json:"detach_type"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildDetachRequest(msg *eps.DetachRequestUE) *DetachRequest {
	out := &DetachRequest{SwitchOff: msg.SwitchOff, DetachType: uint8(msg.TypeOfDetach)}

	out.NASKeySetIdentifier = msg.NASKeySetIdentifier.HalfOctet()
	out.MobileIdentity = mobileIdentity(msg.EPSMobileIdentity)
	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}
