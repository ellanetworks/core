// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"errors"
	"net/url"
	"testing"

	"github.com/ellanetworks/core/internal/roaming/wire"
)

func TestParseAPIRoot(t *testing.T) {
	for in, want := range map[string]APIRoot{
		"http://ausf.5gc.mnc001.mcc001.3gppnetwork.org": {Scheme: "http", Authority: "ausf.5gc.mnc001.mcc001.3gppnetwork.org"},
		" https://example.com/a/b/c ":                   {Scheme: "https", Authority: "example.com", Prefix: "/a/b/c"},
		"HTTPS://example.com:8443":                      {Scheme: "https", Authority: "example.com:8443"},
		"http://10.0.0.1:80/":                           {Scheme: "http", Authority: "10.0.0.1:80", Prefix: "/"},
		"http://[2001:db8::1]:7777/pre%20fix":           {Scheme: "http", Authority: "[2001:db8::1]:7777", Prefix: "/pre%20fix"},
		"http://example.com:":                           {Scheme: "http", Authority: "example.com:"},
		"\thttp://example.com.\t":                       {Scheme: "http", Authority: "example.com."},
	} {
		got, err := ParseAPIRoot(in)
		if err != nil || got != want {
			t.Errorf("ParseAPIRoot(%q) = %+v, %v", in, got, err)
		}
	}

	for _, in := range []string{
		"", "example.com", "ftp://example.com", "http://", "http:///x", "http://user@example.com",
		"http://example.com?x=1", "http://example.com/a?b", "http://example.com/a#f", "http://example.com:99999",
		"http://example.com:x", "http://[::1", "http://[1.2.3.4]", "http://[::1]x", "http://exa mple.com",
		"http://example.com//a", "http://example.com/%zz", "http://-bad.com", "http://a:b:c",
	} {
		if _, err := ParseAPIRoot(in); !errors.Is(err, ErrHeader) {
			t.Errorf("ParseAPIRoot(%q) = %v", in, err)
		}
	}

	r, _ := ParseAPIRoot("https://example.com:8443/p")
	if r.String() != "https://example.com:8443/p" || r.Hostname() != "example.com" || r.Port() != "8443" {
		t.Fatalf("APIRoot accessors = %q %q %q", r.String(), r.Hostname(), r.Port())
	}
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
	} {
		got, err := ParseOriginatingNetworkID(in)
		if err != nil || got != want {
			t.Errorf("ParseOriginatingNetworkID(%q) = %+v, %v", in, got, err)
		}
	}

	for _, in := range []string{
		"", "123", "12-45", "123-4", "123-4567", "123-45-0000", "123-45-000007ed9dz", "123-45-000007ed9d5-1",
		"123-45; src:SEPP-sepp.com", "123-45; source: SEPP-x.com", "123-45; src: NRF-nrf.com", "123-45; src: SEPP-a",
		"123-45; src: SEPP", "123-45; src: SEPP-a_b.com", "123-45;", "abc-45",
	} {
		if _, err := ParseOriginatingNetworkID(in); !errors.Is(err, ErrHeader) {
			t.Errorf("ParseOriginatingNetworkID(%q) = %v", in, err)
		}
	}

	o := OriginatingNetworkID{PlmnID: wire.PlmnID{Mcc: "999", Mnc: "70"}}
	if o.String() != "999-70" {
		t.Fatalf("String = %q", o.String())
	}

	o.SrcType, o.SrcFQDN = "SEPP", "sepp.5gc.mnc070.mcc999.3gppnetwork.org"
	if o.String() != "999-70; src: SEPP-sepp.5gc.mnc070.mcc999.3gppnetwork.org" {
		t.Fatalf("String = %q", o.String())
	}
}

func TestN32HandshakeID(t *testing.T) {
	if id, err := ParseN32HandshakeID(" 955cac631f953ed8 "); err != nil || id != "955cac631f953ed8" {
		t.Fatalf("ParseN32HandshakeID = %q, %v", id, err)
	}

	for _, in := range []string{"", "955cac631f953ed", "955cac631f953ed80", "955cac631f953edg"} {
		if _, err := ParseN32HandshakeID(in); err == nil {
			t.Errorf("ParseN32HandshakeID(%q) accepted", in)
		}
	}
}

func TestVia(t *testing.T) {
	if SEPPVia("sepp1.operator.com") != "2.0 SEPP-sepp1.operator.com" || SEPPServer("sepp1.operator.com") != "SEPP-sepp1.operator.com" {
		t.Fatal("SEPPVia/SEPPServer")
	}

	got, err := ParseVia("HTTP/2.0 SCP-scp1.operator.com, 2.0 SEPP-sepp1.operator.com (relay, x\\)y), 1.1 proxy:8080")
	if err != nil || len(got) != 3 {
		t.Fatalf("ParseVia = %+v, %v", got, err)
	}

	if got[0] != (ViaEntry{Protocol: "HTTP/2.0", ReceivedBy: "SCP-scp1.operator.com"}) ||
		got[1] != (ViaEntry{Protocol: "2.0", ReceivedBy: "SEPP-sepp1.operator.com", Comment: "(relay, x\\)y)"}) ||
		got[2] != (ViaEntry{Protocol: "1.1", ReceivedBy: "proxy:8080"}) {
		t.Fatalf("ParseVia = %+v", got)
	}

	if typ, id, ok := got[1].NetworkEntity(); !ok || typ != "SEPP" || id != "sepp1.operator.com" {
		t.Fatalf("NetworkEntity = %q %q %v", typ, id, ok)
	}

	if _, _, ok := got[2].NetworkEntity(); ok {
		t.Fatal("proxy:8080 is not a network entity")
	}

	if got[1].String() != "2.0 SEPP-sepp1.operator.com (relay, x\\)y)" {
		t.Fatalf("String = %q", got[1].String())
	}

	for _, in := range []string{"", " , ", "2.0", "2.0 a (unterminated", "2.0 a b", "2.0 a@b", "/ a", "2.0 SEPP-x (c) extra"} {
		if _, err := ParseVia(in); err == nil {
			t.Errorf("ParseVia(%q) accepted", in)
		}
	}

	if typ, id, ok := ParseServer("SEPP-sepp1.operator.com"); !ok || typ != "SEPP" || id != "sepp1.operator.com" {
		t.Fatalf("ParseServer = %q %q %v", typ, id, ok)
	}

	if typ, id, ok := ParseServer("SMF-54804518-4191-46b3-955c-ac631f953ed8 (comment)"); !ok || typ != "SMF" || id != "54804518-4191-46b3-955c-ac631f953ed8" {
		t.Fatalf("ParseServer = %q %q %v", typ, id, ok)
	}

	for _, in := range []string{"", "nginx", "-x", "SEPP-", "Open5GS v2.8.0"} {
		if _, _, ok := ParseServer(in); ok {
			t.Errorf("ParseServer(%q) accepted", in)
		}
	}
}

func FuzzParseAPIRoot(f *testing.F) {
	for _, s := range []string{"http://a.example.com", "https://[::1]:443/x/y", "http://1.2.3.4:", "https://a.b/%41"} {
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
	for _, s := range []string{"123-45", "123-456-000007ed9d5", "123-45; src: SEPP-sepp.example.com"} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		o, err := ParseOriginatingNetworkID(s)
		if err != nil {
			return
		}

		again, err := ParseOriginatingNetworkID(o.String())
		if err != nil || again != o {
			t.Fatalf("%q -> %+v -> %q -> %+v, %v", s, o, o.String(), again, err)
		}
	})
}

func FuzzParseVia(f *testing.F) {
	for _, s := range []string{"2.0 SEPP-sepp.example.com", "HTTP/2.0 a, 1.1 b (c)", "1.1 b (\\))"} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		entries, err := ParseVia(s)
		if err != nil {
			return
		}

		for _, e := range entries {
			again, err := ParseVia(e.String())
			if err != nil || len(again) != 1 || again[0] != e {
				t.Fatalf("%q: %+v -> %q -> %+v, %v", s, e, e.String(), again, err)
			}
		}
	})
}

func FuzzParseN32HandshakeID(f *testing.F) {
	f.Add("955cac631f953ed8")

	f.Fuzz(func(t *testing.T, s string) {
		id, err := ParseN32HandshakeID(s)
		if err == nil && len(id) != 16 {
			t.Fatalf("%q -> %q", s, id)
		}
	})
}
