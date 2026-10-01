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

func TestReportSMDeliveryStatusRequestEncoding(t *testing.T) {
	req, err := NewReportSMDeliveryStatusRequest(testEnvelope, DeliveryReport{
		MSISDN:               "15551230002",
		ServiceCentreAddress: testServiceCentreAddress,
		SingleAttempt:        true,
		MME:                  &DeliveryOutcome{Cause: DeliveryCauseAbsentUser, AbsentDiagnostic: absent(2)},
		SMSF3GPP:             &DeliveryOutcome{Cause: DeliveryCauseMemoryCapacityExceeded},
	})
	if err != nil {
		t.Fatal(err)
	}

	if req.CommandCode != CommandReportSMDeliveryStatus || req.ApplicationID != ApplicationID {
		t.Fatalf("header = %+v", req)
	}

	ui, _ := req.Find(tgpp.AVPUserIdentifier, tgpp.VendorID)
	if msisdn, ok := diameter.Find(grouped(t, ui), tgpp.AVPMSISDN, tgpp.VendorID); !ok || !bytes.Equal(msisdn.Data, mustHex(t, "5155210300f2")) {
		t.Fatalf("MSISDN = %+v", msisdn)
	}

	if flags, ok := req.Find(AVPRDRFlags, tgpp.VendorID); !ok || unsigned(t, flags) != RDRFlagSingleAttempt || flags.Flags&diameter.AVPFlagMandatory != 0 {
		t.Fatalf("RDR-Flags = %+v", flags)
	}

	outcome, _ := req.Find(tgpp.AVPSMDeliveryOutcome, tgpp.VendorID)
	outcomes := grouped(t, outcome)

	mme, ok := diameter.Find(outcomes, AVPMMESMDeliveryOutcome, tgpp.VendorID)
	if !ok || mme.Flags&diameter.AVPFlagMandatory == 0 {
		t.Fatalf("MME-SM-Delivery-Outcome = %+v", mme)
	}

	smsf, ok := diameter.Find(outcomes, AVPSMSF3GPPSMDeliveryOutcome, tgpp.VendorID)
	if !ok || smsf.Flags&diameter.AVPFlagMandatory != 0 {
		t.Fatalf("SMSF-3GPP-SM-Delivery-Outcome = %+v", smsf)
	}

	if hasFeatures(t, req) {
		t.Fatal("Supported-Features sent without SMSF-Support")
	}
}

func TestReportSMDeliveryStatusRequestRoundTrip(t *testing.T) {
	for name, rep := range map[string]DeliveryReport{
		"MME absent with failed nodes": {
			MSISDN: "15551230002", ServiceCentreAddress: testServiceCentreAddress, SingleAttempt: true,
			MME:    &DeliveryOutcome{Cause: DeliveryCauseAbsentUser, AbsentDiagnostic: absent(tgpp.AbsentUserIMSIDetached)},
			SGSN:   &DeliveryOutcome{Cause: DeliveryCauseAbsentUser},
			Failed: ServingNodes{Serving: mmeNode(), Additional: &ServingNode{SGSN: &NodeAddress{Number: "15550000020"}}},
		},
		"SMSF with support": {
			MSISDN: "15551230002", ServiceCentreAddress: testServiceCentreAddress, SMSFSupport: true,
			SMSF3GPP: &DeliveryOutcome{Cause: DeliveryCauseMemoryCapacityExceeded},
			Failed:   ServingNodes{SMSF3GPP: smsfNode()},
		},
		"MSC and IP-SM-GW success": {
			MSISDN: "15551230002", ServiceCentreAddress: testServiceCentreAddress,
			MSC:    &DeliveryOutcome{Cause: DeliveryCauseSuccessfulTransfer},
			IPSMGW: &DeliveryOutcome{Cause: DeliveryCauseAbsentUser},
		},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := NewReportSMDeliveryStatusRequest(testEnvelope, rep)
			if err != nil {
				t.Fatal(err)
			}

			got, err := ParseReportSMDeliveryStatusRequest(roundTrip(t, req))
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(got, rep) {
				t.Fatalf("round trip = %+v, want %+v", got, rep)
			}
		})
	}
}

func TestReportSMDeliveryStatusRequestValidation(t *testing.T) {
	outcome := &DeliveryOutcome{Cause: DeliveryCauseAbsentUser}

	for name, rep := range map[string]DeliveryReport{
		"no outcome":    {MSISDN: "15551230002", ServiceCentreAddress: testServiceCentreAddress},
		"no identity":   {ServiceCentreAddress: testServiceCentreAddress, MME: outcome},
		"bad MSISDN":    {MSISDN: "+1", ServiceCentreAddress: testServiceCentreAddress, MME: outcome},
		"no SC address": {MSISDN: "15551230002", MME: outcome},
		"MME and MSC":   {MSISDN: "15551230002", ServiceCentreAddress: testServiceCentreAddress, MME: outcome, MSC: outcome},
		"bad cause":     {MSISDN: "15551230002", ServiceCentreAddress: testServiceCentreAddress, MME: &DeliveryOutcome{Cause: 9}},
		"SMSF node unsupported": {
			MSISDN: "15551230002", ServiceCentreAddress: testServiceCentreAddress, MME: outcome,
			Failed: ServingNodes{SMSF3GPP: smsfNode()},
		},
		"bad failed node": {
			MSISDN: "15551230002", ServiceCentreAddress: testServiceCentreAddress, MME: outcome,
			Failed: ServingNodes{Serving: &ServingNode{MME: &NodeAddress{Name: "mme", Number: "+1"}}},
		},
	} {
		if _, err := NewReportSMDeliveryStatusRequest(testEnvelope, rep); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseReportSMDeliveryStatusRequestErrors(t *testing.T) {
	ui, err := tgpp.NewUserIdentifier(tgpp.UserIdentifier{MSISDN: "15551230002"})
	if err != nil {
		t.Fatal(err)
	}

	sc := diameter.OctetString(tgpp.AVPSCAddress, diameter.AVPFlagMandatory, tgpp.VendorID, mustHex(t, "5155000000f0"))

	outcome := func(inner ...diameter.AVP) diameter.AVP {
		return diameter.Grouped(tgpp.AVPSMDeliveryOutcome, diameter.AVPFlagMandatory, tgpp.VendorID, inner...)
	}

	cause := func(code, v uint32) diameter.AVP {
		return diameter.Grouped(code, diameter.AVPFlagMandatory, tgpp.VendorID,
			diameter.Unsigned32(AVPSMDeliveryCause, diameter.AVPFlagMandatory, tgpp.VendorID, v))
	}

	tests := map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"no outcome":      {request(CommandReportSMDeliveryStatus, ui, sc), diameter.ResultMissingAVP},
		"empty outcome":   {request(CommandReportSMDeliveryStatus, ui, sc, outcome()), diameter.ResultInvalidAVPValue},
		"MME and MSC":     {request(CommandReportSMDeliveryStatus, ui, sc, outcome(cause(AVPMMESMDeliveryOutcome, 1), cause(AVPMSCSMDeliveryOutcome, 1))), diameter.ResultInvalidAVPValue},
		"unknown cause":   {request(CommandReportSMDeliveryStatus, ui, sc, outcome(cause(AVPMMESMDeliveryOutcome, 7))), diameter.ResultInvalidAVPValue},
		"cause missing":   {request(CommandReportSMDeliveryStatus, ui, sc, outcome(diameter.Grouped(AVPMMESMDeliveryOutcome, diameter.AVPFlagMandatory, tgpp.VendorID))), diameter.ResultInvalidAVPValue},
		"empty identity":  {request(CommandReportSMDeliveryStatus, diameter.Grouped(tgpp.AVPUserIdentifier, diameter.AVPFlagMandatory, tgpp.VendorID), sc, outcome(cause(AVPMMESMDeliveryOutcome, 1))), diameter.ResultInvalidAVPValue},
		"bad failed node": {request(CommandReportSMDeliveryStatus, ui, sc, outcome(cause(AVPMMESMDeliveryOutcome, 1)), diameter.Grouped(AVPServingNode, diameter.AVPFlagMandatory, tgpp.VendorID)), diameter.ResultInvalidAVPValue},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ParseReportSMDeliveryStatusRequest(tt.req)
			if code := resultCode(t, err); code != tt.want {
				t.Fatalf("Result-Code = %d, want %d", code, tt.want)
			}
		})
	}
}

func TestReportSMDeliveryStatusAnswerRoundTrip(t *testing.T) {
	req := request(CommandReportSMDeliveryStatus)

	res := ReportResult{
		ServingNodes: ServingNodes{Serving: mmeNode(), SMSF3GPP: smsfNode()},
		AlertMSISDN:  "15559999000",
	}

	ans, err := NewReportSMDeliveryStatusAnswer(req, hssIdentity, res, true)
	if err != nil {
		t.Fatal(err)
	}

	if !hasFeatures(t, ans) {
		t.Fatal("RDA without Supported-Features")
	}

	got, err := ParseReportSMDeliveryStatusAnswer(roundTrip(t, ans))
	if err != nil || !reflect.DeepEqual(got, res) {
		t.Fatalf("round trip = %+v, %v", got, err)
	}

	if _, err := NewReportSMDeliveryStatusAnswer(req, hssIdentity, res, false); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("SMSF address in RDA without SMSF-Support: %v", err)
	}

	res.SMSF3GPP = nil

	ans, err = NewReportSMDeliveryStatusAnswer(req, hssIdentity, res, false)
	if err != nil || !hasFeatures(t, ans) {
		t.Fatalf("RDA without SMSF = %v", err)
	}
}

func TestParseReportSMDeliveryStatusAnswerErrors(t *testing.T) {
	req := request(CommandReportSMDeliveryStatus)

	_, err := ParseReportSMDeliveryStatusAnswer(NewAnswer(req, hssIdentity, tgpp.Experimental(tgpp.ResultErrorMWDListFull)))
	if !tgpp.IsExperimental(err, tgpp.ResultErrorMWDListFull) {
		t.Fatalf("err = %v", err)
	}

	malformed := &diameter.Message{AVPs: []diameter.AVP{
		diameter.Unsigned32(diameter.AVPResultCode, diameter.AVPFlagMandatory, 0, diameter.ResultSuccess),
		diameter.OctetString(tgpp.AVPUserIdentifier, 0, tgpp.VendorID, []byte{0x01}),
	}}

	if _, err := ParseReportSMDeliveryStatusAnswer(malformed); !errors.Is(err, ErrMalformedAnswer) {
		t.Fatalf("err = %v", err)
	}
}
