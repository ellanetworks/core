// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"testing"

	"github.com/ellanetworks/core/nas/fgs"
)

// TestDecodePlainGmm_AcceptsWellFormed is the safety invariant: the validator must
// never reject a well-formed plain NAS body — otherwise a valid message would draw
// a 5GMM STATUS instead of being processed, breaking a genuine UE. (The reverse is
// allowed: the validator accepts a defined type it does not deeply parse — downlink
// or header-only — on its header; a malformed such message is dropped rather than
// answered, and is never processed.)
func TestDecodePlainGmm_AcceptsWellFormed(t *testing.T) {
	wellFormed := [][]byte{
		encodePlainRegistrationRequest(t),
		encodePlainServiceRequest(t),
		encodePlainULNasTransport(t),
		encodePlainDeregistrationRequest(t),
		{0x7e, 0x00, uint8(fgs.MsgGMMStatus), 0x6f},                            // 5GMM status
		{0x7e, 0x00, uint8(fgs.MsgRegistrationComplete)},                       // header-only uplink
		{0x7e, 0x00, uint8(fgs.MsgConfigurationUpdateComplete)},                // header-only uplink
		{0x7e, 0x00, uint8(fgs.MsgDeregistrationAcceptUETerm)},                 // header-only uplink
		{0x7e, 0x00, uint8(fgs.MsgRegistrationAccept), 0x00, 0x00, 0x00, 0x00}, // downlink, well-formed enough
	}

	for i, body := range wellFormed {
		if _, _, gotErr := DecodePlainGmm(body); gotErr != nil {
			t.Errorf("case %d (% x): DecodePlainGmm rejects a well-formed message: %v", i, body, gotErr)
		}
	}
}

// TestDecodePlainGmm_RejectsUnknownAndMalformed confirms the validator rejects the
// inputs the STATUS #96/#97 contract depends on: an unassigned type and a truncated
// body of a type Ella parses.
func TestDecodePlainGmm_RejectsUnknownAndMalformed(t *testing.T) {
	reject := [][]byte{
		nil,
		{0x7e, 0x00},       // too short to carry a type
		{0x7e, 0x00, 0xff}, // unassigned type
		{0x7e, 0x00, uint8(fgs.MsgRegistrationRequest)}, // truncated
		{0x7e, 0x00, uint8(fgs.MsgServiceRequest)},      // truncated
		{0x7e, 0x00, uint8(fgs.MsgULNASTransport)},      // truncated
		{0x99, 0x00, 0x41},                              // disallowed EPD
	}

	for i, body := range reject {
		if _, _, err := DecodePlainGmm(body); err == nil {
			t.Errorf("case %d (% x): expected DecodePlainGmm to reject, but it accepted", i, body)
		}
	}
}
