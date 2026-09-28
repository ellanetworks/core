// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas"
)

type NASTransport struct {
	NASMessageContainer *utils.RawOctets `json:"nas_message_container"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildNASTransport(container []byte, unrecognized []nas.RawIE) *NASTransport {
	return &NASTransport{
		NASMessageContainer: utils.NewRawOctets(container),
		UnrecognizedIEs:     utils.RawIEs(unrecognized),
	}
}
