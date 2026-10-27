// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/ellanetworks/core/internal/roaming/wire"
)

func requireHeaderError(t *testing.T, err error, name string, missing bool) {
	t.Helper()

	var ie *wire.IEError
	if !errors.As(err, &ie) || ie.Param != "header: "+name || ie.Missing != missing {
		t.Fatalf("error = %v, want header %s missing=%v", err, name, missing)
	}

	if pd := wire.ProblemFromError(err); pd.Status != http.StatusBadRequest {
		t.Fatalf("problem = %+v", pd)
	}
}

func TestParseTargetAPIRoot(t *testing.T) {
	r, err := ParseTargetAPIRoot([]string{"https://UDM.example.com:443/x"})
	if err != nil || r != (APIRoot{Scheme: "https", Authority: "udm.example.com:443", Prefix: "/x"}) {
		t.Fatalf("ParseTargetAPIRoot = %+v, %v", r, err)
	}

	_, err = ParseTargetAPIRoot(nil)
	requireHeaderError(t, err, HeaderTargetAPIRoot, true)

	_, err = ParseTargetAPIRoot([]string{"https://a.example.com", "https://b.example.com"})
	requireHeaderError(t, err, HeaderTargetAPIRoot, false)

	_, err = ParseTargetAPIRoot([]string{"ftp://a.example.com"})
	requireHeaderError(t, err, HeaderTargetAPIRoot, false)
}

func TestOriginatingNetworkID(t *testing.T) {
	for in, want := range map[string]OriginatingNetworkID{
		"123-45":    {PlmnID: wire.PlmnID{Mcc: "123", Mnc: "45"}},
		" 999-070 ": {PlmnID: wire.PlmnID{Mcc: "999", Mnc: "070"}},
		"123-45-000007ed9d5": {
			PlmnID: wire.PlmnID{Mcc: "123", Mnc: "45"}, Nid: "000007ed9d5",
		},
		"123-45; src: SEPP-sepp001.sepp.5gc.mnc045.mcc123.3gppnetwork.org": {
			PlmnID: wire.PlmnID{Mcc: "123", Mnc: "45"}, SrcType: "SEPP", SrcFQDN: "sepp001.sepp.5gc.mnc045.mcc123.3gppnetwork.org",
		},
		"123-45;src:\tscp-scp1.op.com": {
			PlmnID: wire.PlmnID{Mcc: "123", Mnc: "45"}, SrcType: "SCP", SrcFQDN: "scp1.op.com",
		},
		"123-45; SRC: Sepp-sepp.op.com": {
			PlmnID: wire.PlmnID{Mcc: "123", Mnc: "45"}, SrcType: "SEPP", SrcFQDN: "sepp.op.com",
		},
	} {
		got, err := ParseOriginatingNetworkID([]string{in})
		if err != nil || got != want {
			t.Errorf("ParseOriginatingNetworkID(%q) = %+v, %v", in, got, err)
		}
	}

	for _, in := range []string{
		"", "123", "12-45", "123-4", "123-4567", "123-45-0000", "123-45-000007ed9dz", "123-45-000007ed9d5-1",
		"123-45; src:SEPP-sepp.com", "123-45; source: SEPP-x.com", "123-45; src: NRF-nrf.com", "123-45; src: SEPP-a",
		"123-45; src: SEPP", "123-45; src: SEPP-a_b.com", "123-45;", "abc-45", "123-45; src :SEPP-sepp.com",
	} {
		_, err := ParseOriginatingNetworkID([]string{in})
		requireHeaderError(t, err, HeaderOriginatingNetworkID, false)
	}

	_, err := ParseOriginatingNetworkID([]string{"123-45", "123-45"})
	requireHeaderError(t, err, HeaderOriginatingNetworkID, false)

	o := OriginatingNetworkID{PlmnID: wire.PlmnID{Mcc: "999", Mnc: "70"}}
	if o.String() != "999-70" || !o.Matches(wire.PlmnID{Mcc: "999", Mnc: "70"}) || o.Matches(wire.PlmnID{Mcc: "999", Mnc: "070"}) {
		t.Fatalf("String/Matches = %q", o.String())
	}

	o.SrcType, o.SrcFQDN = "SEPP", "sepp.5gc.mnc070.mcc999.3gppnetwork.org"
	if o.String() != "999-70; src: SEPP-sepp.5gc.mnc070.mcc999.3gppnetwork.org" {
		t.Fatalf("String = %q", o.String())
	}

	if (OriginatingNetworkID{PlmnID: o.PlmnID, Nid: "000007ed9d5"}).Matches(o.PlmnID) {
		t.Fatal("SNPN matched a PLMN")
	}
}

func TestN32HandshakeID(t *testing.T) {
	if id, err := ParseN32HandshakeID([]string{" 955cac631f953ed8 "}); err != nil || id != "955cac631f953ed8" {
		t.Fatalf("ParseN32HandshakeID = %q, %v", id, err)
	}

	for _, in := range [][]string{{""}, {"955cac631f953ed"}, {"955cac631f953ed80"}, {"955cac631f953edg"}, {"955cac631f953ed8", "955cac631f953ed8"}} {
		_, err := ParseN32HandshakeID(in)
		requireHeaderError(t, err, HeaderN32HandshakeID, false)
	}
}

func TestInterPLMNPurpose(t *testing.T) {
	for in, want := range map[string]InterPLMNPurpose{
		"ROAMING: usecaseA":    {Purpose: PurposeRoaming, AdditionalInfo: "usecaseA"},
		" ROAMING:usecaseA ":   {Purpose: PurposeRoaming, AdditionalInfo: "usecaseA"},
		"INTER_PLMN_MOBILITY":  {Purpose: PurposeInterPLMNMobility},
		"FUTURE_PURPOSE: x%20": {Purpose: "FUTURE_PURPOSE", AdditionalInfo: "x%20"},
	} {
		got, err := ParseInterPLMNPurpose([]string{in})
		if err != nil || got != want {
			t.Errorf("ParseInterPLMNPurpose(%q) = %+v, %v", in, got, err)
		}
	}

	for _, in := range []string{"", ":x", "ROAMING:", "ROAMING: a b", "ROAM ING", "ROAMING: a:b"} {
		_, err := ParseInterPLMNPurpose([]string{in})
		requireHeaderError(t, err, HeaderInterPLMNPurpose, false)
	}

	allowed := []N32Purpose{PurposeRoaming, PurposeInterPLMNMobility}

	for _, ok := range [][]string{nil, {"ROAMING"}, {"INTER_PLMN_MOBILITY: x"}} {
		if err := CheckInterPLMNPurpose(ok, allowed); err != nil {
			t.Errorf("CheckInterPLMNPurpose(%q) = %v", ok, err)
		}
	}

	for _, bad := range [][]string{{"ROAMING_TEST"}, {"SMS_INTERCONNECT"}} {
		var pd wire.ProblemDetails
		if err := CheckInterPLMNPurpose(bad, allowed); !errors.As(err, &pd) || pd.Status != http.StatusForbidden || pd.Cause != wire.CauseRequestedPurposeNotAllowed {
			t.Errorf("CheckInterPLMNPurpose(%q) = %v", bad, err)
		}
	}

	if err := CheckInterPLMNPurpose(nil, []N32Purpose{PurposeSMSInterconnect}); err == nil {
		t.Fatal("absent header accepted without roaming")
	}

	err := CheckInterPLMNPurpose([]string{"ROAMING", "ROAMING"}, allowed)
	requireHeaderError(t, err, HeaderInterPLMNPurpose, false)
}

func TestVia(t *testing.T) {
	if SEPPVia("sepp1.operator.com") != "2.0 SEPP-sepp1.operator.com" || SEPPServer("sepp1.operator.com") != "SEPP-sepp1.operator.com" {
		t.Fatal("SEPPVia/SEPPServer")
	}

	got, err := ParseVia([]string{"HTTP/2.0 SCP-scp1.operator.com, 2.0 SEPP-sepp1.operator.com (relay, x\\)y (nested))", "1.1 proxy:8080, 1.1 [2001:db8::1]:80"})
	if err != nil || len(got) != 4 {
		t.Fatalf("ParseVia = %+v, %v", got, err)
	}

	if got[0] != (ViaEntry{Protocol: "HTTP/2.0", ReceivedBy: "SCP-scp1.operator.com"}) ||
		got[1] != (ViaEntry{Protocol: "2.0", ReceivedBy: "SEPP-sepp1.operator.com", Comment: "(relay, x\\)y (nested))"}) ||
		got[2] != (ViaEntry{Protocol: "1.1", ReceivedBy: "proxy:8080"}) || got[3].ReceivedBy != "[2001:db8::1]:80" {
		t.Fatalf("ParseVia = %+v", got)
	}

	if e, ok := got[1].NetworkEntity(); !ok || e != (NetworkEntity{Type: wire.NFTypeSEPP, ID: "sepp1.operator.com"}) {
		t.Fatalf("NetworkEntity = %+v %v", e, ok)
	}

	if _, ok := got[2].NetworkEntity(); ok {
		t.Fatal("proxy:8080 is not a network entity")
	}

	if got[1].String() != "2.0 SEPP-sepp1.operator.com (relay, x\\)y (nested))" {
		t.Fatalf("String = %q", got[1].String())
	}

	if !IsSEPP(got[1].ReceivedBy, "SEPP1.operator.com.") || IsSEPP(got[0].ReceivedBy, "scp1.operator.com") || IsSEPP("sepp-other.com", "sepp1.operator.com") {
		t.Fatal("IsSEPP")
	}

	if !IsSEPP("sepp-sepp1.operator.com", "sepp1.operator.com") {
		t.Fatal("IsSEPP is case-insensitive on the NF type (TS 29.500 Table 5.2.2.2-2 NOTE)")
	}

	for _, in := range [][]string{
		nil,
		{""},
		{" , "},
		{"2.0"},
		{"2.0 a (unterminated"},
		{"2.0 a b"},
		{"2.0 a@b"},
		{"/ a"},
		{"2.0 SEPP-x (c) extra"},
		{"2.0 a:"},
		{"2.0 a:x"},
		{"2.0 a:123456"},
		{"2.0 a:b:1"},
		{"2.0 [::1"},
		{"2.0 [zz]"},
		{"2.0 [::1]x"},
		{"2.0 [::1]:"},
		{"HTTP/ a"},
		{"HTTP/2.0/1 a"},
		{"2.0 a (c\x01)"},
		{"2.0 a (c)(d)"},
		{"2.0 a (c\\"},
		{"2.0 a (x\\\x01)"},
		{"2.0 a [::1]"},
		{"2.0 a", "2.0 b ("},
	} {
		if _, err := ParseVia(in); err == nil {
			t.Errorf("ParseVia(%q) accepted", in)
		}
	}
}

func TestServer(t *testing.T) {
	for in, want := range map[string]NetworkEntity{
		"SEPP-sepp1.operator.com":                                {Type: wire.NFTypeSEPP, ID: "sepp1.operator.com"},
		"smf-54804518-4191-46b3-955c-ac631f953ed8 (comment)":     {Type: wire.NFTypeSMF, ID: "54804518-4191-46b3-955c-ac631f953ed8"},
		"MB_SMF-54804518-4191-46b3-955c-ac631f953ed8/1.2.0":      {Type: "MB_SMF", ID: "54804518-4191-46b3-955c-ac631f953ed8"},
		"UDM-54804518-4191-46b3-955c-ac631f953ed8/1.2.0 nghttp2": {Type: wire.NFTypeUDM, ID: "54804518-4191-46b3-955c-ac631f953ed8"},
	} {
		if got, ok := ParseServer([]string{in}); !ok || got != want {
			t.Errorf("ParseServer(%q) = %+v %v", in, got, ok)
		}
	}

	for _, in := range [][]string{nil, {""}, {"nginx"}, {"-x"}, {"SEPP-"}, {"vendor v2.8.0"}, {"SEPP-a", "SEPP-b"}, {"SEPP-a/1/2"}} {
		if _, ok := ParseServer(in); ok {
			t.Errorf("ParseServer(%q) accepted", in)
		}
	}
}

func FuzzParseAPIRoot(f *testing.F) {
	for _, s := range []string{"http://a.example.com", "https://[::1]:443/x/y", "http://1.2.3.4:", "https://a.b/%41", "HTTP://A.B:80"} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		r, err := ParseAPIRoot(s)
		if err != nil {
			return
		}

		again, err := ParseAPIRoot(r.String())
		if err != nil || again != r {
			t.Fatalf("%q -> %+v -> %q -> %+v, %v", s, r, r.String(), again, err)
		}

		if _, err := url.Parse(r.String()); err != nil {
			t.Fatalf("%q does not parse as a URL: %v", r.String(), err)
		}
	})
}

func FuzzParseOriginatingNetworkID(f *testing.F) {
	for _, s := range []string{"123-45", "123-456-000007ed9d5", "123-45; src: SEPP-sepp.example.com", "123-45;SRC:\tscp-a.b.c"} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		o, err := parseOriginatingNetworkID(s)
		if err != nil {
			return
		}

		again, err := parseOriginatingNetworkID(o.String())
		if err != nil || again != o {
			t.Fatalf("%q -> %+v -> %q -> %+v, %v", s, o, o.String(), again, err)
		}
	})
}

func FuzzParseVia(f *testing.F) {
	for _, s := range []string{"2.0 SEPP-sepp.example.com", "HTTP/2.0 a, 1.1 b (c)", "1.1 b (\\))", "1.1 [::1]:80 ((x))"} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		entries, err := ParseVia([]string{s})
		if err != nil {
			return
		}

		for _, e := range entries {
			again, err := ParseVia([]string{e.String()})
			if err != nil || len(again) != 1 || again[0] != e {
				t.Fatalf("%q: %+v -> %q -> %+v, %v", s, e, e.String(), again, err)
			}
		}
	})
}

func FuzzParseInterPLMNPurpose(f *testing.F) {
	for _, s := range []string{"ROAMING: usecaseA", "INTER_PLMN_MOBILITY", " X:y "} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		p, err := ParseInterPLMNPurpose([]string{s})
		if err != nil {
			return
		}

		v := string(p.Purpose)
		if p.AdditionalInfo != "" {
			v += ": " + p.AdditionalInfo
		}

		again, err := ParseInterPLMNPurpose([]string{v})
		if err != nil || again != p {
			t.Fatalf("%q -> %+v -> %q -> %+v, %v", s, p, v, again, err)
		}
	})
}

func FuzzParseN32HandshakeID(f *testing.F) {
	f.Add("955cac631f953ed8")

	f.Fuzz(func(t *testing.T, s string) {
		id, err := ParseN32HandshakeID([]string{s})
		if err == nil && (len(id) != 16 || !isHex(id)) {
			t.Fatalf("%q -> %q", s, id)
		}
	})
}
