// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "testing"

// FuzzParseSecurityProtectedNoPanic asserts the security-wrapper parser never
// panics on arbitrary input.
func FuzzParseSecurityProtectedNoPanic(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x27, 0x11, 0x22, 0x33, 0x44, 0x01, 0x07, 0x42})
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

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ParseSecurityProtectedMessage(data)
		_, _ = PeekMessageType(data)
		_, _ = ParseAttachRequest(data)
		_, _ = ParseAuthenticationRequest(data)
		_, _ = ParseAuthenticationResponse(data)
		_, _ = ParseAuthenticationFailure(data)
		_, _ = ParseIdentityRequest(data)
		_, _ = ParseIdentityResponse(data)
		_, _ = ParseSecurityModeCommand(data)
		_, _ = ParseSecurityModeComplete(data)
		_, _ = ParseSecurityModeReject(data)
		_, _ = ParseAttachAccept(data)
		_, _ = ParseAttachComplete(data)
		_, _ = ParseAttachReject(data)
		_, _ = ParseDetachRequestUE(data)
		_, _ = ParseDetachRequestNetwork(data)
		_, _ = ParseDetachAccept(data)
		_, _ = ParseServiceRequest(data)
		_, _ = PeekESMMessageType(data)
		_, _ = ParsePDNConnectivityRequest(data)
		_, _ = ParsePDNConnectivityReject(data)
		_, _ = ParseActivateDefaultEPSBearerContextRequest(data)
		_, _ = ParseActivateDefaultEPSBearerContextAccept(data)
		_, _ = ParseActivateDefaultEPSBearerContextReject(data)
		_, _ = ParseESMInformationRequest(data)
		_, _ = ParseESMInformationResponse(data)
		_, _ = ParseESMStatus(data)
		_, _ = ProtocolDiscriminator(data)
		_, _ = ParsePDNAddress(data)
		_, _ = ParseEPSQoS(data)
		_, _ = DecodeAPN(data)
		_, _ = ParseAPNAMBR(data)
		_, _ = ParseTAIList(data)
		_, _ = ParseUENetworkCapability(data)
	})
}
