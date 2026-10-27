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

func TestHSSInitiatedRequestsNeedDestinationHost(t *testing.T) {
	env := hssEnvelope
	env.DestinationHost = ""

	builders := map[string]func() (*diameter.Message, error){
		"CLR": func() (*diameter.Message, error) {
			return NewCancelLocationRequest(env, CancelLocationRequest{IMSI: testIMSI})
		},
		"IDR": func() (*diameter.Message, error) {
			return NewInsertSubscriberDataRequest(env, InsertSubscriberDataRequest{IMSI: testIMSI})
		},
		"DSR": func() (*diameter.Message, error) {
			return NewDeleteSubscriberDataRequest(env, DeleteSubscriberDataRequest{IMSI: testIMSI})
		},
		"RSR": func() (*diameter.Message, error) { return NewResetRequest(env, ResetRequest{}) },
	}

	for name, build := range builders {
		if _, err := build(); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}

	userName := diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, testIMSI)

	parsers := map[uint32]func(*diameter.Message) error{
		CommandCancelLocation: func(m *diameter.Message) error { _, err := ParseCancelLocationRequest(m); return err },
		CommandInsertSubscriberData: func(m *diameter.Message) error {
			_, err := ParseInsertSubscriberDataRequest(m)
			return err
		},
		CommandDeleteSubscriberData: func(m *diameter.Message) error {
			_, err := ParseDeleteSubscriberDataRequest(m)
			return err
		},
		CommandReset: func(m *diameter.Message) error { _, err := ParseResetRequest(m); return err },
	}

	bodies := map[uint32][]diameter.AVP{
		CommandCancelLocation:       {userName, vendorU32(AVPCancellationType, 0)},
		CommandInsertSubscriberData: {userName, vendorGroup(AVPSubscriptionData)},
		CommandDeleteSubscriberData: {userName, vendorU32(AVPDSRFlags, 0)},
		CommandReset:                nil,
	}

	for command, parse := range parsers {
		req := request(command, env, bodies[command]...)
		if err := parse(with(req, diameter.UTF8String(diameter.AVPDestinationHost, diameter.AVPFlagMandatory, 0, mmeHost))); err != nil {
			t.Errorf("command %d with Destination-Host: %v", command, err)
		}

		var avpErr *diameter.AVPError
		if err := parse(req); !errors.As(err, &avpErr) || avpErr.ResultCode != diameter.ResultMissingAVP || avpErr.AVPs[0].Code != diameter.AVPDestinationHost {
			t.Errorf("command %d: err = %v", command, err)
		}
	}
}

func TestCancelLocationRoundTrip(t *testing.T) {
	for name, r := range map[string]CancelLocationRequest{
		"withdrawal": {
			IMSI: testIMSI, Type: CancellationSubscriptionWithdrawal, Flags: CLRFlagS6aS6dIndicator | CLRFlagReattachRequired,
			Features: Features{List2: FeatureResetIDs},
		},
		"MME update": {IMSI: testIMSI, Type: CancellationMMEUpdateProcedure},
	} {
		t.Run(name, func(t *testing.T) {
			req := must[*diameter.Message](t)(NewCancelLocationRequest(hssEnvelope, r))
			requireNoVendorSpecificApplicationID(t, req)

			if dest, _ := req.Find(diameter.AVPDestinationHost, 0); dest.UTF8String() != mmeHost {
				t.Fatalf("Destination-Host = %q", dest.UTF8String())
			}

			got, err := ParseCancelLocationRequest(roundTrip(t, req))
			if err != nil || !reflect.DeepEqual(got, r) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}

			ans := roundTrip(t, NewCancelLocationAnswer(req, mmeIdentity, Features{List2: FeatureResetIDs}))
			requireNoVendorSpecificApplicationID(t, ans)

			if f, err := ParseCancelLocationAnswer(ans); err != nil || f.List2 != FeatureResetIDs {
				t.Fatalf("answer = %+v, %v", f, err)
			}
		})
	}

	if _, err := NewCancelLocationRequest(hssEnvelope, CancelLocationRequest{IMSI: testIMSI, Type: 6}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("bad type: err = %v", err)
	}

	base := must[*diameter.Message](t)(NewCancelLocationRequest(hssEnvelope, CancelLocationRequest{IMSI: testIMSI}))

	for name, tt := range map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"no type":       {without(base, AVPCancellationType, tgpp.VendorID), diameter.ResultMissingAVP},
		"unknown type":  {replace(base, vendorU32(AVPCancellationType, 9)), diameter.ResultInvalidAVPValue},
		"no User-Name":  {without(base, diameter.AVPUserName, 0), diameter.ResultMissingAVP},
		"short CLR-Flg": {with(base, diameter.OctetString(AVPCLRFlags, 0, tgpp.VendorID, []byte{1})), diameter.ResultInvalidAVPLength},
	} {
		if _, err := ParseCancelLocationRequest(tt.req); avpResult(t, err) != tt.want {
			t.Errorf("%s: err = %v", name, err)
		}
	}

	got, err := ParseCancelLocationRequest(with(base, diameter.Unsigned32(AVPCLRFlags, 0, tgpp.VendorID, 0xfffffffe)))
	if err != nil || got.Flags != CLRFlagReattachRequired {
		t.Fatalf("undefined CLR bits = %+v, %v", got, err)
	}
}

func TestInsertSubscriberDataRoundTrip(t *testing.T) {
	r := InsertSubscriberDataRequest{
		IMSI:         testIMSI,
		Features:     Features{List1: FeatureODBAllAPN},
		Subscription: testSubscription(),
		Flags:        IDRFlagEPSUserStateRequest | IDRFlagEPSLocationInformationRequest,
		ResetIDs:     [][]byte{[]byte(testResetID), {0xff}},
	}

	req := must[*diameter.Message](t)(NewInsertSubscriberDataRequest(hssEnvelope, r))
	requireNoVendorSpecificApplicationID(t, req)

	got, err := ParseInsertSubscriberDataRequest(roundTrip(t, req))
	if err != nil || !reflect.DeepEqual(got, r) {
		t.Fatalf("round trip =\n%+v\n%v\nwant\n%+v", got, err, r)
	}

	empty := InsertSubscriberDataRequest{IMSI: testIMSI, Subscription: SubscriptionData{}}
	if got, err := ParseInsertSubscriberDataRequest(roundTrip(t, must[*diameter.Message](t)(NewInsertSubscriberDataRequest(hssEnvelope, empty)))); err != nil || !reflect.DeepEqual(got, empty) {
		t.Fatalf("empty Subscription-Data = %+v, %v", got, err)
	}

	a := InsertSubscriberData{
		Features:                Features{List1: FeatureODBAllAPN},
		IMSVoiceOverPSSupported: ptr(IMSVoiceOverPSNotSupported),
		LastUEActivityTime:      time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC),
		RATType:                 ptr(tgpp.RATEUTRAN),
		Flags:                   IDAFlagNetworkNodeAreaRestricted,
		LocalTimeZone:           &LocalTimeZone{TimeZone: "+8", DaylightSaving: DaylightSavingPlusOneHour},
	}

	ans := must[*diameter.Message](t)(NewInsertSubscriberDataAnswer(req, mmeIdentity, a))
	requireNoVendorSpecificApplicationID(t, ans)

	gotA, err := ParseInsertSubscriberDataAnswer(roundTrip(t, ans))
	if err != nil || !reflect.DeepEqual(gotA, a) {
		t.Fatalf("answer = %+v, %v", gotA, err)
	}

	if _, err := NewInsertSubscriberDataAnswer(req, mmeIdentity, InsertSubscriberData{LocalTimeZone: &LocalTimeZone{}}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("empty time zone: err = %v", err)
	}

	if _, err := NewInsertSubscriberDataRequest(hssEnvelope, InsertSubscriberDataRequest{IMSI: testIMSI, Flags: 1 << 9}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("undefined IDR flag: err = %v", err)
	}
}

func TestParseInsertSubscriberDataRequestErrors(t *testing.T) {
	base := must[*diameter.Message](t)(NewInsertSubscriberDataRequest(hssEnvelope, InsertSubscriberDataRequest{IMSI: testIMSI}))

	badConfig := vendorGroup(AVPAPNConfiguration,
		vendorU32(AVPContextIdentifier, 0),
		vendorU32(AVPPDNType, 0),
		diameter.UTF8String(AVPServiceSelection, diameter.AVPFlagMandatory, 0, "internet"))

	profile := func(configs ...diameter.AVP) diameter.AVP {
		return vendorGroup(AVPSubscriptionData, vendorGroup(AVPAPNConfigurationProfile,
			append([]diameter.AVP{vendorU32(AVPContextIdentifier, 1), vendorU32(AVPAllAPNConfigurationsIncludedIndicator, 0)}, configs...)...))
	}

	config := func(avps ...diameter.AVP) diameter.AVP { return vendorGroup(AVPAPNConfiguration, avps...) }
	ctx := vendorU32(AVPContextIdentifier, 1)
	pdn := vendorU32(AVPPDNType, 0)
	apn := diameter.UTF8String(AVPServiceSelection, diameter.AVPFlagMandatory, 0, "internet")

	for name, tt := range map[string]struct {
		sub      diameter.AVP
		want     uint32
		failedAt uint32
	}{
		"zero context":     {profile(badConfig), diameter.ResultInvalidAVPValue, AVPContextIdentifier},
		"no APN":           {profile(config(ctx, pdn)), diameter.ResultMissingAVP, AVPServiceSelection},
		"no PDN type":      {profile(config(ctx, apn)), diameter.ResultMissingAVP, AVPPDNType},
		"bad PDN type":     {profile(config(ctx, apn, vendorU32(AVPPDNType, 9))), diameter.ResultInvalidAVPValue, AVPPDNType},
		"duplicate config": {profile(config(ctx, pdn, apn), config(ctx, pdn, apn)), diameter.ResultInvalidAVPValue, AVPAPNConfigurationProfile},
		"bad dynamic":      {profile(config(ctx, pdn, apn, vendorU32(AVPVPLMNDynamicAddressAllowed, 2))), diameter.ResultInvalidAVPValue, AVPVPLMNDynamicAddressAllowed},
		"bad indicator": {vendorGroup(AVPSubscriptionData, vendorGroup(AVPAPNConfigurationProfile,
			vendorU32(AVPContextIdentifier, 1), vendorU32(AVPAllAPNConfigurationsIncludedIndicator, 3), config(ctx, pdn, apn))), diameter.ResultInvalidAVPValue, AVPAllAPNConfigurationsIncludedIndicator},
		"three served IPs": {profile(config(ctx, pdn, apn,
			diameter.Address(AVPServedPartyIPAddress, diameter.AVPFlagMandatory, tgpp.VendorID, mustAddr("10.0.0.1")),
			diameter.Address(AVPServedPartyIPAddress, diameter.AVPFlagMandatory, tgpp.VendorID, mustAddr("10.0.0.2")),
			diameter.Address(AVPServedPartyIPAddress, diameter.AVPFlagMandatory, tgpp.VendorID, mustAddr("10.0.0.3")))), diameter.ResultAVPOccursTooManyTimes, AVPServedPartyIPAddress},
		"no ARP": {
			profile(config(ctx, pdn, apn, vendorGroup(AVPEPSSubscribedQoSProfile, vendorU32(tgpp.AVPQoSClassIdentifier, 9)))),
			diameter.ResultMissingAVP, tgpp.AVPAllocationRetentionPriority,
		},
		"empty MIP6": {
			profile(config(ctx, pdn, apn, diameter.Grouped(AVPMIP6AgentInfo, diameter.AVPFlagMandatory, 0))),
			diameter.ResultInvalidAVPValue, AVPMIP6AgentInfo,
		},
		"bad status": {vendorGroup(AVPSubscriptionData, vendorU32(AVPSubscriberStatus, 4)), diameter.ResultInvalidAVPValue, AVPSubscriberStatus},
		"bad AMBR":   {vendorGroup(AVPSubscriptionData, vendorGroup(AVPAMBR, vendorU32(tgpp.AVPMaxRequestedBandwidthUL, 1))), diameter.ResultMissingAVP, tgpp.AVPMaxRequestedBandwidthDL},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseInsertSubscriberDataRequest(replace(base, tt.sub))

			var avpErr *diameter.AVPError
			if !errors.As(err, &avpErr) || avpErr.ResultCode != tt.want || avpErr.AVPs[0].Code != tt.failedAt {
				t.Fatalf("err = %v", err)
			}
		})
	}

	if _, err := ParseInsertSubscriberDataRequest(without(base, AVPSubscriptionData, tgpp.VendorID)); avpResult(t, err) != diameter.ResultMissingAVP {
		t.Fatalf("no Subscription-Data: err = %v", err)
	}
}

func TestDeleteSubscriberDataRoundTrip(t *testing.T) {
	r := DeleteSubscriberDataRequest{
		IMSI:               testIMSI,
		Flags:              DSRFlagPDNSubscriptionContextsWithdrawal | DSRFlagDeleteMonitoringEvents,
		SCEFID:             "scef.example.org",
		ContextIdentifiers: []uint32{2, 3},
		TraceReference:     []byte{1, 2, 3, 4, 5, 6},
		TSCodes:            [][]byte{{0x21}},
		SSCodes:            [][]byte{{0x92}, {0x93}},
		Features:           Features{List2: FeatureMonitoringEvent},
	}

	req := must[*diameter.Message](t)(NewDeleteSubscriberDataRequest(hssEnvelope, r))
	requireNoVendorSpecificApplicationID(t, req)

	got, err := ParseDeleteSubscriberDataRequest(roundTrip(t, req))
	if err != nil || !reflect.DeepEqual(got, r) {
		t.Fatalf("round trip = %+v, %v", got, err)
	}

	d := DeleteSubscriberData{Flags: DSAFlagNetworkNodeAreaRestricted}

	ans := must[*diameter.Message](t)(NewDeleteSubscriberDataAnswer(req, mmeIdentity, d))
	if gotD, err := ParseDeleteSubscriberDataAnswer(roundTrip(t, ans)); err != nil || gotD != d {
		t.Fatalf("answer = %+v, %v", gotD, err)
	}

	for name, bad := range map[string]DeleteSubscriberDataRequest{
		"no SCEF-ID":   {IMSI: testIMSI, Flags: DSRFlagDeleteMonitoringEvents},
		"zero context": {IMSI: testIMSI, ContextIdentifiers: []uint32{0}},
	} {
		if _, err := NewDeleteSubscriberDataRequest(hssEnvelope, bad); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}

	for name, tt := range map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"no flags":       {without(req, AVPDSRFlags, tgpp.VendorID), diameter.ResultMissingAVP},
		"no SCEF-ID":     {without(req, AVPSCEFID, tgpp.VendorID), diameter.ResultMissingAVP},
		"zero context":   {with(req, vendorU32(AVPContextIdentifier, 0)), diameter.ResultInvalidAVPValue},
		"short flags":    {replace(req, vendorOctets(AVPDSRFlags, []byte{0})), diameter.ResultInvalidAVPLength},
		"twice SCEF-ID ": {with(req, diameter.UTF8String(AVPSCEFID, diameter.AVPFlagMandatory, tgpp.VendorID, "x")), diameter.ResultAVPOccursTooManyTimes},
	} {
		if _, err := ParseDeleteSubscriberDataRequest(tt.req); avpResult(t, err) != tt.want {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestResetRoundTrip(t *testing.T) {
	sub := SubscriptionData{OperatorDeterminedBarring: ptr(ODBAllPacketOrientedServices)}

	r := ResetRequest{
		Features:     Features{List2: FeatureResetIDs},
		UserIDs:      []string{"00101", "001020"},
		ResetIDs:     [][]byte{[]byte(testResetID)},
		Subscription: &sub,
	}

	req := must[*diameter.Message](t)(NewResetRequest(hssEnvelope, r))
	requireNoVendorSpecificApplicationID(t, req)

	if _, ok := req.Find(diameter.AVPUserName, 0); ok {
		t.Fatal("RSR carries User-Name")
	}

	if userID, _ := req.Find(AVPUserID, tgpp.VendorID); userID.Flags != diameter.AVPFlagVendor {
		t.Fatalf("User-Id flags = %#x", userID.Flags)
	}

	got, err := ParseResetRequest(roundTrip(t, req))
	if err != nil || !reflect.DeepEqual(got, r) {
		t.Fatalf("round trip = %+v, %v", got, err)
	}

	if !got.Matches(testIMSI) || got.Matches("208930000000001") || !(ResetRequest{}).Matches(testIMSI) {
		t.Fatal("Matches")
	}

	if f, err := ParseResetAnswer(roundTrip(t, NewResetAnswer(req, mmeIdentity, Features{}))); err != nil || f != (Features{}) {
		t.Fatalf("answer = %+v, %v", f, err)
	}

	if _, err := NewResetRequest(hssEnvelope, ResetRequest{UserIDs: []string{"00a"}}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("bad User-Id: err = %v", err)
	}

	if _, err := ParseResetRequest(with(req, diameter.UTF8String(AVPUserID, 0, tgpp.VendorID, ""))); avpResult(t, err) != diameter.ResultInvalidAVPValue {
		t.Fatalf("empty User-Id: err = %v", err)
	}
}

func TestPurgeUERoundTrip(t *testing.T) {
	r := PurgeUERequest{IMSI: testIMSI, Flags: PURFlagUEPurgedInMME, Features: Features{List1: FeaturePartialPurge}}

	req := must[*diameter.Message](t)(NewPurgeUERequest(mmeEnvelope, r))
	requireNoVendorSpecificApplicationID(t, req)

	got, err := ParsePurgeUERequest(roundTrip(t, req))
	if err != nil || !reflect.DeepEqual(got, r) {
		t.Fatalf("round trip = %+v, %v", got, err)
	}

	p := PurgeUE{Flags: PUAFlagFreezeMTMSI}

	ans := must[*diameter.Message](t)(NewPurgeUEAnswer(req, hssIdentity, p))
	if gotP, err := ParsePurgeUEAnswer(roundTrip(t, ans)); err != nil || gotP != p {
		t.Fatalf("answer = %+v, %v", gotP, err)
	}

	if _, err := NewPurgeUERequest(mmeEnvelope, PurgeUERequest{IMSI: testIMSI, Flags: 1 << 2}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("undefined PUR flag: err = %v", err)
	}

	unknown := tgpp.NewExperimentalAnswer(req, hssIdentity, tgpp.ResultErrorUserUnknown)
	if _, err := ParsePurgeUEAnswer(unknown); !tgpp.IsExperimental(err, tgpp.ResultErrorUserUnknown) {
		t.Fatalf("user unknown: err = %v", err)
	}
}

func TestNotifyRoundTrip(t *testing.T) {
	r := NotifyRequest{
		IMSI:     testIMSI,
		Features: Features{List1: FeatureUEReachabilityNotification},
		Terminal: &TerminalInformation{IMEI: testIMEI, SoftwareVersion: "12"},
		PDNGateway: &MIP6AgentInfo{
			HomeAgentHost: "pgw.epc.mnc093.mcc208.3gppnetwork.org", HomeAgentRealm: visitedRealm,
		},
		VisitedNetworkIdentifier:  "mnc093.mcc208.3gppnetwork.org",
		ContextIdentifier:         1,
		ServiceSelection:          "internet",
		AlertReason:               ptr(AlertReasonUEPresent),
		UESRVCCCapability:         ptr(UESRVCCSupported),
		Flags:                     NORFlagHomogeneousSupportOfIMSVoiceOverPS | NORFlagS6aS6dIndicator,
		HomogeneousIMSVoiceOverPS: ptr(IMSVoiceOverPSNotSupported),
		MaximumUEAvailabilityTime: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	req := must[*diameter.Message](t)(NewNotifyRequest(mmeEnvelope, r))
	requireNoVendorSpecificApplicationID(t, req)

	got, err := ParseNotifyRequest(roundTrip(t, req))
	if err != nil || !reflect.DeepEqual(got, r) {
		t.Fatalf("round trip = %+v, %v", got, err)
	}

	if f, err := ParseNotifyAnswer(roundTrip(t, NewNotifyAnswer(req, hssIdentity, r.Features))); err != nil || f != r.Features {
		t.Fatalf("answer = %+v, %v", f, err)
	}

	if _, err := NewNotifyRequest(mmeEnvelope, NotifyRequest{IMSI: testIMSI, Flags: 1 << 4}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("reserved NOR bit: err = %v", err)
	}

	parsed, err := ParseNotifyRequest(replace(req, vendorU32(AVPNORFlags, 1<<4|uint32(NORFlagUEReachableFromMME))))
	if err != nil || parsed.Flags != NORFlagUEReachableFromMME {
		t.Fatalf("reserved bit discarded = %+v, %v", parsed.Flags, err)
	}

	if _, err := ParseNotifyRequest(replace(req, vendorU32(AVPAlertReason, 2))); avpResult(t, err) != diameter.ResultInvalidAVPValue {
		t.Fatalf("bad Alert-Reason: err = %v", err)
	}
}
