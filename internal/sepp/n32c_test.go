// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/internal/roaming/wire"
)

const (
	ellaFQDN = "sepp.5gc.mnc070.mcc999.3gppnetwork.org"
	peerFQDN = "sepp.5gc.mnc001.mcc001.3gppnetwork.org"
)

var (
	ellaPLMN = wire.PlmnID{Mcc: "999", Mnc: "70"}
	peerPLMN = wire.PlmnID{Mcc: "001", Mnc: "01"}
)

func negotiation() Negotiation {
	return Negotiation{
		LocalFQDN:         ellaFQDN,
		LocalPlmnID:       ellaPLMN,
		PeerFQDN:          peerFQDN,
		PeerPlmnID:        peerPLMN,
		PeerCertDNSNames:  []string{"SEPP.5gc.mnc001.mcc001.3gppnetwork.org"},
		SupportedFeatures: LocalFeatures,
	}
}

func TestNegotiateRequestEncoding(t *testing.T) {
	b, err := wire.Encode(negotiation().Request())
	if err != nil {
		t.Fatal(err)
	}

	want := `{"sender":"sepp.5gc.mnc070.mcc999.3gppnetwork.org","supportedSecCapabilityList":["TLS"],` +
		`"3GppSbiTargetApiRootSupported":true,"plmnIdList":[{"mcc":"999","mnc":"70"}],` +
		`"targetPlmnId":{"mcc":"001","mnc":"01"},"supportedFeatures":"1"}`
	if string(b) != want {
		t.Fatalf("request =\n%s\nwant\n%s", b, want)
	}

	td, err := wire.Encode(negotiation().TeardownRequest())
	if err != nil || string(td) != `{"sender":"sepp.5gc.mnc070.mcc999.3gppnetwork.org","supportedSecCapabilityList":["NONE"]}` {
		t.Fatalf("teardown = %s, %v", td, err)
	}
}

func TestDecodeOpen5GSRequest(t *testing.T) {
	body := `{"sender":"sepp.5gc.mnc001.mcc001.3gppnetwork.org","supportedSecCapabilityList":["TLS","PRINS"],` +
		`"3GppSbiTargetApiRootSupported":true,"plmnIdList":[{"mcc":"001","mnc":"01"}],` +
		`"targetPlmnId":{"mcc":"999","mnc":"70"},"supportedFeatures":"1"}`

	req, err := wire.Decode[SecNegotiateReqData]([]byte(body))
	if err != nil {
		t.Fatal(err)
	}

	rsp, outcome, err := negotiation().Respond(req)
	if err != nil || outcome != OutcomeEstablish {
		t.Fatalf("Respond = %v, %v", outcome, err)
	}

	b, err := wire.Encode(rsp)
	if err != nil {
		t.Fatal(err)
	}

	want := `{"sender":"sepp.5gc.mnc070.mcc999.3gppnetwork.org","selectedSecCapability":"TLS",` +
		`"3GppSbiTargetApiRootSupported":true,"plmnIdList":[{"mcc":"999","mnc":"70"}],"supportedFeatures":"1"}`
	if string(b) != want {
		t.Fatalf("response =\n%s\nwant\n%s", b, want)
	}
}

func TestDecodeRequestErrors(t *testing.T) {
	for _, tc := range []struct {
		body, param string
		missing     bool
	}{
		{`{"supportedSecCapabilityList":["TLS"]}`, "/sender", true},
		{`{"sender":"x","supportedSecCapabilityList":["TLS"]}`, "/sender", false},
		{`{"sender":"sepp.example.com"}`, "/supportedSecCapabilityList", true},
		{`{"sender":"sepp.example.com","supportedSecCapabilityList":[]}`, "/supportedSecCapabilityList", true},
		{`{"sender":"sepp.example.com","supportedSecCapabilityList":[""]}`, "/supportedSecCapabilityList/0", true},
		{`{"sender":"sepp.example.com","supportedSecCapabilityList":["TLS"],"plmnIdList":[{"mcc":"001"}]}`, "/plmnIdList/0/mnc", true},
		{`{"sender":"sepp.example.com","supportedSecCapabilityList":["TLS"],"targetPlmnId":{"mcc":"01","mnc":"01"}}`, "/targetPlmnId/mcc", false},
		{`{"sender":"sepp.example.com","supportedSecCapabilityList":["TLS"],"intendedUsagePurpose":[{}]}`, "/intendedUsagePurpose/0/usagePurpose", true},
		{`{"sender":"sepp.example.com","supportedSecCapabilityList":["TLS"],"supportedFeatures":"xyz"}`, "/supportedFeatures", false},
	} {
		_, err := wire.Decode[SecNegotiateReqData]([]byte(tc.body))

		var ie *wire.IEError
		if !errors.As(err, &ie) || ie.Param != tc.param || ie.Missing != tc.missing {
			t.Errorf("Decode(%s) = %v, want %s missing=%v", tc.body, err, tc.param, tc.missing)

			continue
		}

		if pd := wire.ProblemFromError(err); pd.Status != http.StatusBadRequest {
			t.Errorf("ProblemFromError = %+v", pd)
		}
	}

	if _, err := wire.Decode[SecNegotiateReqData]([]byte(`{"sender":`)); !errors.Is(err, wire.ErrMalformed) {
		t.Fatalf("truncated body: %v", err)
	}
}

func TestUnknownValuesTolerated(t *testing.T) {
	body := `{"sender":"sepp.5gc.mnc001.mcc001.3gppnetwork.org","supportedSecCapabilityList":["ALPS","TLS"],` +
		`"3GppSbiTargetApiRootSupported":true,"intendedUsagePurpose":[{"usagePurpose":"FUTURE"},{"usagePurpose":"ROAMING"}],` +
		`"n32HandshakeId":"not-hex","senderN32fPortList":[70000]}`

	req, err := wire.Decode[SecNegotiateReqData]([]byte(body))
	if err != nil {
		t.Fatal(err)
	}

	if err := req.Check(wire.Strict); err == nil {
		t.Fatal("strict check accepted unknown values")
	}

	rsp, _, err := negotiation().Respond(req)
	if err != nil {
		t.Fatal(err)
	}

	if len(rsp.AllowedUsagePurpose) != 1 || rsp.AllowedUsagePurpose[0].UsagePurpose != PurposeRoaming ||
		len(rsp.RejectedUsagePurpose) != 1 || rsp.RejectedUsagePurpose[0].UsagePurpose != "FUTURE" || rsp.N32HandshakeID != "" {
		t.Fatalf("purposes = %+v", rsp)
	}

	if rsp.SupportedFeatures != "" {
		t.Fatalf("features = %q, want omitted", rsp.SupportedFeatures)
	}
}

func TestRespondChecks(t *testing.T) {
	base := func() SecNegotiateReqData {
		target := ellaPLMN

		return SecNegotiateReqData{
			Sender:                     peerFQDN,
			SupportedSecCapabilityList: []SecurityCapability{SecurityTLS},
			TargetAPIRootSupported:     true,
			PlmnIDList:                 []wire.PlmnID{peerPLMN},
			TargetPlmnID:               &target,
		}
	}

	for name, tc := range map[string]struct {
		mutate func(*SecNegotiateReqData, *Negotiation)
		cause  string
	}{
		"sender mismatch":   {func(r *SecNegotiateReqData, _ *Negotiation) { r.Sender = "sepp.other.org" }, CauseNegotiationNotAllowed},
		"sender not in SAN": {func(_ *SecNegotiateReqData, n *Negotiation) { n.PeerCertDNSNames = []string{"x.org"} }, CauseNegotiationNotAllowed},
		"PRINS only": {func(r *SecNegotiateReqData, _ *Negotiation) {
			r.SupportedSecCapabilityList = []SecurityCapability{SecurityPRINS}
		}, CauseNegotiationNotAllowed},
		"no target apiRoot": {func(r *SecNegotiateReqData, _ *Negotiation) { r.TargetAPIRootSupported = false }, CauseNegotiationNotAllowed},
		"wrong PLMN list": {func(r *SecNegotiateReqData, _ *Negotiation) {
			r.PlmnIDList = []wire.PlmnID{{Mcc: "001", Mnc: "001"}}
		}, CauseNegotiationNotAllowed},
		"wrong target PLMN": {func(r *SecNegotiateReqData, _ *Negotiation) { r.TargetPlmnID = &peerPLMN }, CauseNegotiationNotAllowed},
		"no roaming purpose": {func(r *SecNegotiateReqData, _ *Negotiation) {
			r.IntendedUsagePurpose = []IntendedN32Purpose{{UsagePurpose: PurposeSMSInterconnect}}
		}, CauseRequestedPurposeNotAllowed},
	} {
		req, n := base(), negotiation()
		tc.mutate(&req, &n)

		_, _, err := n.Respond(req)

		var pd wire.ProblemDetails
		if !errors.As(err, &pd) || pd.Status != http.StatusForbidden || pd.Cause != tc.cause {
			t.Errorf("%s: %v", name, err)
		}
	}

	req := base()
	req.PlmnIDList, req.TargetPlmnID = nil, nil
	req.Sender = "SEPP.5gc.mnc001.mcc001.3gppnetwork.org."

	if _, outcome, err := negotiation().Respond(req); err != nil || outcome != OutcomeEstablish {
		t.Fatalf("absent PLMN IEs: %v, %v", outcome, err)
	}

	req = base()
	req.SupportedSecCapabilityList = []SecurityCapability{SecurityNone}
	req.TargetAPIRootSupported = false

	rsp, outcome, err := negotiation().Respond(req)
	if err != nil || outcome != OutcomeTeardown || rsp.SelectedSecCapability != SecurityNone || rsp.Sender != ellaFQDN {
		t.Fatalf("teardown = %+v, %v, %v", rsp, outcome, err)
	}

	if _, err := wire.Encode(rsp); err != nil {
		t.Fatal(err)
	}
}

func TestCheckResponse(t *testing.T) {
	open5gs := `{"sender":"sepp.5gc.mnc001.mcc001.3gppnetwork.org","selectedSecCapability":"TLS",` +
		`"3GppSbiTargetApiRootSupported":true,"plmnIdList":[{"mcc":"001","mnc":"01"}],"supportedFeatures":""}`

	rsp, err := wire.Decode[SecNegotiateRspData]([]byte(open5gs))
	if err != nil {
		t.Fatal(err)
	}

	nftlst, err := negotiation().CheckResponse(rsp)
	if err != nil || nftlst {
		t.Fatalf("Open5GS on-the-fly node = %v, %v", nftlst, err)
	}

	rsp.SupportedFeatures = "1"
	if nftlst, err := negotiation().CheckResponse(rsp); err != nil || !nftlst {
		t.Fatalf("NFTLST = %v, %v", nftlst, err)
	}

	bad := rsp
	bad.TargetAPIRootSupported = false

	if nftlst, err := negotiation().CheckResponse(bad); !errors.Is(err, ErrNegotiation) || !nftlst {
		t.Fatalf("no apiRoot = %v, %v", nftlst, err)
	}

	for name, mutate := range map[string]func(*SecNegotiateRspData){
		"sender": func(r *SecNegotiateRspData) { r.Sender = "sepp.other.org" },
		"PRINS":  func(r *SecNegotiateRspData) { r.SelectedSecCapability = SecurityPRINS },
		"PLMN":   func(r *SecNegotiateRspData) { r.PlmnIDList = []wire.PlmnID{ellaPLMN} },
	} {
		r := rsp
		mutate(&r)

		if _, err := negotiation().CheckResponse(r); !errors.Is(err, ErrNegotiation) {
			t.Errorf("%s: %v", name, err)
		}
	}

	if _, err := wire.Decode[SecNegotiateRspData]([]byte(`{"sender":"sepp.example.com"}`)); !errors.Is(err, wire.ErrMissingIE) {
		t.Fatalf("missing selectedSecCapability: %v", err)
	}

	if r, err := wire.Decode[SecNegotiateRspData]([]byte(`{"sender":"sepp.example.com","selectedSecCapability":"QUANTUM"}`)); err != nil || r.SelectedSecCapability != "QUANTUM" {
		t.Fatalf("unknown capability = %+v, %v", r, err)
	}
}

func TestNegotiateRoundTrip(t *testing.T) {
	port, keepalive := 443, 30
	target := ellaPLMN
	in := SecNegotiateReqData{
		Sender:                     peerFQDN,
		N32HandshakeID:             "955cac631f953ed8",
		SupportedSecCapabilityList: []SecurityCapability{SecurityTLS, SecurityPRINS},
		TargetAPIRootSupported:     true,
		PlmnIDList:                 []wire.PlmnID{peerPLMN},
		SnpnIDList:                 []wire.PlmnIDNid{{Mcc: "001", Mnc: "01", Nid: "000007ed9d5"}},
		TargetPlmnID:               &target,
		IntendedUsagePurpose:       []IntendedN32Purpose{{UsagePurpose: PurposeRoaming, AdditionalInfo: "x"}},
		SupportedFeatures:          wire.Features(FeatureNFTLST, FeatureTLSCOR),
		SenderN32fFqdn:             "n32f.example.com",
		SenderN32fPortList:         []int{port},
		N32KeepaliveTimer:          &keepalive,
	}

	b, err := wire.Encode(in)
	if err != nil {
		t.Fatal(err)
	}

	out, err := wire.Decode[SecNegotiateReqData](b)
	if err != nil {
		t.Fatal(err)
	}

	again, _ := json.Marshal(out)
	if string(again) != string(b) {
		t.Fatalf("round trip\n%s\n%s", b, again)
	}

	rsp := SecNegotiateRspData{
		Sender: ellaFQDN, SelectedSecCapability: SecurityTLS, N32HandshakeID: "955CAC631F953ED8", TargetAPIRootSupported: true,
		PlmnIDList: []wire.PlmnID{ellaPLMN}, AllowedUsagePurpose: []IntendedN32Purpose{{UsagePurpose: PurposeRoaming}},
		RejectedUsagePurpose: []IntendedN32Purpose{{UsagePurpose: PurposeSMSInterconnect, Cause: "x"}},
		SupportedFeatures:    "11", SenderN32fFqdn: "n32f.example.com", SenderN32fPort: &port, N32KeepaliveTimer: &keepalive,
	}

	b, err = wire.Encode(rsp)
	if err != nil {
		t.Fatal(err)
	}

	back, err := wire.Decode[SecNegotiateRspData](b)
	if err != nil || back.SupportedFeatures != "11" || *back.SenderN32fPort != 443 || back.RejectedUsagePurpose[0].Cause != "x" {
		t.Fatalf("response round trip = %+v, %v", back, err)
	}

	bigPort := 70000
	rsp.SenderN32fPort = &bigPort

	if _, err := wire.Encode(rsp); err == nil {
		t.Fatal("port 70000 encoded")
	}
}
