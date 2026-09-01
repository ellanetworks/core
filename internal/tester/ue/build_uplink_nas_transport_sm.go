// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/nas/fgs"
)

// BuildUplinkNasTransportSM builds a UL NAS Transport carrying a 5GSM message
// for a PDU session the network already knows (TS 24.501 §8.2.10). Request
// type, DNN and S-NSSAI are the establishment-time IEs and stay out: an
// Initial request on an existing PDU session ID makes the AMF drop its routing
// context and re-establish instead of forwarding the message.
func BuildUplinkNasTransportSM(pduSessionID uint8, payloadContainer []byte) ([]byte, error) {
	if pduSessionID == 0 {
		return nil, fmt.Errorf("PDUSessionID is required to build UplinkNasTransport for PDU Session")
	}

	if len(payloadContainer) == 0 {
		return nil, fmt.Errorf("PayloadContainer is required to build UplinkNasTransport for PDU Session")
	}

	psi := fgs.PDUSessionID(pduSessionID)

	m := &fgs.ULNASTransport{
		PayloadContainerType: fgs.PayloadContainerTypeN1SMInfo,
		PayloadContainer:     payloadContainer,
		PDUSessionID:         &psi,
	}

	return m.MarshalBinary()
}
