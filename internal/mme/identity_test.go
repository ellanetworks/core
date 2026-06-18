// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

func TestEncodePLMN(t *testing.T) {
	cases := []struct {
		mcc, mnc string
		want     s1ap.PLMNIdentity
	}{
		{"001", "01", s1ap.PLMNIdentity{0x00, 0xf1, 0x10}},  // 2-digit MNC
		{"310", "260", s1ap.PLMNIdentity{0x13, 0x20, 0x06}}, // 3-digit MNC
	}
	for _, c := range cases {
		got, err := encodePLMN(models.PlmnID{Mcc: c.mcc, Mnc: c.mnc})
		if err != nil {
			t.Fatalf("%s/%s: %v", c.mcc, c.mnc, err)
		}

		if got != c.want {
			t.Fatalf("%s/%s: got % x, want % x", c.mcc, c.mnc, got, c.want)
		}
	}
}

func TestEncodePLMNInvalid(t *testing.T) {
	if _, err := encodePLMN(models.PlmnID{Mcc: "1", Mnc: "01"}); err == nil {
		t.Fatal("expected error for malformed MCC")
	}
}

func TestBuildS1SetupResponseMarshals(t *testing.T) {
	resp, err := buildS1SetupResponse(models.PlmnID{Mcc: "001", Mnc: "01"}, 0x1234, 0x56)
	if err != nil {
		t.Fatal(err)
	}

	b, err := resp.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// successfulOutcome / procedureCode 17 (s1Setup).
	if b[0] != 0x20 || b[1] != 0x11 {
		t.Fatalf("envelope prefix = % x, want 20 11", b[:2])
	}

	pdu, err := s1ap.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	so, ok := pdu.(*s1ap.SuccessfulOutcome)
	if !ok {
		t.Fatalf("got %T, want *SuccessfulOutcome", pdu)
	}

	out, err := s1ap.ParseS1SetupResponse(so.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.MMEName != mmeName || out.RelativeMMECapacity != relativeMMECapacity {
		t.Fatalf("scalar mismatch: %+v", out)
	}

	if len(out.ServedGUMMEIs) != 1 ||
		out.ServedGUMMEIs[0].ServedPLMNs[0] != (s1ap.PLMNIdentity{0x00, 0xf1, 0x10}) ||
		out.ServedGUMMEIs[0].ServedGroupIDs[0] != (s1ap.MMEGroupID{0x12, 0x34}) ||
		out.ServedGUMMEIs[0].ServedMMECs[0] != s1ap.MMECode(0x56) {
		t.Fatalf("ServedGUMMEIs mismatch: %+v", out.ServedGUMMEIs)
	}
}
