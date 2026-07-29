// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"

	"github.com/ellanetworks/core/nas"
)

func TestEMMInformationMarshal(t *testing.T) {
	// "Ella" in the GSM 7-bit default alphabet, packed (TS 24.008):
	// coding-scheme octet 0x84 (ext, GSM 7-bit, 4 spare bits) + 45 36 3b 0c.
	b, err := (&EMMInformation{FullNameForNetwork: ptr(nas.NewNetworkName("Ella")), ShortNameForNetwork: ptr(nas.NewNetworkName("Ella"))}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	// EMM header (07 61) + Full name (43, len 05, 8445363b0c) + Short name (45, len 05, …).
	want := "076143058445363b0c45058445363b0c"
	if hex.EncodeToString(b) != want {
		t.Fatalf("EMM INFORMATION = %s, want %s", hex.EncodeToString(b), want)
	}
}

func TestEMMInformationMarshalEmpty(t *testing.T) {
	b, err := (&EMMInformation{}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	// No network-name IEs: just the EMM header.
	if hex.EncodeToString(b) != "0761" {
		t.Fatalf("empty EMM INFORMATION = %s, want 0761", hex.EncodeToString(b))
	}
}

// TestEMMInformationTimeRoundTrip checks the time-zone elements TS 24.301 table
// 8.2.13.1 defines survive a round trip, including the two the message carries
// as type-3 TVs whose length the table alone declares.
func TestEMMInformationTimeRoundTrip(t *testing.T) {
	when, err := nas.NewTimeZoneAndTime(time.Date(2026, time.July, 28, 9, 41, 0, 0, time.FixedZone("", 3600)))
	if err != nil {
		t.Fatal(err)
	}

	zone, err := nas.NewTimeZone(time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	dst := nas.DaylightSavingOneHour

	in := &EMMInformation{
		FullNameForNetwork: ptr(nas.NewNetworkName("Ella")),
		LocalTimeZone:      &zone,
		UniversalTime:      &when,
		DaylightSavingTime: &dst,
	}

	b, err := in.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseEMMInformation(b)
	if err != nil {
		t.Fatal(err)
	}

	if out.LocalTimeZone == nil || *out.LocalTimeZone != zone ||
		out.UniversalTime == nil || *out.UniversalTime != when ||
		out.DaylightSavingTime == nil || *out.DaylightSavingTime != dst ||
		out.FullNameForNetwork == nil || out.FullNameForNetwork.Name != "Ella" {
		t.Fatalf("round-trip mismatch:\n in  %+v\n out %+v", in, out)
	}

	again, err := out.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(again, b) {
		t.Fatalf("re-encode = % x, want % x", again, b)
	}
}
