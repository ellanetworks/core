// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"errors"
	"net/url"
	"testing"

	"github.com/ellanetworks/core/internal/roaming/wire"
)

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()

	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}

	return u
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
			"http://AUSF.5gc.mnc001.mcc001.3gppnetwork.org",
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
		want.Scheme = "http"

		if back.String() != want.String() {
			t.Errorf("TargetURL = %q, want %q", back, want)
		}
	}

	prefixed := mustURL(t, "https://sepp.example.com/n32f/")

	out, _, err := RewriteToN32f(mustURL(t, "http://nrf.5gc.mnc001.mcc001.3gppnetwork.org/nnrf-disc/v1/nf-instances"), prefixed)
	if err != nil || out.String() != "https://sepp.example.com/n32f/nnrf-disc/v1/nf-instances" {
		t.Fatalf("prefixed base = %q, %v", out, err)
	}

	for _, target := range []string{"/relative", "http://user:pw@host.example.com/x", "mailto:x@example.com", "http://exa%20mple.com/"} {
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

	if _, err := TargetURL(root, mustURL(t, "/n32fx/a"), "/n32f"); !errors.Is(err, ErrTargetURI) {
		t.Fatalf("outside local prefix: %v", err)
	}

	if _, err := TargetURL(root, nil, ""); err == nil {
		t.Fatal("nil URL accepted")
	}
}

func TestCallback(t *testing.T) {
	op := wire.PlmnID{Mcc: "999", Mnc: "70"}
	c := Callback{NF: CallbackAMF, NodeID: 3, Supi: "imsi-001010000000001", Kind: CallbackUECMDereg}

	uri, err := c.URI(op)
	if err != nil || uri != "https://amf.5gc.mnc070.mcc999.3gppnetwork.org/roaming-callbacks/v1/3/amf/imsi-001010000000001/uecm-dereg" {
		t.Fatalf("URI = %q, %v", uri, err)
	}

	u := mustURL(t, uri)

	back, err := ParseCallback(u.Host, u.Path, op)
	if err != nil || back != c {
		t.Fatalf("ParseCallback = %+v, %v", back, err)
	}

	if back, err := ParseCallback("AMF.5gc.mnc070.mcc999.3gppnetwork.org.:443", u.Path, op); err != nil || back != c {
		t.Fatalf("ParseCallback with port = %+v, %v", back, err)
	}

	smf := Callback{NF: CallbackSMF, NodeID: 0, Supi: "imsi-001010000000001", Kind: CallbackSDMChange}

	uri, err = smf.URI(op)
	if err != nil || uri != "https://smf.5gc.mnc070.mcc999.3gppnetwork.org/roaming-callbacks/v1/0/smf/imsi-001010000000001/sdm-change" {
		t.Fatalf("SMF URI = %q, %v", uri, err)
	}

	for _, bad := range []Callback{
		{NF: CallbackSMF, Supi: "imsi-001010000000001", Kind: CallbackUECMDereg},
		{NF: "pcf", Supi: "imsi-001010000000001", Kind: CallbackSDMChange},
		{NF: CallbackAMF, NodeID: -1, Supi: "imsi-001010000000001", Kind: CallbackSDMChange},
		{NF: CallbackAMF, Supi: "nai-x@y", Kind: CallbackSDMChange},
		{NF: CallbackAMF, Supi: "imsi-001010000000001/../x", Kind: CallbackSDMChange},
	} {
		if _, err := bad.URI(op); !errors.Is(err, ErrCallbackURI) {
			t.Errorf("URI(%+v) = %v", bad, err)
		}
	}

	if _, err := c.URI(wire.PlmnID{}); err == nil {
		t.Fatal("invalid operator PLMN accepted")
	}

	host := "amf.5gc.mnc070.mcc999.3gppnetwork.org"
	for _, tc := range []struct{ host, path string }{
		{host, "/roaming-callbacks/v1/3/amf/imsi-001010000000001/uecm-dereg/x"},
		{host, "/roaming-callbacks/v1/03/amf/imsi-001010000000001/uecm-dereg"},
		{host, "/roaming-callbacks/v1/+3/amf/imsi-001010000000001/uecm-dereg"},
		{host, "/roaming-callbacks/v2/3/amf/imsi-001010000000001/uecm-dereg"},
		{host, "/roaming-callbacks/v1/3/smf/imsi-001010000000001/uecm-dereg"},
		{host, "/roaming-callbacks/v1/3/smf/imsi-001010000000001/sdm-change"},
		{"smf.5gc.mnc070.mcc999.3gppnetwork.org", "/roaming-callbacks/v1/3/amf/imsi-001010000000001/sdm-change"},
		{"amf.5gc.mnc001.mcc001.3gppnetwork.org", "/roaming-callbacks/v1/3/amf/imsi-001010000000001/sdm-change"},
		{host, "/roaming-callbacks/v1/3/amf/suci-0-001-01-0-0-0-1/sdm-change"},
		{host, "/roaming-callbacks/v1//amf/imsi-001010000000001/sdm-change"},
	} {
		if _, err := ParseCallback(tc.host, tc.path, op); !errors.Is(err, ErrCallbackURI) {
			t.Errorf("ParseCallback(%q, %q) = %v", tc.host, tc.path, err)
		}
	}
}
