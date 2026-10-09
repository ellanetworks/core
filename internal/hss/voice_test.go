// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss_test

import (
	"context"
	"errors"
	"testing"
)

func TestVoiceSupported(t *testing.T) {
	cases := []struct {
		name    string
		imsi    string
		noPCSCF bool
		want    bool
	}{
		{name: "subscriber with an MSISDN and the ims data network", imsi: testIMSI, want: true},
		{name: "no P-CSCF address", imsi: testIMSI, noPCSCF: true},
		{name: "no MSISDN", imsi: testNoMSISDN},
		{name: "no ims data network", imsi: testNoIMS},
		{name: "unknown subscriber", imsi: "001010000000099"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeIMSStore()
			store.noPCSCF = tc.noPCSCF

			got, err := newHSS(store).VoiceSupported(context.Background(), tc.imsi)
			if err != nil {
				t.Fatalf("VoiceSupported: %v", err)
			}

			if got != tc.want {
				t.Fatalf("VoiceSupported = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestVoiceSupportedStoreFailure(t *testing.T) {
	store := newFakeIMSStore()
	store.fail = true

	if _, err := newHSS(store).VoiceSupported(context.Background(), testIMSI); !errors.Is(err, errStoreDown) {
		t.Fatalf("VoiceSupported error = %v, want %v", err, errStoreDown)
	}
}
