// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
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
		LocalFQDN:        ellaFQDN,
		LocalPlmnID:      ellaPLMN,
		PeerFQDN:         peerFQDN,
		PeerPlmnID:       peerPLMN,
		PeerCertDNSNames: []string{"SEPP.5gc.mnc001.mcc001.3gppnetwork.org"},
	}
}

func TestNegotiateRequestEncoding(t *testing.T) {
	b, err := wire.Encode(negotiation().Request())
	if err != nil {
		t.Fatal(err)
	}

	want := `{"sender":"sepp.5gc.mnc070.mcc999.3gppnetwork.org","supportedSecCapabilityList":["TLS"],` +
		`"3GppSbiTargetApiRootSupported":true,"plmnIdList":[{"mcc":"999","mnc":"70"}],` +
		`"targetPlmnId":{"mcc":"001","mnc":"01"}}`
	if string(b) != want {
		t.Fatalf("request =\n%s\nwant\n%s", b, want)
	}

	if N32cExchangeCapabilityPath != "/n32c-handshake/v1/exchange-capability" {
		t.Fatal("path")
	}
}

func TestRespondEstablish(t *testing.T) {
	body := `{"sender":"sepp.5gc.mnc001.mcc001.3gppnetwork.org","supportedSecCapabilityList":["TLS","PRINS"],` +
		`"3GppSbiTargetApiRootSupported":true,"plmnIdList":[{"mcc":"001","mnc":"01"}],` +
		`"targetPlmnId":{"mcc":"999","mnc":"70"},"supportedFeatures":"1"}`

	req, err := wire.Decode[SecNegotiateReqData]([]byte(body))
	if err != nil {
		t.Fatal(err)
	}

	rsp, err := negotiation().Respond(req)
	if err != nil {
		t.Fatalf("Respond = %v", err)
	}

	b, err := wire.Encode(rsp)
	if err != nil {
		t.Fatal(err)
	}

	want := `{"sender":"sepp.5gc.mnc070.mcc999.3gppnetwork.org","selectedSecCapability":"TLS",` +
		`"3GppSbiTargetApiRootSupported":true,"plmnIdList":[{"mcc":"999","mnc":"70"}]}`
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

		want := wire.CauseMandatoryIEIncorrect

		switch {
		case tc.missing:
			want = wire.CauseMandatoryIEMissing
		case ie.Optional:
			want = wire.CauseOptionalIEIncorrect
		}

		if pd := wire.ProblemFromError(err); pd.Status != http.StatusBadRequest || pd.Cause != want {
			t.Errorf("ProblemFromError = %+v", pd)
		}
	}

	if _, err := wire.Decode[SecNegotiateReqData]([]byte(`{"sender":`)); !errors.Is(err, wire.ErrMalformed) {
		t.Fatalf("truncated body: %v", err)
	}
}

func TestUnknownValuesTolerated(t *testing.T) {
	body := `{"sender":"sepp.5gc.mnc001.mcc001.3gppnetwork.org","supportedSecCapabilityList":["ALPS","TLS"],` +
		`"3GppSbiTargetApiRootSupported":true,"intendedUsagePurpose":[{"usagePurpose":"FUTURE"},{"usagePurpose":"ROAMING"},` +
		`{"usagePurpose":"FUTURE","additionalInfo":"again"},{"usagePurpose":"ROAMING_TEST"},{"usagePurpose":"ROAMING"},` +
		`{"usagePurpose":"INTER_PLMN_MOBILITY"}],"n32HandshakeId":"not-hex","senderN32fPortList":[70000]}`

	req, err := wire.Decode[SecNegotiateReqData]([]byte(body))
	if err != nil {
		t.Fatal(err)
	}

	if err := req.Check(wire.Strict); err == nil {
		t.Fatal("strict check accepted unknown values")
	}

	rsp, err := negotiation().Respond(req)
	if err != nil {
		t.Fatal(err)
	}

	purposes := func(l []IntendedN32Purpose) []N32Purpose {
		var out []N32Purpose
		for _, ip := range l {
			out = append(out, ip.UsagePurpose)
		}

		return out
	}

	if !slices.Equal(purposes(rsp.AllowedUsagePurpose), []N32Purpose{PurposeRoaming, PurposeInterPLMNMobility}) ||
		!slices.Equal(purposes(rsp.RejectedUsagePurpose), []N32Purpose{"FUTURE", PurposeRoamingTest}) || rsp.N32HandshakeID != "" {
		t.Fatalf("purposes = %+v", rsp)
	}

	if rsp.SupportedFeatures != "" {
		t.Fatalf("features = %q, want omitted", rsp.SupportedFeatures)
	}

	b, err := wire.Encode(rsp)
	if err != nil {
		t.Fatalf("response echoing an unknown peer purpose does not encode: %v", err)
	}

	back, err := wire.Decode[SecNegotiateRspData](b)
	if err != nil || len(back.RejectedUsagePurpose) != 2 {
		t.Fatalf("round trip = %+v, %v", back, err)
	}
}

func baseRequest() SecNegotiateReqData {
	target := ellaPLMN

	return SecNegotiateReqData{
		Sender:                     peerFQDN,
		SupportedSecCapabilityList: []SecurityCapability{SecurityTLS},
		TargetAPIRootSupported:     true,
		PlmnIDList:                 []wire.PlmnID{peerPLMN},
		TargetPlmnID:               &target,
	}
}

func TestRespondChecks(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(*SecNegotiateReqData, *Negotiation)
		cause  wire.Cause
	}{
		"sender mismatch":   {func(r *SecNegotiateReqData, _ *Negotiation) { r.Sender = "sepp.other.org" }, CauseNegotiationNotAllowed},
		"sender not in SAN": {func(_ *SecNegotiateReqData, n *Negotiation) { n.PeerCertDNSNames = []string{"x.org"} }, CauseNegotiationNotAllowed},
		"another SEPP of the partner, not in SAN": {func(r *SecNegotiateReqData, _ *Negotiation) {
			r.Sender = "sepp2.5gc.mnc001.mcc001.3gppnetwork.org"
		}, CauseNegotiationNotAllowed},
		"SEPP of another PLMN in SAN": {func(r *SecNegotiateReqData, n *Negotiation) {
			r.Sender = "sepp1.5gc.mnc002.mcc001.3gppnetwork.org"
			n.PeerCertDNSNames = append(n.PeerCertDNSNames, r.Sender)
		}, CauseNegotiationNotAllowed},
		"well-known SEPP FQDN of the partner in SAN": {func(r *SecNegotiateReqData, n *Negotiation) {
			n.PeerFQDN = "sepp1.5gc.mnc001.mcc001.3gppnetwork.org"
			n.PeerCertDNSNames = append(n.PeerCertDNSNames, r.Sender)
		}, CauseNegotiationNotAllowed},
		"PRINS only": {func(r *SecNegotiateReqData, _ *Negotiation) {
			r.SupportedSecCapabilityList = []SecurityCapability{SecurityPRINS}
		}, CauseNegotiationNotAllowed},
		"NONE": {func(r *SecNegotiateReqData, _ *Negotiation) {
			r.SupportedSecCapabilityList = []SecurityCapability{SecurityNone}
		}, CauseNegotiationNotAllowed},
		"no target apiRoot": {func(r *SecNegotiateReqData, _ *Negotiation) { r.TargetAPIRootSupported = false }, CauseNegotiationNotAllowed},
		"wrong PLMN list": {func(r *SecNegotiateReqData, _ *Negotiation) {
			r.PlmnIDList = []wire.PlmnID{{Mcc: "001", Mnc: "001"}}
		}, CauseNegotiationNotAllowed},
		"wrong target PLMN": {func(r *SecNegotiateReqData, _ *Negotiation) { r.TargetPlmnID = &peerPLMN }, CauseNegotiationNotAllowed},
		"target SNPN": {func(r *SecNegotiateReqData, _ *Negotiation) {
			r.TargetSnpnID = &wire.PlmnIDNid{Mcc: "999", Mnc: "70", Nid: "000007ed9d5"}
		}, CauseNegotiationNotAllowed},
		"SNPN list": {func(r *SecNegotiateReqData, _ *Negotiation) {
			r.SnpnIDList = []wire.PlmnIDNid{{Mcc: "001", Mnc: "01", Nid: "000007ed9d5"}}
		}, CauseNegotiationNotAllowed},
		"SMS only": {func(r *SecNegotiateReqData, _ *Negotiation) {
			r.IntendedUsagePurpose = []IntendedN32Purpose{{UsagePurpose: PurposeSMSInterconnect}}
		}, wire.CauseRequestedPurposeNotAllowed},
		"roaming test only": {func(r *SecNegotiateReqData, _ *Negotiation) {
			r.IntendedUsagePurpose = []IntendedN32Purpose{{UsagePurpose: PurposeRoamingTest}}
		}, wire.CauseRequestedPurposeNotAllowed},
	} {
		req, n := baseRequest(), negotiation()
		tc.mutate(&req, &n)

		rsp, err := n.Respond(req)
		if rsp.Sender != "" {
			t.Errorf("%s: a response with an error", name)
		}

		var pd wire.ProblemDetails
		if !errors.As(err, &pd) || pd.Status != http.StatusForbidden || pd.Cause != tc.cause {
			t.Errorf("%s: %v", name, err)
		}
	}

	req := baseRequest()
	req.PlmnIDList, req.TargetPlmnID = nil, nil
	req.Sender = "SEPP.5gc.mnc001.mcc001.3gppnetwork.org."

	if _, err := negotiation().Respond(req); err != nil {
		t.Fatalf("absent PLMN IEs: %v", err)
	}

	req = baseRequest()
	req.Sender = "sepp2.5gc.mnc001.mcc001.3gppnetwork.org"
	n := negotiation()
	n.PeerPlmnIDs = []wire.PlmnID{peerPLMN, {Mcc: "001", Mnc: "02"}}
	n.PeerCertDNSNames = []string{req.Sender}

	if _, err := n.Respond(req); err != nil {
		t.Fatalf("another SEPP of the partner: %v", err)
	}

	req.Sender = "sepp3.5gc.mnc002.mcc001.3gppnetwork.org"
	n.PeerCertDNSNames = []string{req.Sender}

	if _, err := n.Respond(req); err != nil {
		t.Fatalf("a SEPP of the partner's second PLMN: %v", err)
	}

	req = baseRequest()
	req.IntendedUsagePurpose = []IntendedN32Purpose{{UsagePurpose: PurposeInterPLMNMobility}}

	rsp, err := negotiation().Respond(req)
	if err != nil || len(rsp.AllowedUsagePurpose) != 1 || rsp.AllowedUsagePurpose[0].UsagePurpose != PurposeInterPLMNMobility || rsp.RejectedUsagePurpose != nil {
		t.Fatalf("mobility only = %+v, %v", rsp, err)
	}
}

func TestCheckResponse(t *testing.T) {
	noFeatures := `{"sender":"sepp.5gc.mnc001.mcc001.3gppnetwork.org","selectedSecCapability":"TLS",` +
		`"3GppSbiTargetApiRootSupported":true,"plmnIdList":[{"mcc":"001","mnc":"01"}],"supportedFeatures":""}`

	rsp, err := wire.Decode[SecNegotiateRspData]([]byte(noFeatures))
	if err != nil {
		t.Fatal(err)
	}

	if err := negotiation().CheckResponse(rsp); err != nil {
		t.Fatalf("empty supportedFeatures (TS 29.571 pattern ^[A-Fa-f0-9]*$): %v", err)
	}

	rsp.SupportedFeatures = "1"
	rsp.AllowedUsagePurpose = []IntendedN32Purpose{{UsagePurpose: PurposeRoaming}}

	if err := negotiation().CheckResponse(rsp); err != nil {
		t.Fatalf("features and purposes: %v", err)
	}

	for name, mutate := range map[string]func(*SecNegotiateRspData){
		"no apiRoot": func(r *SecNegotiateRspData) { r.TargetAPIRootSupported = false },
		"sender":     func(r *SecNegotiateRspData) { r.Sender = "sepp.other.org" },
		"PRINS":      func(r *SecNegotiateRspData) { r.SelectedSecCapability = SecurityPRINS },
		"wrong PLMN": func(r *SecNegotiateRspData) { r.PlmnIDList = []wire.PlmnID{ellaPLMN} },
		"SNPN":       func(r *SecNegotiateRspData) { r.SnpnIDList = []wire.PlmnIDNid{{Mcc: "001", Mnc: "01"}} },
		"roaming rejected": func(r *SecNegotiateRspData) {
			r.RejectedUsagePurpose = []IntendedN32Purpose{{UsagePurpose: PurposeRoaming}}
		},
		"roaming not listed": func(r *SecNegotiateRspData) {
			r.AllowedUsagePurpose = []IntendedN32Purpose{{UsagePurpose: PurposeSMSInterconnect}}
		},
	} {
		r := rsp
		mutate(&r)

		err := negotiation().CheckResponse(r)

		var ne *NegotiationError
		if !errors.As(err, &ne) || !errors.Is(err, ErrNegotiation) || ne.Error() == "" {
			t.Errorf("%s: %v", name, err)
		}
	}

	if _, err := wire.Decode[SecNegotiateRspData]([]byte(`{"sender":"sepp.example.com"}`)); !errors.Is(err, wire.ErrMissingIE) {
		t.Fatalf("missing selectedSecCapability: %v", err)
	}

	if r, err := wire.Decode[SecNegotiateRspData]([]byte(`{"sender":"sepp.example.com","selectedSecCapability":"QUANTUM"}`)); err != nil || r.SelectedSecCapability != "QUANTUM" {
		t.Fatalf("unknown capability = %+v, %v", r, err)
	}

	if _, err := wire.Decode[SecNegotiateRspData]([]byte(`{"sender":"sepp.example.com","selectedSecCapability":"TLS","allowedUsagePurpose":[{}]}`)); !errors.Is(err, wire.ErrMissingIE) {
		t.Fatalf("allowed purpose without usagePurpose: %v", err)
	}
}

func TestLocalPrecedes(t *testing.T) {
	a, b := "sepp.5gc.mnc345.mcc012.3gppnetwork.org", "sepp.5gc.mnc346.mcc012.3gppnetwork.org"

	if !LocalPrecedes(a, b) || LocalPrecedes(b, a) || LocalPrecedes(a, a) {
		t.Fatal("TS 29.573 §5.2.2 EXAMPLE: SEPP A precedes SEPP B")
	}

	if !LocalPrecedes("SEPP.5gc.mnc345.mcc012.3gppnetwork.org.", b) || LocalPrecedes(a, "SEPP.5GC.MNC345.mcc012.3gppnetwork.org") {
		t.Fatal("comparison is on lower-cased FQDNs without the trailing dot")
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
		SupportedFeatures:          wire.Features(1, 5),
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

	for name, mutate := range map[string]func(*SecNegotiateReqData){
		"handshake ID":  func(r *SecNegotiateReqData) { r.N32HandshakeID = "x" },
		"port":          func(r *SecNegotiateReqData) { r.SenderN32fPortList = []int{-1} },
		"N32-f FQDN":    func(r *SecNegotiateReqData) { r.SenderN32fFqdn = "x" },
		"SNPN":          func(r *SecNegotiateReqData) { r.SnpnIDList = []wire.PlmnIDNid{{Mcc: "001", Mnc: "01", Nid: "x"}} },
		"target SNPN":   func(r *SecNegotiateReqData) { r.TargetSnpnID = &wire.PlmnIDNid{Mcc: "1"} },
		"unknown cap":   func(r *SecNegotiateReqData) { r.SupportedSecCapabilityList = []SecurityCapability{"X"} },
		"unknown usage": func(r *SecNegotiateReqData) { r.IntendedUsagePurpose = []IntendedN32Purpose{{UsagePurpose: "X"}} },
	} {
		r := in
		mutate(&r)

		if _, err := wire.Encode(r); err == nil {
			t.Errorf("%s: encoded", name)
		}
	}

	rsp := SecNegotiateRspData{
		Sender: ellaFQDN, SelectedSecCapability: SecurityTLS, N32HandshakeID: "955CAC631F953ED8", TargetAPIRootSupported: true,
		PlmnIDList: []wire.PlmnID{ellaPLMN}, AllowedUsagePurpose: []IntendedN32Purpose{{UsagePurpose: PurposeRoaming}},
		RejectedUsagePurpose: []IntendedN32Purpose{{UsagePurpose: "SOMETHING_ELSE", Cause: "x"}},
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

	for name, mutate := range map[string]func(*SecNegotiateRspData){
		"port": func(r *SecNegotiateRspData) { r.SenderN32fPort = &bigPort },
		"cause allowed": func(r *SecNegotiateRspData) {
			r.AllowedUsagePurpose = []IntendedN32Purpose{{UsagePurpose: PurposeRoaming, Cause: "x"}}
		},
		"unknown allowed": func(r *SecNegotiateRspData) {
			r.AllowedUsagePurpose = []IntendedN32Purpose{{UsagePurpose: "X"}}
		},
		"handshake ID": func(r *SecNegotiateRspData) { r.N32HandshakeID = "x" },
		"N32-f FQDN":   func(r *SecNegotiateRspData) { r.SenderN32fFqdn = "x" },
		"SNPN":         func(r *SecNegotiateRspData) { r.SnpnIDList = []wire.PlmnIDNid{{Mcc: "1"}} },
		"unknown cap":  func(r *SecNegotiateRspData) { r.SelectedSecCapability = "X" },
	} {
		r := rsp
		mutate(&r)

		if _, err := wire.Encode(r); err == nil {
			t.Errorf("%s: encoded", name)
		}
	}
}

func FuzzRespond(f *testing.F) {
	f.Add([]byte(`{"sender":"sepp.5gc.mnc001.mcc001.3gppnetwork.org","supportedSecCapabilityList":["TLS"],"3GppSbiTargetApiRootSupported":true,` +
		`"intendedUsagePurpose":[{"usagePurpose":"FUTURE"},{"usagePurpose":"ROAMING"}],"supportedFeatures":"1f"}`))
	f.Add([]byte(`{"sender":"sepp.5gc.mnc001.mcc001.3gppnetwork.org","supportedSecCapabilityList":["NONE"]}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		req, err := wire.Decode[SecNegotiateReqData](data)
		if err != nil {
			if pd := wire.ProblemFromError(err); pd.Status != http.StatusBadRequest {
				t.Fatalf("decode error maps to %d", pd.Status)
			}

			return
		}

		rsp, err := negotiation().Respond(req)
		if err != nil {
			if pd := wire.ProblemFromError(err); pd.Status != http.StatusForbidden {
				t.Fatalf("respond error maps to %d", pd.Status)
			}

			return
		}

		b, err := wire.Encode(rsp)
		if err != nil {
			t.Fatalf("response to %s does not encode: %v", data, err)
		}

		back, err := wire.Decode[SecNegotiateRspData](b)
		if err != nil {
			t.Fatalf("%s does not decode: %v", b, err)
		}

		roaming := len(back.AllowedUsagePurpose) == 0 || hasPurpose(back.AllowedUsagePurpose, PurposeRoaming)
		if !roaming {
			return
		}

		peer := Negotiation{
			LocalFQDN: peerFQDN, LocalPlmnID: peerPLMN, PeerFQDN: ellaFQDN, PeerPlmnID: ellaPLMN,
			PeerCertDNSNames: []string{ellaFQDN},
		}

		if err := peer.CheckResponse(back); err != nil {
			t.Fatalf("own response %s rejected: %v", b, err)
		}
	})
}
