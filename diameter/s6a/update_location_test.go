// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"errors"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func testULR() UpdateLocationRequest {
	return UpdateLocationRequest{
		IMSI:                      testIMSI,
		Features:                  Features{List1: FeatureODBAllAPN | FeatureODBHPLMNAPN | FeatureODBVPLMNAPN},
		Terminal:                  &TerminalInformation{IMEI: testIMEI, SoftwareVersion: testSoftware},
		RATType:                   tgpp.RATEUTRAN,
		Flags:                     ULRFlagS6aS6dIndicator | ULRFlagInitialAttachIndicator,
		UESRVCCCapability:         ptr(UESRVCCNotSupported),
		VisitedPLMN:               visitedPLMN,
		HomogeneousIMSVoiceOverPS: ptr(IMSVoiceOverPSNotSupported),
		EquivalentPLMNs:           []tgpp.PLMN{{MCC: "208", MNC: "01"}, {MCC: "310", MNC: "410"}},
		MMENumberForMTSMS:         "15550000001",
		SMSRegisterRequest:        ptr(SMSRegistrationNotPreferred),
		SGsMMEIdentity:            mmeHost,
		CoupledNodeDiameterID:     "sgsn.example.org",
	}
}

func TestUpdateLocationRequestRoundTrip(t *testing.T) {
	for name, r := range map[string]UpdateLocationRequest{
		"full":    testULR(),
		"minimal": {IMSI: testIMSI, RATType: tgpp.RATEUTRAN, VisitedPLMN: visitedPLMN},
	} {
		t.Run(name, func(t *testing.T) {
			req := must[*diameter.Message](t)(NewUpdateLocationRequest(mmeEnvelope, r))
			requireNoVendorSpecificApplicationID(t, req)

			if req.CommandCode != CommandUpdateLocation || !req.IsRequest() {
				t.Fatalf("header = %+v", req)
			}

			if _, ok := req.Find(diameter.AVPDestinationHost, 0); ok {
				t.Fatal("first ULR carries Destination-Host")
			}

			got, err := ParseUpdateLocationRequest(roundTrip(t, req))
			if err != nil || !reflect.DeepEqual(got, r) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}
		})
	}
}

func TestUpdateLocationRequestValidation(t *testing.T) {
	for name, mutate := range map[string]func(*UpdateLocationRequest){
		"bad IMSI":         func(r *UpdateLocationRequest) { r.IMSI = "00101" },
		"no visited PLMN":  func(r *UpdateLocationRequest) { r.VisitedPLMN = tgpp.PLMN{} },
		"undefined flag":   func(r *UpdateLocationRequest) { r.Flags |= 1 << 20 },
		"short IMEI":       func(r *UpdateLocationRequest) { r.Terminal.IMEI = "1234" },
		"bad SVN":          func(r *UpdateLocationRequest) { r.Terminal.SoftwareVersion = "7" },
		"empty terminal":   func(r *UpdateLocationRequest) { r.Terminal = &TerminalInformation{} },
		"bad SRVCC":        func(r *UpdateLocationRequest) { r.UESRVCCCapability = ptr(UESRVCCCapability(2)) },
		"bad SMS register": func(r *UpdateLocationRequest) { r.SMSRegisterRequest = ptr(SMSRegisterRequest(3)) },
		"bad MME number":   func(r *UpdateLocationRequest) { r.MMENumberForMTSMS = "+1555" },
		"bad equivalent":   func(r *UpdateLocationRequest) { r.EquivalentPLMNs = []tgpp.PLMN{{MCC: "20", MNC: "01"}} },
		"bad homogeneous":  func(r *UpdateLocationRequest) { r.HomogeneousIMSVoiceOverPS = ptr(IMSVoiceOverPS(5)) },
		"bad SGSN number":  func(r *UpdateLocationRequest) { r.SGSNNumber = "x" },
	} {
		t.Run(name, func(t *testing.T) {
			r := testULR()
			mutate(&r)

			if _, err := NewUpdateLocationRequest(mmeEnvelope, r); !errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("err = %v", err)
			}
		})
	}

	env := mmeEnvelope
	env.Origin = diameter.Identity{}

	if _, err := NewUpdateLocationRequest(env, testULR()); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("no origin: err = %v", err)
	}
}

func TestParseUpdateLocationRequestErrors(t *testing.T) {
	base := must[*diameter.Message](t)(NewUpdateLocationRequest(mmeEnvelope, testULR()))

	tests := map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"no User-Name":        {without(base, diameter.AVPUserName, 0), diameter.ResultMissingAVP},
		"no Visited-PLMN-Id":  {without(base, AVPVisitedPLMNID, tgpp.VendorID), diameter.ResultMissingAVP},
		"no RAT-Type":         {without(base, tgpp.AVPRATType, tgpp.VendorID), diameter.ResultMissingAVP},
		"no ULR-Flags":        {without(base, AVPULRFlags, tgpp.VendorID), diameter.ResultMissingAVP},
		"no Session-Id":       {without(base, diameter.AVPSessionID, 0), diameter.ResultMissingAVP},
		"no Auth-Session":     {without(base, diameter.AVPAuthSessionState, 0), diameter.ResultMissingAVP},
		"bad IMSI":            {replace(base, diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, "abc")), diameter.ResultInvalidAVPValue},
		"short PLMN":          {replace(base, vendorOctets(AVPVisitedPLMNID, []byte{0x02, 0xf8})), diameter.ResultInvalidAVPValue},
		"bad PLMN digit":      {replace(base, vendorOctets(AVPVisitedPLMNID, []byte{0x0a, 0xf8, 0x39})), diameter.ResultInvalidAVPValue},
		"short ULR-Flags":     {replace(base, vendorOctets(AVPULRFlags, []byte{1})), diameter.ResultInvalidAVPLength},
		"twice ULR-Flags":     {with(base, vendorU32(AVPULRFlags, 0)), diameter.ResultAVPOccursTooManyTimes},
		"unknown M-bit AVP":   {with(base, vendorU32(9999, 0)), diameter.ResultAVPUnsupported},
		"bad SRVCC":           {replace(base, diameter.Unsigned32(AVPUESRVCCCapability, 0, tgpp.VendorID, 7)), diameter.ResultInvalidAVPValue},
		"bad IMEI":            {replace(base, vendorGroup(AVPTerminalInformation, vendorOctets(AVPIMEI, []byte("12ab")))), diameter.ResultInvalidAVPValue},
		"bad terminal group":  {replace(base, vendorOctets(AVPTerminalInformation, []byte{0, 0, 5})), diameter.ResultInvalidAVPLength},
		"empty equivalent":    {replace(base, diameter.Grouped(AVPEquivalentPLMNList, 0, tgpp.VendorID)), diameter.ResultMissingAVP},
		"bad MME SMS number":  {replace(base, diameter.OctetString(tgpp.AVPMMENumberForMTSMS, 0, tgpp.VendorID, []byte{0xff})), diameter.ResultInvalidAVPValue},
		"empty SGs MME ident": {replace(base, diameter.OctetString(AVPSGsMMEIdentity, 0, tgpp.VendorID, nil)), diameter.ResultInvalidAVPValue},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ParseUpdateLocationRequest(tt.req)
			if code := avpResult(t, err); code != tt.want {
				t.Fatalf("Result-Code = %d, want %d (%v)", code, tt.want, err)
			}
		})
	}
}

func TestParseUpdateLocationRequestTolerance(t *testing.T) {
	base := must[*diameter.Message](t)(NewUpdateLocationRequest(mmeEnvelope, testULR()))

	vsai := diameter.Grouped(diameter.AVPVendorSpecificApplicationID, diameter.AVPFlagMandatory, 0,
		diameter.Unsigned32(diameter.AVPVendorID, diameter.AVPFlagMandatory, 0, tgpp.VendorID),
		diameter.Unsigned32(diameter.AVPAuthApplicationID, diameter.AVPFlagMandatory, 0, ApplicationID))

	req := with(base, vsai, diameter.Unsigned32(9999, 0, tgpp.VendorID, 1), diameter.Unsigned32(AVPSFProvisionalIndication, 0, tgpp.VendorID, 0))
	req = replace(req, vendorU32(AVPULRFlags, uint32(ULRFlagS6aS6dIndicator|1<<30)))
	req = replace(req, vendorGroup(AVPTerminalInformation, vendorOctets(AVPIMEI, []byte(testIMEI)), vendorU32(4242, 1)))

	got, err := ParseUpdateLocationRequest(roundTrip(t, req))
	if err != nil {
		t.Fatal(err)
	}

	if got.Flags != ULRFlagS6aS6dIndicator || got.Terminal.IMEI != testIMEI {
		t.Fatalf("parsed = %+v", got)
	}
}

func TestUpdateLocationAnswerRoundTrip(t *testing.T) {
	req := must[*diameter.Message](t)(NewUpdateLocationRequest(mmeEnvelope, testULR()))
	sub := testSubscription()

	u := UpdateLocation{
		Features:     Features{List1: FeatureODBAllAPN, List2: FeatureResetIDs},
		Flags:        ULAFlagSeparationIndication,
		Subscription: &sub,
		ResetIDs:     [][]byte{[]byte(testResetID)},
	}

	ans := must[*diameter.Message](t)(NewUpdateLocationAnswer(req, hssIdentity, u))
	requireNoVendorSpecificApplicationID(t, ans)

	if ans.IsRequest() || ans.CommandCode != CommandUpdateLocation {
		t.Fatalf("header = %+v", ans)
	}

	got, err := ParseUpdateLocationAnswer(roundTrip(t, ans))
	if err != nil || !reflect.DeepEqual(got, u) {
		t.Fatalf("round trip =\n%+v\n%v\nwant\n%+v", got, err, u)
	}

	if _, err := ParseUpdateLocationAnswer(must[*diameter.Message](t)(NewUpdateLocationAnswer(req, hssIdentity, UpdateLocation{}))); err != nil {
		t.Fatalf("empty ULA: %v", err)
	}
}

func TestUpdateLocationAnswerValidation(t *testing.T) {
	req := request(CommandUpdateLocation, mmeEnvelope)

	for name, mutate := range map[string]func(*SubscriptionData){
		"no APN":            func(s *SubscriptionData) { s.APNConfigurationProfile.Configurations = nil },
		"no default":        func(s *SubscriptionData) { s.APNConfigurationProfile.ContextIdentifier = 9 },
		"zero default":      func(s *SubscriptionData) { s.APNConfigurationProfile.ContextIdentifier = 0 },
		"duplicate context": func(s *SubscriptionData) { s.APNConfigurationProfile.Configurations[1].ContextIdentifier = 1 },
		"duplicate APN":     func(s *SubscriptionData) { s.APNConfigurationProfile.Configurations[1].ServiceSelection = "INTERNET" },
		"zero context":      func(s *SubscriptionData) { s.APNConfigurationProfile.Configurations[0].ContextIdentifier = 0 },
		"empty APN":         func(s *SubscriptionData) { s.APNConfigurationProfile.Configurations[0].ServiceSelection = "" },
		"bad PDN type":      func(s *SubscriptionData) { s.APNConfigurationProfile.Configurations[0].PDNType = 4 },
		"allocation without gw": func(s *SubscriptionData) {
			s.APNConfigurationProfile.Configurations[0].PDNGWAllocationType = ptr(PDNGWAllocationDynamic)
		},
		"two IPv4": func(s *SubscriptionData) {
			s.APNConfigurationProfile.Configurations[1].ServedPartyIPAddresses = []netip.Addr{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.2")}
		},
		"priority 0":       func(s *SubscriptionData) { s.APNConfigurationProfile.Configurations[0].QoS.ARP.PriorityLevel = 0 },
		"bad MSISDN":       func(s *SubscriptionData) { s.MSISDN = "12a" },
		"bad status":       func(s *SubscriptionData) { s.SubscriberStatus = ptr(SubscriberStatus(2)) },
		"reserved NAM":     func(s *SubscriptionData) { s.NetworkAccessMode = ptr(NetworkAccessMode(1)) },
		"RFSP 257":         func(s *SubscriptionData) { s.RATFrequencySelectionPriorityID = 257 },
		"UE usage 256":     func(s *SubscriptionData) { s.UEUsageType = ptr(uint32(256)) },
		"sub-second timer": func(s *SubscriptionData) { s.SubscribedPeriodicRAUTAUTimer = time.Millisecond },
		"empty gateway":    func(s *SubscriptionData) { s.APNConfigurationProfile.Configurations[1].PDNGateway = &MIP6AgentInfo{} },
		"host without realm": func(s *SubscriptionData) {
			s.APNConfigurationProfile.Configurations[1].PDNGateway.HomeAgentRealm = ""
		},
		"eleven zone codes": func(s *SubscriptionData) { s.RegionalSubscriptionZoneCodes = make([][]byte, 11) },
	} {
		t.Run(name, func(t *testing.T) {
			sub := testSubscription()
			mutate(&sub)

			if _, err := NewUpdateLocationAnswer(req, hssIdentity, UpdateLocation{Subscription: &sub}); !errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("err = %v", err)
			}
		})
	}

	if _, err := NewUpdateLocationAnswer(req, hssIdentity, UpdateLocation{Flags: 1 << 5}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("undefined ULA flag: err = %v", err)
	}
}

func open5GSSubscriptionData() diameter.AVP {
	v4 := netip.MustParseAddr("10.45.0.7")
	v6 := netip.MustParseAddr("2001:db8:cafe::")
	smf := netip.MustParseAddr("127.0.0.4")

	arp := func(level, capability, vulnerability uint32) diameter.AVP {
		return diameter.Grouped(tgpp.AVPAllocationRetentionPriority, 0, tgpp.VendorID,
			diameter.Unsigned32(tgpp.AVPPriorityLevel, 0, tgpp.VendorID, level),
			diameter.Unsigned32(tgpp.AVPPreemptionCapability, 0, tgpp.VendorID, capability),
			diameter.Unsigned32(tgpp.AVPPreemptionVulnerability, 0, tgpp.VendorID, vulnerability),
		)
	}

	ambr := func(ul, dl uint32) diameter.AVP {
		return vendorGroup(AVPAMBR,
			vendorU32(tgpp.AVPMaxRequestedBandwidthUL, ul),
			vendorU32(tgpp.AVPMaxRequestedBandwidthDL, dl),
		)
	}

	return vendorGroup(AVPSubscriptionData,
		vendorOctets(tgpp.AVPMSISDN, []byte{0x51, 0x55, 0x21, 0x03, 0x00, 0xf2}),
		vendorU32(AVPAccessRestrictionData, 0x20),
		vendorU32(AVPSubscriberStatus, 0),
		vendorU32(AVPNetworkAccessMode, 2),
		ambr(1_000_000_000, 1_000_000_000),
		diameter.Unsigned32(AVPSubscribedPeriodicRAUTAUTimer, 0, tgpp.VendorID, 3240),
		vendorGroup(AVPAPNConfigurationProfile,
			vendorU32(AVPContextIdentifier, 1),
			vendorU32(AVPAllAPNConfigurationsIncludedIndicator, 0),
			vendorGroup(AVPAPNConfiguration,
				vendorU32(AVPContextIdentifier, 1),
				vendorU32(AVPPDNType, 0),
				diameter.UTF8String(AVPServiceSelection, diameter.AVPFlagMandatory, 0, "internet"),
				vendorGroup(AVPEPSSubscribedQoSProfile,
					vendorU32(tgpp.AVPQoSClassIdentifier, 9),
					arp(8, 1, 1),
				),
				vendorU32(AVPVPLMNDynamicAddressAllowed, 1),
				ambr(1_000_000_000, 1_000_000_000),
			),
			vendorGroup(AVPAPNConfiguration,
				vendorU32(AVPContextIdentifier, 2),
				vendorU32(AVPPDNType, 2),
				diameter.Address(AVPServedPartyIPAddress, diameter.AVPFlagMandatory, tgpp.VendorID, v4),
				diameter.Address(AVPServedPartyIPAddress, diameter.AVPFlagMandatory, tgpp.VendorID, v6),
				diameter.UTF8String(AVPServiceSelection, diameter.AVPFlagMandatory, 0, "ims"),
				vendorGroup(AVPEPSSubscribedQoSProfile,
					vendorU32(tgpp.AVPQoSClassIdentifier, 5),
					arp(1, 0, 1),
				),
				diameter.Grouped(AVPMIP6AgentInfo, diameter.AVPFlagMandatory, 0,
					diameter.Address(AVPMIPHomeAgentAddress, diameter.AVPFlagMandatory, 0, smf),
				),
				vendorU32(AVPPDNGWAllocationType, 1),
				vendorU32(AVPVPLMNDynamicAddressAllowed, 1),
				diameter.Unsigned32(4321, 0, tgpp.VendorID, 1),
			),
		),
	)
}

func TestParseOpen5GSUpdateLocationAnswer(t *testing.T) {
	req := must[*diameter.Message](t)(NewUpdateLocationRequest(mmeEnvelope, testULR()))

	ans := tgpp.NewAnswer(req, hssIdentity, diameter.ResultSuccess)
	ans.AVPs = append(ans.AVPs,
		diameter.Grouped(diameter.AVPVendorSpecificApplicationID, diameter.AVPFlagMandatory, 0,
			diameter.Unsigned32(diameter.AVPVendorID, diameter.AVPFlagMandatory, 0, tgpp.VendorID),
			diameter.Unsigned32(diameter.AVPAuthApplicationID, diameter.AVPFlagMandatory, 0, ApplicationID)),
		vendorU32(AVPULAFlags, 1),
		vendorGroup(tgpp.AVPSupportedFeatures,
			diameter.Unsigned32(diameter.AVPVendorID, diameter.AVPFlagMandatory, 0, tgpp.VendorID),
			diameter.Unsigned32(tgpp.AVPFeatureListID, 0, tgpp.VendorID, 1),
			diameter.Unsigned32(tgpp.AVPFeatureList, 0, tgpp.VendorID, 0x0000000b)),
		vendorGroup(tgpp.AVPSupportedFeatures,
			diameter.Unsigned32(diameter.AVPVendorID, diameter.AVPFlagMandatory, 0, tgpp.VendorID),
			diameter.Unsigned32(tgpp.AVPFeatureListID, 0, tgpp.VendorID, 2),
			diameter.Unsigned32(tgpp.AVPFeatureList, 0, tgpp.VendorID, 0x08000001)),
		open5GSSubscriptionData(),
	)

	got, err := ParseUpdateLocationAnswer(roundTrip(t, ans))
	if err != nil {
		t.Fatal(err)
	}

	want := UpdateLocation{
		Features: Features{
			List1: FeatureODBAllAPN | FeatureODBHPLMNAPN | FeatureODBAllOG,
			List2: FeatureSMSInMME | FeatureNRAsSecondaryRAT,
		},
		Flags: ULAFlagSeparationIndication,
		Subscription: &SubscriptionData{
			SubscriberStatus:              ptr(SubscriberStatusServiceGranted),
			MSISDN:                        "15551230002",
			NetworkAccessMode:             ptr(NetworkAccessOnlyPacket),
			AccessRestrictionData:         ptr(ARDHOToNon3GPPAccessNotAllowed),
			AMBR:                          &AMBR{UplinkBPS: 1_000_000_000, DownlinkBPS: 1_000_000_000},
			SubscribedPeriodicRAUTAUTimer: 54 * time.Minute,
			APNConfigurationProfile: &APNConfigurationProfile{
				ContextIdentifier: 1,
				Included:          AllAPNConfigurationsIncluded,
				Configurations: []APNConfiguration{
					{
						ContextIdentifier:          1,
						PDNType:                    PDNTypeIPv4,
						ServiceSelection:           "internet",
						QoS:                        &EPSSubscribedQoSProfile{QCI: 9, ARP: AllocationRetentionPriority{PriorityLevel: 8}},
						VPLMNDynamicAddressAllowed: true,
						AMBR:                       &AMBR{UplinkBPS: 1_000_000_000, DownlinkBPS: 1_000_000_000},
					},
					{
						ContextIdentifier:      2,
						PDNType:                PDNTypeIPv4v6,
						ServedPartyIPAddresses: []netip.Addr{netip.MustParseAddr("10.45.0.7"), netip.MustParseAddr("2001:db8:cafe::")},
						ServiceSelection:       "ims",
						QoS: &EPSSubscribedQoSProfile{
							QCI: 5, ARP: AllocationRetentionPriority{PriorityLevel: 1, PreemptionCapability: true},
						},
						VPLMNDynamicAddressAllowed: true,
						PDNGateway:                 &MIP6AgentInfo{HomeAgentAddresses: []netip.Addr{netip.MustParseAddr("127.0.0.4")}},
						PDNGWAllocationType:        ptr(PDNGWAllocationDynamic),
					},
				},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsed =\n%+v\nwant\n%+v", got, want)
	}

	def, ok := got.Subscription.APNConfigurationProfile.Default()
	if !ok || def.ServiceSelection != "internet" {
		t.Fatalf("default = %+v, %v", def, ok)
	}

	if c, ok := got.Subscription.APNConfigurationProfile.Lookup("IMS"); !ok || c.ContextIdentifier != 2 {
		t.Fatalf("lookup = %+v, %v", c, ok)
	}
}

func TestParseUpdateLocationAnswerFailures(t *testing.T) {
	req := request(CommandUpdateLocation, mmeEnvelope)

	ans := tgpp.NewExperimentalAnswer(req, hssIdentity, tgpp.ResultErrorUnknownEPSSubscription)
	ans.AVPs = append(ans.AVPs, diameter.Unsigned32(AVPErrorDiagnostic, 0, tgpp.VendorID, uint32(ErrorDiagnosticNoGPRSDataSubscribed)))

	var re *ResultError

	_, err := ParseUpdateLocationAnswer(roundTrip(t, ans))
	if !errors.As(err, &re) || !re.IsExperimental(tgpp.ResultErrorUnknownEPSSubscription) ||
		re.ErrorDiagnostic == nil || *re.ErrorDiagnostic != ErrorDiagnosticNoGPRSDataSubscribed {
		t.Fatalf("err = %v", err)
	}

	for name, sub := range map[string]diameter.AVP{
		"short profile":  vendorOctets(AVPSubscriptionData, []byte{1, 2, 3}),
		"no APN config":  vendorGroup(AVPSubscriptionData, vendorGroup(AVPAPNConfigurationProfile, vendorU32(AVPContextIdentifier, 1), vendorU32(AVPAllAPNConfigurationsIncludedIndicator, 0))),
		"bad MSISDN":     vendorGroup(AVPSubscriptionData, vendorOctets(tgpp.AVPMSISDN, []byte{0xff})),
		"bad ARD length": vendorGroup(AVPSubscriptionData, vendorOctets(AVPAccessRestrictionData, []byte{1})),
	} {
		t.Run(name, func(t *testing.T) {
			ans := NewAnswer(req, hssIdentity, tgpp.Result{Code: diameter.ResultSuccess}, Features{})
			ans.AVPs = append(ans.AVPs, sub)

			if _, err := ParseUpdateLocationAnswer(ans); !errors.Is(err, ErrMalformedAnswer) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}
