// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"testing"

	"github.com/ellanetworks/core/nas/fgs"
)

func TestDecodePlainGmm_AcceptsWellFormed(t *testing.T) {
	wellFormed := [][]byte{
		encodePlainRegistrationRequest(t),
		encodePlainServiceRequest(t),
		encodePlainULNasTransport(t),
		encodePlainDeregistrationRequest(t),
		{0x7e, 0x00, uint8(fgs.MsgGMMStatus), 0x6f},
		{0x7e, 0x00, uint8(fgs.MsgRegistrationComplete)},
		{0x7e, 0x00, uint8(fgs.MsgConfigurationUpdateComplete)},
		{0x7e, 0x00, uint8(fgs.MsgDeregistrationAcceptUETerm)},
		{0x7e, 0x00, uint8(fgs.MsgRegistrationAccept), 0x00, 0x00, 0x00, 0x00},
	}

	for i, body := range wellFormed {
		if _, _, gotErr := DecodePlainGmm(body); gotErr != nil {
			t.Errorf("case %d (% x): DecodePlainGmm rejects a well-formed message: %v", i, body, gotErr)
		}
	}
}

func TestDecodePlainGmm_RejectsUnknownAndMalformed(t *testing.T) {
	reject := [][]byte{
		nil,
		{0x7e, 0x00},
		{0x7e, 0x00, 0xff},
		{0x7e, 0x00, uint8(fgs.MsgRegistrationRequest)},
		{0x7e, 0x00, uint8(fgs.MsgServiceRequest)},
		{0x7e, 0x00, uint8(fgs.MsgULNASTransport)},
		{0x99, 0x00, 0x41},
	}

	for i, body := range reject {
		if _, _, err := DecodePlainGmm(body); err == nil {
			t.Errorf("case %d (% x): expected DecodePlainGmm to reject, but it accepted", i, body)
		}
	}
}
