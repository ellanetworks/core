// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"
)

var (
	homePLMN = PlmnID{Mcc: "001", Mnc: "01"}
	opPLMN   = PlmnID{Mcc: "999", Mnc: "70"}
	ausfSel  = Selector{Service: ServiceNausfAuth, Target: homePLMN, Requester: opPLMN}
)

const ausfProfile = `{"nfInstanceId":"3b8b0a4e-6e1b-41ee-9b6b-3d8b3f6b0e01",` +
	`"nfType":"AUSF","nfStatus":"REGISTERED","plmnList":[{"mcc":"001","mnc":"01"}],"fqdn":"ausf.5gc.mnc001.mcc001.3gppnetwork.org",` +
	`"ipv4Addresses":["127.0.0.11"],"allowedNfTypes":["AMF"],"priority":0,"capacity":100,"load":0,` +
	`"nfServices":[{"serviceInstanceId":"3b8c4a5e-6e1b-41ee-9b6b-3d8b3f6b0e01","serviceName":"nausf-auth",` +
	`"versions":[{"apiVersionInUri":"v1","apiFullVersion":"1.0.0"}],"scheme":"http","nfServiceStatus":"REGISTERED",` +
	`"fqdn":"ausf.5gc.mnc001.mcc001.3gppnetwork.org","ipEndPoints":[{"ipv4Address":"127.0.0.11","port":80}],` +
	`"allowedNfTypes":["AMF"],"priority":0,"capacity":100,"load":0}],"nfProfileChangesSupportInd":true,` +
	`"ausfInfo":{"routingIndicators":["0000"]}}`

const searchResult = `{"validityPeriod":30,"nfInstances":[` + ausfProfile + `]}`

func TestSearchResultDeprecatedNfServicesArray(t *testing.T) {
	r, err := Decode[SearchResult]([]byte(searchResult))
	if err != nil {
		t.Fatal(err)
	}

	if *r.ValidityPeriod != 30 || len(r.NfInstances) != 1 {
		t.Fatalf("result = %+v", r)
	}

	if err := r.Check(Strict); err != nil {
		t.Fatal(err)
	}

	cands := r.Candidates(ausfSel)
	if len(cands) != 1 || cands[0].APIRoot != "http://ausf.5gc.mnc001.mcc001.3gppnetwork.org:80" ||
		cands[0].NfInstanceID != "3b8b0a4e-6e1b-41ee-9b6b-3d8b3f6b0e01" || cands[0].Service.ServiceName != ServiceNausfAuth {
		t.Fatalf("Candidates = %+v", cands)
	}

	if c := r.Candidates(Selector{Service: ServiceNudmSDM, Target: homePLMN, Requester: opPLMN}); len(c) != 0 {
		t.Fatal("nudm-sdm found on an AUSF")
	}

	b, err := Encode(r)
	if err != nil {
		t.Fatal(err)
	}

	back, err := Decode[SearchResult](b)
	if err != nil || back.NfInstances[0].NfServices[0].IPEndPoints[0].Port == nil {
		t.Fatalf("round trip = %s, %v", b, err)
	}
}

func TestSearchResultDropsBadInstances(t *testing.T) {
	bad := []string{
		`{"nfType":"AUSF","nfStatus":"REGISTERED"}`,
		`{"nfInstanceId":"x","nfType":"AUSF","nfStatus":"REGISTERED","plmnList":[{"mcc":"001"}]}`,
		`{"nfInstanceId":"x","nfType":"AUSF","nfStatus":"REGISTERED","priority":"high"}`,
		`7`,
		`null`,
	}

	body := `{"validityPeriod":30,"nfInstances":[` + strings.Join(bad, ",") + `,` + ausfProfile + `]}`

	r, err := Decode[SearchResult]([]byte(body))
	if err != nil || len(r.NfInstances) != 1 || r.NfInstances[0].NfType != NFTypeAUSF {
		t.Fatalf("filtered = %+v, %v", r, err)
	}

	badFQDN := strings.ReplaceAll(ausfProfile, "ausf.5gc.mnc001.mcc001.3gppnetwork.org", "ausf_1.example")

	r, err = Decode[SearchResult]([]byte(`{"validityPeriod":1,"nfInstances":[` + badFQDN + `,` + ausfProfile + `]}`))
	if err != nil || len(r.NfInstances) != 2 {
		t.Fatalf("non-RFC FQDN decoded = %+v, %v", r, err)
	}

	if c := r.Candidates(ausfSel); len(c) != 1 || !strings.HasPrefix(c[0].APIRoot, "http://ausf.5gc.") {
		t.Fatalf("selection did not skip the bad FQDN: %+v", c)
	}
}

func TestSearchResultErrors(t *testing.T) {
	for _, body := range []string{`{"nfInstances":[]}`, `{"validityPeriod":30}`, `{"validityPeriod":30,"nfInstances":null}`, `{"validityPeriod":null,"nfInstances":[]}`} {
		if _, err := Decode[SearchResult]([]byte(body)); !errors.Is(err, ErrMissingIE) {
			t.Errorf("Decode(%s) = %v", body, err)
		}
	}

	if _, err := Decode[SearchResult]([]byte(`{"validityPeriod":-1,"nfInstances":[]}`)); !errors.Is(err, ErrIncorrectIE) {
		t.Fatalf("negative validity: %v", err)
	}

	if _, err := Decode[SearchResult]([]byte(`{"validityPeriod":1,"nfInstances":{}}`)); !errors.Is(err, ErrMalformed) {
		t.Fatalf("object nfInstances: %v", err)
	}

	empty, err := Decode[SearchResult]([]byte(`{"validityPeriod":0,"nfInstances":[]}`))
	if err != nil || len(empty.NfInstances) != 0 {
		t.Fatalf("empty result = %+v, %v", empty, err)
	}

	zero := 0
	if _, err := Encode(SearchResult{ValidityPeriod: &zero, NfInstances: []NFProfile{}, NrfSupportedFeatures: "x"}); err == nil {
		t.Fatal("bad nrfSupportedFeatures encoded")
	}

	requireIE(t, SearchResult{ValidityPeriod: &zero, NfInstances: []NFProfile{{}}}.Check(Lenient), "/nfInstances/0/nfInstanceId", true)
}

func TestNFProfileServiceMapAndAPIRoot(t *testing.T) {
	body := `{"nfInstanceId":"udm-1","nfType":"UDM","nfStatus":"REGISTERED","fqdn":"udm.5gc.mnc001.mcc001.3gppnetwork.org.",` +
		`"nfServiceList":{"b":{"serviceName":"nudm-uecm","versions":[{"apiVersionInUri":"v1","apiFullVersion":"1.3.0"}],` +
		`"scheme":"https","nfServiceStatus":"REGISTERED","apiPrefix":"/udm/"},` +
		`"a":{"serviceInstanceId":"a","serviceName":"nudm-sdm","versions":[{"apiVersionInUri":"v2","apiFullVersion":"2.3.0"}],` +
		`"scheme":"https","nfServiceStatus":"SUSPENDED","fqdn":"sdm.5gc.mnc001.mcc001.3gppnetwork.org"},` +
		`"c":{"serviceInstanceId":"c","serviceName":"nudm-sdm","versions":[{"apiVersionInUri":"v2","apiFullVersion":"2.3.0"}],` +
		`"scheme":"https","nfServiceStatus":"REGISTERED","fqdn":"sdm2.5gc.mnc001.mcc001.3gppnetwork.org","ipEndPoints":[{"ipv6Address":"2001:db8::1","port":8443}]},` +
		`"d":{"serviceName":"nudm-sdm","scheme":"https","nfServiceStatus":"REGISTERED"}}}`

	nf, err := Decode[NFProfile]([]byte(body))
	if err != nil {
		t.Fatal(err)
	}

	services := nf.Services()
	if len(services) != 3 || services[0].ServiceInstanceID != "a" || services[1].ServiceInstanceID != "b" {
		t.Fatalf("Services = %+v", services)
	}

	uecm, ok := nf.Candidate(Selector{Service: ServiceNudmUECM, Target: homePLMN, Requester: opPLMN})
	if !ok || uecm.APIRoot != "https://udm.5gc.mnc001.mcc001.3gppnetwork.org/udm" || uecm.Service.ServiceInstanceID != "b" {
		t.Fatalf("UECM = %+v", uecm)
	}

	sdm, ok := nf.Candidate(Selector{Service: ServiceNudmSDM, Target: homePLMN, Requester: opPLMN})
	if !ok || sdm.APIRoot != "https://sdm2.5gc.mnc001.mcc001.3gppnetwork.org:8443" {
		t.Fatalf("SDM = %+v", sdm)
	}

	if nf.Check(Strict) == nil {
		t.Fatal("strict check accepted a non-UUID nfInstanceId")
	}

	nf.NfServiceList = map[string]NFService{"x": {ServiceName: ServiceNudmSDM}}
	requireIE(t, nf.Check(Lenient), "/nfServiceList/x/serviceInstanceId", true)

	ipOnly := NFProfile{NfInstanceID: "x", NfType: NFTypeAUSF, NfStatus: NFStatusRegistered, Ipv4Addresses: []string{"10.0.0.1"}}
	if _, err := ipOnly.APIRoot(NFService{ServiceName: ServiceNausfAuth, Scheme: SchemeHTTP}); err == nil {
		t.Fatal("IP-only instance produced an apiRoot")
	}

	badPort := 0
	for name, svc := range map[string]NFService{
		"ftp scheme": {Scheme: "ftp", Fqdn: "a.example.com"},
		"bad FQDN":   {Scheme: SchemeHTTP, Fqdn: "1.2.3.4"},
		"port 0":     {Scheme: SchemeHTTP, Fqdn: "a.example.com", IPEndPoints: []IPEndPoint{{Port: &badPort}}},
		"bad prefix": {Scheme: SchemeHTTP, Fqdn: "a.example.com", APIPrefix: "relative"},
	} {
		if _, err := ipOnly.APIRoot(svc); err == nil {
			t.Errorf("%s: apiRoot built", name)
		}
	}
}

func TestAPIRootPrefersInterPlmnFqdn(t *testing.T) {
	nf := NFProfile{Fqdn: "nf.intra.example", InterPlmnFqdn: "nf.5gc.mnc001.mcc001.3gppnetwork.org"}

	for _, tc := range []struct {
		svc  NFService
		want string
	}{
		{NFService{Scheme: SchemeHTTPS, Fqdn: "svc.intra.example", InterPlmnFqdn: "svc.5gc.mnc001.mcc001.3gppnetwork.org"}, "https://svc.5gc.mnc001.mcc001.3gppnetwork.org"},
		{NFService{Scheme: SchemeHTTPS, Fqdn: "svc.intra.example"}, "https://nf.5gc.mnc001.mcc001.3gppnetwork.org"},
	} {
		if got, err := nf.APIRoot(tc.svc); err != nil || got != tc.want {
			t.Errorf("APIRoot(%+v) = %q, %v", tc.svc, got, err)
		}
	}

	nf.InterPlmnFqdn = ""
	if got, _ := nf.APIRoot(NFService{Scheme: SchemeHTTPS, Fqdn: "svc.intra.example"}); got != "https://svc.intra.example" {
		t.Fatalf("service fqdn = %q", got)
	}

	if got, _ := nf.APIRoot(NFService{Scheme: SchemeHTTPS}); got != "https://nf.intra.example" {
		t.Fatalf("profile fqdn = %q", got)
	}
}

func TestCandidateFilters(t *testing.T) {
	version := []NFServiceVersion{{APIVersionInURI: "v2", APIFullVersion: "2.0.0"}}
	svc := NFService{
		ServiceInstanceID: "s", ServiceName: ServiceNudmSDM, Versions: version, Scheme: SchemeHTTPS,
		NfServiceStatus: NFStatusRegistered, Fqdn: "udm.5gc.mnc001.mcc001.3gppnetwork.org",
	}
	base := NFProfile{NfInstanceID: "u", NfType: NFTypeUDM, NfStatus: NFStatusRegistered, NfServices: []NFService{svc}}
	sel := Selector{Service: ServiceNudmSDM, Target: homePLMN, Requester: opPLMN}

	if _, ok := base.Candidate(sel); !ok {
		t.Fatal("absent plmnList: TS 29.510 Table 6.2.6.2.2-1 assumes the NRF's PLMN")
	}

	for name, tc := range map[string]struct {
		mutate func(*NFProfile)
		ok     bool
	}{
		"suspended NF":           {func(n *NFProfile) { n.NfStatus = NFStatusSuspended }, false},
		"suspended service":      {func(n *NFProfile) { n.NfServices[0].NfServiceStatus = NFStatusUndiscoverable }, false},
		"target in plmnList":     {func(n *NFProfile) { n.PlmnList = []PlmnID{{Mcc: "001", Mnc: "02"}, homePLMN} }, true},
		"target not in plmnList": {func(n *NFProfile) { n.PlmnList = []PlmnID{{Mcc: "001", Mnc: "001"}} }, false},
		"v1 only":                {func(n *NFProfile) { n.NfServices[0].Versions = []NFServiceVersion{{APIVersionInURI: "v1"}} }, false},
		"v1 and v2": {func(n *NFProfile) {
			n.NfServices[0].Versions = append(version, NFServiceVersion{APIVersionInURI: "v1"})
		}, true},
		"OP allowed by profile":    {func(n *NFProfile) { n.AllowedPlmns = []PlmnID{opPLMN} }, true},
		"OP not allowed (profile)": {func(n *NFProfile) { n.AllowedPlmns = []PlmnID{homePLMN} }, false},
		"service list prevails": {func(n *NFProfile) {
			n.AllowedPlmns = []PlmnID{homePLMN}
			n.NfServices[0].AllowedPlmns = []PlmnID{opPLMN}
		}, true},
		"OP not allowed (service)": {func(n *NFProfile) { n.NfServices[0].AllowedPlmns = []PlmnID{homePLMN} }, false},
		"no FQDN":                  {func(n *NFProfile) { n.NfServices[0].Fqdn = "" }, false},
	} {
		n := base
		n.NfServices = []NFService{svc}
		tc.mutate(&n)

		if _, ok := n.Candidate(sel); ok != tc.ok {
			t.Errorf("%s: Candidate = %v", name, ok)
		}
	}
}

func TestNFProfileErrors(t *testing.T) {
	requireParams[NFProfile](t, []paramCase{
		{`{"nfType":"AUSF","nfStatus":"REGISTERED"}`, "/nfInstanceId"},
		{`{"nfInstanceId":"x","nfStatus":"REGISTERED"}`, "/nfType"},
		{`{"nfInstanceId":"x","nfType":"AUSF"}`, "/nfStatus"},
		{`{"nfInstanceId":"x","nfType":"AUSF","nfStatus":"REGISTERED","plmnList":[{"mcc":"001"}]}`, "/plmnList/0/mnc"},
		{`{"nfInstanceId":"x","nfType":"AUSF","nfStatus":"REGISTERED","allowedPlmns":[{"mnc":"01"}]}`, "/allowedPlmns/0/mcc"},
	})

	lenient, err := Decode[NFProfile]([]byte(`{"nfInstanceId":"x","nfType":"AUSF","nfStatus":"REGISTERED","fqdn":"bad fqdn",` +
		`"nfServices":[{"scheme":"http","nfServiceStatus":"REGISTERED"},` +
		`{"serviceInstanceId":"1","serviceName":"nausf-auth","scheme":"http","nfServiceStatus":"REGISTERED","versions":[{"apiVersionInUri":"v1","apiFullVersion":"1"}],"ipEndPoints":[{"port":70000}]},` +
		`{"serviceInstanceId":"2","serviceName":"nausf-auth","scheme":"http","nfServiceStatus":"REGISTERED","versions":[{"apiVersionInUri":"v1"}]},` +
		`{"serviceInstanceId":"3","serviceName":"nausf-auth","scheme":"http","nfServiceStatus":"REGISTERED","versions":[]},` +
		`{"serviceInstanceId":"4","serviceName":"nausf-auth","scheme":"http","nfServiceStatus":"REGISTERED","versions":[{"apiVersionInUri":"v1","apiFullVersion":"1"}],"apiPrefix":"x"}]}`))
	if err != nil || len(lenient.NfServices) != 1 || lenient.NfServices[0].ServiceInstanceID != "4" {
		t.Fatalf("lenient profile = %+v, %v", lenient, err)
	}

	if _, ok := lenient.Candidate(ausfSel); ok {
		t.Fatal("bad apiPrefix selected")
	}

	future, err := Decode[NFProfile]([]byte(`{"nfInstanceId":"x","nfType":"QUANTUM_NF","nfStatus":"HIBERNATING",` +
		`"nfServices":[{"serviceInstanceId":"1","serviceName":"nfoo-bar","versions":[{"apiVersionInUri":"v1","apiFullVersion":"1"}],"scheme":"gopher","nfServiceStatus":"NEW","ipEndPoints":[{"transport":"QUIC"}]}]}`))
	if err != nil || len(future.NfServices) != 1 {
		t.Fatalf("unknown enum values: %+v, %v", future, err)
	}

	if future.NfType.Known() || future.NfStatus.Known() || future.Check(Strict) == nil {
		t.Fatal("unknown values not flagged")
	}

	for _, ep := range []IPEndPoint{
		{Ipv4Address: "10.0.0.1", Ipv6Address: "::1"},
		{Ipv4Address: "::ffff:10.0.0.1"},
		{Ipv6Address: "10.0.0.1"},
		{Transport: "UDP"},
	} {
		if ep.Check(Strict) == nil {
			t.Errorf("IPEndPoint %+v accepted", ep)
		}
	}

	big := 70000
	if (NFService{}).Check(Strict) == nil || (NFServiceVersion{APIVersionInURI: "v1", APIFullVersion: "1", Expiry: "x"}).Check(Strict) == nil ||
		(NFProfile{NfInstanceID: "x", NfType: NFTypeAMF, NfStatus: NFStatusRegistered, Priority: &big}).Check(Strict) == nil {
		t.Fatal("strict accepted bad values")
	}
}

func TestDiscoveryQuery(t *testing.T) {
	v, err := DiscoveryQuery{
		TargetNFType:      NFTypeAUSF,
		RequesterNFType:   NFTypeAMF,
		ServiceNames:      []ServiceName{ServiceNausfAuth},
		TargetPlmnList:    []PlmnID{homePLMN},
		RequesterPlmnList: []PlmnID{opPLMN},
		RoutingIndicator:  "0000",
	}.Values()
	if err != nil {
		t.Fatal(err)
	}

	want := "requester-nf-type=AMF&requester-plmn-list=%5B%7B%22mcc%22%3A%22999%22%2C%22mnc%22%3A%2270%22%7D%5D" +
		"&routing-indicator=0000&service-names=nausf-auth&target-nf-type=AUSF" +
		"&target-plmn-list=%5B%7B%22mcc%22%3A%22001%22%2C%22mnc%22%3A%2201%22%7D%5D"
	if v.Encode() != want {
		t.Fatalf("query =\n%s\nwant\n%s", v.Encode(), want)
	}

	parsed, _ := url.ParseQuery(v.Encode())

	var plmns []PlmnID
	if err := json.Unmarshal([]byte(parsed.Get(QueryTargetPlmnList)), &plmns); err != nil || plmns[0] != homePLMN {
		t.Fatalf("target-plmn-list = %v, %v", plmns, err)
	}

	v, err = DiscoveryQuery{
		TargetNFType: NFTypeUDM, RequesterNFType: NFTypeSMF,
		ServiceNames:          []ServiceName{ServiceNudmSDM, ServiceNudmUECM},
		Supi:                  "imsi-001010000000001",
		RequesterNFInstanceID: "54804518-4191-46b3-955c-ac631f953ed8",
		RequesterFeatures:     "1",
		TargetPlmnList:        []PlmnID{opPLMN, {Mcc: "999", Mnc: "71"}},
		RequesterPlmnList:     []PlmnID{opPLMN, {Mcc: "999", Mnc: "71"}},
	}.Values()
	if err != nil || v.Get(QueryServiceNames) != "nudm-sdm,nudm-uecm" || v.Get(QuerySupi) != "imsi-001010000000001" ||
		v.Get(QueryRequesterNFInstanceID) == "" || v.Get(QueryRequesterFeatures) != "1" {
		t.Fatalf("query = %v, %v", v, err)
	}

	for name, q := range map[string]DiscoveryQuery{
		"no target":            {RequesterNFType: NFTypeAMF},
		"no requester":         {TargetNFType: NFTypeAUSF},
		"unknown target":       {TargetNFType: "FOO", RequesterNFType: NFTypeAMF},
		"repeated service":     {TargetNFType: NFTypeUDM, RequesterNFType: NFTypeAMF, ServiceNames: []ServiceName{"a", "a"}},
		"comma service":        {TargetNFType: NFTypeUDM, RequesterNFType: NFTypeAMF, ServiceNames: []ServiceName{"a,b"}},
		"bad PLMN":             {TargetNFType: NFTypeUDM, RequesterNFType: NFTypeAMF, TargetPlmnList: []PlmnID{{Mcc: "1"}}, RequesterPlmnList: []PlmnID{opPLMN}},
		"target w/o requester": {TargetNFType: NFTypeUDM, RequesterNFType: NFTypeAMF, TargetPlmnList: []PlmnID{homePLMN}},
		"two inter-PLMN targets": {
			TargetNFType: NFTypeUDM, RequesterNFType: NFTypeAMF,
			TargetPlmnList: []PlmnID{homePLMN, {Mcc: "001", Mnc: "02"}}, RequesterPlmnList: []PlmnID{opPLMN},
		},
		"bad RI":       {TargetNFType: NFTypeUDM, RequesterNFType: NFTypeAMF, RoutingIndicator: "12345"},
		"bad SUPI":     {TargetNFType: NFTypeUDM, RequesterNFType: NFTypeAMF, Supi: "suci-0-001-01-0-0-0-1"},
		"bad instance": {TargetNFType: NFTypeUDM, RequesterNFType: NFTypeAMF, RequesterNFInstanceID: "amf"},
		"bad features": {TargetNFType: NFTypeUDM, RequesterNFType: NFTypeAMF, RequesterFeatures: "x"},
	} {
		_, err := q.Values()

		var ie *IEError
		if err == nil || errors.As(err, &ie) {
			t.Errorf("%s: %v (want a plain error)", name, err)
		}
	}
}
