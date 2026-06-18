// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"encoding/hex"
	"testing"
)

func TestEMMInformationMarshal(t *testing.T) {
	// "Ella" in the GSM 7-bit default alphabet, packed (TS 24.008 §10.5.3.5a):
	// coding-scheme octet 0x84 (ext, GSM 7-bit, 4 spare bits) + 45 36 3b 0c.
	b, err := (&EMMInformation{FullNetworkName: "Ella", ShortNetworkName: "Ella"}).Marshal()
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
	b, err := (&EMMInformation{}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// No network-name IEs: just the EMM header.
	if hex.EncodeToString(b) != "0761" {
		t.Fatalf("empty EMM INFORMATION = %s, want 0761", hex.EncodeToString(b))
	}
}
