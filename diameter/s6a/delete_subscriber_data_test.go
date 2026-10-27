// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"errors"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func testDSR() DeleteSubscriberDataRequest {
	return DeleteSubscriberDataRequest{
		IMSI:     testIMSI,
		Features: Features{List2: FeatureMonitoringEvent},
		SubscriptionDataDeletion: SubscriptionDataDeletion{
			Flags: DSRFlagPDNSubscriptionContextsWithdrawal | DSRFlagTraceDataWithdrawal | DSRFlagSMSWithdrawal |
				DSRFlagDeleteMonitoringEvents | DSRFlagMSISDNWithdrawal,
			SCEFID:             "scef.example.org",
			ContextIdentifiers: []uint32{2, 3},
			TraceReference:     []byte{1, 2, 3, 4, 5, 6},
			TSCodes:            [][]byte{{0x21}},
			SSCodes:            [][]byte{{0x92}},
		},
	}
}

func TestDeleteSubscriberDataRoundTrip(t *testing.T) {
	for name, r := range map[string]DeleteSubscriberDataRequest{
		"full":    testDSR(),
		"minimal": {IMSI: testIMSI, SubscriptionDataDeletion: SubscriptionDataDeletion{Flags: DSRFlagSTNSR}},
		"LCS SS":  {IMSI: testIMSI, SubscriptionDataDeletion: SubscriptionDataDeletion{Flags: DSRFlagLCSWithdrawal, SSCodes: [][]byte{{0x11}}}},
		"PDP":     {IMSI: testIMSI, SubscriptionDataDeletion: SubscriptionDataDeletion{Flags: DSRFlagPDPContextsWithdrawal, ContextIdentifiers: []uint32{7}}},
	} {
		t.Run(name, func(t *testing.T) {
			req := must[*diameter.Message](t)(NewDeleteSubscriberDataRequest(hssEnvelope, r))
			requireNoVendorSpecificApplicationID(t, req)

			got, err := ParseDeleteSubscriberDataRequest(roundTrip(t, req))
			if err != nil || !reflect.DeepEqual(got, r) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}
		})
	}

	req := must[*diameter.Message](t)(NewDeleteSubscriberDataRequest(hssEnvelope, testDSR()))
	d := DeleteSubscriberData{Features: Features{List1: FeatureRegSub}, Flags: DSAFlagNetworkNodeAreaRestricted}

	ans := must[*diameter.Message](t)(NewDeleteSubscriberDataAnswer(req, mmeIdentity, d))
	if back, err := ParseDeleteSubscriberDataAnswer(roundTrip(t, ans)); err != nil || back != d {
		t.Fatalf("DSA = %+v, %v", back, err)
	}

	if _, err := NewDeleteSubscriberDataAnswer(req, mmeIdentity, DeleteSubscriberData{Flags: 1 << 1}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("undefined DSA flag: %v", err)
	}

	if back, err := ParseDeleteSubscriberDataAnswer(success(req, optionalU32(AVPDSAFlags, 1<<4))); err != nil || back.Flags != 1<<4 {
		t.Fatalf("raw DSA-Flags = %+v, %v", back, err)
	}

	if _, err := ParseDeleteSubscriberDataAnswer(success(req, vendorOctets(AVPDSAFlags, []byte{1}))); !errors.Is(err, ErrMalformedAnswer) {
		t.Fatalf("short DSA-Flags: %v", err)
	}
}

func TestDeleteSubscriberDataValidation(t *testing.T) {
	for name, mutate := range map[string]func(*SubscriptionDataDeletion){
		"PDN and PDP":             func(d *SubscriptionDataDeletion) { d.Flags |= DSRFlagPDPContextsWithdrawal },
		"monitoring without SCEF": func(d *SubscriptionDataDeletion) { d.SCEFID = "" },
		"withdrawal without id":   func(d *SubscriptionDataDeletion) { d.ContextIdentifiers = nil },
		"id without withdrawal":   func(d *SubscriptionDataDeletion) { d.Flags &^= DSRFlagPDNSubscriptionContextsWithdrawal },
		"zero context":            func(d *SubscriptionDataDeletion) { d.ContextIdentifiers = []uint32{0} },
		"trace without flag":      func(d *SubscriptionDataDeletion) { d.Flags &^= DSRFlagTraceDataWithdrawal },
		"TS code without SMS":     func(d *SubscriptionDataDeletion) { d.Flags &^= DSRFlagSMSWithdrawal },
		"SS code without SMS":     func(d *SubscriptionDataDeletion) { d.Flags &^= DSRFlagSMSWithdrawal; d.TSCodes = nil },
	} {
		t.Run(name, func(t *testing.T) {
			r := testDSR()
			mutate(&r.SubscriptionDataDeletion)

			if _, err := NewDeleteSubscriberDataRequest(hssEnvelope, r); !errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("err = %v", err)
			}
		})
	}

	if _, err := NewDeleteSubscriberDataRequest(hssEnvelope, DeleteSubscriberDataRequest{}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("no IMSI: %v", err)
	}
}

func TestParseDeleteSubscriberDataRequestErrors(t *testing.T) {
	base := must[*diameter.Message](t)(NewDeleteSubscriberDataRequest(hssEnvelope, testDSR()))
	flags := func(f DSRFlags) diameter.AVP { return vendorU32(AVPDSRFlags, uint32(f)) }

	for name, tt := range map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"no flags":            {without(base, AVPDSRFlags, tgpp.VendorID), diameter.ResultMissingAVP},
		"short flags":         {replace(base, vendorOctets(AVPDSRFlags, []byte{1})), diameter.ResultInvalidAVPLength},
		"long flags":          {replace(base, vendorOctets(AVPDSRFlags, make([]byte, 5))), diameter.ResultInvalidAVPLength},
		"PDN and PDP":         {replace(base, flags(DSRFlagPDNSubscriptionContextsWithdrawal|DSRFlagPDPContextsWithdrawal)), diameter.ResultInvalidAVPValue},
		"no SCEF-ID":          {without(base, AVPSCEFID, tgpp.VendorID), diameter.ResultMissingAVP},
		"empty SCEF-ID":       {replace(base, diameter.OctetString(AVPSCEFID, 0, tgpp.VendorID, nil)), diameter.ResultInvalidAVPValue},
		"no context":          {without(base, AVPContextIdentifier, tgpp.VendorID), diameter.ResultMissingAVP},
		"zero context":        {replace(base, vendorU32(AVPContextIdentifier, 0)), diameter.ResultInvalidAVPValue},
		"short context":       {replace(base, vendorOctets(AVPContextIdentifier, []byte{1})), diameter.ResultInvalidAVPLength},
		"twice trace":         {with(base, vendorOctets(AVPTraceReference, []byte{1})), diameter.ResultAVPOccursTooManyTimes},
		"no Destination-Host": {without(base, diameter.AVPDestinationHost, 0), diameter.ResultMissingAVP},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseDeleteSubscriberDataRequest(tt.req); avpResult(t, err) != tt.want {
				t.Fatalf("err = %v, want %d", err, tt.want)
			}
		})
	}

	got, err := ParseDeleteSubscriberDataRequest(replace(base, flags(DSRFlagSTNSR)))
	if err != nil || !reflect.DeepEqual(got.SubscriptionDataDeletion, SubscriptionDataDeletion{Flags: DSRFlagSTNSR, SCEFID: "scef.example.org"}) {
		t.Fatalf("unflagged members kept: %+v, %v", got, err)
	}
}
