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

func TestAPNConfigurationProfileLookups(t *testing.T) {
	p := *testSubscription().APNConfigurationProfile

	for apn, want := range map[string]uint32{"internet": 1, "IMS": 2, "Sensors": 4, "*": 3, "corp": 3, "anything.else": 3} {
		if c, ok := p.Resolve(apn); !ok || c.ContextIdentifier != want {
			t.Errorf("Resolve(%q) = %d, %v; want %d", apn, c.ContextIdentifier, ok, want)
		}
	}

	if _, ok := p.Lookup("corp"); ok {
		t.Fatal("Lookup fell back to the wildcard")
	}

	if c, ok := p.Configuration(4); !ok || c.IPBased() {
		t.Fatalf("Configuration(4) = %+v, %v", c, ok)
	}

	if _, ok := p.Configuration(9); ok {
		t.Fatal("Configuration(9) found")
	}

	if c, ok := p.Default(); !ok || c.ContextIdentifier != 1 {
		t.Fatalf("Default = %+v, %v", c, ok)
	}

	p.ContextIdentifier = 3
	if _, ok := p.Default(); ok {
		t.Fatal("Default returned the wildcard APN")
	}

	p.Configurations = p.Configurations[:2]
	if _, ok := p.Resolve("corp"); ok {
		t.Fatal("Resolve without a wildcard")
	}

	for _, c := range []APNConfiguration{{EthernetPDNType: true}, {NonIPPDNType: true}} {
		if c.IPBased() {
			t.Errorf("%+v is IP-based", c)
		}
	}
}

func TestAPNConfigurationProfileMerge(t *testing.T) {
	stored := *testSubscription().APNConfigurationProfile
	configs := testAPNConfigurations()

	replacement := APNConfigurationProfile{
		ContextIdentifier: 2, Included: AllAPNConfigurationsIncluded, Configurations: []APNConfiguration{configs[1]},
	}

	if got := stored.Merge(replacement); !reflect.DeepEqual(got, replacement) {
		t.Fatalf("ALL merge = %+v", got)
	}

	updated := configs[1]
	updated.ServiceSelection = "ims2"
	added := APNConfiguration{ContextIdentifier: 9, PDNType: PDNTypeIPv6, ServiceSelection: "new"}

	got := stored.Merge(APNConfigurationProfile{
		ContextIdentifier: 1, ThirdContextIdentifier: 9, Included: ModifiedAddedAPNConfigurationsIncluded,
		Configurations: []APNConfiguration{updated, added},
	})

	want := stored
	want.ThirdContextIdentifier = 9
	want.Configurations = []APNConfiguration{configs[0], updated, configs[2], configs[3], added}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MODIFIED_ADDED merge =\n%+v\nwant\n%+v", got, want)
	}

	if !reflect.DeepEqual(stored.Configurations, configs) {
		t.Fatal("Merge modified the stored profile")
	}
}

func TestSubscriptionDataMerge(t *testing.T) {
	stored := testSubscription()

	if got := stored.Merge(SubscriptionData{}); !reflect.DeepEqual(got, stored) {
		t.Fatal("empty Subscription-Data changed the stored data")
	}

	update := SubscriptionData{
		SubscriberStatus:                ptr(SubscriberStatusOperatorDeterminedBarring),
		MSISDN:                          "15559999999",
		AMSISDN:                         "15559999998",
		STNSR:                           "15559999997",
		NetworkAccessMode:               ptr(NetworkAccessPacketAndCircuit),
		OperatorDeterminedBarring:       ptr(ODBAllPacketOrientedServices),
		HPLMNODB:                        ptr(HPLMNODBType1),
		RegionalSubscriptionZoneCodes:   [][]byte{{9}},
		AccessRestrictionData:           ptr(ARDNRIn5GSNotAllowed),
		APNOIReplacement:                "other.gprs",
		ChargingCharacteristics:         "0400",
		AMBR:                            &AMBR{UL: 5, DL: 6},
		RATFrequencySelectionPriorityID: 3,
		SubscribedPeriodicRAUTAUTimer:   testRAUTimer * 2,
		UEUsageType:                     ptr(UEUsageType(200)),
		APNConfigurationProfile: &APNConfigurationProfile{
			ContextIdentifier: 1, Included: ModifiedAddedAPNConfigurationsIncluded,
			Configurations: []APNConfiguration{{ContextIdentifier: 7, PDNType: PDNTypeIPv4, ServiceSelection: "seven"}},
		},
	}

	got := stored.Merge(update)

	want := update
	want.RoamingRestrictedDueToUnsupportedFeature = true
	want.SubscribedVSRVCC = true
	want.APNConfigurationProfile = ptr(stored.APNConfigurationProfile.Merge(*update.APNConfigurationProfile))

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merge =\n%+v\nwant\n%+v", got, want)
	}

	fresh := SubscriptionData{}.Merge(SubscriptionData{APNConfigurationProfile: update.APNConfigurationProfile})
	if len(fresh.APNConfigurationProfile.Configurations) != 1 {
		t.Fatalf("merge into nothing = %+v", fresh.APNConfigurationProfile)
	}
}

func TestSubscriptionDataWithdraw(t *testing.T) {
	stored := testSubscription()

	got, err := stored.Withdraw(SubscriptionDataDeletion{
		Flags: DSRFlagRegionalSubscriptionWithdrawal | DSRFlagSubscribedChargingCharacteristics | DSRFlagSTNSR |
			DSRFlagRoamingRestrictedDueToUnsupportedFeature | DSRFlagAPNOIReplacement | DSRFlagSubscribedPeriodicRAUTAUTimerWithdrawal |
			DSRFlagSubscribedVSRVCCWithdrawal | DSRFlagAMSISDNWithdrawal | DSRFlagMSISDNWithdrawal | DSRFlagUEUsageTypeWithdrawal |
			DSRFlagPDNSubscriptionContextsWithdrawal | DSRFlagTraceDataWithdrawal,
		ContextIdentifiers: []uint32{2, 3},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := stored
	want.RegionalSubscriptionZoneCodes = nil
	want.ChargingCharacteristics, want.STNSR, want.APNOIReplacement, want.AMSISDN, want.MSISDN = "", "", "", "", ""
	want.RoamingRestrictedDueToUnsupportedFeature, want.SubscribedVSRVCC = false, false
	want.SubscribedPeriodicRAUTAUTimer = 0
	want.UEUsageType = nil

	profile := *stored.APNConfigurationProfile
	profile.Configurations = []APNConfiguration{profile.Configurations[0], profile.Configurations[3]}
	want.APNConfigurationProfile = &profile

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("withdraw =\n%+v\nwant\n%+v", got, want)
	}

	if len(stored.APNConfigurationProfile.Configurations) != 4 {
		t.Fatal("Withdraw modified the stored profile")
	}

	for name, d := range map[string]SubscriptionDataDeletion{
		"default context":    {Flags: DSRFlagPDNSubscriptionContextsWithdrawal, ContextIdentifiers: []uint32{1}},
		"additional default": {Flags: DSRFlagPDNSubscriptionContextsWithdrawal, ContextIdentifiers: []uint32{2, 4}},
		"complete profile":   {Flags: DSRFlagCompleteAPNConfigurationProfileWithdrawal},
	} {
		_, err := stored.Withdraw(d)

		var re *ResultError
		if !errors.As(err, &re) || re.Code != diameter.ResultUnableToComply {
			t.Errorf("%s: err = %v", name, err)
		}
	}

	if got, err := (SubscriptionData{}).Withdraw(SubscriptionDataDeletion{Flags: DSRFlagPDNSubscriptionContextsWithdrawal, ContextIdentifiers: []uint32{5}}); err != nil || got.APNConfigurationProfile != nil {
		t.Fatalf("withdraw from nothing = %+v, %v", got, err)
	}
}

func TestSubscriptionDataRoundTrip(t *testing.T) {
	sub := testSubscription()

	a := must[diameter.AVP](t)(sub.avp())

	for name, d := range map[string]decoder{"strict": requestDecoder, "lenient": answerDecoder} {
		got, err := d.subscriptionData(a)
		if err != nil || !reflect.DeepEqual(*got, sub) {
			t.Fatalf("%s: round trip =\n%+v\n%v\nwant\n%+v", name, got, err, sub)
		}
	}

	inner := must[[]diameter.AVP](t)(a.Grouped())

	odb, _ := diameter.Find(inner, AVPOperatorDeterminedBarring, tgpp.VendorID)
	if v := must[uint32](t)(odb.Unsigned32()); v != uint32(ODBRoamerAccessHPLMNAP) {
		t.Fatalf("ODB = %#x", v)
	}

	ard := diameter.Unsigned32(AVPAccessRestrictionData, diameter.AVPFlagMandatory, tgpp.VendorID, uint32(ARDUTRANNotAllowed)|1<<31)

	got := must[*SubscriptionData](t)(answerDecoder.subscriptionData(vendorGroup(AVPSubscriptionData, ard)))
	if *got.AccessRestrictionData != ARDUTRANNotAllowed|1<<31 {
		t.Fatalf("ARD with bit 31 = %v", *got.AccessRestrictionData)
	}
}
