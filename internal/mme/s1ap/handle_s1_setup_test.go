// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

func TestBuildS1SetupResponseMarshals(t *testing.T) {
	resp, err := buildS1SetupResponse(models.PlmnID{Mcc: "001", Mnc: "01"}, 0x1234, 0x56, "ella", 0xff)
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

	if (out.MMEName == nil || *out.MMEName != "ella") || out.RelativeMMECapacity == nil || *out.RelativeMMECapacity != 0xff {
		t.Fatalf("scalar mismatch: %+v", out)
	}

	if len(out.ServedGUMMEIs) != 1 ||
		out.ServedGUMMEIs[0].ServedPLMNs[0] != (s1ap.PLMNIdentity{0x00, 0xf1, 0x10}) ||
		out.ServedGUMMEIs[0].ServedGroupIDs[0] != (s1ap.MMEGroupID{0x12, 0x34}) ||
		out.ServedGUMMEIs[0].ServedMMECs[0] != s1ap.MMECode(0x56) {
		t.Fatalf("ServedGUMMEIs mismatch: %+v", out.ServedGUMMEIs)
	}
}

// An empty list broadcasts no PLMN at all, which is not the same as broadcasting
// one this MME does not serve; the AMF reports unspecified for the same input.
func TestServedTAICauseEmptySupportedTAs(t *testing.T) {
	plmn := s1ap.PLMNIdentity{0x00, 0xf1, 0x10}

	cause, ok := servedTAICause(s1ap.SupportedTAs{}, plmn, []uint16{1})
	if ok {
		t.Fatal("an empty Supported-TA list must not be accepted")
	}

	if cause != causeNoServedTAC {
		t.Errorf("cause = %+v, want unspecified (%+v)", cause, causeNoServedTAC)
	}
}

// A PLMN that is broadcast but not served is a genuine Unknown PLMN, so the two
// rejections stay distinguishable.
func TestServedTAICauseUnservedPLMN(t *testing.T) {
	served := s1ap.PLMNIdentity{0x00, 0xf1, 0x10}
	other := s1ap.PLMNIdentity{0x00, 0xf1, 0x20}

	tas := s1ap.SupportedTAs{{TAC: 1, BroadcastPLMNs: []s1ap.PLMNIdentity{other}}}

	cause, ok := servedTAICause(tas, served, []uint16{1})
	if ok {
		t.Fatal("an unserved PLMN must not be accepted")
	}

	if cause != causeUnknownPLMN {
		t.Errorf("cause = %+v, want unknown-PLMN (%+v)", cause, causeUnknownPLMN)
	}
}
