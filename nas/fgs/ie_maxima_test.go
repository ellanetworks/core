// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "testing"

// TestIEMaxima checks that each element with a length TS 24.501 fixes rejects a
// value longer than that, rather than decoding it and re-encoding an element no
// peer would accept.
func TestIEMaxima(t *testing.T) {
	tests := []struct {
		name  string
		parse func([]byte) error
		max   int
	}{
		{"UE security capability", func(b []byte) error { _, err := ParseUESecurityCapability(b); return err }, maxUESecurityCapabilityLen},
		{"5GMM capability", func(b []byte) error { _, err := ParseGMMCapability(b); return err }, maxGMMCapabilityLen},
		{"5GSM capability", func(b []byte) error { _, err := ParseGSMCapability(b); return err }, maxGSMCapabilityLen},
		{"network feature support", func(b []byte) error { _, err := ParseNetworkFeatureSupport(b); return err }, maxNetworkFeatureSupportLen},
		{"PDU session status", func(b []byte) error { _, err := ParsePSIBitmap(b); return err }, maxPSIBitmapLen},
		{"NSSAI", func(b []byte) error { _, err := ParseNSSAI(b); return err }, maxNSSAILen},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.parse(make([]byte, tc.max+1)); err == nil {
				t.Errorf("%d octets: want an error, got none", tc.max+1)
			}
		})
	}
}

// TestIEMaximaOnEncode checks the same bounds hold on the encode side, so a
// value built in memory cannot exceed what the element carries.
func TestIEMaximaOnEncode(t *testing.T) {
	if _, err := (GMMCapability{Rest: make([]byte, maxGMMCapabilityLen)}).MarshalBinary(); err == nil {
		t.Error("5GMM capability: want an error, got none")
	}

	if _, err := (GSMCapability{Rest: make([]byte, maxGSMCapabilityLen)}).MarshalBinary(); err == nil {
		t.Error("5GSM capability: want an error, got none")
	}

	if _, err := (PSIBitmap{Rest: make([]byte, maxPSIBitmapLen)}).MarshalBinary(); err == nil {
		t.Error("PDU session status: want an error, got none")
	}

	c := UESecurityCapability{HasEPS: true, Spare: make([]byte, maxUESecurityCapabilityLen)}
	if _, err := c.MarshalBinary(); err == nil {
		t.Error("UE security capability: want an error, got none")
	}

	n := NetworkFeatureSupport{HasOctet4: true, Rest: make([]byte, maxNetworkFeatureSupportLen)}
	if _, err := n.MarshalBinary(); err == nil {
		t.Error("network feature support: want an error, got none")
	}

	sd := [3]byte{1, 2, 3}

	nssai := make(NSSAI, 0, 64)
	for range 64 {
		nssai = append(nssai, SNSSAI{SST: 1, SD: &sd})
	}

	if _, err := nssai.MarshalBinary(); err == nil {
		t.Error("NSSAI: want an error, got none")
	}
}
