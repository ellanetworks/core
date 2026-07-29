// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"testing"

	"github.com/ellanetworks/core/nas"
)

// The targets here are split by what they decode — a whole plain message, the
// security wrapper, one information element — so that a long fuzzing session
// spends its budget where its mutations are meaningful. A single target running
// every decoder over one input costs a multiple of the work per execution, and
// credits the coverage of all of them to whichever mutation arrived, which
// leaves the engine no gradient to follow into any one of them.

// FuzzParseMessage exercises the plain-message entry point, and through it every
// 5GMM and 5GSM decoder: the message type octet selects the decoder, so the
// engine reaches each of them by mutating the header. They run on untrusted N1
// data, so they must never panic — every read is bounded by the nas.Reader, and a
// malformed message returns an error.
func FuzzParseMessage(f *testing.F) {
	f.Add([]byte{uint8(EPD5GSM), 5, 1, uint8(MsgPDUSessionEstablishmentRequest), 0xFF, 0xFF, 0x91, 0xA1, iei5GSMCapability, 0x01, 0x03, 0xB1})
	f.Add([]byte{uint8(EPD5GSM), 5, 1, uint8(MsgGSMStatus), 0x2F})
	// A PDU SESSION MODIFICATION REQUEST: header, 5GSM cause (TV), always-on requested (type 1).
	f.Add([]byte{uint8(EPD5GSM), 5, 1, uint8(MsgPDUSessionModificationRequest), iei5GSMCause, 0x24, 0xB1})
	f.Add([]byte{uint8(EPD5GSM), 5, 1, uint8(MsgPDUSessionReleaseRequest), iei5GSMCause, 0x24})
	// A REGISTRATION REQUEST: header, registration-type/ngKSI octet, then an LV-E
	// 5GS mobile identity (5G-GUTI), to seed the mandatory + optional-IE walk.
	f.Add([]byte{uint8(EPD5GMM), 0x00, uint8(MsgRegistrationRequest), 0x79, 0x00, 0x0b, 0xf2, 0x00, 0xf1, 0x10, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01})
	// A SERVICE REQUEST: service-type/ngKSI octet then the 7-octet 5G-S-TMSI (LV-E).
	f.Add([]byte{uint8(EPD5GMM), 0x00, uint8(MsgServiceRequest), 0x70, 0x00, 0x07, 0xf4, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05})
	// A UL NAS TRANSPORT: payload-container-type octet, an LV-E payload container,
	// then a PDU session id (0x12) and request type (0x8-) optional IE.
	f.Add([]byte{uint8(EPD5GMM), 0x00, uint8(MsgULNASTransport), 0x01, 0x00, 0x02, 0x2e, 0x01, 0x12, 0x05, 0x81})
	// A DEREGISTRATION REQUEST (UE originating): de-registration-type octet then an
	// LV-E 5GS mobile identity.
	f.Add([]byte{uint8(EPD5GMM), 0x00, uint8(MsgDeregistrationRequestUEOrig), 0x01, 0x00, 0x0b, 0xf2, 0x00, 0xf1, 0x10, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01})
	// A SECURITY MODE COMPLETE with an IMEISV (0x77) and NAS message container (0x71).
	f.Add([]byte{uint8(EPD5GMM), 0x00, uint8(MsgSecurityModeComplete), 0x77, 0x00, 0x09, 0x35, 0x21, 0x43, 0x65, 0x87, 0x09, 0x21, 0x43, 0x65})

	f.Fuzz(func(t *testing.T, b []byte) {
		roundTrip(t, "ParseMessage", ParseMessage, b)

		// The invariant the entry point promises: a message comes back exactly
		// when the error is soft.
		msg, err := ParseMessage(b)
		if (msg != nil) != nas.SoftOnly(err) {
			t.Fatalf("ParseMessage(% x) = %v, %v: a message must come back exactly when the error is soft", b, msg, err)
		}

		// The peekers read the same header without decoding, and must not panic
		// where the decoder would have reported an error.
		_, _ = PeekProtocolDiscriminator(b)
		_, _ = PeekSecurityHeaderType(b)
		_, _ = PeekMessageType(b)
		_, _ = PeekGSMMessageType(b)
	})
}

// FuzzParseSecurityProtectedMessage exercises the wrapper parser, which reads the
// least trusted octets in the stack: they arrive before any MAC has been checked.
func FuzzParseSecurityProtectedMessage(f *testing.F) {
	f.Add([]byte{uint8(EPD5GMM), 0x02, 0, 0, 0, 0, 0x2A, 0xDE})
	f.Add([]byte{uint8(EPD5GMM), 0x01, 0xAA, 0xBB, 0xCC, 0xDD, 0x00, uint8(EPD5GMM), 0x00, uint8(MsgRegistrationComplete)})
	f.Add([]byte{uint8(EPD5GMM), 0x04})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, b []byte) {
		roundTrip(t, "SecurityProtectedMessage", ParseSecurityProtectedMessage, b)
	})
}

// ieCodecs are the information elements a message hands attacker-controlled
// octets to. Each is fuzzed on its own because an element is reached through a
// message only when everything before it decoded, which a whole-message target
// rarely achieves for the ones deep in a variable part.
var ieCodecs = []func(*testing.T, []byte){
	func(t *testing.T, b []byte) { roundTrip(t, "SUCI", ParseSUCI, b) },
	func(t *testing.T, b []byte) { roundTrip(t, "PEI", ParsePEI, b) },
	func(t *testing.T, b []byte) { roundTrip(t, "MobileIdentity", ParseMobileIdentity, b) },
	func(t *testing.T, b []byte) { roundTrip(t, "DNN", ParseDNN, b) },
	func(t *testing.T, b []byte) { roundTrip(t, "GUTI", ParseGUTI, b) },
	func(t *testing.T, b []byte) { roundTrip(t, "STMSI", ParseSTMSI, b) },
	func(t *testing.T, b []byte) {
		roundTrip(t, "UESecurityCapability", ParseUESecurityCapability, b)
	},
	func(t *testing.T, b []byte) { roundTrip(t, "SNSSAI", ParseSNSSAI, b) },
	func(t *testing.T, b []byte) { roundTrip(t, "SessionAMBR", ParseSessionAMBR, b) },
	func(t *testing.T, b []byte) { roundTrip(t, "PDUAddress", ParsePDUAddress, b) },
	func(t *testing.T, b []byte) { roundTrip(t, "QoSRules", ParseQoSRules, b) },
	func(t *testing.T, b []byte) {
		roundTrip(t, "QoSFlowDescriptions", ParseQoSFlowDescriptions, b)
	},
	func(t *testing.T, b []byte) { roundTrip(t, "TAIList", ParseTAIList, b) },
	func(t *testing.T, b []byte) { roundTrip(t, "NSSAI", ParseNSSAI, b) },
	func(t *testing.T, b []byte) { roundTrip(t, "PSIBitmap", ParsePSIBitmap, b) },
	func(t *testing.T, b []byte) { roundTrip(t, "GMMCapability", ParseGMMCapability, b) },
	func(t *testing.T, b []byte) {
		roundTrip(t, "NetworkFeatureSupport", ParseNetworkFeatureSupport, b)
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
	f.Add(0, []byte{0x01, 0x00, 0xf1, 0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10, 0x32, 0x54, 0x76, 0x98, 0x00, 0x00, 0x00, 0x01})
	f.Add(1, []byte{0x4b, 0x09, 0x51, 0x24, 0x30, 0x32, 0x57, 0x81})       // IMEI
	f.Add(1, []byte{0x35, 0x21, 0x43, 0x65, 0x87, 0x09, 0x21, 0x43, 0x65}) // IMEISV
	f.Add(3, []byte{0x08, 'i', 'n', 't', 'e', 'r', 'n', 'e', 't'})         // DNN label
	f.Add(4, []byte{0xf2, 0x00, 0xf1, 0x10, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01})
	f.Add(6, []byte{0xf0, 0xf0})
	f.Add(8, []byte{0x06, 0x00, 0x64, 0x06, 0x00, 0x32})
	f.Add(10, []byte{0x00, 0x01, 0x00, 0x03, 0x21, 0x01, 0x02})
	f.Add(12, []byte{0x00, 0x00, 0xf1, 0x10, 0x00, 0x00, 0x01}) // TAI list
	f.Add(17, []byte{0x80, 0x00, 0x0d, 0x04, 0x08, 0x08, 0x08, 0x08})
	f.Add(0, []byte{})
	f.Add(0, []byte{0x00})

	f.Fuzz(func(t *testing.T, which int, b []byte) {
		if len(b) > 0 {
			_ = TypeOfIdentity(b[0])
		}

		ieCodecs[uint(which)%uint(len(ieCodecs))](t, b)
	})
}
