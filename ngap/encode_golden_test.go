// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/per"
)

// Pins the exact octets, which a round-trip test cannot: it re-reads whatever
// the encoder just wrote. The bodies are payloads of the golden PDUs, so they
// carry the same second-implementation provenance.
func TestEncodeBodyGolden(t *testing.T) {
	tests := []struct {
		name string
		body func(*per.Writer, per.Encoding) error
		want string
	}{
		{
			"ErrorIndication",
			goldErrorIndicationFull().encodeBody,
			"000004000a400680ffffffffff005540020009000f400162001340087815000000001b40",
		},
		{
			"ErrorIndicationFiveGSTMSI",
			errorIndicationWithSTMSI().encodeBody,
			"000004000a400680ffffffffff005540020009000f400162001a40070010c0deadbeef",
		},
		{
			"NGReset",
			goldNGResetPart().encodeBody,
			"000002000f40018600580008400260070009200b",
		},
		{
			"NGResetAcknowledge",
			goldNGResetAcknowledge().encodeBody,
			"000001006f40050160070009",
		},
		{
			"NGSetupRequest",
			goldRequest().encodeBody,
			"000004001b00080002f839100001020052400a0380656c6c612d676e620066001200000000010002f8390001100801020300100015400140",
		},
		{
			"NGSetupResponse",
			goldResponse().encodeBody,
			"0000040001000a0380656c6c612d616d6600600008000002f83902004300564001ff0050000b0002f83900001008010203",
		},
		{
			"NGSetupFailure",
			goldFailure().encodeBody,
			"000002000f400188006b400130",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := per.NewWriter()
			if err := tt.body(w, per.Aligned); err != nil {
				t.Fatalf("encode: %v", err)
			}

			if got := hex.EncodeToString(perBytes(w)); got != tt.want {
				t.Fatalf("body mismatch:\n  got  %s\n  want %s", got, tt.want)
			}
		})
	}
}

// The one IE the reference encoder cannot express, so these octets are derived
// from X.691 rather than compared against it: two-bit preamble, 10-bit AMF Set
// ID 0x001, 6-bit AMF Pointer 0x03, then the realigned 5G-TMSI (§16.9, §16.10).
func errorIndicationWithSTMSI() *ErrorIndication {
	m := goldErrorIndication()
	m.FiveGSTMSI = &FiveGSTMSI{AMFSetID: 0x001, AMFPointer: 0x03, FiveGTMSI: 0xdeadbeef}

	return m
}
