// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"encoding/json"
	"errors"
	"net/url"
	"testing"
)

const open5gsSearchResult = `{"validityPeriod":30,"nfInstances":[{"nfInstanceId":"3b8b0a4e-6e1b-41ee-9b6b-3d8b3f6b0e01",` +
	`"nfType":"AUSF","nfStatus":"REGISTERED","plmnList":[{"mcc":"001","mnc":"01"}],"fqdn":"ausf.5gc.mnc001.mcc001.3gppnetwork.org",` +
	`"ipv4Addresses":["127.0.0.11"],"allowedNfTypes":["AMF"],"priority":0,"capacity":100,"load":0,` +
	`"nfServices":[{"serviceInstanceId":"3b8c4a5e-6e1b-41ee-9b6b-3d8b3f6b0e01","serviceName":"nausf-auth",` +
	`"versions":[{"apiVersionInUri":"v1","apiFullVersion":"1.0.0"}],"scheme":"http","nfServiceStatus":"REGISTERED",` +
	`"fqdn":"ausf.5gc.mnc001.mcc001.3gppnetwork.org","ipEndPoints":[{"ipv4Address":"127.0.0.11","port":80}],` +
	`"allowedNfTypes":["AMF"],"priority":0,"capacity":100,"load":0}],"nfProfileChangesSupportInd":true,` +
	`"ausfInfo":{"routingIndicators":["0000"]}}]}`

func TestSearchResultOpen5GS(t *testing.T) {
	r, err := Decode[SearchResult]([]byte(open5gsSearchResult))
	if err != nil {
		t.Fatal(err)
	}

	if r.ValidityPeriod != 30 || len(r.NfInstances) != 1 {
		t.Fatalf("result = %+v", r)
	}

	if err := r.Check(Strict); err != nil {
		t.Fatal(err)
	}

	nf := r.NfInstances[0]

	s, ok := nf.Service(ServiceNameNausfAuth)
	if !ok {
		t.Fatal("nausf-auth not found")
	}

	root, err := nf.APIRoot(s)
	if err != nil || root != "http://ausf.5gc.mnc001.mcc001.3gppnetwork.org:80" {
		t.Fatalf("APIRoot = %q, %v", root, err)
	}

	if _, ok := nf.Service(ServiceNameNudmSDM); ok {
		t.Fatal("nudm-sdm found on an AUSF")
	}
}

func TestNFProfileServiceMapAndAPIRoot(t *testing.T) {
	body := `{"nfInstanceId":"udm-1","nfType":"UDM","nfStatus":"REGISTERED","fqdn":"udm.5gc.mnc001.mcc001.3gppnetwork.org.",` +
		`"nfServiceList":{"b":{"serviceName":"nudm-uecm","versions":[{"apiVersionInUri":"v1","apiFullVersion":"1.3.0"}],` +
		`"scheme":"https","nfServiceStatus":"REGISTERED","apiPrefix":"/udm/"},` +
		`"a":{"serviceInstanceId":"a","serviceName":"nudm-sdm","versions":[{"apiVersionInUri":"v2","apiFullVersion":"2.3.0"}],` +
		`"scheme":"https","nfServiceStatus":"SUSPENDED","fqdn":"sdm.5gc.mnc001.mcc001.3gppnetwork.org"},` +
		`"c":{"serviceInstanceId":"c","serviceName":"nudm-sdm","versions":[{"apiVersionInUri":"v2","apiFullVersion":"2.3.0"}],` +
		`"scheme":"https","nfServiceStatus":"REGISTERED","fqdn":"sdm2.5gc.mnc001.mcc001.3gppnetwork.org","ipEndPoints":[{"ipv6Address":"2001:db8::1","port":8443}]}}}`

	nf, err := Decode[NFProfile]([]byte(body))
	if err != nil {
		t.Fatal(err)
	}

	services := nf.Services()
	if len(services) != 3 || services[0].ServiceInstanceID != "a" || services[1].ServiceInstanceID != "b" {
		t.Fatalf("Services = %+v", services)
	}

	uecm, ok := nf.Service(ServiceNameNudmUECM)
	if root, err := nf.APIRoot(uecm); !ok || err != nil || root != "https://udm.5gc.mnc001.mcc001.3gppnetwork.org/udm" {
		t.Fatalf("UECM apiRoot = %q, %v", root, err)
	}

	sdm, ok := nf.Service(ServiceNameNudmSDM)
	if root, err := nf.APIRoot(sdm); !ok || err != nil || root != "https://sdm2.5gc.mnc001.mcc001.3gppnetwork.org:8443" {
		t.Fatalf("SDM apiRoot = %q, %v", root, err)
	}

	if nf.Check(Strict) == nil {
		t.Fatal("strict check accepted a non-UUID nfInstanceId")
	}

	ipOnly := NFProfile{NfInstanceID: "x", NfType: NFTypeAUSF, NfStatus: NFStatusRegistered, Ipv4Addresses: []string{"10.0.0.1"}}
	if _, err := ipOnly.APIRoot(NFService{ServiceName: ServiceNameNausfAuth, Scheme: SchemeHTTP}); err == nil {
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

func TestNFProfileErrors(t *testing.T) {
	for _, tc := range []struct{ body, param string }{
		{`{"nfType":"AUSF","nfStatus":"REGISTERED"}`, "/nfInstanceId"},
		{`{"nfInstanceId":"x","nfStatus":"REGISTERED"}`, "/nfType"},
		{`{"nfInstanceId":"x","nfType":"AUSF"}`, "/nfStatus"},
		{`{"nfInstanceId":"x","nfType":"AUSF","nfStatus":"REGISTERED","fqdn":"bad fqdn"}`, "/fqdn"},
		{`{"nfInstanceId":"x","nfType":"AUSF","nfStatus":"REGISTERED","plmnList":[{"mcc":"001"}]}`, "/plmnList/0/mnc"},
		{`{"nfInstanceId":"x","nfType":"AUSF","nfStatus":"REGISTERED","nfServices":[{"scheme":"http","nfServiceStatus":"REGISTERED"}]}`, "/nfServices/0/serviceName"},
		{`{"nfInstanceId":"x","nfType":"AUSF","nfStatus":"REGISTERED","nfServices":[{"serviceName":"nausf-auth","nfServiceStatus":"REGISTERED"}]}`, "/nfServices/0/scheme"},
		{`{"nfInstanceId":"x","nfType":"AUSF","nfStatus":"REGISTERED","nfServiceList":{"k":{"serviceName":"nausf-auth","scheme":"http"}}}`, "/nfServiceList/k/nfServiceStatus"},
		{`{"nfInstanceId":"x","nfType":"AUSF","nfStatus":"REGISTERED","nfServices":[{"serviceName":"nausf-auth","scheme":"http","nfServiceStatus":"REGISTERED","ipEndPoints":[{"port":70000}]}]}`, "/nfServices/0/ipEndPoints/0/port"},
		{`{"nfInstanceId":"x","nfType":"AUSF","nfStatus":"REGISTERED","nfServices":[{"serviceName":"nausf-auth","scheme":"http","nfServiceStatus":"REGISTERED","apiPrefix":"x"}]}`, "/nfServices/0/apiPrefix"},
	} {
		_, err := Decode[NFProfile]([]byte(tc.body))

		var ie *IEError
		if !errors.As(err, &ie) || ie.Param != tc.param {
			t.Errorf("Decode(%s) = %v, want %s", tc.body, err, tc.param)
		}
	}

	future, err := Decode[NFProfile]([]byte(`{"nfInstanceId":"x","nfType":"QUANTUM_NF","nfStatus":"HIBERNATING",` +
		`"nfServices":[{"serviceName":"nfoo-bar","scheme":"gopher","nfServiceStatus":"NEW","ipEndPoints":[{"transport":"QUIC"}]}]}`))
	if err != nil {
		t.Fatalf("unknown enum values: %v", err)
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
}

func TestSearchResultErrors(t *testing.T) {
	for _, body := range []string{`{"nfInstances":[]}`, `{"validityPeriod":30}`, `{"validityPeriod":30,"nfInstances":null}`} {
		if _, err := Decode[SearchResult]([]byte(body)); !errors.Is(err, ErrMissingIE) {
			t.Errorf("Decode(%s) = %v", body, err)
		}
	}

	if _, err := Decode[SearchResult]([]byte(`{"validityPeriod":-1,"nfInstances":[]}`)); !errors.Is(err, ErrIncorrectIE) {
		t.Fatalf("negative validity: %v", err)
	}

	empty, err := Decode[SearchResult]([]byte(`{"validityPeriod":0,"nfInstances":[]}`))
	if err != nil || len(empty.NfInstances) != 0 {
		t.Fatalf("empty result = %+v, %v", empty, err)
	}

	r, _ := Decode[SearchResult]([]byte(open5gsSearchResult))

	b, err := Encode(r)
	if err != nil {
		t.Fatal(err)
	}

	back, err := Decode[SearchResult](b)
	if err != nil || back.NfInstances[0].NfServices[0].IPEndPoints[0].Port == nil {
		t.Fatalf("round trip = %s, %v", b, err)
	}
}

func TestDiscoveryQuery(t *testing.T) {
	v, err := DiscoveryQuery{
		TargetNFType:      NFTypeAUSF,
		RequesterNFType:   NFTypeAMF,
		ServiceNames:      []ServiceName{ServiceNameNausfAuth},
		TargetPlmnList:    []PlmnID{{Mcc: "001", Mnc: "01"}},
		RequesterPlmnList: []PlmnID{{Mcc: "999", Mnc: "70"}},
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
	if err := json.Unmarshal([]byte(parsed.Get(QueryTargetPlmnList)), &plmns); err != nil || plmns[0] != (PlmnID{Mcc: "001", Mnc: "01"}) {
		t.Fatalf("target-plmn-list = %v, %v", plmns, err)
	}

	v, err = DiscoveryQuery{
		TargetNFType: NFTypeUDM, RequesterNFType: NFTypeSMF,
		ServiceNames:          []ServiceName{ServiceNameNudmSDM, ServiceNameNudmUECM},
		Supi:                  "imsi-001010000000001",
		RequesterNFInstanceID: "54804518-4191-46b3-955c-ac631f953ed8",
		RequesterFeatures:     "1",
	}.Values()
	if err != nil || v.Get(QueryServiceNames) != "nudm-sdm,nudm-uecm" || v.Get(QuerySupi) != "imsi-001010000000001" ||
		v.Get(QueryRequesterNFInstanceID) == "" || v.Get(QueryRequesterFeatures) != "1" {
		t.Fatalf("query = %v, %v", v, err)
	}

	for name, q := range map[string]DiscoveryQuery{
		"no target":        {RequesterNFType: NFTypeAMF},
		"no requester":     {TargetNFType: NFTypeAUSF},
		"unknown target":   {TargetNFType: "FOO", RequesterNFType: NFTypeAMF},
		"repeated service": {TargetNFType: NFTypeUDM, RequesterNFType: NFTypeAMF, ServiceNames: []ServiceName{"a", "a"}},
		"comma service":    {TargetNFType: NFTypeUDM, RequesterNFType: NFTypeAMF, ServiceNames: []ServiceName{"a,b"}},
		"bad PLMN":         {TargetNFType: NFTypeUDM, RequesterNFType: NFTypeAMF, TargetPlmnList: []PlmnID{{Mcc: "1"}}},
		"bad RI":           {TargetNFType: NFTypeUDM, RequesterNFType: NFTypeAMF, RoutingIndicator: "12345"},
		"bad SUPI":         {TargetNFType: NFTypeUDM, RequesterNFType: NFTypeAMF, Supi: "suci-0-001-01-0-0-0-1"},
		"bad instance":     {TargetNFType: NFTypeUDM, RequesterNFType: NFTypeAMF, RequesterNFInstanceID: "amf"},
		"bad features":     {TargetNFType: NFTypeUDM, RequesterNFType: NFTypeAMF, RequesterFeatures: "x"},
	} {
		_, err := q.Values()

		var ie *IEError
		if !errors.As(err, &ie) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
