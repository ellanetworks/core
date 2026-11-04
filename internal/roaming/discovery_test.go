// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/roaming/wire"
)

type fakeN32 struct {
	handler http.Handler
	calls   atomic.Int32
	last    atomic.Pointer[http.Request]
}

func (f *fakeN32) Client(partnerID string) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if partnerID != "p" {
			return nil, errors.New("unknown partner")
		}

		f.calls.Add(1)
		f.last.Store(r)

		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, r)

		return rec.Result(), nil
	})}
}

func (f *fakeN32) HPLMNNRF(_ string, home wire.PlmnID) (string, error) {
	return "http://" + home.NFFQDN(wire.FQDNLabelNRF), nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

const searchBody = `{
  "validityPeriod": 30,
  "nfInstances": [
    {"nfInstanceId": "00000000-0000-0000-0000-000000000001", "nfType": "AUSF", "nfStatus": "REGISTERED", "priority": 5,
     "plmnList": [{"mcc": "001", "mnc": "01"}], "fqdn": "ausf1.5gc.mnc001.mcc001.3gppnetwork.org",
     "nfServices": [{"serviceInstanceId": "a", "serviceName": "nausf-auth", "versions": [{"apiVersionInUri": "v1", "apiFullVersion": "1.0.0"}], "scheme": "http", "nfServiceStatus": "REGISTERED"}]},
    {"nfInstanceId": "00000000-0000-0000-0000-000000000002", "nfType": "AUSF", "nfStatus": "REGISTERED",
     "fqdn": "ausf2.5gc.mnc001.mcc001.3gppnetwork.org",
     "nfServices": [{"serviceInstanceId": "b", "serviceName": "nausf-auth", "versions": [{"apiVersionInUri": "v1", "apiFullVersion": "1.0.0"}], "scheme": "https", "nfServiceStatus": "REGISTERED", "priority": 1,
       "ipEndPoints": [{"port": 8443}]}]},
    {"nfInstanceId": "00000000-0000-0000-0000-000000000003", "nfType": "AUSF", "nfStatus": "SUSPENDED",
     "fqdn": "ausf3.5gc.mnc001.mcc001.3gppnetwork.org",
     "nfServices": [{"serviceInstanceId": "c", "serviceName": "nausf-auth", "versions": [{"apiVersionInUri": "v1", "apiFullVersion": "1.0.0"}], "scheme": "http", "nfServiceStatus": "REGISTERED", "priority": 0}]},
    {"nfInstanceId": "00000000-0000-0000-0000-000000000004", "nfType": "AUSF", "nfStatus": "REGISTERED", "ipv4Addresses": ["10.0.0.4"],
     "nfServices": [{"serviceInstanceId": "d", "serviceName": "nausf-auth", "versions": [{"apiVersionInUri": "v1", "apiFullVersion": "1.0.0"}], "scheme": "http", "nfServiceStatus": "REGISTERED", "priority": 0}]}
  ]
}`

var (
	home    = wire.PlmnID{Mcc: "001", Mnc: "01"}
	serving = wire.PlmnID{Mcc: "999", Mnc: "70"}
)

func ausfRequest() DiscoveryRequest {
	return DiscoveryRequest{PartnerID: "p", HomePLMN: home, ServingPLMN: serving, Target: wire.NFTypeAUSF, Requester: wire.NFTypeAMF, Service: wire.ServiceNausfAuth, RoutingIndicator: "0000"}
}

func TestDiscoveryQueryAndRanking(t *testing.T) {
	f := &fakeN32{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(searchBody))
	})}

	d := NewDiscovery(f)

	c, err := d.Discover(context.Background(), ausfRequest())
	if err != nil {
		t.Fatal(err)
	}

	if c.NfInstanceID != "00000000-0000-0000-0000-000000000002" || c.APIRoot != "https://ausf2.5gc.mnc001.mcc001.3gppnetwork.org:8443" {
		t.Fatalf("candidate = %+v", c)
	}

	r := f.last.Load()
	if r.URL.Host != "nrf.5gc.mnc001.mcc001.3gppnetwork.org" || r.URL.Path != "/nnrf-disc/v1/nf-instances" || r.Header.Get("User-Agent") != "AMF" {
		t.Fatalf("request %s %v", r.URL, r.Header)
	}

	want := url.Values{
		"target-nf-type":      {"AUSF"},
		"requester-nf-type":   {"AMF"},
		"service-names":       {"nausf-auth"},
		"target-plmn-list":    {`[{"mcc":"001","mnc":"01"}]`},
		"requester-plmn-list": {`[{"mcc":"999","mnc":"70"}]`},
		"routing-indicator":   {"0000"},
	}

	got := r.URL.Query()
	for k, v := range want {
		if got.Get(k) != v[0] {
			t.Errorf("%s = %q, want %q", k, got.Get(k), v[0])
		}
	}

	if _, err := d.Discover(context.Background(), ausfRequest()); err != nil || f.calls.Load() != 1 {
		t.Fatalf("cached discovery: %v, %d calls", err, f.calls.Load())
	}

	d.Drop("p", "00000000-0000-0000-0000-000000000002")

	c, err = d.Discover(context.Background(), ausfRequest())
	if err != nil || c.NfInstanceID != "00000000-0000-0000-0000-000000000001" || c.APIRoot != "http://ausf1.5gc.mnc001.mcc001.3gppnetwork.org" || f.calls.Load() != 1 {
		t.Fatalf("after a drop: %+v, %v, %d calls", c, err, f.calls.Load())
	}

	d.Drop("p", "00000000-0000-0000-0000-000000000001")

	if _, err := d.Discover(context.Background(), ausfRequest()); err != nil || f.calls.Load() != 2 {
		t.Fatalf("every candidate dropped: %v, %d calls", err, f.calls.Load())
	}

	now := time.Now()
	d.now = func() time.Time { return now.Add(31 * time.Second) }

	if _, err := d.Discover(context.Background(), ausfRequest()); err != nil || f.calls.Load() != 3 {
		t.Fatalf("expired entry: %v, %d calls", err, f.calls.Load())
	}

	other := ausfRequest()
	other.RoutingIndicator = "0001"

	if _, err := d.Discover(context.Background(), other); err != nil || f.calls.Load() != 3 {
		t.Fatalf("instances without ausfInfo serve every routing indicator: %v, %d calls", err, f.calls.Load())
	}

	d.Forget("p")

	if _, err := d.Discover(context.Background(), ausfRequest()); err != nil || f.calls.Load() != 4 {
		t.Fatalf("after Forget: %v, %d calls", err, f.calls.Load())
	}
}

func TestDiscoveryFailures(t *testing.T) {
	for name, tc := range map[string]struct {
		status int
		body   string
		want   error
	}{
		"no instance":     {200, `{"validityPeriod": 30, "nfInstances": []}`, ErrNoCandidate},
		"only suspended":  {200, `{"validityPeriod": 30, "nfInstances": [{"nfInstanceId": "00000000-0000-0000-0000-000000000003", "nfType": "AUSF", "nfStatus": "SUSPENDED", "fqdn": "a.5gc.mnc001.mcc001.3gppnetwork.org"}]}`, ErrNoCandidate},
		"wrong NF type":   {200, `{"validityPeriod": 30, "nfInstances": [{"nfInstanceId": "00000000-0000-0000-0000-000000000003", "nfType": "UDM", "nfStatus": "REGISTERED", "fqdn": "a.5gc.mnc001.mcc001.3gppnetwork.org", "nfServices": [{"serviceInstanceId": "a", "serviceName": "nausf-auth", "versions": [{"apiVersionInUri": "v1", "apiFullVersion": "1.0.0"}], "scheme": "http", "nfServiceStatus": "REGISTERED"}]}]}`, ErrNoCandidate},
		"no validity":     {200, `{"nfInstances": []}`, nil},
		"NRF error":       {404, `{"status": 404, "cause": "TARGET_NF_NOT_REACHABLE"}`, nil},
		"malformed JSON":  {200, `{`, wire.ErrMalformed},
		"negative expiry": {200, `{"validityPeriod": -1, "nfInstances": []}`, nil},
	} {
		f := &fakeN32{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		})}

		_, err := NewDiscovery(f).Discover(context.Background(), ausfRequest())

		switch {
		case err == nil:
			t.Errorf("%s: no error", name)
		case tc.want != nil && !errors.Is(err, tc.want):
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}

	f := &fakeN32{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(searchBody))
	})}

	bad := ausfRequest()
	bad.PartnerID = "nope"

	if _, err := NewDiscovery(f).Discover(context.Background(), bad); err == nil {
		t.Error("unknown partner: no error")
	}
}

func TestDiscoveryWithoutValidityIsNotCached(t *testing.T) {
	f := &fakeN32{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"validityPeriod": 0, "nfInstances": [{"nfInstanceId": "00000000-0000-0000-0000-000000000001", "nfType": "AUSF", "nfStatus": "REGISTERED", "fqdn": "ausf.5gc.mnc001.mcc001.3gppnetwork.org",
			"nfServices": [{"serviceInstanceId": "a", "serviceName": "nausf-auth", "versions": [{"apiVersionInUri": "v1", "apiFullVersion": "1.0.0"}], "scheme": "http", "nfServiceStatus": "REGISTERED"}]}]}`))
	})}

	d := NewDiscovery(f)

	for range 2 {
		if _, err := d.Discover(context.Background(), ausfRequest()); err != nil {
			t.Fatal(err)
		}
	}

	if f.calls.Load() != 2 {
		t.Fatalf("%d calls, want 2", f.calls.Load())
	}
}
