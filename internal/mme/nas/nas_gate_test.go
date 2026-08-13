// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

func TestIsAttachRequest(t *testing.T) {
	attach := plainAttachNAS(t)

	tests := []struct {
		name string
		nas  []byte
		want bool
	}{
		{"plain attach", attach, true},
		{"integrity-only attach", append([]byte{0x17, 0x00, 0x00, 0x00, 0x00, 0x00}, attach...), true},
		{"ciphered (unpeekable)", append([]byte{0x27, 0x00, 0x00, 0x00, 0x00, 0x00}, attach...), false},
		{"plain EMM STATUS", []byte{0x07, 0x60, 0x00}, false},
		{"non-EMM PD", []byte{0x02, 0x41}, false},
		{"empty", nil, false},
		{"short protected", []byte{0x17, 0x00}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := peekEMMMessageType(tt.nas) == eps.MsgAttachRequest; got != tt.want {
				t.Fatalf("peekEMMMessageType names an ATTACH REQUEST = %v, want %v", got, tt.want)
			}
		})
	}
}

func plainAttachNAS(t *testing.T) []byte {
	t.Helper()

	esm, err := (&eps.PDNConnectivityRequest{PTI: 1, RequestType: 1, PDNType: eps.PDNTypeIPv4}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	nas, err := (&eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: nas.KeySetIdentifier{Value: 7},
		EPSMobileIdentity:   eps.IMSIIdentity(eps.IMSI(testSubscriber.IMSI)),
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70},
		ESMMessageContainer: esm,
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	return nas
}
