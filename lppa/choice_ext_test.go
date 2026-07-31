// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lppa

import (
	"testing"

	"github.com/ellanetworks/core/per"
)

// X.691 §23.8: a CHOICE extension index is a normally-small number, not the
// root bit-field. The wrong width leaves the following open type misaligned.
func TestDecodeChoiceIndexExtensionUsesNormallySmall(t *testing.T) {
	for _, extIdx := range []int64{0, 1, 63, 64, 200} {
		w := per.NewWriter()
		w.WriteBit(true) // extension alternative

		if err := per.EncodeNormallySmall(w, per.Aligned, extIdx); err != nil {
			t.Fatal(err)
		}

		if err := per.EncodeOpenTypeBytes(w, per.Aligned, []byte{0xde, 0xad}); err != nil {
			t.Fatal(err)
		}
		// A marker that must still be readable after the open type.
		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 255, 0x5a); err != nil {
			t.Fatal(err)
		}

		w.AlignToByte()
		r := per.NewReader(w.Bytes())

		idx, isExt, err := decodeChoiceIndex(r, causeRootCount)
		if err != nil {
			t.Fatalf("extIdx %d: %v", extIdx, err)
		}

		if !isExt || idx != extIdx {
			t.Fatalf("extIdx %d: got idx=%d isExt=%v", extIdx, idx, isExt)
		}

		body, err := per.DecodeOpenTypeBytes(r, per.Aligned)
		if err != nil {
			t.Fatalf("extIdx %d: open type: %v", extIdx, err)
		}

		if len(body) != 2 || body[0] != 0xde || body[1] != 0xad {
			t.Fatalf("extIdx %d: open type = % x", extIdx, body)
		}

		marker, err := per.DecodeConstrainedWholeNumber(r, per.Aligned, 0, 255)
		if err != nil || marker != 0x5a {
			t.Fatalf("extIdx %d: trailing marker = %d, err = %v (reader desynchronised)", extIdx, marker, err)
		}
	}
}

func TestDecodeChoiceIndexRootUnaffected(t *testing.T) {
	for want := int64(0); want < causeRootCount; want++ {
		w := per.NewWriter()
		w.WriteBit(false)

		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, causeRootCount-1, want); err != nil {
			t.Fatal(err)
		}

		w.AlignToByte()

		idx, isExt, err := decodeChoiceIndex(per.NewReader(w.Bytes()), causeRootCount)
		if err != nil || isExt || idx != want {
			t.Fatalf("want %d: got idx=%d isExt=%v err=%v", want, idx, isExt, err)
		}
	}
}
