// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps_test

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/nastest"
)

// roundTrip asserts that a built message encodes, that its encoding decodes, and
// that encoding it again yields the same octets. This test package is external —
// nastest imports eps, so the in-package helper of the same name is out of reach —
// and value identity is left to the in-package targets, which can normalize the
// order encoding gives preserved elements before comparing.
func roundTrip[T interface{ MarshalBinary() ([]byte, error) }](
	t *testing.T, name string, parse func([]byte) (T, error), b []byte,
) {
	t.Helper()

	msg, err := parse(b)
	if err != nil && !nas.SoftOnly(err) {
		return
	}

	raw, err := msg.MarshalBinary()
	if err != nil {
		t.Fatalf("%s: decoded % x but would not encode: %v", name, b, err)
	}

	again, err := parse(raw)
	if err != nil && !nas.SoftOnly(err) {
		t.Fatalf("%s: own encoding % x did not decode: %v (from % x)", name, raw, err, b)
	}

	stable, err := again.MarshalBinary()
	if err != nil {
		t.Fatalf("%s: re-encode failed: %v", name, err)
	}

	if !bytes.Equal(stable, raw) {
		t.Fatalf("%s: encoding is not idempotent\n first % x\nsecond % x", name, raw, stable)
	}
}

// FuzzBuildAttachRequest is structure-aware: it frames a valid EMM header and the
// NAS-key-set/attach-type octet, then appends fuzz-chosen bytes as the mandatory +
// optional part (EPS mobile identity, UE network capability, ESM container, optional
// IEs). This drives the mandatory-field readers and optional-IE walker past the
// header, and asserts the round-trip properties hold for the result.
func FuzzBuildAttachRequest(f *testing.F) {
	// A plausible mandatory tail: LV mobile identity, LV UE network capability, LV-E
	// ESM container.
	f.Add([]byte{0x0b, 0xf6, 0x00, 0xf1, 0x10, 0x00, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x06, 0x02, 0xf0, 0x70, 0xc0, 0x40, 0x19, 0x00, 0x02, 0x01, 0xd0})
	f.Add([]byte{})
	f.Add([]byte{0xff})

	f.Fuzz(func(t *testing.T, body []byte) {
		msg := nastest.BuildEMM(eps.MsgAttachRequest).U8(0x71).Raw(body...).Bytes()
		roundTrip(t, "AttachRequest", eps.ParseAttachRequest, msg)
	})
}

// FuzzBuildTrackingAreaUpdateRequest frames the TAU header and octet, then fuzzes the
// remainder, exercising the mobility-update path's optional-IE walk, and asserts the
// round-trip properties hold for the result.
func FuzzBuildTrackingAreaUpdateRequest(f *testing.F) {
	f.Add([]byte{0x0b, 0xf6, 0x00, 0xf1, 0x10, 0x00, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, body []byte) {
		msg := nastest.BuildEMM(eps.MsgTrackingAreaUpdateRequest).U8(0x0a).Raw(body...).Bytes()
		roundTrip(t, "TrackingAreaUpdateRequest", eps.ParseTrackingAreaUpdateRequest, msg)
	})
}
