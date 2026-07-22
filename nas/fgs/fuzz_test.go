// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "testing"

// FuzzParse exercises the 5GSM decoders on arbitrary input. They run on
// untrusted N1 data, so they must never panic — every read is bounded by the
// common.Reader; a malformed message returns an error.
func FuzzParse(f *testing.F) {
	f.Add([]byte{EPD5GSM, 5, 1, uint8(MsgPDUSessionEstablishmentRequest), 0xFF, 0xFF, 0x91, 0xB1})
	f.Add([]byte{EPD5GSM, 5, 1, uint8(Msg5GSMStatus), 0x2F})
	f.Add([]byte{EPD5GSM, 5, 1, uint8(MsgPDUSessionReleaseRequest), iei5GSMCause, 0x24})
	f.Add([]byte{EPD5GMM, 0x02, 0, 0, 0, 0, 0x2A, 0xDE})

	f.Fuzz(func(_ *testing.T, b []byte) {
		_, _ = ParsePDUSessionEstablishmentRequest(b)
		_, _ = ParseStatus5GSM(b)
		_, _ = ParsePDUSessionReleaseRequest(b)
		_, _ = ParsePDUSessionReleaseComplete(b)
		_, _ = ParsePDUSessionModificationComplete(b)
		_, _ = ParsePDUSessionModificationCommandReject(b)
		_, _ = ParseSecurityProtectedMessage(b)
		_, _ = ParseAuthenticationResponse(b)
		_, _ = ParseAuthenticationFailure(b)
		_, _ = ParseIdentityResponse(b)
		_, _ = ParseStatus5GMM(b)
		_, _ = ParseSecurityModeReject(b)
	})
}
