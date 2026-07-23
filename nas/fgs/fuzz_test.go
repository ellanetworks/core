// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "testing"

// FuzzParse exercises every 5GMM/5GSM decoder on arbitrary input. They run on
// untrusted N1 data, so they must never panic — every read is bounded by the
// common.Reader; a malformed message returns an error.
func FuzzParse(f *testing.F) {
	f.Add([]byte{EPD5GSM, 5, 1, uint8(MsgPDUSessionEstablishmentRequest), 0xFF, 0xFF, 0x91, 0xB1})
	f.Add([]byte{EPD5GSM, 5, 1, uint8(MsgGSMStatus), 0x2F})
	f.Add([]byte{EPD5GSM, 5, 1, uint8(MsgPDUSessionReleaseRequest), iei5GSMCause, 0x24})
	f.Add([]byte{EPD5GMM, 0x02, 0, 0, 0, 0, 0x2A, 0xDE})
	// A REGISTRATION REQUEST: header, registration-type/ngKSI octet, then an LV-E
	// 5GS mobile identity (5G-GUTI), to seed the mandatory + optional-IE walk.
	f.Add([]byte{EPD5GMM, 0x00, uint8(MsgRegistrationRequest), 0x79, 0x00, 0x0b, 0xf2, 0x00, 0xf1, 0x10, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01})
	// A SERVICE REQUEST: service-type/ngKSI octet then the 7-octet 5G-S-TMSI (LV-E).
	f.Add([]byte{EPD5GMM, 0x00, uint8(MsgServiceRequest), 0x70, 0x00, 0x07, 0xf4, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05})
	// A UL NAS TRANSPORT: payload-container-type octet, an LV-E payload container,
	// then a PDU session id (0x12) and request type (0x8-) optional IE.
	f.Add([]byte{EPD5GMM, 0x00, uint8(MsgULNASTransport), 0x01, 0x00, 0x02, 0x2e, 0x01, 0x12, 0x05, 0x81})
	// A DEREGISTRATION REQUEST (UE originating): de-registration-type octet then an
	// LV-E 5GS mobile identity.
	f.Add([]byte{EPD5GMM, 0x00, uint8(MsgDeregistrationRequestUEOrig), 0x01, 0x00, 0x0b, 0xf2, 0x00, 0xf1, 0x10, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01})
	// A SECURITY MODE COMPLETE with an IMEISV (0x77) and NAS message container (0x71).
	f.Add([]byte{EPD5GMM, 0x00, uint8(MsgSecurityModeComplete), 0x77, 0x00, 0x09, 0x35, 0x21, 0x43, 0x65, 0x87, 0x09, 0x21, 0x43, 0x65})

	f.Fuzz(func(_ *testing.T, b []byte) {
		// Security wrapper and 5GMM uplink messages.
		_, _ = ParseSecurityProtectedMessage(b)
		_, _ = ParseRegistrationRequest(b)
		_, _ = ParseServiceRequest(b)
		_, _ = ParseULNASTransport(b)
		_, _ = ParseDeregistrationRequestUEOriginating(b)
		_, _ = ParseSecurityModeComplete(b)
		_, _ = ParseSecurityModeReject(b)
		_, _ = ParseAuthenticationResponse(b)
		_, _ = ParseAuthenticationFailure(b)
		_, _ = ParseIdentityResponse(b)
		_, _ = ParseNotificationResponse(b)
		_, _ = ParseGMMStatus(b)
		// 5GSM messages.
		_, _ = ParsePDUSessionEstablishmentRequest(b)
		_, _ = ParsePDUSessionEstablishmentAccept(b)
		_, _ = ParsePDUSessionModificationCommand(b)
		_, _ = ParsePDUSessionModificationComplete(b)
		_, _ = ParsePDUSessionModificationCommandReject(b)
		_, _ = ParsePDUSessionReleaseRequest(b)
		_, _ = ParsePDUSessionReleaseComplete(b)
		_, _ = ParseGSMStatus(b)
		// Standalone IE / container decoders.
		_, _ = ParseUESecurityCapability(b)
		_, _ = ParsePCOContainerIDs(b)
		// Peekers must also never panic.
		_, _ = PeekMessageType(b)
		_, _ = PeekGSMMessageType(b)
	})
}

// FuzzIdentityCodecs exercises the 5GS mobile-identity decoders (SUCI, PEI/IMEISV,
// DNN, type-of-identity), which unpack attacker-controlled lengths and nibbles — a
// prime spot for an off-by-one. They must never panic.
func FuzzIdentityCodecs(f *testing.F) {
	f.Add([]byte{0x01, 0x00, 0xf1, 0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10, 0x32, 0x54, 0x76, 0x98, 0x00, 0x00, 0x00, 0x01})
	f.Add([]byte{0x4b, 0x09, 0x51, 0x24, 0x30, 0x32, 0x57, 0x81})       // IMEI
	f.Add([]byte{0x35, 0x21, 0x43, 0x65, 0x87, 0x09, 0x21, 0x43, 0x65}) // IMEISV
	f.Add([]byte{0x08, 'i', 'n', 't', 'e', 'r', 'n', 'e', 't'})         // DNN label
	f.Add([]byte{})
	f.Add([]byte{0x00})

	f.Fuzz(func(_ *testing.T, b []byte) {
		if len(b) > 0 {
			_ = TypeOfIdentity(b[0])
		}

		_, _, _ = SUCIToString(b)
		_, _ = PEIToString(b)
		_ = DNNToString(b)
	})
}

// FuzzBuildRegistrationRequest is structure-aware: it frames a valid 5GMM header,
// registration-type octet, and LV-E mobile identity, then appends fuzz-chosen bytes
// as the optional-IE part. This drives the IE table and optional-IE walker far
// deeper than random input, and asserts the parser never panics on the result.
func FuzzBuildRegistrationRequest(f *testing.F) {
	f.Add([]byte{0xf2, 0x00, 0xf1, 0x10, 0x01}, []byte{0x2e, 0x02, 0xe0, 0xe0})
	f.Add([]byte{}, []byte{0x10, 0x01, 0xff})
	f.Add([]byte{0x01}, []byte{})

	f.Fuzz(func(_ *testing.T, mobileID, optionalIEs []byte) {
		msg := Build(MsgRegistrationRequest).U8(0x01).LVE(mobileID).Raw(optionalIEs...).Bytes()
		_, _ = ParseRegistrationRequest(msg)
	})
}

// FuzzBuildULNASTransport frames a valid header and LV-E payload container, then
// fuzzes the optional-IE part (PDU session id, request type, S-NSSAI, DNN, …).
func FuzzBuildULNASTransport(f *testing.F) {
	f.Add([]byte{0x2e, 0x01}, []byte{0x12, 0x05, 0x81, 0x22, 0x01, 0x01})
	f.Add([]byte{}, []byte{0x25, 0x08, 0x08, 'i', 'n', 't', 'e', 'r', 'n', 'e', 't'})

	f.Fuzz(func(_ *testing.T, payloadContainer, optionalIEs []byte) {
		msg := Build(MsgULNASTransport).U8(0x01).LVE(payloadContainer).Raw(optionalIEs...).Bytes()
		_, _ = ParseULNASTransport(msg)
	})
}
