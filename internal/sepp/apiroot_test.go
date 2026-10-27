// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/ellanetworks/core/internal/roaming/wire"
)

func mustURL(t testing.TB, s string) *url.URL {
	t.Helper()

	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}

	return u
}

func TestParseAPIRoot(t *testing.T) {
	for in, want := range map[string]APIRoot{
		"http://ausf.5gc.mnc001.mcc001.3gppnetwork.org": {Scheme: "http", Authority: "ausf.5gc.mnc001.mcc001.3gppnetwork.org"},
		" https://example.com/a/b/c ":                   {Scheme: "https", Authority: "example.com", Prefix: "/a/b/c"},
		"HTTPS://Example.COM:8443":                      {Scheme: "https", Authority: "example.com:8443"},
		"http://10.0.0.1:80/":                           {Scheme: "http", Authority: "10.0.0.1:80", Prefix: "/"},
		"http://[2001:DB8::1]:7777/pre%20fix":           {Scheme: "http", Authority: "[2001:db8::1]:7777", Prefix: "/pre%20fix"},
		"http://[::1]":                                  {Scheme: "http", Authority: "[::1]"},
		"http://example.com:":                           {Scheme: "http", Authority: "example.com"},
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
		"http://example.com//a", "http://example.com/%zz", "http://-bad.com", "http://a:b:c", "http://0/\x00",
		"http://a.com/%00", "http://a.com/x%1f", "http://a.com/%7F", "http://999.1.1.1", "http://1.2.3", "http://0",
		"http://a.com/../x", "http://a.com/x/%2e%2E/y", "http://[fe80::1%25eth0]", "http://a.com:-1",
	} {
		if _, err := ParseAPIRoot(in); err == nil {
			t.Errorf("ParseAPIRoot(%q) accepted", in)
		}
	}

	r, _ := ParseAPIRoot("https://example.com:8443/p")
	if r.String() != "https://example.com:8443/p" || r.Hostname() != "example.com" || r.Port() != "8443" {
		t.Fatalf("APIRoot accessors = %q %q %q", r.String(), r.Hostname(), r.Port())
	}
}

func TestRewriteToN32f(t *testing.T) {
	base := mustURL(t, "https://sepp.5gc.mnc001.mcc001.3gppnetwork.org:443")

	for _, tc := range []struct {
		target, wantURL, wantRoot string
	}{
		{
			"http://ausf.5gc.mnc001.mcc001.3gppnetwork.org/nausf-auth/v1/ue-authentications",
			"https://sepp.5gc.mnc001.mcc001.3gppnetwork.org:443/nausf-auth/v1/ue-authentications",
			"http://ausf.5gc.mnc001.mcc001.3gppnetwork.org",
		},
		{
			"http://udm.5gc.mnc001.mcc001.3gppnetwork.org:7777/pfx/nudm-sdm/v2/imsi-001010000000001/sm-data?single-nssai=%7B%22sst%22%3A1%7D&dnn=internet",
			"https://sepp.5gc.mnc001.mcc001.3gppnetwork.org:443/pfx/nudm-sdm/v2/imsi-001010000000001/sm-data?single-nssai=%7B%22sst%22%3A1%7D&dnn=internet",
			"http://udm.5gc.mnc001.mcc001.3gppnetwork.org:7777",
		},
		{
			"HTTP://AUSF.5gc.mnc001.mcc001.3gppnetwork.org/a%2Fb/c",
			"https://sepp.5gc.mnc001.mcc001.3gppnetwork.org:443/a%2Fb/c",
			"http://ausf.5gc.mnc001.mcc001.3gppnetwork.org",
		},
	} {
		out, root, err := RewriteToN32f(mustURL(t, tc.target), base)
		if err != nil {
			t.Fatalf("RewriteToN32f(%q): %v", tc.target, err)
		}

		if out.String() != tc.wantURL || root.String() != tc.wantRoot {
			t.Errorf("RewriteToN32f(%q) = %q, %q", tc.target, out, root)
		}

		back, err := TargetURL(root, out, "")
		if err != nil {
			t.Fatal(err)
		}

		want := mustURL(t, tc.target)
		want.Scheme, want.Host = "http", root.Authority

		if back.String() != want.String() {
			t.Errorf("TargetURL = %q, want %q", back, want)
		}
	}

	prefixed := mustURL(t, "https://sepp.example.com/n32f/")

	out, _, err := RewriteToN32f(mustURL(t, "http://nrf.5gc.mnc001.mcc001.3gppnetwork.org/nnrf-disc/v1/nf-instances"), prefixed)
	if err != nil || out.String() != "https://sepp.example.com/n32f/nnrf-disc/v1/nf-instances" {
		t.Fatalf("prefixed base = %q, %v", out, err)
	}

	for _, target := range []string{"/relative", "http://user:pw@host.example.com/x", "mailto:x@example.com", "http://exa%20mple.com/", "http://a.example.com/x/../y", "http://a.example.com/%2e"} {
		u, err := url.Parse(target)
		if err != nil {
			continue
		}

		if _, _, err := RewriteToN32f(u, base); !errors.Is(err, ErrTargetURI) {
			t.Errorf("RewriteToN32f(%q) = %v", target, err)
		}
	}

	if _, _, err := RewriteToN32f(mustURL(t, "http://a.example.com/x"), mustURL(t, "/x")); err == nil {
		t.Fatal("relative N32-f base accepted")
	}
}

func TestTargetURL(t *testing.T) {
	root, err := ParseAPIRoot("https://amf.5gc.mnc070.mcc999.3gppnetwork.org/pre")
	if err != nil {
		t.Fatal(err)
	}

	got, err := TargetURL(root, mustURL(t, "/n32f/roaming-callbacks/v1/1/amf/imsi-001010000000001/uecm-dereg?x=1"), "/n32f/")
	if err != nil || got.String() != "https://amf.5gc.mnc070.mcc999.3gppnetwork.org/pre/roaming-callbacks/v1/1/amf/imsi-001010000000001/uecm-dereg?x=1" {
		t.Fatalf("TargetURL = %q, %v", got, err)
	}

	for _, p := range []string{"/n32f/a/../b", "/n32f/a/%2E%2e/b", "/n32f/./b"} {
		if _, err := TargetURL(root, mustURL(t, p), "/n32f"); !errors.Is(err, ErrTargetURI) {
			t.Errorf("TargetURL(%q) = %v", p, err)
		}
	}

	if _, err := TargetURL(root, mustURL(t, "/n32fx/a"), "/n32f"); !errors.Is(err, ErrTargetURI) {
		t.Fatalf("outside local prefix: %v", err)
	}

	if _, err := TargetURL(root, nil, ""); err == nil {
		t.Fatal("nil URL accepted")
	}
}

func TestCallback(t *testing.T) {
	op := wire.PlmnID{Mcc: "999", Mnc: "70"}
	c := Callback{NF: CallbackAMF, AMFPointer: 3, Supi: "imsi-001010000000001", Kind: CallbackUECMDereg}

	uri, err := c.URI(op)
	if err != nil || uri != "https://amf.5gc.mnc070.mcc999.3gppnetwork.org/roaming-callbacks/v1/3/amf/imsi-001010000000001/uecm-dereg" {
		t.Fatalf("URI = %q, %v", uri, err)
	}

	u := mustURL(t, uri)

	back, err := ParseCallback(http.MethodPost, u.Host, u.EscapedPath(), op)
	if err != nil || back != c {
		t.Fatalf("ParseCallback = %+v, %v", back, err)
	}

	if back, err := ParseCallback(http.MethodPost, "AMF.5gc.mnc070.mcc999.3gppnetwork.org.:443", u.EscapedPath(), op); err != nil || back != c {
		t.Fatalf("ParseCallback with port = %+v, %v", back, err)
	}

	smf := Callback{NF: CallbackSMF, AMFPointer: MaxAMFPointer, Supi: "imsi-001010000000001", Kind: CallbackSDMChange}

	uri, err = smf.URI(op)
	if err != nil || uri != "https://smf.5gc.mnc070.mcc999.3gppnetwork.org/roaming-callbacks/v1/63/smf/imsi-001010000000001/sdm-change" {
		t.Fatalf("SMF URI = %q, %v", uri, err)
	}

	for _, bad := range []Callback{
		{NF: CallbackSMF, AMFPointer: 1, Supi: "imsi-001010000000001", Kind: CallbackUECMDereg},
		{NF: "pcf", AMFPointer: 1, Supi: "imsi-001010000000001", Kind: CallbackSDMChange},
		{NF: CallbackAMF, AMFPointer: 0, Supi: "imsi-001010000000001", Kind: CallbackSDMChange},
		{NF: CallbackAMF, AMFPointer: 64, Supi: "imsi-001010000000001", Kind: CallbackSDMChange},
		{NF: CallbackAMF, AMFPointer: -1, Supi: "imsi-001010000000001", Kind: CallbackSDMChange},
		{NF: CallbackAMF, AMFPointer: 1, Supi: "nai-x@y", Kind: CallbackSDMChange},
		{NF: CallbackAMF, AMFPointer: 1, Supi: "imsi-001010000000001/../x", Kind: CallbackSDMChange},
	} {
		if _, err := bad.URI(op); !errors.Is(err, ErrCallbackURI) {
			t.Errorf("URI(%+v) = %v", bad, err)
		}
	}

	if _, err := c.URI(wire.PlmnID{}); err == nil {
		t.Fatal("invalid operator PLMN accepted")
	}

	host := "amf.5gc.mnc070.mcc999.3gppnetwork.org"
	for _, tc := range []struct{ method, host, path string }{
		{http.MethodPost, host, "/roaming-callbacks/v1/3/amf/imsi-001010000000001/uecm-dereg/x"},
		{http.MethodPost, host, "/roaming-callbacks/v1/03/amf/imsi-001010000000001/uecm-dereg"},
		{http.MethodPost, host, "/roaming-callbacks/v1/+3/amf/imsi-001010000000001/uecm-dereg"},
		{http.MethodPost, host, "/roaming-callbacks/v1/0/amf/imsi-001010000000001/uecm-dereg"},
		{http.MethodPost, host, "/roaming-callbacks/v1/64/amf/imsi-001010000000001/uecm-dereg"},
		{http.MethodPost, host, "/roaming-callbacks/v2/3/amf/imsi-001010000000001/uecm-dereg"},
		{http.MethodPost, host, "/roaming-callbacks/v1/3/smf/imsi-001010000000001/uecm-dereg"},
		{http.MethodPost, host, "/roaming-callbacks/v1/3/smf/imsi-001010000000001/sdm-change"},
		{http.MethodPost, "smf.5gc.mnc070.mcc999.3gppnetwork.org", "/roaming-callbacks/v1/3/amf/imsi-001010000000001/sdm-change"},
		{http.MethodPost, "amf.5gc.mnc001.mcc001.3gppnetwork.org", "/roaming-callbacks/v1/3/amf/imsi-001010000000001/sdm-change"},
		{http.MethodPost, host, "/roaming-callbacks/v1/3/amf/suci-0-001-01-0-0-0-1/sdm-change"},
		{http.MethodPost, host, "/roaming-callbacks/v1//amf/imsi-001010000000001/sdm-change"},
		{http.MethodPost, host, "/roaming-callbacks/v1/3/amf/imsi-001010000000001%2Fx/sdm-change"},
		{http.MethodPost, host, "/roaming-callbacks/v1/3/amf/../amf/imsi-001010000000001/sdm-change"},
		{http.MethodPost, host, "/roaming-callbacks/v1/3/amf/./imsi-001010000000001/sdm-change"},
		{http.MethodPut, host, "/roaming-callbacks/v1/3/amf/imsi-001010000000001/sdm-change"},
	} {
		if _, err := ParseCallback(tc.method, tc.host, tc.path, op); !errors.Is(err, ErrCallbackURI) {
			t.Errorf("ParseCallback(%s %q, %q) = %v", tc.method, tc.host, tc.path, err)
		}
	}
}

func TestCallbackNotFound(t *testing.T) {
	dereg := CallbackUECMDereg.NotFound("unknown visitor")
	if dereg.Status != http.StatusNotFound || dereg.Cause != wire.CauseContextNotFound || dereg.Detail != "unknown visitor" {
		t.Fatalf("UECM dereg = %+v", dereg)
	}

	change := CallbackSDMChange.NotFound("")
	if change.Status != http.StatusNotFound || change.Cause != wire.CauseSubscriptionNotFound {
		t.Fatalf("SDM change = %+v", change)
	}

	if CallbackUECMDereg.Method() != http.MethodPost || CallbackSDMChange.Method() != http.MethodPost {
		t.Fatal("Method")
	}
}

func FuzzCallbackURI(f *testing.F) {
	f.Add("amf", 3, "imsi-001010000000001", "uecm-dereg", "999", "70")
	f.Add("smf", 63, "imsi-00101", "sdm-change", "001", "001")

	f.Fuzz(func(t *testing.T, nf string, ptr int, supi, kind, mcc, mnc string) {
		c := Callback{NF: CallbackNF(nf), AMFPointer: ptr, Supi: supi, Kind: CallbackKind(kind)}
		op := wire.PlmnID{Mcc: mcc, Mnc: mnc}

		uri, err := c.URI(op)
		if err != nil {
			return
		}

		u, err := url.Parse(uri)
		if err != nil {
			t.Fatalf("%q: %v", uri, err)
		}

		back, err := ParseCallback(c.Kind.Method(), u.Host, u.EscapedPath(), op)
		if err != nil || back != c {
			t.Fatalf("%q -> %+v, %v", uri, back, err)
		}
	})
}

func FuzzParseCallback(f *testing.F) {
	f.Add("amf.5gc.mnc070.mcc999.3gppnetwork.org", "/roaming-callbacks/v1/3/amf/imsi-001010000000001/uecm-dereg")

	op := wire.PlmnID{Mcc: "999", Mnc: "70"}

	f.Fuzz(func(t *testing.T, host, path string) {
		c, err := ParseCallback(http.MethodPost, host, path, op)
		if err != nil {
			return
		}

		if c.Path() != path || c.AMFPointer < MinAMFPointer || c.AMFPointer > MaxAMFPointer {
			t.Fatalf("%q -> %+v (%q)", path, c, strconv.Itoa(c.AMFPointer))
		}
	})
}

func FuzzRewriteToN32f(f *testing.F) {
	f.Add("http://udm.5gc.mnc001.mcc001.3gppnetwork.org:7777/pfx/nudm-sdm/v2/x?a=1", "https://sepp.example.com/n32f/")
	f.Add("HTTP://AUSF.5gc.mnc001.mcc001.3gppnetwork.org/a%2Fb/c", "https://sepp.example.com")

	f.Fuzz(func(t *testing.T, target, base string) {
		tu, err := url.Parse(target)
		if err != nil {
			return
		}

		bu, err := url.Parse(base)
		if err != nil || bu.RawQuery != "" || bu.Fragment != "" || hasDotSegment(bu.EscapedPath()) {
			return
		}

		out, root, err := RewriteToN32f(tu, bu)
		if err != nil {
			return
		}

		back, err := TargetURL(root, out, bu.EscapedPath())
		if err != nil {
			t.Fatalf("%q via %q -> %q: %v", target, base, out, err)
		}

		if back.EscapedPath() != joinPath("", tu.EscapedPath()) || back.RawQuery != tu.RawQuery || back.Host != root.Authority {
			t.Fatalf("%q via %q -> %q -> %q", target, base, out, back)
		}
	})
}
