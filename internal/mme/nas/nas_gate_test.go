// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

func TestIsInitialAttach(t *testing.T) {
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
			if got := isInitialAttach(tt.nas); got != tt.want {
				t.Fatalf("isInitialAttach = %v, want %v", got, tt.want)
			}
		})
	}
}

func plainAttachNAS(t *testing.T) []byte {
	t.Helper()

	esm, err := (&eps.PDNConnectivityRequest{ProcedureTransactionIdentity: 1, RequestType: 1, PDNType: eps.PDNTypeIPv4}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	nas, err := (&eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: 7,
		EPSMobileIdentity:   eps.EPSMobileIdentity{Type: eps.IdentityIMSI, Digits: testSubscriber.IMSI},
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70}.Marshal(),
		ESMMessageContainer: esm,
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return nas
}
