// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/per"
)

// A value outside an extensible ENUMERATED's root must not encode. Left to
// per.EncodeEnumerated it becomes the (v-nRoot)'th extension addition, putting
// a value TS 36.413 has not defined on the wire — the encode-side mirror of the
// decode guard in TestEnumExtensionIsNotComprehended.
func TestEnumOutsideRootIsNotEncoded(t *testing.T) {
	tests := []struct {
		name  string
		nRoot int64
		make  func(int64) per.Marshaler
	}{
		{"PagingDRX", pagingDRXRootCount, func(v int64) per.Marshaler { return PagingDRX(v) }},
		{"TimeToWait", timeToWaitRootCount, func(v int64) per.Marshaler { return TimeToWait(v) }},
		{"RRCEstablishmentCause", rrcEstablishmentCauseRootCount, func(v int64) per.Marshaler { return RRCEstablishmentCause(v) }},
		{"UERetentionInformation", ueRetentionInformationRootCount, func(v int64) per.Marshaler { return UERetentionInformation(v) }},

		// HandoverType is absent deliberately: TS 36.413 §9.2.1.13 assigns two
		// extension additions Ella encodes, so its bound is the assigned count
		// rather than the root. TestHandoverTypeInterSystemValues covers it.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The last root value must still encode.
			w := per.NewWriter()
			if err := tt.make(tt.nRoot-1).MarshalPER(w, per.Aligned); err != nil {
				t.Fatalf("root value %d: %v", tt.nRoot-1, err)
			}

			for _, v := range []int64{tt.nRoot, tt.nRoot + 1, tt.nRoot + 200} {
				w := per.NewWriter()
				if err := tt.make(v).MarshalPER(w, per.Aligned); err == nil {
					t.Errorf("value %d outside the %d root values encoded as %x, want an error", v, tt.nRoot, w.Bytes())
				}
			}
		})
	}
}
