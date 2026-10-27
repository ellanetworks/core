// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func testIDA() InsertSubscriberData {
	return InsertSubscriberData{
		Features:                Features{List1: FeatureLocalTimeZoneRetrieval},
		IMSVoiceOverPSSupported: ptr(IMSVoiceOverPSSupported),
		LastUEActivityTime:      time.Date(2026, 9, 1, 8, 30, 0, 0, time.UTC),
		RATType:                 ptr(tgpp.RATEUTRAN),
		Flags:                   IDAFlagNetworkNodeAreaRestricted,
		LocalTimeZone:           &LocalTimeZone{TimeZone: "+02:00", DaylightSaving: DaylightSavingPlusOneHour},
	}
}

func TestInsertSubscriberDataRoundTrip(t *testing.T) {
	sub := testSubscription()

	for name, r := range map[string]InsertSubscriberDataRequest{
		"full": {
			IMSI: testIMSI, Features: Features{List2: FeatureResetIDs}, Subscription: sub,
			Flags: IDRFlagEPSLocationInformationRequest | IDRFlagLocalTimeZoneRequest, ResetIDs: []ResetID{testResetID},
		},
		"empty subscription data": {IMSI: testIMSI, Flags: IDRFlagEPSUserStateRequest},
		"modified APN without the default": {IMSI: testIMSI, Subscription: SubscriptionData{
			APNConfigurationProfile: &APNConfigurationProfile{
				ContextIdentifier: 1, Included: ModifiedAddedAPNConfigurationsIncluded,
				Configurations: []APNConfiguration{testAPNConfigurations()[1]},
			},
		}},
	} {
		t.Run(name, func(t *testing.T) {
			req := must[*diameter.Message](t)(NewInsertSubscriberDataRequest(hssEnvelope, r))
			requireNoVendorSpecificApplicationID(t, req)

			got, err := ParseInsertSubscriberDataRequest(roundTrip(t, req))
			if err != nil || !reflect.DeepEqual(got, r) {
				t.Fatalf("round trip =\n%+v\n%v\nwant\n%+v", got, err, r)
			}
		})
	}

	req := must[*diameter.Message](t)(NewInsertSubscriberDataRequest(hssEnvelope, InsertSubscriberDataRequest{IMSI: testIMSI}))

	for name, a := range map[string]InsertSubscriberData{"full": testIDA(), "empty": {}} {
		t.Run("IDA "+name, func(t *testing.T) {
			ans := must[*diameter.Message](t)(NewInsertSubscriberDataAnswer(req, mmeIdentity, a))

			got, err := ParseInsertSubscriberDataAnswer(roundTrip(t, ans))
			if err != nil || !reflect.DeepEqual(got, a) {
				t.Fatalf("IDA = %+v, %v", got, err)
			}
		})
	}
}

func TestInsertSubscriberDataValidation(t *testing.T) {
	if _, err := NewInsertSubscriberDataRequest(hssEnvelope, InsertSubscriberDataRequest{IMSI: testIMSI, Flags: 1 << 9}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("undefined IDR flag: %v", err)
	}

	if _, err := NewInsertSubscriberDataRequest(hssEnvelope, InsertSubscriberDataRequest{IMSI: "x"}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("bad IMSI: %v", err)
	}

	if _, err := NewInsertSubscriberDataRequest(hssEnvelope, InsertSubscriberDataRequest{IMSI: testIMSI, Subscription: SubscriptionData{MSISDN: "x"}}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("bad subscription: %v", err)
	}

	req := request(CommandInsertSubscriberData, hssEnvelope)

	for name, mutate := range map[string]func(*InsertSubscriberData){
		"bad IMS voice":   func(a *InsertSubscriberData) { a.IMSVoiceOverPSSupported = ptr(IMSVoiceOverPS(2)) },
		"WLAN RAT":        func(a *InsertSubscriberData) { a.RATType = ptr(tgpp.RATWLAN) },
		"undefined flag":  func(a *InsertSubscriberData) { a.Flags = 1 << 1 },
		"no time zone":    func(a *InsertSubscriberData) { a.LocalTimeZone.TimeZone = "" },
		"bad DST":         func(a *InsertSubscriberData) { a.LocalTimeZone.DaylightSaving = 3 },
		"sub-second time": func(a *InsertSubscriberData) { a.LastUEActivityTime = time.Unix(1_800_000_000, 5) },
	} {
		t.Run(name, func(t *testing.T) {
			a := testIDA()
			mutate(&a)

			if _, err := NewInsertSubscriberDataAnswer(req, mmeIdentity, a); !errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestParseInsertSubscriberDataRequestErrors(t *testing.T) {
	base := must[*diameter.Message](t)(NewInsertSubscriberDataRequest(hssEnvelope, InsertSubscriberDataRequest{IMSI: testIMSI, Subscription: testSubscription()}))
	apn := diameter.UTF8String(diameter.AVPServiceSelection, diameter.AVPFlagMandatory, 0, "internet")
	profile := func(avps ...diameter.AVP) diameter.AVP {
		return vendorGroup(AVPSubscriptionData, vendorGroup(AVPAPNConfigurationProfile, append([]diameter.AVP{
			vendorU32(AVPContextIdentifier, 1), vendorU32(AVPAllAPNConfigurationsIncludedIndicator, 0),
		}, avps...)...))
	}

	tests := map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"no Subscription-Data":  {without(base, AVPSubscriptionData, tgpp.VendorID), diameter.ResultMissingAVP},
		"no Destination-Host":   {without(base, diameter.AVPDestinationHost, 0), diameter.ResultMissingAVP},
		"bad Subscription-Data": {replace(base, vendorOctets(AVPSubscriptionData, []byte{1})), diameter.ResultInvalidAVPLength},
		"bad MSISDN":            {replace(base, vendorGroup(AVPSubscriptionData, vendorOctets(tgpp.AVPMSISDN, []byte{0xff}))), diameter.ResultInvalidAVPValue},
		"unknown M-bit AVP":     {replace(base, vendorGroup(AVPSubscriptionData, vendorU32(4242, 1))), diameter.ResultAVPUnsupported},
		"repeated status":       {replace(base, vendorGroup(AVPSubscriptionData, vendorU32(AVPSubscriberStatus, 0), vendorU32(AVPSubscriberStatus, 0))), diameter.ResultAVPOccursTooManyTimes},
		"unknown M-bit in APN": {replace(base, profile(vendorGroup(AVPAPNConfiguration,
			vendorU32(AVPContextIdentifier, 1), vendorU32(AVPPDNType, 0), apn, vendorU32(4242, 1)))), diameter.ResultAVPUnsupported},
		"unknown M-bit in ARP": {replace(base, profile(vendorGroup(AVPAPNConfiguration,
			vendorU32(AVPContextIdentifier, 1), vendorU32(AVPPDNType, 0), apn,
			vendorGroup(AVPEPSSubscribedQoSProfile, vendorU32(tgpp.AVPQoSClassIdentifier, 9),
				vendorGroup(tgpp.AVPAllocationRetentionPriority, vendorU32(tgpp.AVPPriorityLevel, 1), vendorU32(4242, 1)))))), diameter.ResultAVPUnsupported},
		"zero APN context": {replace(base, profile(vendorGroup(AVPAPNConfiguration,
			vendorU32(AVPContextIdentifier, 0), vendorU32(AVPPDNType, 0), apn))), diameter.ResultInvalidAVPValue},
		"zero default context": {replace(base, vendorGroup(AVPSubscriptionData, vendorGroup(AVPAPNConfigurationProfile,
			vendorU32(AVPContextIdentifier, 0), vendorU32(AVPAllAPNConfigurationsIncludedIndicator, 0),
			vendorGroup(AVPAPNConfiguration, vendorU32(AVPContextIdentifier, 1), vendorU32(AVPPDNType, 0), apn)))), diameter.ResultInvalidAVPValue},
		"repeated QoS": {replace(base, profile(vendorGroup(AVPAPNConfiguration,
			vendorU32(AVPContextIdentifier, 1), vendorU32(AVPPDNType, 0), apn,
			vendorGroup(AVPEPSSubscribedQoSProfile), vendorGroup(AVPEPSSubscribedQoSProfile)))), diameter.ResultAVPOccursTooManyTimes},
		"short IDR-Flags": {with(base, vendorOctets(AVPIDRFlags, []byte{1})), diameter.ResultInvalidAVPLength},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ParseInsertSubscriberDataRequest(tt.req)
			if code := avpResult(t, err); code != tt.want {
				t.Fatalf("Result-Code = %d, want %d (%v)", code, tt.want, err)
			}
		})
	}

	_, err := ParseInsertSubscriberDataRequest(tests["unknown M-bit in ARP"].req)

	path := []uint32{AVPSubscriptionData, AVPAPNConfigurationProfile, AVPAPNConfiguration, AVPEPSSubscribedQoSProfile, tgpp.AVPAllocationRetentionPriority, 4242}
	failed := avpError(t, err).AVPs

	for depth, code := range path {
		if len(failed) != 1 || failed[0].Code != code {
			t.Fatalf("depth %d: Failed-AVP = %+v, want code %d", depth, failed, code)
		}

		if depth < len(path)-1 {
			failed = must[[]diameter.AVP](t)(failed[0].Grouped())
		}
	}

	tolerant := replace(base, vendorGroup(AVPSubscriptionData,
		optionalU32(4242, 1), optionalU32(AVPSubscriberStatus, 9), vendorGroup(AVPTraceData), vendorGroup(AVPGPRSSubscriptionData)))

	got, err := ParseInsertSubscriberDataRequest(tolerant)
	if err != nil || *got.Subscription.SubscriberStatus != 9 {
		t.Fatalf("tolerated subscription = %+v, %v", got.Subscription, err)
	}

	got, err = ParseInsertSubscriberDataRequest(with(base, optionalU32(AVPIDRFlags, uint32(IDRFlagRemoveSMSRegistration|1<<20))))
	if err != nil || got.Flags != IDRFlagRemoveSMSRegistration {
		t.Fatalf("IDR-Flags = %v, %v", got.Flags, err)
	}
}

func TestParseInsertSubscriberDataAnswerErrors(t *testing.T) {
	req := request(CommandInsertSubscriberData, hssEnvelope)
	tz := func(avps ...diameter.AVP) diameter.AVP {
		return diameter.Grouped(AVPLocalTimeZone, 0, tgpp.VendorID, avps...)
	}
	zone := diameter.UTF8String(AVPTimeZone, 0, tgpp.VendorID, "+01:00")

	for name, avps := range map[string][]diameter.AVP{
		"short IMS voice": {diameter.OctetString(AVPIMSVoiceOverPSSessionsSupported, 0, tgpp.VendorID, []byte{1})},
		"short time":      {diameter.OctetString(AVPLastUEActivityTime, 0, tgpp.VendorID, []byte{1})},
		"short RAT":       {vendorOctets(tgpp.AVPRATType, []byte{1})},
		"short flags":     {vendorOctets(AVPIDAFlags, []byte{1})},
		"bad time zone":   {diameter.OctetString(AVPLocalTimeZone, 0, tgpp.VendorID, []byte{1})},
		"no time zone":    {tz(optionalU32(AVPDaylightSavingTime, 0))},
		"empty time zone": {tz(diameter.UTF8String(AVPTimeZone, 0, tgpp.VendorID, ""), optionalU32(AVPDaylightSavingTime, 0))},
		"no DST":          {tz(zone)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseInsertSubscriberDataAnswer(success(req, avps...)); !errors.Is(err, ErrMalformedAnswer) {
				t.Fatalf("err = %v", err)
			}
		})
	}

	got, err := ParseInsertSubscriberDataAnswer(success(req,
		optionalU32(AVPIMSVoiceOverPSSessionsSupported, 4), optionalU32(tgpp.AVPRATType, uint32(tgpp.RATWLAN)),
		optionalU32(AVPIDAFlags, 1<<3), tz(zone, optionalU32(AVPDaylightSavingTime, 5))))
	if err != nil || *got.IMSVoiceOverPSSupported != 4 || *got.RATType != tgpp.RATWLAN || got.Flags != 1<<3 || got.LocalTimeZone.DaylightSaving != 5 {
		t.Fatalf("lenient IDA = %+v, %v", got, err)
	}
}
