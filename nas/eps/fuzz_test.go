// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// The targets here are split by what they decode — a whole plain message, the
// security wrapper, one information element — so that a long fuzzing session
// spends its budget where its mutations are meaningful. A single target running
// every decoder over one input costs a multiple of the work per execution, and
// credits the coverage of all of them to whichever mutation arrived, which
// leaves the engine no gradient to follow into any one of them.

// mustHex decodes a constant hex seed; it panics on a malformed literal since the
// seeds are compile-time constants.
func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}

	return b
}

// FuzzParseMessage exercises the plain-message entry point, and through it every
// EMM and ESM decoder: the message type octet selects the decoder, so the engine
// reaches each of them by mutating the header. They run on untrusted S1 data, so
// they must never panic — every read is bounded by the nas.Reader, and a
// malformed message returns an error.
//
// Both directions run: TS 24.301 table 9.8.1 gives DETACH REQUEST one message
// type in both, and the direction picks the variant.
func FuzzParseMessage(f *testing.F) {
	f.Add([]byte{})
	// the real Attach Request NAS-PDU (testdata/captures/attach_request_nas.hex)
	f.Add([]byte{0x3b, 0x17, 0xdf, 0x67, 0x5a, 0xa8, 0x05, 0x07, 0x41, 0x02, 0x0b, 0xf6})
	// a SERVICE REQUEST (security header type 12, KSI/seq, short MAC)
	f.Add([]byte{0xc7, 0x00, 0x12, 0x34})
	// ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST with the ESM cause (58) and PCO
	// (27) optional IEs, to seed the optional-IE parse loop.
	f.Add([]byte{0x52, 0x01, 0xc1, 0x01, 0x09, 0x01, 0x78, 0x05, 0x01, 0x0a, 0x2d, 0x00, 0x01, 0x58, 0x32, 0x27, 0x08, 0x80, 0x00, 0x0d, 0x04, 0x08, 0x08, 0x08, 0x08})
	// same prefix with a truncated ESM cause (IEI, no value), a PCO with a length
	// past the buffer, and an unrecognised optional IEI — malformed optional IEs.
	f.Add([]byte{0x52, 0x01, 0xc1, 0x01, 0x09, 0x00, 0x00, 0x58})
	f.Add([]byte{0x52, 0x01, 0xc1, 0x01, 0x09, 0x00, 0x00, 0x27, 0xff})
	f.Add([]byte{0x52, 0x01, 0xc1, 0x01, 0x09, 0x00, 0x00, 0x5e, 0x02, 0x00})
	// an ATTACH REQUEST carrying a long chain of optional IEs — TV, TLV, type-1
	// half-octet, and IEs ordered after MS network capability — to drive the
	// optional-IE walk through every format.
	f.Add(mustHex("0741310bf600f1100001010000000103f070c000040201d011190102035200f11030395c00083103e5e03491f2d132010038020001"))
	// same optional chain truncated mid-TLV (N1 UE network capability IEI with no
	// length octet) to exercise the walk's bounds checks.
	f.Add(mustHex("0741310bf600f1100001010000000103f070c000040201d0111901020332"))

	f.Fuzz(func(t *testing.T, b []byte) {
		for _, dir := range []nas.Direction{nas.DirectionUplink, nas.DirectionDownlink} {
			roundTrip(t, "ParseMessage", func(b []byte) (Message, error) { return ParseMessage(b, dir) }, b)

			// The invariant the entry point promises: a message comes back
			// exactly when the error is soft.
			msg, err := ParseMessage(b, dir)
			if (msg != nil) != nas.SoftOnly(err) {
				t.Fatalf("ParseMessage(% x, %s) = %v, %v: a message must come back exactly when the error is soft", b, dir, msg, err)
			}
		}

		// The peekers read the same header without decoding, and must not panic
		// where the decoder would have reported an error.
		_, _ = PeekProtocolDiscriminator(b)
		_, _ = PeekSecurityHeaderType(b)
		_, _ = PeekMessageType(b)
		_, _ = PeekESMMessageType(b)
	})
}

// FuzzParseSecurityProtectedMessage exercises the wrapper parser, which reads the
// least trusted octets in the stack: they arrive before any MAC has been checked.
func FuzzParseSecurityProtectedMessage(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x27, 0x11, 0x22, 0x33, 0x44, 0x01, 0x07, 0x42})
	f.Add([]byte{0x37, 0x11, 0x22, 0x33, 0x44, 0x01, 0x07, 0x42})
	// a SERVICE REQUEST, whose security header type is its message identity and
	// which this parser must refuse (TS 24.301 §8.2.25).
	f.Add([]byte{0xc7, 0x00, 0x12, 0x34})

	f.Fuzz(func(t *testing.T, b []byte) {
		roundTrip(t, "SecurityProtectedMessage", ParseSecurityProtectedMessage, b)
	})
}

// ieCodecs are the information elements a message hands attacker-controlled
// octets to. Each is fuzzed on its own because an element is reached through a
// message only when everything before it decoded, which a whole-message target
// rarely achieves for the ones deep in a variable part.
var ieCodecs = []func(*testing.T, []byte){
	func(t *testing.T, b []byte) { roundTrip(t, "EPSMobileIdentity", ParseEPSMobileIdentity, b) },
	func(t *testing.T, b []byte) { roundTrip(t, "MobileIdentity", ParseMobileIdentity, b) },
	func(t *testing.T, b []byte) { roundTrip(t, "PDNAddress", ParsePDNAddress, b) },
	func(t *testing.T, b []byte) { roundTrip(t, "EPSQoS", ParseEPSQoS, b) },
	func(t *testing.T, b []byte) { roundTrip(t, "APN", ParseAPN, b) },
	func(t *testing.T, b []byte) { roundTrip(t, "APNAMBR", ParseAPNAMBR, b) },
	func(t *testing.T, b []byte) { roundTrip(t, "TAIList", ParseTAIList, b) },
	func(t *testing.T, b []byte) {
		roundTrip(t, "UENetworkCapability", ParseUENetworkCapability, b)
	},
	func(t *testing.T, b []byte) {
		roundTrip(t, "UESecurityCapability", ParseUESecurityCapability, b)
	},
	func(t *testing.T, b []byte) {
		roundTrip(t, "MSNetworkCapability", ParseMSNetworkCapability, b)
	},
	func(t *testing.T, b []byte) {
		roundTrip(t, "PCO downlink", func(v []byte) (nas.ProtocolConfigurationOptions, error) {
			return nas.ParseProtocolConfigurationOptions(v, nas.PCONetworkToMS)
		}, b)
	},
	func(t *testing.T, b []byte) {
		roundTrip(t, "PCO uplink", func(v []byte) (nas.ProtocolConfigurationOptions, error) {
			return nas.ParseProtocolConfigurationOptions(v, nas.PCOMSToNetwork)
		}, b)
	},
}

// FuzzIECodecs decodes one information-element value with the codec the first
// argument selects. The selector is an argument rather than a loop so that the
// engine mutates it: each codec then accumulates its own corpus entries instead
// of sharing the coverage credit of whichever one the input happened to reach.
func FuzzIECodecs(f *testing.F) {
	f.Add(0, []byte{0xf6, 0x00, 0xf1, 0x10, 0x00, 0x01, 0x01, 0x00, 0x00, 0x00, 0x01}) // GUTI
	f.Add(0, []byte{0x19, 0x00, 0x10, 0x32, 0x54, 0x76, 0x98})                         // IMSI
	f.Add(2, []byte{0x01, 0x0a, 0x00, 0x00, 0x01})                                     // PDN address, IPv4
	f.Add(3, []byte{0x09})
	f.Add(4, []byte{0x08, 'i', 'n', 't', 'e', 'r', 'n', 'e', 't'})
	f.Add(5, []byte{0xfe, 0xfe})
	f.Add(6, []byte{0x00, 0x00, 0xf1, 0x10, 0x00, 0x01})
	f.Add(7, []byte{0xe5, 0xe0})
	f.Add(10, []byte{0x80, 0x00, 0x0d, 0x04, 0x08, 0x08, 0x08, 0x08})
	f.Add(0, []byte{})
	f.Add(0, []byte{0x00})

	f.Fuzz(func(t *testing.T, which int, b []byte) {
		ieCodecs[uint(which)%uint(len(ieCodecs))](t, b)
	})
}
