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

func TestResetRoundTrip(t *testing.T) {
	sub := SubscriptionData{OperatorDeterminedBarring: ptr(ODBAllPacketOrientedServices)}
	deletion := SubscriptionDataDeletion{Flags: DSRFlagAPNOIReplacement}

	for name, r := range map[string]ResetRequest{
		"restart":         {UserIDs: []string{"00101", "20893"}},
		"all":             {},
		"reset ids":       {Features: Features{List2: FeatureResetIDs}, ResetIDs: []ResetID{testResetID, "fe-1"}},
		"shared data":     {ResetIDs: []ResetID{testResetID}, Subscription: &sub},
		"shared deletion": {ResetIDs: []ResetID{testResetID}, Deletion: &deletion},
	} {
		t.Run(name, func(t *testing.T) {
			req := must[*diameter.Message](t)(NewResetRequest(hssEnvelope, r))
			requireNoVendorSpecificApplicationID(t, req)

			got, err := ParseResetRequest(roundTrip(t, req))
			if err != nil || !reflect.DeepEqual(got, r) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}
		})
	}

	req := must[*diameter.Message](t)(NewResetRequest(hssEnvelope, ResetRequest{}))
	r := Reset{Features: Features{List2: FeatureResetIDs}}

	ans := must[*diameter.Message](t)(NewResetAnswer(req, mmeIdentity, r))
	if back, err := ParseResetAnswer(roundTrip(t, ans)); err != nil || back != r {
		t.Fatalf("RSA = %+v, %v", back, err)
	}
}

func TestResetValidation(t *testing.T) {
	sub := SubscriptionData{}
	deletion := SubscriptionDataDeletion{Flags: DSRFlagSTNSR}

	for name, r := range map[string]ResetRequest{
		"bad User-Id":               {UserIDs: []string{"0010a"}},
		"long User-Id":              {UserIDs: []string{"0010100000000001"}},
		"subscription without id":   {Subscription: &sub},
		"deletion without id":       {Deletion: &deletion},
		"subscription and deletion": {ResetIDs: []ResetID{testResetID}, Subscription: &sub, Deletion: &deletion},
		"bad subscription":          {ResetIDs: []ResetID{testResetID}, Subscription: &SubscriptionData{MSISDN: "x"}},
		"bad deletion":              {ResetIDs: []ResetID{testResetID}, Deletion: &SubscriptionDataDeletion{Flags: DSRFlagPDNSubscriptionContextsWithdrawal}},
	} {
		if _, err := NewResetRequest(hssEnvelope, r); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseResetRequestErrors(t *testing.T) {
	base := must[*diameter.Message](t)(NewResetRequest(hssEnvelope, ResetRequest{}))
	sub := vendorGroup(AVPSubscriptionData)
	del := diameter.Grouped(AVPSubscriptionDataDeletion, 0, tgpp.VendorID, vendorU32(AVPDSRFlags, uint32(DSRFlagSTNSR)))
	resetID := diameter.OctetString(AVPResetID, 0, tgpp.VendorID, []byte{1})

	tests := map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"subscription without Reset-ID": {with(base, sub), diameter.ResultMissingAVP},
		"deletion without Reset-ID":     {with(base, del), diameter.ResultMissingAVP},
		"subscription and deletion":     {with(base, resetID, sub, del), diameter.ResultContradictingAVPs},
		"bad User-Id":                   {with(base, diameter.UTF8String(AVPUserID, 0, tgpp.VendorID, "x")), diameter.ResultInvalidAVPValue},
		"bad subscription":              {with(base, resetID, vendorGroup(AVPSubscriptionData, vendorU32(4242, 0))), diameter.ResultAVPUnsupported},
		"deletion without flags":        {with(base, resetID, diameter.Grouped(AVPSubscriptionDataDeletion, 0, tgpp.VendorID)), diameter.ResultMissingAVP},
		"unknown M-bit in deletion": {with(base, resetID, diameter.Grouped(AVPSubscriptionDataDeletion, 0, tgpp.VendorID,
			vendorU32(AVPDSRFlags, 0), vendorU32(4242, 0))), diameter.ResultAVPUnsupported},
		"deletion without context": {with(base, resetID, diameter.Grouped(AVPSubscriptionDataDeletion, 0, tgpp.VendorID,
			vendorU32(AVPDSRFlags, uint32(DSRFlagPDNSubscriptionContextsWithdrawal)))), diameter.ResultMissingAVP},
		"twice Subscription-Data": {with(base, resetID, sub, sub), diameter.ResultAVPOccursTooManyTimes},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseResetRequest(tt.req); avpResult(t, err) != tt.want {
				t.Fatalf("err = %v, want %d", err, tt.want)
			}
		})
	}

	_, err := ParseResetRequest(tests["subscription and deletion"].req)
	if failed := avpError(t, err).AVPs; len(failed) != 2 {
		t.Fatalf("contradicting Failed-AVP = %+v", failed)
	}

	_, err = ParseResetRequest(tests["deletion without context"].req)
	if failed := avpError(t, err).AVPs[0]; failed.Code != AVPSubscriptionDataDeletion {
		t.Fatalf("Failed-AVP = %+v, want the Subscription-Data-Deletion parent", failed)
	}
}

func TestResetAffects(t *testing.T) {
	stored := []ResetID{"fe-1", "fe-2"}

	for name, tt := range map[string]struct {
		r    ResetRequest
		imsi string
		want bool
	}{
		"everyone":               {ResetRequest{}, testIMSI, true},
		"matching User-Id":       {ResetRequest{UserIDs: []string{"20893", "00101"}}, testIMSI, true},
		"other User-Id":          {ResetRequest{UserIDs: []string{"20893"}}, testIMSI, false},
		"matching Reset-ID":      {ResetRequest{ResetIDs: []ResetID{"fe-9", "fe-2"}}, testIMSI, true},
		"other Reset-ID":         {ResetRequest{ResetIDs: []ResetID{"fe-9"}}, testIMSI, false},
		"Reset-ID over User-Id":  {ResetRequest{UserIDs: []string{"00101"}, ResetIDs: []ResetID{"fe-9"}}, testIMSI, false},
		"Reset-ID with no store": {ResetRequest{ResetIDs: []ResetID{"fe-1"}}, testIMSI, false},
	} {
		s := stored
		if name == "Reset-ID with no store" {
			s = nil
		}

		if got := tt.r.Affects(tt.imsi, s); got != tt.want {
			t.Errorf("%s: Affects = %v", name, got)
		}
	}
}
