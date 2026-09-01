// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

type SecurityModeComplete struct {
	IMEISV              *string `json:"imeisv,omitempty"`
	NASMessageContainer []byte  `json:"nas_message_container,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildSecurityModeComplete(msg *fgs.SecurityModeComplete) *SecurityModeComplete {
	out := &SecurityModeComplete{
		NASMessageContainer: msg.NASMessageContainer,
	}

	if msg.IMEISV != nil {
		pei := msg.IMEISV.String()
		out.IMEISV = &pei
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}
