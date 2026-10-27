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

func TestHSSInitiatedRequestsNeedDestinationHost(t *testing.T) {
	env := hssEnvelope
	env.DestinationHost = ""

	sub := testSubscription()

	for name, build := range map[string]func() (*diameter.Message, error){
		"CLR": func() (*diameter.Message, error) {
			return NewCancelLocationRequest(env, CancelLocationRequest{IMSI: testIMSI})
		},
		"IDR": func() (*diameter.Message, error) {
			return NewInsertSubscriberDataRequest(env, InsertSubscriberDataRequest{IMSI: testIMSI, Subscription: sub})
		},
		"DSR": func() (*diameter.Message, error) { return NewDeleteSubscriberDataRequest(env, testDSR()) },
		"RSR": func() (*diameter.Message, error) { return NewResetRequest(env, ResetRequest{}) },
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := build(); !errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("err = %v", err)
			}
		})
	}

	for name, rules := range map[string]func(*diameter.Message) error{
		"CLR": CheckCancelLocation, "IDR": CheckInsertSubscriberData, "DSR": CheckDeleteSubscriberData, "RSR": CheckReset,
	} {
		if err := rules(request(0, env)); avpResult(t, err) != diameter.ResultMissingAVP {
			t.Errorf("%s without Destination-Host: %v", name, err)
		}
	}
}

func TestCancelLocationRoundTrip(t *testing.T) {
	r := CancelLocationRequest{
		IMSI: testIMSI, Features: Features{List2: FeatureResetIDs},
		Type: CancellationSubscriptionWithdrawal, Flags: CLRFlagS6aS6dIndicator | CLRFlagReattachRequired,
	}

	req := must[*diameter.Message](t)(NewCancelLocationRequest(hssEnvelope, r))
	requireNoVendorSpecificApplicationID(t, req)

	if host, _ := req.Find(diameter.AVPDestinationHost, 0); host.UTF8String() != mmeHost {
		t.Fatalf("Destination-Host = %q", host.UTF8String())
	}

	got, err := ParseCancelLocationRequest(roundTrip(t, req))
	if err != nil || !reflect.DeepEqual(got, r) {
		t.Fatalf("round trip = %+v, %v", got, err)
	}

	c := CancelLocation{Features: Features{List1: FeatureODBAllAPN}}

	ans := must[*diameter.Message](t)(NewCancelLocationAnswer(req, mmeIdentity, c))
	requireNoVendorSpecificApplicationID(t, ans)

	if back, err := ParseCancelLocationAnswer(roundTrip(t, ans)); err != nil || back != c {
		t.Fatalf("CLA = %+v, %v", back, err)
	}
}

func TestCancelLocationValidation(t *testing.T) {
	for name, r := range map[string]CancelLocationRequest{
		"bad IMSI":       {IMSI: "1"},
		"bad type":       {IMSI: testIMSI, Type: 6},
		"undefined flag": {IMSI: testIMSI, Flags: 1 << 2},
	} {
		if _, err := NewCancelLocationRequest(hssEnvelope, r); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseCancelLocationRequestErrors(t *testing.T) {
	base := must[*diameter.Message](t)(NewCancelLocationRequest(hssEnvelope, CancelLocationRequest{IMSI: testIMSI}))

	for name, tt := range map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"no type":       {without(base, AVPCancellationType, tgpp.VendorID), diameter.ResultMissingAVP},
		"unknown type":  {replace(base, vendorU32(AVPCancellationType, 9)), diameter.ResultInvalidAVPValue},
		"short type":    {replace(base, vendorOctets(AVPCancellationType, []byte{1})), diameter.ResultInvalidAVPLength},
		"long type":     {replace(base, vendorOctets(AVPCancellationType, make([]byte, 5))), diameter.ResultInvalidAVPLength},
		"twice flags":   {with(base, optionalU32(AVPCLRFlags, 0), optionalU32(AVPCLRFlags, 0)), diameter.ResultAVPOccursTooManyTimes},
		"no User-Name":  {without(base, diameter.AVPUserName, 0), diameter.ResultMissingAVP},
		"bad User-Name": {replace(base, diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, "x")), diameter.ResultInvalidAVPValue},
		"bad CLR-Flags": {with(base, vendorOctets(AVPCLRFlags, []byte{1})), diameter.ResultInvalidAVPLength},
		"unknown M AVP": {with(base, vendorU32(4242, 1)), diameter.ResultAVPUnsupported},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseCancelLocationRequest(tt.req); avpResult(t, err) != tt.want {
				t.Fatalf("err = %v, want %d", err, tt.want)
			}
		})
	}

	got, err := ParseCancelLocationRequest(with(base, optionalU32(AVPCLRFlags, uint32(CLRFlagReattachRequired|1<<9))))
	if err != nil || got.Flags != CLRFlagReattachRequired {
		t.Fatalf("undefined CLR-Flags bits not discarded: %+v, %v", got, err)
	}
}
