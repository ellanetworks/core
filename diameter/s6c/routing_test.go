// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6c

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func absent(v tgpp.AbsentUserDiagnostic) *tgpp.AbsentUserDiagnostic {
	return &v
}

func TestSendRoutingInfoForSMRequestEncoding(t *testing.T) {
	req, err := NewSendRoutingInfoForSMRequest(testEnvelope, RoutingRequest{
		MSISDN:               "15551230002",
		ServiceCentreAddress: testServiceCentreAddress,
		GPRSIndicator:        true,
		SingleAttempt:        true,
		SMSFSupport:          true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if req.CommandCode != CommandSendRoutingInfoForSM || req.ApplicationID != ApplicationID ||
		req.Flags != diameter.FlagRequest|diameter.FlagProxiable || req.AVPs[0].Code != diameter.AVPSessionID {
		t.Fatalf("header = %+v", req)
	}

	checks := map[string]struct {
		code, vendor uint32
		want         []byte
	}{
		"Destination-Host":   {diameter.AVPDestinationHost, 0, []byte("hss.example.org")},
		"MSISDN":             {tgpp.AVPMSISDN, tgpp.VendorID, mustHex(t, "5155210300f2")},
		"SC-Address":         {tgpp.AVPSCAddress, tgpp.VendorID, mustHex(t, "5155000000f0")},
		"SM-RP-MTI":          {AVPSMRPMTI, tgpp.VendorID, mustHex(t, "00000000")},
		"SRR-Flags":          {AVPSRRFlags, tgpp.VendorID, mustHex(t, "00000005")},
		"Auth-Session-State": {diameter.AVPAuthSessionState, 0, mustHex(t, "00000001")},
	}

	for name, c := range checks {
		a, ok := req.Find(c.code, c.vendor)
		if !ok || !bytes.Equal(a.Data, c.want) || (c.vendor != 0 && a.Flags&diameter.AVPFlagMandatory == 0) {
			t.Errorf("%s = %+v (present %v), want %x", name, a, ok, c.want)
		}
	}

	features, _ := req.Find(tgpp.AVPSupportedFeatures, tgpp.VendorID)
	if features.Flags&diameter.AVPFlagMandatory != 0 || !hasFeatures(t, req) {
		t.Fatalf("Supported-Features = %+v", features)
	}
}

func TestSendRoutingInfoForSMRequestRoundTrip(t *testing.T) {
	notIntended := SMDeliveryNotIntendedMCCMNC

	for name, r := range map[string]RoutingRequest{
		"MSISDN deliver": {MSISDN: "15551230002", ServiceCentreAddress: testServiceCentreAddress, GPRSIndicator: true, SMSFSupport: true},
		"status report priority": {
			MSISDN: "15551230002", ServiceCentreAddress: testServiceCentreAddress, MTI: SMRPMTIStatusReport,
			Priority: true, SingleAttempt: true, SMEA: []byte{0x0b, 0x91, 0x51, 0x55},
		},
		"IMSI not intended": {IMSI: "001010000000001", ServiceCentreAddress: testServiceCentreAddress, DeliveryNotIntended: &notIntended},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := NewSendRoutingInfoForSMRequest(testEnvelope, r)
			if err != nil {
				t.Fatal(err)
			}

			got, err := ParseSendRoutingInfoForSMRequest(roundTrip(t, req))
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(got, r) {
				t.Fatalf("round trip = %+v, want %+v", got, r)
			}
		})
	}
}

func TestSendRoutingInfoForSMRequestValidation(t *testing.T) {
	bad := DeliveryNotIntended(7)

	for name, r := range map[string]RoutingRequest{
		"no identity": {ServiceCentreAddress: testServiceCentreAddress},
		"bad MSISDN":  {MSISDN: "+1555", ServiceCentreAddress: testServiceCentreAddress},
		"bad IMSI":    {IMSI: "12", ServiceCentreAddress: testServiceCentreAddress},
		"bad MTI":     {MSISDN: "1", ServiceCentreAddress: testServiceCentreAddress, MTI: 5},
		"bad not intended": {
			MSISDN: "1", ServiceCentreAddress: testServiceCentreAddress, DeliveryNotIntended: &bad,
		},
	} {
		if _, err := NewSendRoutingInfoForSMRequest(testEnvelope, r); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseSendRoutingInfoForSMRequestErrors(t *testing.T) {
	sc := diameter.OctetString(tgpp.AVPSCAddress, diameter.AVPFlagMandatory, tgpp.VendorID, mustHex(t, "5155000000f0"))
	msisdn := diameter.OctetString(tgpp.AVPMSISDN, diameter.AVPFlagMandatory, tgpp.VendorID, mustHex(t, "5155210300f2"))

	tests := map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"no identity":      {request(CommandSendRoutingInfoForSM, sc), diameter.ResultMissingAVP},
		"two SC addresses": {request(CommandSendRoutingInfoForSM, msisdn, sc, sc), diameter.ResultAVPOccursTooManyTimes},
		"unknown M AVP": {
			request(CommandSendRoutingInfoForSM, msisdn, sc, diameter.Unsigned32(9999, diameter.AVPFlagMandatory, tgpp.VendorID, 0)),
			diameter.ResultAVPUnsupported,
		},
		"bad MSISDN": {
			request(CommandSendRoutingInfoForSM, diameter.OctetString(tgpp.AVPMSISDN, diameter.AVPFlagMandatory, tgpp.VendorID, []byte{0xba}), sc),
			diameter.ResultInvalidAVPValue,
		},
		"bad MTI": {
			request(CommandSendRoutingInfoForSM, msisdn, sc, diameter.Unsigned32(AVPSMRPMTI, diameter.AVPFlagMandatory, tgpp.VendorID, 3)),
			diameter.ResultInvalidAVPValue,
		},
		"bad IMSI": {
			request(CommandSendRoutingInfoForSM, sc, diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, "hss-id")),
			diameter.ResultInvalidAVPValue,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ParseSendRoutingInfoForSMRequest(tt.req)
			if code := resultCode(t, err); code != tt.want {
				t.Fatalf("Result-Code = %d, want %d", code, tt.want)
			}
		})
	}

	withOriginState := request(CommandSendRoutingInfoForSM, msisdn, sc,
		diameter.Unsigned32(diameter.AVPOriginStateID, diameter.AVPFlagMandatory, 0, 7))
	if _, err := ParseSendRoutingInfoForSMRequest(withOriginState); err != nil {
		t.Fatalf("Origin-State-Id rejected: %v", err)
	}
}

func TestSendRoutingInfoForSMAnswerRoundTrip(t *testing.T) {
	req := request(CommandSendRoutingInfoForSM)

	routing := Routing{
		ServingNodes: ServingNodes{
			Serving:    mmeNode(),
			Additional: &ServingNode{SGSN: &NodeAddress{Name: "sgsn.example.org", Realm: "example.org", Number: "15550000020"}},
			SMSF3GPP:   smsfNode(),
		},
		IMSI:        "001010000000001",
		LMSI:        []byte{1, 2, 3, 4},
		MWDStatus:   MWDStatusMNRF,
		Absent:      AbsentUserDiagnostics{MSC: absent(tgpp.AbsentUserIMSIDetached), SMSF3GPP: absent(tgpp.AbsentUserTemporarilyUnavailable)},
		AlertMSISDN: "15559999000",
	}

	ans, err := NewSendRoutingInfoForSMAnswer(req, hssIdentity, routing, true)
	if err != nil {
		t.Fatal(err)
	}

	if !hasFeatures(t, ans) {
		t.Fatal("SRA without Supported-Features")
	}

	got, err := ParseSendRoutingInfoForSMAnswer(roundTrip(t, ans))
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, routing) {
		t.Fatalf("round trip = %+v, want %+v", got, routing)
	}

	serving, _ := ans.Find(AVPServingNode, tgpp.VendorID)
	for _, a := range grouped(t, serving) {
		if a.Flags&diameter.AVPFlagMandatory == 0 {
			t.Errorf("Serving-Node AVP %d without the M bit (TS 29.338 Table 5.3.3.1/2)", a.Code)
		}
	}
}

func TestSendRoutingInfoForSMAnswerWithoutSMSFSupport(t *testing.T) {
	req := request(CommandSendRoutingInfoForSM)

	routing := Routing{
		ServingNodes: ServingNodes{Serving: mmeNode()},
		IMSI:         "001010000000001",
		Absent:       AbsentUserDiagnostics{SMSF3GPP: absent(1), MME: absent(2)},
	}

	ans, err := NewSendRoutingInfoForSMAnswer(req, hssIdentity, routing, false)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := ans.Find(AVPSMSF3GPPAbsentUserDiagnosticSM, tgpp.VendorID); ok {
		t.Fatal("SMSF diagnostic sent without SMSF-Support")
	}

	if !hasFeatures(t, ans) {
		t.Fatal("SRA must always list the sender's features (TS 29.229 §7.2)")
	}

	routing.SMSF3GPP = smsfNode()

	if _, err := NewSendRoutingInfoForSMAnswer(req, hssIdentity, routing, false); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("SMSF address without SMSF-Support: %v", err)
	}
}

func TestSendRoutingInfoForSMAnswerValidation(t *testing.T) {
	req := request(CommandSendRoutingInfoForSM)

	for name, routing := range map[string]Routing{
		"no IMSI":         {ServingNodes: ServingNodes{Serving: mmeNode()}},
		"no serving node": {IMSI: "001010000000001"},
		"MME without number": {IMSI: "001010000000001", ServingNodes: ServingNodes{
			Serving: &ServingNode{MME: &NodeAddress{Name: "mme.example.org", Realm: "example.org"}},
		}},
		"MME and SGSN together": {IMSI: "001010000000001", ServingNodes: ServingNodes{
			Serving: &ServingNode{MME: mmeNode().MME, SGSN: &NodeAddress{Number: "1"}},
		}},
		"empty serving node": {IMSI: "001010000000001", ServingNodes: ServingNodes{Serving: &ServingNode{}}},
		"IP-SM-GW additional": {IMSI: "001010000000001", ServingNodes: ServingNodes{
			Serving: mmeNode(), Additional: &ServingNode{IPSMGW: &NodeAddress{Number: "1"}},
		}},
		"additional without serving": {IMSI: "001010000000001", ServingNodes: ServingNodes{
			Additional: mmeNode(), SMSF3GPP: smsfNode(),
		}},
		"SMSF without number": {IMSI: "001010000000001", ServingNodes: ServingNodes{
			SMSF3GPP: &NodeAddress{Name: "smsf.example.org", Realm: "example.org"},
		}},
	} {
		if _, err := NewSendRoutingInfoForSMAnswer(req, hssIdentity, routing, true); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestSendRoutingInfoForSMAnswerWithoutServingNodeWhenDeliveryNotIntended(t *testing.T) {
	req := request(CommandSendRoutingInfoForSM,
		diameter.Unsigned32(AVPSMDeliveryNotIntended, diameter.AVPFlagMandatory, tgpp.VendorID, uint32(SMDeliveryNotIntendedIMSI)))

	ans, err := NewSendRoutingInfoForSMAnswer(req, hssIdentity, Routing{IMSI: "001010000000001"}, true)
	if err != nil {
		t.Fatalf("NewSendRoutingInfoForSMAnswer: %v", err)
	}

	if name, ok := ans.Find(diameter.AVPUserName, 0); !ok || name.UTF8String() != "001010000000001" {
		t.Fatalf("answer without the IMSI: %+v", ans)
	}

	if _, ok := ans.Find(AVPServingNode, tgpp.VendorID); ok {
		t.Fatal("answer names a serving node")
	}
}

func TestSendRoutingInfoForSMAllowedServingNodes(t *testing.T) {
	req := request(CommandSendRoutingInfoForSM)

	for name, node := range map[string]*ServingNode{
		"a SGSN number":         {SGSN: &NodeAddress{Number: "15550000020"}},
		"b SGSN name and realm": {SGSN: &NodeAddress{Name: "sgsn", Realm: "example.org", Number: "15550000020"}},
		"c MME":                 mmeNode(),
		"d MSC":                 {MSCNumber: "15550000040"},
		"e MSC and MME":         {MSCNumber: "15550000040", MME: &NodeAddress{Name: "mme.example.org", Realm: "example.org"}},
		"f IP-SM-GW":            {IPSMGW: &NodeAddress{Number: "15550000050"}},
		"g IP-SM-GW named":      {IPSMGW: &NodeAddress{Number: "15550000050", Name: "ipsmgw.example.org"}},
	} {
		routing := Routing{IMSI: "001010000000001", ServingNodes: ServingNodes{Serving: node}}
		if _, err := NewSendRoutingInfoForSMAnswer(req, hssIdentity, routing, false); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestSendRoutingInfoForSMErrorAnswerRoundTrip(t *testing.T) {
	req := request(CommandSendRoutingInfoForSM)

	e := ResultError{
		Result:      tgpp.Experimental(tgpp.ResultErrorAbsentUser),
		MWDStatus:   MWDStatusMNRF | MWDStatusMNR5G,
		Absent:      AbsentUserDiagnostics{MME: absent(tgpp.AbsentUserIMSIDetached)},
		AlertMSISDN: "15559999000",
	}

	ans, err := NewSendRoutingInfoForSMErrorAnswer(req, hssIdentity, e, true)
	if err != nil {
		t.Fatal(err)
	}

	if !hasFeatures(t, ans) {
		t.Fatal("error SRA without Supported-Features")
	}

	_, err = ParseSendRoutingInfoForSMAnswer(roundTrip(t, ans))
	if !tgpp.IsExperimental(err, tgpp.ResultErrorAbsentUser) {
		t.Fatalf("err = %v", err)
	}

	var got *ResultError
	if !errors.As(err, &got) || !reflect.DeepEqual(*got, e) {
		t.Fatalf("result error = %+v, want %+v", got, e)
	}

	if _, err := NewSendRoutingInfoForSMErrorAnswer(req, hssIdentity, ResultError{Result: tgpp.Result{Code: diameter.ResultSuccess}}, true); err == nil {
		t.Fatal("success result accepted as an error answer")
	}

	base, err := NewSendRoutingInfoForSMErrorAnswer(req, hssIdentity, ResultError{Result: tgpp.Result{Code: diameter.ResultUnableToComply}}, true)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ParseSendRoutingInfoForSMAnswer(base); !errors.As(err, &got) || got.Experimental || got.Code != diameter.ResultUnableToComply {
		t.Fatalf("base error = %v", err)
	}
}

func TestParseSendRoutingInfoForSMAnswerMalformed(t *testing.T) {
	success := func(avps ...diameter.AVP) *diameter.Message {
		return &diameter.Message{AVPs: append([]diameter.AVP{
			diameter.Unsigned32(diameter.AVPResultCode, diameter.AVPFlagMandatory, 0, diameter.ResultSuccess),
			diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, "001010000000001"),
		}, avps...)}
	}

	for name, ans := range map[string]*diameter.Message{
		"no result":       {},
		"no user name":    {AVPs: []diameter.AVP{diameter.Unsigned32(diameter.AVPResultCode, diameter.AVPFlagMandatory, 0, diameter.ResultSuccess)}},
		"no serving node": success(),
		"MME without number only": success(diameter.Grouped(AVPServingNode, diameter.AVPFlagMandatory, tgpp.VendorID,
			diameter.UTF8String(AVPMMEName, diameter.AVPFlagMandatory, tgpp.VendorID, "mme.example.org"))),
		"bad node number": success(diameter.Grouped(AVPServingNode, diameter.AVPFlagMandatory, tgpp.VendorID,
			diameter.OctetString(AVPMSCNumber, diameter.AVPFlagMandatory, tgpp.VendorID, []byte{0xaa}))),
		"short MWD-Status": success(diameter.Grouped(AVPServingNode, diameter.AVPFlagMandatory, tgpp.VendorID,
			diameter.OctetString(AVPMSCNumber, diameter.AVPFlagMandatory, tgpp.VendorID, []byte{0x51})),
			diameter.OctetString(AVPMWDStatus, diameter.AVPFlagMandatory, tgpp.VendorID, []byte{1})),
	} {
		if _, err := ParseSendRoutingInfoForSMAnswer(ans); !errors.Is(err, ErrMalformedAnswer) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseSendRoutingInfoForSMRequestWithoutSCAddress(t *testing.T) {
	msisdn := diameter.OctetString(tgpp.AVPMSISDN, diameter.AVPFlagMandatory, tgpp.VendorID, mustHex(t, "5155210300f2"))

	r, err := ParseSendRoutingInfoForSMRequest(request(CommandSendRoutingInfoForSM, msisdn))
	if err != nil || r.ServiceCentreAddress != "" {
		t.Fatalf("request = %+v (%v), want an SRR without SC-Address accepted (29.338 §5.3.2.3)", r, err)
	}
}
