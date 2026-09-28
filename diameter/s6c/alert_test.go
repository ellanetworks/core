// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6c

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

var hssEnvelope = tgpp.Envelope{
	SessionID:        "hss.example.org;1;1",
	Origin:           hssIdentity,
	DestinationHost:  "smsc.example.org",
	DestinationRealm: "example.org",
}

func TestHSSAlertServiceCentreRoundTrip(t *testing.T) {
	a := Alert{
		ServiceCentreAddress:      testServiceCentreAddress,
		User:                      tgpp.UserIdentifier{MSISDN: "15551230002"},
		MaximumUEAvailabilityTime: time.Date(2040, 1, 2, 3, 4, 5, 0, time.UTC),
	}

	req, err := NewHSSAlertServiceCentreRequest(hssEnvelope, a)
	if err != nil {
		t.Fatal(err)
	}

	if req.CommandCode != CommandAlertServiceCentre || req.ApplicationID != ApplicationID || !hasFeatures(t, req) {
		t.Fatalf("header = %+v", req)
	}

	if mua, _ := req.Find(AVPMaximumUEAvailabilityTime, tgpp.VendorID); mua.Flags&diameter.AVPFlagMandatory != 0 {
		t.Fatalf("Maximum-UE-Availability-Time = %+v", mua)
	}

	got, err := ParseAlertServiceCentreRequest(roundTrip(t, req))
	if err != nil || !reflect.DeepEqual(got, a) {
		t.Fatalf("round trip = %+v, %v", got, err)
	}
}

func TestMMEAlertServiceCentreRoundTrip(t *testing.T) {
	for name, a := range map[string]Alert{
		"available": {
			ServiceCentreAddress: testServiceCentreAddress,
			User:                 tgpp.UserIdentifier{IMSI: "001010000000001"},
			Event:                AlertEventUEAvailableForMTSMS,
		},
		"new node": {
			ServiceCentreAddress: testServiceCentreAddress,
			User:                 tgpp.UserIdentifier{IMSI: "001010000000001"},
			Event:                AlertEventUEUnderNewNode,
			ServingNode:          mmeNode(),
		},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := NewMMEAlertServiceCentreRequest(hssEnvelope, a)
			if err != nil {
				t.Fatal(err)
			}

			got, err := ParseAlertServiceCentreRequest(roundTrip(t, req))
			if err != nil || !reflect.DeepEqual(got, a) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}
		})
	}
}

func TestAlertServiceCentreValidation(t *testing.T) {
	msisdn := tgpp.UserIdentifier{MSISDN: "15551230002"}
	imsi := tgpp.UserIdentifier{IMSI: "001010000000001"}

	for name, a := range map[string]Alert{
		"no MSISDN":     {ServiceCentreAddress: testServiceCentreAddress, User: imsi},
		"with event":    {ServiceCentreAddress: testServiceCentreAddress, User: msisdn, Event: AlertEventUEAvailableForMTSMS},
		"with node":     {ServiceCentreAddress: testServiceCentreAddress, User: msisdn, ServingNode: mmeNode()},
		"no SC address": {User: msisdn},
	} {
		if _, err := NewHSSAlertServiceCentreRequest(hssEnvelope, a); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("HSS %s: err = %v", name, err)
		}
	}

	for name, a := range map[string]Alert{
		"no event":         {ServiceCentreAddress: testServiceCentreAddress, User: imsi},
		"unknown event":    {ServiceCentreAddress: testServiceCentreAddress, User: imsi, Event: 1 << 5},
		"MSISDN":           {ServiceCentreAddress: testServiceCentreAddress, User: msisdn, Event: AlertEventUEAvailableForMTSMS},
		"node without bit": {ServiceCentreAddress: testServiceCentreAddress, User: imsi, Event: AlertEventUEAvailableForMTSMS, ServingNode: mmeNode()},
		"MSC node": {
			ServiceCentreAddress: testServiceCentreAddress, User: imsi, Event: AlertEventUEUnderNewNode,
			ServingNode: &ServingNode{MSCNumber: "15550000040"},
		},
	} {
		if _, err := NewMMEAlertServiceCentreRequest(hssEnvelope, a); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("MME %s: err = %v", name, err)
		}
	}
}

func TestParseAlertServiceCentreRequestErrors(t *testing.T) {
	ui, err := tgpp.NewUserIdentifier(tgpp.UserIdentifier{MSISDN: "15551230002"})
	if err != nil {
		t.Fatal(err)
	}

	sc := diameter.OctetString(tgpp.AVPSCAddress, diameter.AVPFlagMandatory, tgpp.VendorID, mustHex(t, "5155000000f0"))

	tests := map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"no SC address":  {request(CommandAlertServiceCentre, ui), diameter.ResultMissingAVP},
		"no identity":    {request(CommandAlertServiceCentre, sc), diameter.ResultMissingAVP},
		"bad SC address": {request(CommandAlertServiceCentre, ui, diameter.OctetString(tgpp.AVPSCAddress, diameter.AVPFlagMandatory, tgpp.VendorID, []byte{0xba})), diameter.ResultInvalidAVPValue},
		"empty identity": {request(CommandAlertServiceCentre, sc, diameter.Grouped(tgpp.AVPUserIdentifier, diameter.AVPFlagMandatory, tgpp.VendorID)), diameter.ResultInvalidAVPValue},
		"SRR-Flags":      {request(CommandAlertServiceCentre, ui, sc, diameter.Unsigned32(AVPSRRFlags, diameter.AVPFlagMandatory, tgpp.VendorID, 0)), diameter.ResultAVPUnsupported},
		"short time":     {request(CommandAlertServiceCentre, ui, sc, diameter.OctetString(AVPMaximumUEAvailabilityTime, 0, tgpp.VendorID, []byte{1})), diameter.ResultInvalidAVPValue},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ParseAlertServiceCentreRequest(tt.req)
			if code := resultCode(t, err); code != tt.want {
				t.Fatalf("Result-Code = %d, want %d", code, tt.want)
			}
		})
	}
}

func TestParseAlertServiceCentreAnswer(t *testing.T) {
	req := request(CommandAlertServiceCentre)

	if err := ParseAlertServiceCentreAnswer(NewAnswer(req, smscIdentity, tgpp.Result{Code: diameter.ResultSuccess})); err != nil {
		t.Fatal(err)
	}

	var re *ResultError
	if err := ParseAlertServiceCentreAnswer(NewAnswer(req, smscIdentity, tgpp.Result{Code: diameter.ResultUnableToComply})); !errors.As(err, &re) {
		t.Fatalf("err = %v", err)
	}

	if err := ParseAlertServiceCentreAnswer(&diameter.Message{}); !errors.Is(err, ErrMalformedAnswer) {
		t.Fatalf("err = %v", err)
	}
}
