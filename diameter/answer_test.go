// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import "testing"

func TestNewExperimentalAnswer(t *testing.T) {
	req := &Message{
		Flags:         FlagRequest | FlagProxiable,
		CommandCode:   8388645,
		ApplicationID: 16777313,
		HopByHopID:    3,
		EndToEndID:    4,
		AVPs: []AVP{
			UTF8String(AVPSessionID, AVPFlagMandatory, 0, "s;1"),
			Grouped(AVPProxyInfo, AVPFlagMandatory, 0, UTF8String(280, AVPFlagMandatory, 0, "proxy")),
		},
	}

	ans := NewExperimentalAnswer(req, Identity{OriginHost: "h", OriginRealm: "r"}, 10415, 5555)

	if ans.IsRequest() || ans.Flags&FlagError != 0 || ans.Flags&FlagProxiable == 0 || ans.HopByHopID != 3 || ans.EndToEndID != 4 {
		t.Fatalf("header = %+v", ans)
	}

	if ans.AVPs[0].Code != AVPSessionID {
		t.Fatalf("first AVP = %d, want Session-Id", ans.AVPs[0].Code)
	}

	if _, ok := ans.Find(AVPResultCode, 0); ok {
		t.Fatal("Result-Code must not accompany Experimental-Result")
	}

	er, ok := ans.Find(AVPExperimentalResult, 0)
	if !ok {
		t.Fatal("Experimental-Result missing")
	}

	inner, err := er.Grouped()
	if err != nil {
		t.Fatal(err)
	}

	vendor, _ := Find(inner, AVPVendorID, 0)
	code, _ := Find(inner, AVPExperimentalResultCode, 0)

	if v, _ := vendor.Unsigned32(); v != 10415 {
		t.Fatalf("Vendor-Id = %d", v)
	}

	if v, _ := code.Unsigned32(); v != 5555 {
		t.Fatalf("Experimental-Result-Code = %d", v)
	}

	if _, ok := ans.Find(AVPProxyInfo, 0); !ok {
		t.Fatal("Proxy-Info not copied")
	}
}

func TestFailedAVP(t *testing.T) {
	inner, err := FailedAVP(Unsigned32(3300, AVPFlagMandatory, 10415, 0)).Grouped()
	if err != nil || len(inner) != 1 || inner[0].Code != 3300 || inner[0].VendorID != 10415 {
		t.Fatalf("Failed-AVP contents = %+v, %v", inner, err)
	}
}
