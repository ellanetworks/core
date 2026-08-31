// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
)

// TS 38.413 §8.3.3.2
func TestUEContextReleaseCommandSelectsIDAlternative(t *testing.T) {
	cause := ngap.Cause{Group: ngap.CauseGroupNAS, Value: ngap.CauseNASNormalRelease}

	tests := []struct {
		name string
		ran  models.RanUeNgapID
		want string
	}{
		{
			"assigned RAN UE NGAP ID uses the pair",
			2,
			"002900100000020072000400010002000f400140",
		},
		{
			"unassigned RAN UE NGAP ID uses the AMF UE NGAP ID alone",
			models.RanUeNgapIDUnspecified,
			"0029000e000002007200024001000f400140",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt, err := ueContextReleaseCommandBytes(ngap.AMFUENGAPID(1), ngap.RANUENGAPID(tt.ran), cause)
			if err != nil {
				t.Fatalf("build: %v", err)
			}

			pdu, err := ngap.Unmarshal(pkt)
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			im, ok := pdu.(*ngap.InitiatingMessage)
			if !ok {
				t.Fatalf("got %T, want *ngap.InitiatingMessage", pdu)
			}

			cmd, err := ngap.ParseUEContextReleaseCommand(im.Value)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			if got := hex.EncodeToString(pkt); got != tt.want {
				t.Fatalf("encoded\n got %s\nwant %s", got, tt.want)
			}

			wantPair := tt.ran != models.RanUeNgapIDUnspecified
			if cmd.UENGAPIDs.Pair != wantPair {
				t.Fatalf("Pair = %v, want %v", cmd.UENGAPIDs.Pair, wantPair)
			}

			if cmd.UENGAPIDs.AMFUENGAPID != 1 {
				t.Fatalf("AMFUENGAPID = %d, want 1", cmd.UENGAPIDs.AMFUENGAPID)
			}
		})
	}
}
