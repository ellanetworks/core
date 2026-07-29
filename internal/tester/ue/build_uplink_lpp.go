// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/nas/fgs"
)

// BuildUplinkNasTransportLPP builds a UL NAS Transport message with LPP payload
// container type. Used by the UE tester to send LPP responses (capabilities,
// location information) back to the core.
func BuildUplinkNasTransportLPP(lppPayload []byte) ([]byte, error) {
	if lppPayload == nil {
		return nil, fmt.Errorf("LPP payload is required")
	}

	m := &fgs.ULNASTransport{
		PayloadContainerType: fgs.PayloadContainerTypeLPP,
		PayloadContainer:     lppPayload,
	}

	return m.MarshalBinary()
}
