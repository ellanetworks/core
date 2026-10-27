// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"bytes"
	"encoding/hex"
	"errors"
	"net/netip"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func testULR() UpdateLocationRequest {
	return UpdateLocationRequest{
		IMSI:                      testIMSI,
		Features:                  Features{List1: FeatureODBAllAPN | FeatureODBHPLMNAPN | FeatureODBVPLMNAPN},
		Terminal:                  &TerminalInformation{IMEI: testIMEI, SoftwareVersion: testSoftware, MEID: []byte{1, 2, 3}},
		RATType:                   tgpp.RATEUTRAN,
		Flags:                     ULRFlagS6aS6dIndicator | ULRFlagInitialAttachIndicator,
		UESRVCCCapability:         ptr(UESRVCCNotSupported),
		VisitedPLMN:               visitedPLMN,
		SGSNNumber:                "15550000002",
		HomogeneousIMSVoiceOverPS: ptr(IMSVoiceOverPSNotSupported),
		EquivalentPLMNs:           []tgpp.PLMN{{MCC: "208", MNC: "01"}, {MCC: "310", MNC: "410"}},
		MMENumberForMTSMS:         "15550000001",
		SMSRegisterRequest:        ptr(SMSRegistrationNotPreferred),
		SGsMMEIdentity:            mmeHost,
		CoupledNodeDiameterID:     "sgsn.example.org",
	}
}

func TestULRFlagsEncoding(t *testing.T) {
	r := UpdateLocationRequest{
		IMSI: testIMSI, RATType: tgpp.RATEUTRAN, VisitedPLMN: visitedPLMN,
		Flags: ULRFlagS6aS6dIndicator | ULRFlagInitialAttachIndicator,
	}

	req := must[*diameter.Message](t)(NewUpdateLocationRequest(mmeEnvelope, r))

	flags, _ := req.Find(AVPULRFlags, tgpp.VendorID)
	if got := encodeAVP(t, flags); !bytes.Equal(got, mustHex(t, "0000057dc0000010000028af00000022")) {
		t.Fatalf("ULR-Flags = %x", got)
	}

	rat, _ := req.Find(tgpp.AVPRATType, tgpp.VendorID)
	if got := encodeAVP(t, rat); !bytes.Equal(got, mustHex(t, "00000408c0000010000028af000003ec")) {
		t.Fatalf("RAT-Type = %x", got)
	}

	if s := r.Flags.String(); s != "S6A_S6D_INDICATOR|INITIAL_ATTACH_INDICATOR" {
		t.Fatalf("String = %q", s)
	}
}

func TestUpdateLocationRequestRoundTrip(t *testing.T) {
	for name, r := range map[string]UpdateLocationRequest{
		"full":            testULR(),
		"minimal":         {IMSI: testIMSI, RATType: tgpp.RATEUTRAN, VisitedPLMN: visitedPLMN},
		"future 3GPP RAT": {IMSI: testIMSI, RATType: 1999, VisitedPLMN: visitedPLMN},
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
		"zero RAT is WLAN": func(r *UpdateLocationRequest) { r.RATType = 0 },
		"3GPP2 RAT":        func(r *UpdateLocationRequest) { r.RATType = tgpp.RATEHRPD },
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
		"empty Session-Id":    {replace(base, diameter.UTF8String(diameter.AVPSessionID, diameter.AVPFlagMandatory, 0, "")), diameter.ResultInvalidAVPLength},
		"bad IMSI":            {replace(base, diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, "abc")), diameter.ResultInvalidAVPValue},
		"short PLMN":          {replace(base, vendorOctets(AVPVisitedPLMNID, []byte{0x02, 0xf8})), diameter.ResultInvalidAVPLength},
		"long PLMN":           {replace(base, vendorOctets(AVPVisitedPLMNID, []byte{0x02, 0xf8, 0x39, 0})), diameter.ResultInvalidAVPValue},
		"bad PLMN digit":      {replace(base, vendorOctets(AVPVisitedPLMNID, []byte{0x0a, 0xf8, 0x39})), diameter.ResultInvalidAVPValue},
		"WLAN RAT":            {replace(base, vendorU32(tgpp.AVPRATType, uint32(tgpp.RATWLAN))), diameter.ResultInvalidAVPValue},
		"long RAT":            {replace(base, vendorOctets(tgpp.AVPRATType, make([]byte, 5))), diameter.ResultInvalidAVPLength},
		"short ULR-Flags":     {replace(base, vendorOctets(AVPULRFlags, []byte{1})), diameter.ResultInvalidAVPLength},
		"twice ULR-Flags":     {with(base, vendorU32(AVPULRFlags, 0)), diameter.ResultAVPOccursTooManyTimes},
		"unknown M-bit AVP":   {with(base, vendorU32(9999, 0)), diameter.ResultAVPUnsupported},
		"bad SRVCC":           {replace(base, optionalU32(AVPUESRVCCCapability, 7)), diameter.ResultInvalidAVPValue},
		"bad homogeneous":     {replace(base, optionalU32(AVPHomogeneousSupportOfIMSVoiceOverPSSession, 7)), diameter.ResultInvalidAVPValue},
		"bad SMS register":    {replace(base, optionalU32(AVPSMSRegisterRequest, 7)), diameter.ResultInvalidAVPValue},
		"bad IMEI":            {replace(base, vendorGroup(AVPTerminalInformation, vendorOctets(AVPIMEI, []byte("12ab")))), diameter.ResultInvalidAVPValue},
		"bad SVN":             {replace(base, vendorGroup(AVPTerminalInformation, vendorOctets(AVPSoftwareVersion, []byte("7")))), diameter.ResultInvalidAVPValue},
		"unknown in terminal": {replace(base, vendorGroup(AVPTerminalInformation, vendorU32(4242, 1))), diameter.ResultAVPUnsupported},
		"bad terminal group":  {replace(base, vendorOctets(AVPTerminalInformation, []byte{0, 0, 5})), diameter.ResultInvalidAVPLength},
		"empty equivalent":    {replace(base, diameter.Grouped(AVPEquivalentPLMNList, 0, tgpp.VendorID)), diameter.ResultMissingAVP},
		"bad equivalent":      {replace(base, diameter.Grouped(AVPEquivalentPLMNList, 0, tgpp.VendorID, vendorOctets(AVPVisitedPLMNID, []byte{1}))), diameter.ResultInvalidAVPValue},
		"bad SGSN number":     {replace(base, vendorOctets(tgpp.AVPSGSNNumber, []byte{0xff})), diameter.ResultInvalidAVPValue},
		"bad MME SMS number":  {replace(base, diameter.OctetString(tgpp.AVPMMENumberForMTSMS, 0, tgpp.VendorID, []byte{0xff})), diameter.ResultInvalidAVPValue},
		"empty SGs MME ident": {replace(base, diameter.OctetString(AVPSGsMMEIdentity, 0, tgpp.VendorID, nil)), diameter.ResultInvalidAVPValue},
		"empty coupled node":  {replace(base, diameter.OctetString(AVPCoupledNodeDiameterID, 0, tgpp.VendorID, nil)), diameter.ResultInvalidAVPValue},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ParseUpdateLocationRequest(tt.req)
			if code := avpResult(t, err); code != tt.want {
				t.Fatalf("Result-Code = %d, want %d (%v)", code, tt.want, err)
			}
		})
	}

	_, err := ParseUpdateLocationRequest(tests["unknown in terminal"].req)
	if failed := avpError(t, err).AVPs[0]; failed.Code != AVPTerminalInformation {
		t.Fatalf("Failed-AVP = %+v, want the Terminal-Information parent", failed)
	}
}

func TestParseUpdateLocationRequestTolerance(t *testing.T) {
	base := must[*diameter.Message](t)(NewUpdateLocationRequest(mmeEnvelope, testULR()))

	vsai := diameter.Grouped(diameter.AVPVendorSpecificApplicationID, diameter.AVPFlagMandatory, 0,
		diameter.Unsigned32(diameter.AVPVendorID, diameter.AVPFlagMandatory, 0, tgpp.VendorID),
		diameter.Unsigned32(diameter.AVPAuthApplicationID, diameter.AVPFlagMandatory, 0, ApplicationID))

	req := with(base, vsai, optionalU32(9999, 1), optionalU32(AVPSFProvisionalIndication, 0))
	req = replace(req, vendorU32(AVPULRFlags, uint32(ULRFlagS6aS6dIndicator|1<<30)))
	req = replace(req, vendorGroup(AVPTerminalInformation, vendorOctets(AVPIMEI, []byte(testIMEI)), optionalU32(4242, 1)))

	got, err := ParseUpdateLocationRequest(roundTrip(t, req))
	if err != nil {
		t.Fatal(err)
	}

	if got.Flags != ULRFlagS6aS6dIndicator || got.Terminal.IMEI != testIMEI {
		t.Fatalf("parsed = %+v", got)
	}

	got, err = ParseUpdateLocationRequest(replace(base, vendorGroup(AVPTerminalInformation)))
	if err != nil || got.Terminal != nil {
		t.Fatalf("empty terminal = %+v, %v", got.Terminal, err)
	}
}

func TestUpdateLocationAnswerRoundTrip(t *testing.T) {
	req := must[*diameter.Message](t)(NewUpdateLocationRequest(mmeEnvelope, testULR()))
	sub := testSubscription()

	for name, u := range map[string]UpdateLocation{
		"full": {
			HSS:          hssIdentity,
			Features:     Features{List1: FeatureODBAllAPN, List2: FeatureResetIDs},
			Flags:        ULAFlagSeparationIndication | ULAFlagMMERegisteredForSMS,
			Subscription: &sub,
			ResetIDs:     []ResetID{testResetID, "hss-fe-2"},
		},
		"skipped subscriber data": {HSS: hssIdentity, Flags: ULAFlagSeparationIndication},
		"no APN profile": {
			HSS: hssIdentity, Flags: ULAFlagSeparationIndication,
			Subscription: &SubscriptionData{SubscriberStatus: ptr(SubscriberStatusOperatorDeterminedBarring), HPLMNODB: ptr(HPLMNODB(0))},
		},
	} {
		t.Run(name, func(t *testing.T) {
			ans := must[*diameter.Message](t)(NewUpdateLocationAnswer(req, hssIdentity, u))
			requireNoVendorSpecificApplicationID(t, ans)

			if ans.IsRequest() || ans.CommandCode != CommandUpdateLocation {
				t.Fatalf("header = %+v", ans)
			}

			if _, ok := ans.Find(AVPULAFlags, tgpp.VendorID); !ok {
				t.Fatal("successful ULA without ULA-Flags")
			}

			got, err := ParseUpdateLocationAnswer(roundTrip(t, ans))
			if err != nil || !reflect.DeepEqual(got, u) {
				t.Fatalf("round trip =\n%+v\n%v\nwant\n%+v", got, err, u)
			}
		})
	}
}

func TestUpdateLocationAnswerValidation(t *testing.T) {
	req := request(CommandUpdateLocation, mmeEnvelope)

	for name, mutate := range map[string]func(*SubscriptionData){
		"no APN":           func(s *SubscriptionData) { s.APNConfigurationProfile.Configurations = nil },
		"no default":       func(s *SubscriptionData) { s.APNConfigurationProfile.ContextIdentifier = 9 },
		"wildcard default": func(s *SubscriptionData) { s.APNConfigurationProfile.ContextIdentifier = 3 },
		"zero default":     func(s *SubscriptionData) { s.APNConfigurationProfile.ContextIdentifier = 0 },
		"no status":        func(s *SubscriptionData) { s.SubscriberStatus = nil },
		"barring no ODB": func(s *SubscriptionData) {
			s.SubscriberStatus, s.OperatorDeterminedBarring, s.HPLMNODB = ptr(SubscriberStatusOperatorDeterminedBarring), nil, nil
		},
		"no UE-AMBR":  func(s *SubscriptionData) { s.AMBR = nil },
		"no APN QoS":  func(s *SubscriptionData) { s.APNConfigurationProfile.Configurations[1].QoS = nil },
		"no APN-AMBR": func(s *SubscriptionData) { s.APNConfigurationProfile.Configurations[1].AMBR = nil },
		"modified no default": func(s *SubscriptionData) {
			s.APNConfigurationProfile.Included = ModifiedAddedAPNConfigurationsIncluded
			s.APNConfigurationProfile.ContextIdentifier = 9
		},
		"duplicate context": func(s *SubscriptionData) { s.APNConfigurationProfile.Configurations[1].ContextIdentifier = 1 },
		"duplicate APN":     func(s *SubscriptionData) { s.APNConfigurationProfile.Configurations[1].ServiceSelection = "INTERNET" },
		"zero context":      func(s *SubscriptionData) { s.APNConfigurationProfile.Configurations[0].ContextIdentifier = 0 },
		"empty APN":         func(s *SubscriptionData) { s.APNConfigurationProfile.Configurations[0].ServiceSelection = "" },
		"bad PDN type":      func(s *SubscriptionData) { s.APNConfigurationProfile.Configurations[0].PDNType = 4 },
		"bad included":      func(s *SubscriptionData) { s.APNConfigurationProfile.Included = 2 },
		"GBR QCI":           func(s *SubscriptionData) { s.APNConfigurationProfile.Configurations[0].QoS.QCI = 1 },
		"allocation without gw": func(s *SubscriptionData) {
			s.APNConfigurationProfile.Configurations[0].PDNGWAllocationType = ptr(PDNGWAllocationDynamic)
		},
		"bad allocation": func(s *SubscriptionData) {
			s.APNConfigurationProfile.Configurations[1].PDNGWAllocationType = ptr(PDNGWAllocationType(2))
		},
		"two IPv4": func(s *SubscriptionData) {
			s.APNConfigurationProfile.Configurations[1].ServedPartyIPAddresses = []netip.Addr{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.2")}
		},
		"three addresses": func(s *SubscriptionData) {
			s.APNConfigurationProfile.Configurations[1].ServedPartyIPAddresses = make([]netip.Addr, 3)
		},
		"invalid address": func(s *SubscriptionData) {
			s.APNConfigurationProfile.Configurations[1].ServedPartyIPAddresses = []netip.Addr{{}}
		},
		"non-IP delivery on IP": func(s *SubscriptionData) {
			s.APNConfigurationProfile.Configurations[0].NonIPDataDeliveryMechanism = ptr(NonIPDataDeliverySCEF)
		},
		"non-IP and ethernet": func(s *SubscriptionData) { s.APNConfigurationProfile.Configurations[3].EthernetPDNType = true },
		"bad non-IP delivery": func(s *SubscriptionData) {
			s.APNConfigurationProfile.Configurations[3].NonIPDataDeliveryMechanism = ptr(NonIPDataDeliveryMechanism(5))
		},
		"priority 0": func(s *SubscriptionData) { s.APNConfigurationProfile.Configurations[0].QoS.ARP.PriorityLevel = 0 },
		"bad pre-emption": func(s *SubscriptionData) {
			s.APNConfigurationProfile.Configurations[0].QoS.ARP.PreemptionCapability = 2
		},
		"zero AMBR":       func(s *SubscriptionData) { s.AMBR = &AMBR{UL: 0, DL: 1} },
		"AMBR in the gap": func(s *SubscriptionData) { s.AMBR = &AMBR{UL: 1 << 32, DL: 1} },
		"bad APN AMBR": func(s *SubscriptionData) {
			s.APNConfigurationProfile.Configurations[0].AMBR = &AMBR{UL: 4294968001, DL: 1}
		},
		"bad MSISDN":       func(s *SubscriptionData) { s.MSISDN = "12a" },
		"bad A-MSISDN":     func(s *SubscriptionData) { s.AMSISDN = "12a" },
		"bad STN-SR":       func(s *SubscriptionData) { s.STNSR = "12a" },
		"bad status":       func(s *SubscriptionData) { s.SubscriberStatus = ptr(SubscriberStatus(2)) },
		"reserved NAM":     func(s *SubscriptionData) { s.NetworkAccessMode = ptr(NetworkAccessMode(1)) },
		"undefined ODB":    func(s *SubscriptionData) { s.OperatorDeterminedBarring = ptr(OperatorDeterminedBarring(1 << 12)) },
		"undefined HPLMN":  func(s *SubscriptionData) { s.HPLMNODB = ptr(HPLMNODB(1 << 4)) },
		"undefined ARD":    func(s *SubscriptionData) { s.AccessRestrictionData = ptr(AccessRestrictionData(1 << 31)) },
		"RFSP 257":         func(s *SubscriptionData) { s.RATFrequencySelectionPriorityID = 257 },
		"sub-second timer": func(s *SubscriptionData) { s.SubscribedPeriodicRAUTAUTimer = time.Millisecond },
		"negative timer":   func(s *SubscriptionData) { s.SubscribedPeriodicRAUTAUTimer = -time.Second },
		"empty gateway":    func(s *SubscriptionData) { s.APNConfigurationProfile.Configurations[1].PDNGateway = &MIP6AgentInfo{} },
		"three gateways": func(s *SubscriptionData) {
			s.APNConfigurationProfile.Configurations[1].PDNGateway.HomeAgentAddresses = make([]netip.Addr, 3)
		},
		"invalid gateway": func(s *SubscriptionData) {
			s.APNConfigurationProfile.Configurations[1].PDNGateway.HomeAgentAddresses = []netip.Addr{{}}
		},
		"host without realm": func(s *SubscriptionData) {
			s.APNConfigurationProfile.Configurations[1].PDNGateway.HomeAgentRealm = ""
		},
		"specific without APN": func(s *SubscriptionData) {
			s.APNConfigurationProfile.Configurations[2].SpecificAPNInfo[0].ServiceSelection = ""
		},
		"specific bad gateway": func(s *SubscriptionData) {
			s.APNConfigurationProfile.Configurations[2].SpecificAPNInfo[0].PDNGateway = MIP6AgentInfo{}
		},
		"eleven zone codes": func(s *SubscriptionData) { s.RegionalSubscriptionZoneCodes = make([][]byte, 11) },
	} {
		t.Run(name, func(t *testing.T) {
			sub := testSubscription()
			mutate(&sub)

			if _, err := NewUpdateLocationAnswer(req, hssIdentity, UpdateLocation{Flags: ULAFlagSeparationIndication, Subscription: &sub}); !errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("err = %v", err)
			}
		})
	}

	for name, flags := range map[string]ULAFlags{"undefined ULA flag": ULAFlagSeparationIndication | 1<<5, "no separation indication": ULAFlagMMERegisteredForSMS} {
		if _, err := NewUpdateLocationAnswer(req, hssIdentity, UpdateLocation{Flags: flags}); !errors.Is(err, ErrInvalidMessage) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
}

func open5GSUpdateLocationAnswer(t testing.TB) *diameter.Message {
	t.Helper()

	raw, err := os.ReadFile("testdata/open5gs-ula.hex")
	if err != nil {
		t.Fatal(err)
	}

	b, err := hex.DecodeString(strings.Join(strings.Fields(string(raw)), ""))
	if err != nil {
		t.Fatal(err)
	}

	m, err := diameter.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	return m
}

func TestParseOpen5GSUpdateLocationAnswer(t *testing.T) {
	got, err := ParseUpdateLocationAnswer(open5GSUpdateLocationAnswer(t))
	if err != nil {
		t.Fatal(err)
	}

	internetARP := tgpp.AllocationRetentionPriority{
		PriorityLevel: 8, PreemptionCapability: tgpp.PreemptionCapabilityDisabled, PreemptionVulnerability: tgpp.PreemptionVulnerabilityDisabled,
	}
	imsARP := tgpp.AllocationRetentionPriority{
		PriorityLevel: 1, PreemptionCapability: tgpp.PreemptionCapabilityEnabled, PreemptionVulnerability: tgpp.PreemptionVulnerabilityDisabled,
	}

	want := UpdateLocation{
		HSS: diameter.Identity{OriginHost: "hss.localdomain", OriginRealm: "localdomain"},
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
			AMBR:                          &AMBR{UL: 1_000_000_000, DL: 1_000_000_000},
			SubscribedPeriodicRAUTAUTimer: 54 * time.Minute,
			APNConfigurationProfile: &APNConfigurationProfile{
				ContextIdentifier: 1,
				Included:          AllAPNConfigurationsIncluded,
				Configurations: []APNConfiguration{
					{
						ContextIdentifier:          1,
						PDNType:                    PDNTypeIPv4,
						ServiceSelection:           "internet",
						QoS:                        &EPSSubscribedQoSProfile{QCI: 9, ARP: internetARP},
						VPLMNDynamicAddressAllowed: true,
						AMBR:                       &AMBR{UL: 1_000_000_000, DL: 1_000_000_000},
					},
					{
						ContextIdentifier:          2,
						PDNType:                    PDNTypeIPv4v6,
						ServedPartyIPAddresses:     []netip.Addr{netip.MustParseAddr("10.45.0.7"), netip.MustParseAddr("2001:db8:cafe::")},
						ServiceSelection:           "ims",
						QoS:                        &EPSSubscribedQoSProfile{QCI: 5, ARP: imsARP},
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

	profile := got.Subscription.APNConfigurationProfile

	def, ok := profile.Default()
	if !ok || def.ServiceSelection != "internet" || !def.IPBased() {
		t.Fatalf("default = %+v, %v", def, ok)
	}

	if c, ok := profile.Lookup("IMS"); !ok || c.ContextIdentifier != 2 {
		t.Fatalf("lookup = %+v, %v", c, ok)
	}

	if _, ok := profile.Resolve("corp"); ok {
		t.Fatal("resolved an APN without a wildcard")
	}
}

func TestParseUpdateLocationAnswerLenient(t *testing.T) {
	req := request(CommandUpdateLocation, mmeEnvelope)
	apn := func(extra ...diameter.AVP) diameter.AVP {
		return vendorGroup(AVPAPNConfiguration, append([]diameter.AVP{
			vendorU32(AVPContextIdentifier, 1),
			vendorU32(AVPPDNType, 9),
			diameter.UTF8String(diameter.AVPServiceSelection, diameter.AVPFlagMandatory, 0, "internet"),
		}, extra...)...)
	}

	ans := success(req,
		vendorU32(AVPULAFlags, uint32(ULAFlagSeparationIndication|1<<7)),
		vendorGroup(AVPSubscriptionData,
			vendorU32(AVPSubscriberStatus, 7),
			vendorU32(AVPNetworkAccessMode, 1),
			vendorU32(AVPAccessRestrictionData, uint32(ARDUTRANNotAllowed|1<<31)),
			vendorU32(AVPOperatorDeterminedBarring, 1<<20),
			vendorU32(AVPHPLMNODB, 1<<10),
			vendorU32(AVPRoamingRestrictedDueToUnsupportedFeature, 5),
			vendorU32(9999, 1),
			vendorGroup(AVPAPNConfigurationProfile,
				vendorU32(AVPContextIdentifier, 1),
				vendorU32(AVPAllAPNConfigurationsIncludedIndicator, 3),
				apn(
					vendorGroup(AVPEPSSubscribedQoSProfile,
						vendorU32(tgpp.AVPQoSClassIdentifier, 1),
						diameter.Grouped(tgpp.AVPAllocationRetentionPriority, 0, tgpp.VendorID,
							optionalU32(tgpp.AVPPriorityLevel, 16), optionalU32(tgpp.AVPPreemptionCapability, 4)),
					),
					vendorU32(AVPVPLMNDynamicAddressAllowed, 2),
					vendorU32(AVPPDNGWAllocationType, 9),
					optionalU32(AVPNonIPPDNTypeIndicator, 1),
					optionalU32(AVPNonIPDataDeliveryMechanism, 7),
					optionalU32(AVPEthernetPDNTypeIndicator, 2),
					optionalU32(AVPInterworking5GSIndicator, 1),
				),
			),
		),
	)

	got, err := ParseUpdateLocationAnswer(roundTrip(t, ans))
	if err != nil {
		t.Fatal(err)
	}

	sub := got.Subscription
	c := sub.APNConfigurationProfile.Configurations[0]

	switch {
	case got.Flags != ULAFlagSeparationIndication|1<<7 || got.Flags.Known():
		t.Fatalf("ULA-Flags = %v", got.Flags)
	case *sub.SubscriberStatus != 7 || sub.SubscriberStatus.Known():
		t.Fatalf("Subscriber-Status = %v", *sub.SubscriberStatus)
	case *sub.NetworkAccessMode != 1:
		t.Fatalf("Network-Access-Mode = %v", *sub.NetworkAccessMode)
	case *sub.AccessRestrictionData != ARDUTRANNotAllowed|1<<31 || sub.AccessRestrictionData.Known():
		t.Fatalf("Access-Restriction-Data = %v", *sub.AccessRestrictionData)
	case *sub.OperatorDeterminedBarring != 1<<20 || *sub.HPLMNODB != 1<<10:
		t.Fatalf("ODB = %v, HPLMN-ODB = %v", *sub.OperatorDeterminedBarring, *sub.HPLMNODB)
	case sub.RoamingRestrictedDueToUnsupportedFeature:
		t.Fatal("unknown Roaming-Restricted value read as set")
	case sub.APNConfigurationProfile.Included != 3 || c.PDNType != 9 || *c.PDNGWAllocationType != 9:
		t.Fatalf("enums = %+v", c)
	case c.QoS.QCI != 1 || c.QoS.Known() || c.QoS.ARP.PriorityLevel != 16 || c.QoS.ARP.PreemptionCapability != 4:
		t.Fatalf("QoS = %+v", c.QoS)
	case c.VPLMNDynamicAddressAllowed || c.EthernetPDNType || !c.NonIPPDNType || !c.Interworking5GSIndicator:
		t.Fatalf("indicators = %+v", c)
	case *c.NonIPDataDeliveryMechanism != 7 || c.IPBased():
		t.Fatalf("non-IP = %+v", c)
	}

	if _, err := NewUpdateLocationAnswer(req, hssIdentity, got); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("unknown values rebuild: %v", err)
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

	profile := func(avps ...diameter.AVP) diameter.AVP {
		return vendorGroup(AVPSubscriptionData, vendorGroup(AVPAPNConfigurationProfile, avps...))
	}
	ctx1, all := vendorU32(AVPContextIdentifier, 1), vendorU32(AVPAllAPNConfigurationsIncludedIndicator, 0)
	apn := func(avps ...diameter.AVP) diameter.AVP { return vendorGroup(AVPAPNConfiguration, avps...) }
	pdn, internet := vendorU32(AVPPDNType, 0), diameter.UTF8String(diameter.AVPServiceSelection, diameter.AVPFlagMandatory, 0, "internet")

	for name, avps := range map[string][]diameter.AVP{
		"short ULA-Flags":   {vendorOctets(AVPULAFlags, []byte{1})},
		"short profile":     {vendorOctets(AVPSubscriptionData, []byte{1, 2, 3})},
		"no APN config":     {profile(ctx1, all)},
		"zero default":      {profile(vendorU32(AVPContextIdentifier, 0), all, apn(ctx1, pdn, internet))},
		"no default":        {profile(all, apn(ctx1, pdn, internet))},
		"no indicator":      {profile(ctx1, apn(ctx1, pdn, internet))},
		"no context":        {profile(ctx1, all, apn(pdn, internet))},
		"zero context":      {profile(ctx1, all, apn(vendorU32(AVPContextIdentifier, 0), pdn, internet))},
		"no PDN type":       {profile(ctx1, all, apn(ctx1, internet))},
		"no APN":            {profile(ctx1, all, apn(ctx1, pdn))},
		"empty APN":         {profile(ctx1, all, apn(ctx1, pdn, diameter.UTF8String(diameter.AVPServiceSelection, 0, 0, "")))},
		"duplicate context": {profile(ctx1, all, apn(ctx1, pdn, internet), apn(ctx1, pdn, diameter.UTF8String(diameter.AVPServiceSelection, 0, 0, "ims")))},
		"bad QoS":           {profile(ctx1, all, apn(ctx1, pdn, internet, vendorGroup(AVPEPSSubscribedQoSProfile)))},
		"QoS without ARP":   {profile(ctx1, all, apn(ctx1, pdn, internet, vendorGroup(AVPEPSSubscribedQoSProfile, vendorU32(tgpp.AVPQoSClassIdentifier, 9))))},
		"bad ARP": {profile(ctx1, all, apn(ctx1, pdn, internet, vendorGroup(AVPEPSSubscribedQoSProfile,
			vendorU32(tgpp.AVPQoSClassIdentifier, 9), vendorOctets(tgpp.AVPAllocationRetentionPriority, []byte{1}))))},
		"AMBR without DL":   {vendorGroup(AVPSubscriptionData, vendorGroup(AVPAMBR, vendorU32(tgpp.AVPMaxRequestedBandwidthUL, 1)))},
		"bad extended AMBR": {vendorGroup(AVPSubscriptionData, vendorGroup(AVPAMBR, vendorU32(tgpp.AVPMaxRequestedBandwidthUL, 1), vendorU32(tgpp.AVPMaxRequestedBandwidthDL, 1), vendorOctets(tgpp.AVPExtendedMaxRequestedBWDL, []byte{1})))},
		"bad served IP":     {profile(ctx1, all, apn(ctx1, pdn, internet, vendorOctets(AVPServedPartyIPAddress, []byte{0, 1, 1})))},
		"three served IPs": {profile(ctx1, all, apn(ctx1, pdn, internet,
			vendorOctets(AVPServedPartyIPAddress, []byte{0, 1, 1, 1, 1, 1}), vendorOctets(AVPServedPartyIPAddress, []byte{0, 1, 1, 1, 1, 2}),
			vendorOctets(AVPServedPartyIPAddress, []byte{0, 1, 1, 1, 1, 3})))},
		"bad gateway":              {profile(ctx1, all, apn(ctx1, pdn, internet, diameter.Grouped(diameter.AVPMIP6AgentInfo, diameter.AVPFlagMandatory, 0)))},
		"bad gateway addr":         {profile(ctx1, all, apn(ctx1, pdn, internet, diameter.Grouped(diameter.AVPMIP6AgentInfo, diameter.AVPFlagMandatory, 0, diameter.OctetString(diameter.AVPMIPHomeAgentAddress, 0, 0, []byte{9}))))},
		"host without realm":       {profile(ctx1, all, apn(ctx1, pdn, internet, diameter.Grouped(diameter.AVPMIP6AgentInfo, diameter.AVPFlagMandatory, 0, diameter.Grouped(diameter.AVPMIPHomeAgentHost, 0, 0, diameter.UTF8String(diameter.AVPDestinationHost, 0, 0, "pgw")))))},
		"specific without gateway": {profile(ctx1, all, apn(ctx1, pdn, internet, vendorGroup(AVPSpecificAPNInfo, internet)))},
		"specific without APN":     {profile(ctx1, all, apn(ctx1, pdn, internet, vendorGroup(AVPSpecificAPNInfo)))},
		"bad visited network":      {profile(ctx1, all, apn(ctx1, pdn, internet, diameter.OctetString(tgpp.AVPVisitedNetworkIdentifier, 0, tgpp.VendorID, nil)))},
		"bad MSISDN":               {vendorGroup(AVPSubscriptionData, vendorOctets(tgpp.AVPMSISDN, []byte{0xff}))},
		"bad ARD length":           {vendorGroup(AVPSubscriptionData, vendorOctets(AVPAccessRestrictionData, []byte{1}))},
		"bad ODB length":           {vendorGroup(AVPSubscriptionData, vendorOctets(AVPOperatorDeterminedBarring, []byte{1}))},
		"bad status length":        {vendorGroup(AVPSubscriptionData, vendorOctets(AVPSubscriberStatus, []byte{1}))},
		"RFSP 0":                   {vendorGroup(AVPSubscriptionData, vendorU32(AVPRATFrequencySelectionPriorityID, 0))},
		"UE usage 256":             {vendorGroup(AVPSubscriptionData, optionalU32(AVPUEUsageType, 256))},
		"empty APN-OI":             {vendorGroup(AVPSubscriptionData, vendorOctets(AVPAPNOIReplacement, nil))},
		"eleven zone codes": {vendorGroup(AVPSubscriptionData, func() []diameter.AVP {
			var zc []diameter.AVP
			for range 11 {
				zc = append(zc, vendorOctets(AVPRegionalSubscriptionZoneCode, []byte{1}))
			}

			return zc
		}()...)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseUpdateLocationAnswer(success(req, avps...)); !errors.Is(err, ErrMalformedAnswer) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}
