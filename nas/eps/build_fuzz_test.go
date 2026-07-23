// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "testing"

// FuzzBuildAttachRequest is structure-aware: it frames a valid EMM header and the
// NAS-key-set/attach-type octet, then appends fuzz-chosen bytes as the mandatory +
// optional part (EPS mobile identity, UE network capability, ESM container, optional
// IEs). This drives the mandatory-field readers and optional-IE walker past the
// header, and asserts the parser never panics on the result.
func FuzzBuildAttachRequest(f *testing.F) {
	// A plausible mandatory tail: LV mobile identity, LV UE network capability, LV-E
	// ESM container.
	f.Add([]byte{0x0b, 0xf6, 0x00, 0xf1, 0x10, 0x00, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x06, 0x02, 0xf0, 0x70, 0xc0, 0x40, 0x19, 0x00, 0x02, 0x01, 0xd0})
	f.Add([]byte{})
	f.Add([]byte{0xff})

	f.Fuzz(func(_ *testing.T, body []byte) {
		msg := Build(MsgAttachRequest).U8(0x71).Raw(body...).Bytes()
		_, _ = ParseAttachRequest(msg)
	})
}

// FuzzBuildTrackingAreaUpdateRequest frames the TAU header and octet, then fuzzes the
// remainder, exercising the mobility-update path's optional-IE walk.
func FuzzBuildTrackingAreaUpdateRequest(f *testing.F) {
	f.Add([]byte{0x0b, 0xf6, 0x00, 0xf1, 0x10, 0x00, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00})
	f.Add([]byte{})

	f.Fuzz(func(_ *testing.T, body []byte) {
		msg := Build(MsgTrackingAreaUpdateRequest).U8(0x0a).Raw(body...).Bytes()
		_, _ = ParseTrackingAreaUpdateRequest(msg)
	})
}
