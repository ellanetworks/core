// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "github.com/ellanetworks/core/per/ap"

func tlaOf(octets ...byte) TransportLayerAddress {
	a, ok := ap.TransportLayerAddressFromBits(octets, len(octets)*8)
	if !ok {
		panic("tlaOf")
	}

	return a
}
