// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "testing"

// TestIEMaxima checks that each element with a length its spec fixes rejects a
// value longer than that, rather than decoding it and re-encoding an element no
// peer would accept.
func TestIEMaxima(t *testing.T) {
	tests := []struct {
		name  string
		parse func([]byte) error
		max   int
	}{
		{"EPS QoS", func(b []byte) error { _, err := ParseEPSQoS(b); return err }, maxEPSQoSLen},
		{"APN-AMBR", func(b []byte) error { _, err := ParseAPNAMBR(b); return err }, maxAPNAMBRLen},
		{"UE network capability", func(b []byte) error { _, err := ParseUENetworkCapability(b); return err }, maxUENetworkCapabilityLen},
		{"MS network capability", func(b []byte) error { _, err := ParseMSNetworkCapability(b); return err }, maxMSNetworkCapabilityLen},
		{"UE security capability", func(b []byte) error { _, err := ParseUESecurityCapability(b); return err }, 5},
		{"mobile identity", func(b []byte) error { _, err := ParseMobileIdentity(b); return err }, maxMobileIdentityLen},
		{"EPS mobile identity", func(b []byte) error { _, err := ParseEPSMobileIdentity(b); return err }, maxEPSMobileIdentityLen},
		{"EPS network feature support", func(b []byte) error { _, err := ParseNetworkFeatureSupport(b); return err }, maxNetworkFeatureSupportLen},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.parse(make([]byte, tc.max+1)); err == nil {
				t.Errorf("%d octets: want an error, got none", tc.max+1)
			}
		})
	}
}

// TestEPSQoSValueLengths pins the value lengths TS 24.301 §9.9.4.3 allows: the
// QCI octet alone, then groups of four.
func TestEPSQoSValueLengths(t *testing.T) {
	for n := range maxEPSQoSLen + 2 {
		want := n == 1 || n == 5 || n == 9 || n == 13

		_, err := ParseEPSQoS(make([]byte, n))
		if (err == nil) != want {
			t.Errorf("%d octets: err = %v, want ok = %t", n, err, want)
		}

		q := EPSQoS{BitRates: make([]byte, max(n-1, 0))}
		if _, err := q.MarshalBinary(); n > 0 && (err == nil) != want {
			t.Errorf("encode %d octets: err = %v, want ok = %t", n, err, want)
		}
	}
}

// TestAPNAMBRValueLengths pins the value lengths TS 24.301 §9.9.4.2 allows.
func TestAPNAMBRValueLengths(t *testing.T) {
	for n := range maxAPNAMBRLen + 2 {
		want := n == 2 || n == 4 || n == 6

		_, err := ParseAPNAMBR(make([]byte, n))
		if (err == nil) != want {
			t.Errorf("%d octets: err = %v, want ok = %t", n, err, want)
		}

		a := APNAMBR{Extended: make([]byte, max(n-2, 0))}
		if _, err := a.MarshalBinary(); n >= 2 && (err == nil) != want {
			t.Errorf("encode %d octets: err = %v, want ok = %t", n, err, want)
		}
	}
}

// TestTMSIMobileIdentityLength pins the TMSI form of the mobile identity to the
// five octets TS 24.008 §10.5.1.4 gives it, so a longer value is rejected rather
// than silently truncated to something that re-encodes shorter than it arrived.
func TestTMSIMobileIdentityLength(t *testing.T) {
	valid := []byte{uint8(MobileIdentityTMSI), 0xDE, 0xAD, 0xBE, 0xEF}

	got, err := ParseMobileIdentity(valid)
	if err != nil {
		t.Fatalf("ParseMobileIdentity(% x): %v", valid, err)
	}

	if got.TMSI == nil || *got.TMSI != [4]byte{0xDE, 0xAD, 0xBE, 0xEF} {
		t.Fatalf("TMSI = %v, want deadbeef", got.TMSI)
	}

	for _, n := range []int{4, 6, 7} {
		b := make([]byte, n)
		b[0] = uint8(MobileIdentityTMSI)

		if _, err := ParseMobileIdentity(b); err == nil {
			t.Errorf("%d octets: want an error, got none", n)
		}
	}
}
