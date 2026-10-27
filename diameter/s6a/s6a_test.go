// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"bytes"
	"encoding/hex"
	"errors"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const (
	testIMSI      = "001010000000001"
	visitedRealm  = "epc.mnc093.mcc208.3gppnetwork.org"
	homeRealm     = "epc.mnc001.mcc001.3gppnetwork.org"
	mmeHost       = "mme.epc.mnc093.mcc208.3gppnetwork.org"
	hssHost       = "hss.epc.mnc001.mcc001.3gppnetwork.org"
	testIMEI      = "35846012345678"
	testSoftware  = "07"
	testResetID   = ResetID("\x01\x02")
	testMSISDN    = "15551230002"
	testRAUTimer  = 54 * time.Minute
	testUEAMBRUL  = 1_000_000_000
	testUEAMBRDL  = 2_000_000_000
	testAPNAMBRUL = 500_000_000
	testAPNAMBRDL = 6_000_000_000
)

var (
	mmeIdentity = diameter.Identity{OriginHost: mmeHost, OriginRealm: visitedRealm}
	hssIdentity = diameter.Identity{OriginHost: hssHost, OriginRealm: homeRealm}
	mmeEnvelope = tgpp.Envelope{SessionID: mmeHost + ";1;1", Origin: mmeIdentity, DestinationRealm: homeRealm}
	hssEnvelope = tgpp.Envelope{
		SessionID: hssHost + ";1;1", Origin: hssIdentity, DestinationHost: mmeHost, DestinationRealm: visitedRealm,
	}
	visitedPLMN = tgpp.PLMN{MCC: "208", MNC: "93"}
)

func ptr[T any](v T) *T { return &v }

func mustHex(t *testing.T, s string) []byte {
	t.Helper()

	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func must[T any](t testing.TB) func(T, error) T {
	return func(v T, err error) T {
		t.Helper()

		if err != nil {
			t.Fatal(err)
		}

		return v
	}
}

func roundTrip(t *testing.T, m *diameter.Message) *diameter.Message {
	t.Helper()

	b, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	out, err := diameter.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	return out
}

func avpError(t *testing.T, err error) *diameter.AVPError {
	t.Helper()

	var avpErr *diameter.AVPError
	if !errors.As(err, &avpErr) {
		t.Fatalf("err = %v, want an AVP error", err)
	}

	return avpErr
}

func avpResult(t *testing.T, err error) uint32 {
	t.Helper()

	return avpError(t, err).ResultCode
}

func request(command uint32, env tgpp.Envelope, avps ...diameter.AVP) *diameter.Message {
	return &diameter.Message{
		Flags:         diameter.FlagRequest | diameter.FlagProxiable,
		CommandCode:   command,
		ApplicationID: ApplicationID,
		AVPs:          append(env.AVPs(), avps...),
	}
}

func success(req *diameter.Message, avps ...diameter.AVP) *diameter.Message {
	ans := NewAnswer(req, hssIdentity, tgpp.Result{Code: diameter.ResultSuccess}, Features{})
	ans.AVPs = append(ans.AVPs, avps...)

	return ans
}

func without(m *diameter.Message, code, vendorID uint32) *diameter.Message {
	out := *m
	out.AVPs = nil

	for _, a := range m.AVPs {
		if a.Code != code || a.VendorID != vendorID {
			out.AVPs = append(out.AVPs, a)
		}
	}

	return &out
}

func with(m *diameter.Message, avps ...diameter.AVP) *diameter.Message {
	out := *m
	out.AVPs = append(append([]diameter.AVP(nil), m.AVPs...), avps...)

	return &out
}

func replace(m *diameter.Message, a diameter.AVP) *diameter.Message {
	return with(without(m, a.Code, a.VendorID), a)
}

func vendorU32(code, v uint32) diameter.AVP {
	return diameter.Unsigned32(code, diameter.AVPFlagMandatory, tgpp.VendorID, v)
}

func vendorOctets(code uint32, v []byte) diameter.AVP {
	return diameter.OctetString(code, diameter.AVPFlagMandatory, tgpp.VendorID, v)
}

func vendorGroup(code uint32, avps ...diameter.AVP) diameter.AVP {
	return diameter.Grouped(code, diameter.AVPFlagMandatory, tgpp.VendorID, avps...)
}

func optionalU32(code, v uint32) diameter.AVP {
	return diameter.Unsigned32(code, 0, tgpp.VendorID, v)
}

func encodeAVP(t *testing.T, a diameter.AVP) []byte {
	t.Helper()

	m := &diameter.Message{AVPs: []diameter.AVP{a}}

	b, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return b[20:]
}

func testAPNConfigurations() []APNConfiguration {
	return []APNConfiguration{
		{
			ContextIdentifier: 1,
			PDNType:           PDNTypeIPv4,
			ServiceSelection:  "internet",
			QoS: &EPSSubscribedQoSProfile{
				QCI: 9, ARP: tgpp.AllocationRetentionPriority{
					PriorityLevel: 8, PreemptionCapability: tgpp.PreemptionCapabilityDisabled,
					PreemptionVulnerability: tgpp.PreemptionVulnerabilityEnabled,
				},
			},
			VPLMNDynamicAddressAllowed: true,
			AMBR:                       &AMBR{UL: testAPNAMBRUL, DL: testAPNAMBRDL},
			Interworking5GSIndicator:   true,
		},
		{
			ContextIdentifier:      2,
			PDNType:                PDNTypeIPv4v6,
			ServiceSelection:       "ims",
			ServedPartyIPAddresses: []netip.Addr{netip.MustParseAddr("10.45.0.2"), netip.MustParseAddr("2001:db8::")},
			QoS: &EPSSubscribedQoSProfile{
				QCI: 5, ARP: tgpp.AllocationRetentionPriority{
					PriorityLevel: 1, PreemptionCapability: tgpp.PreemptionCapabilityEnabled,
					PreemptionVulnerability: tgpp.PreemptionVulnerabilityDisabled,
				},
			},
			PDNGateway: &MIP6AgentInfo{
				HomeAgentAddresses: []netip.Addr{netip.MustParseAddr("192.0.2.1")},
				HomeAgentHost:      "topon.s5.pgw.epc.mnc001.mcc001.3gppnetwork.org",
				HomeAgentRealm:     homeRealm,
			},
			PDNGWAllocationType:      ptr(PDNGWAllocationStatic),
			VisitedNetworkIdentifier: "mnc001.mcc001.3gppnetwork.org",
			AMBR:                     &AMBR{UL: 1000, DL: 2000},
			APNOIReplacement:         "ims.mnc001.mcc001.gprs",
		},
		{
			ContextIdentifier: 3,
			PDNType:           PDNTypeIPv4OrIPv6,
			ServiceSelection:  WildcardAPN,
			QoS:               &EPSSubscribedQoSProfile{QCI: 8, ARP: tgpp.DefaultAllocationRetentionPriority(15)},
			AMBR:              &AMBR{UL: 1, DL: 1},
			SpecificAPNInfo: []SpecificAPNInfo{{
				ServiceSelection: "corp",
				PDNGateway:       MIP6AgentInfo{HomeAgentAddresses: []netip.Addr{netip.MustParseAddr("2001:db8::1")}},
			}},
		},
		{
			ContextIdentifier:          4,
			PDNType:                    PDNTypeIPv4,
			ServiceSelection:           "sensors",
			QoS:                        &EPSSubscribedQoSProfile{QCI: 128, ARP: tgpp.DefaultAllocationRetentionPriority(9)},
			AMBR:                       &AMBR{UL: 64000, DL: 64000},
			NonIPPDNType:               true,
			NonIPDataDeliveryMechanism: ptr(NonIPDataDeliverySGi),
		},
	}
}

func testSubscription() SubscriptionData {
	return SubscriptionData{
		SubscriberStatus:              ptr(SubscriberStatusServiceGranted),
		MSISDN:                        testMSISDN,
		AMSISDN:                       "15551230003",
		STNSR:                         "15550000999",
		NetworkAccessMode:             ptr(NetworkAccessOnlyPacket),
		OperatorDeterminedBarring:     ptr(ODBRoamerAccessHPLMNAP),
		HPLMNODB:                      ptr(HPLMNODBType2),
		AccessRestrictionData:         ptr(ARDUTRANNotAllowed | ARDGERANNotAllowed),
		RegionalSubscriptionZoneCodes: [][]byte{{0x01, 0x02}},
		APNOIReplacement:              "mnc001.mcc001.gprs",
		ChargingCharacteristics:       "0800",
		AMBR:                          &AMBR{UL: testUEAMBRUL, DL: testUEAMBRDL},
		APNConfigurationProfile: &APNConfigurationProfile{
			ContextIdentifier:           1,
			AdditionalContextIdentifier: 4,
			Included:                    AllAPNConfigurationsIncluded,
			Configurations:              testAPNConfigurations(),
		},
		RATFrequencySelectionPriorityID:          12,
		RoamingRestrictedDueToUnsupportedFeature: true,
		SubscribedPeriodicRAUTAUTimer:            testRAUTimer,
		UEUsageType:                              ptr(UEUsageType(0)),
		SubscribedVSRVCC:                         true,
	}
}

func requireNoVendorSpecificApplicationID(t *testing.T, m *diameter.Message) {
	t.Helper()

	if _, ok := m.Find(diameter.AVPVendorSpecificApplicationID, 0); ok {
		t.Fatal("S6a command carries Vendor-Specific-Application-Id")
	}

	state, ok := m.Find(diameter.AVPAuthSessionState, 0)
	if v, _ := state.Unsigned32(); !ok || v != diameter.AuthSessionStateNoStateMaintained {
		t.Fatalf("Auth-Session-State = %+v", state)
	}

	if m.ApplicationID != ApplicationID || m.Flags&diameter.FlagProxiable == 0 {
		t.Fatalf("header = %+v", m)
	}
}

func requireFlagRules(t *testing.T, avps []diameter.AVP) {
	t.Helper()

	for _, a := range avps {
		if a.VendorID == tgpp.VendorID {
			if a.Code == tgpp.AVPSupportedFeatures || a.Code == tgpp.AVPPriorityLevel ||
				a.Code == tgpp.AVPPreemptionCapability || a.Code == tgpp.AVPPreemptionVulnerability {
				continue
			}

			want, ok := mandatoryBit[a.Code]
			if !ok {
				t.Errorf("AVP %d has no flag rule", a.Code)
			}

			if got := a.Flags&diameter.AVPFlagMandatory != 0; got != want {
				t.Errorf("AVP %d: M = %v, want %v", a.Code, got, want)
			}
		}

		if inner, err := a.Grouped(); err == nil && len(a.Data) > 0 && isGrouped(a) {
			requireFlagRules(t, inner)
		}
	}
}

func isGrouped(a diameter.AVP) bool {
	switch a.Code {
	case AVPSubscriptionData, AVPAPNConfigurationProfile, AVPAPNConfiguration, AVPEPSSubscribedQoSProfile, AVPAMBR,
		AVPSpecificAPNInfo, AVPTerminalInformation, AVPRequestedEUTRANAuthenticationInfo,
		AVPRequestedUTRANGERANAuthenticationInfo, AVPAuthenticationInfo, AVPEUTRANVector, AVPUTRANVector, AVPGERANVector,
		AVPEquivalentPLMNList, AVPLocalTimeZone, AVPSubscriptionDataDeletion, tgpp.AVPAllocationRetentionPriority:
		return a.VendorID == tgpp.VendorID
	case diameter.AVPMIP6AgentInfo, diameter.AVPMIPHomeAgentHost:
		return a.VendorID == 0
	}

	return false
}

func TestVisitedPLMNIDEncoding(t *testing.T) {
	for _, tt := range []struct {
		plmn tgpp.PLMN
		want string
	}{
		{tgpp.PLMN{MCC: "208", MNC: "93"}, "0000057fc000000f000028af02f839"},
		{tgpp.PLMN{MCC: "310", MNC: "410"}, "0000057fc000000f000028af130014"},
	} {
		a := must[diameter.AVP](t)(visitedPLMNAVP(tt.plmn))
		if got := encodeAVP(t, a); !bytes.Equal(got, mustHex(t, tt.want+"00")) {
			t.Errorf("%v = %x, want %s00", tt.plmn, got, tt.want)
		}
	}
}

func TestAVPFlagRules(t *testing.T) {
	for _, tt := range []struct {
		code      uint32
		mandatory bool
	}{
		{AVPSubscriptionData, true},
		{AVPULRFlags, true},
		{AVPIDRFlags, true},
		{tgpp.AVPSGSNNumber, true},
		{AVPUserID, false},
		{AVPHomogeneousSupportOfIMSVoiceOverPSSession, false},
		{AVPErrorDiagnostic, false},
		{AVPCLRFlags, false},
		{AVPPURFlags, false},
		{AVPSubscribedPeriodicRAUTAUTimer, false},
		{AVPResetID, false},
		{AVPUEUsageType, false},
		{AVPNonIPPDNTypeIndicator, false},
		{AVPSubscriptionDataDeletion, false},
		{tgpp.AVPMSISDN, true},
		{tgpp.AVPQoSClassIdentifier, true},
		{tgpp.AVPAllocationRetentionPriority, false},
		{tgpp.AVPMaxRequestedBandwidthUL, true},
		{tgpp.AVPExtendedMaxRequestedBWUL, false},
		{AVPServedPartyIPAddress, true},
		{tgpp.AVPRATType, true},
		{tgpp.AVPVisitedNetworkIdentifier, false},
		{tgpp.AVP3GPPChargingCharacteristics, false},
		{AVPSCEFID, false},
	} {
		if got := avpFlags(tt.code)&diameter.AVPFlagMandatory != 0; got != tt.mandatory {
			t.Errorf("AVP %d: M = %v, want %v", tt.code, got, tt.mandatory)
		}
	}
}

func TestEmittedAVPsFollowFlagTable(t *testing.T) {
	sub := testSubscription()
	ulr := must[*diameter.Message](t)(NewUpdateLocationRequest(mmeEnvelope, testULR()))

	for name, m := range map[string]*diameter.Message{
		"ULR": ulr,
		"ULA": must[*diameter.Message](t)(NewUpdateLocationAnswer(ulr, hssIdentity, UpdateLocation{
			Flags: ULAFlagSeparationIndication, Subscription: &sub, ResetIDs: []ResetID{testResetID},
		})),
		"AIR": must[*diameter.Message](t)(NewAuthenticationInformationRequest(mmeEnvelope, testAIR())),
		"AIA": must[*diameter.Message](t)(NewAuthenticationInformationAnswer(ulr, hssIdentity, testAIA())),
		"IDR": must[*diameter.Message](t)(NewInsertSubscriberDataRequest(hssEnvelope, InsertSubscriberDataRequest{
			IMSI: testIMSI, Subscription: sub, Flags: IDRFlagRATTypeRequested,
		})),
		"IDA": must[*diameter.Message](t)(NewInsertSubscriberDataAnswer(ulr, mmeIdentity, testIDA())),
		"DSR": must[*diameter.Message](t)(NewDeleteSubscriberDataRequest(hssEnvelope, testDSR())),
		"NOR": must[*diameter.Message](t)(NewNotifyRequest(mmeEnvelope, testNOR())),
		"RSR": must[*diameter.Message](t)(NewResetRequest(hssEnvelope, ResetRequest{
			UserIDs: []string{"00101"}, ResetIDs: []ResetID{testResetID}, Deletion: ptr(testDSR().SubscriptionDataDeletion),
		})),
		"CLR": must[*diameter.Message](t)(NewCancelLocationRequest(hssEnvelope, CancelLocationRequest{IMSI: testIMSI, Flags: CLRFlagReattachRequired})),
		"PUR": must[*diameter.Message](t)(NewPurgeUERequest(mmeEnvelope, PurgeUERequest{IMSI: testIMSI, Flags: PURFlagUEPurgedInMME})),
		"error": NewErrorAnswer(ulr, hssIdentity, &ResultError{
			Result: tgpp.Experimental(tgpp.ResultErrorUnknownEPSSubscription), ErrorDiagnostic: ptr(ErrorDiagnosticGPRSDataSubscribed),
		}, Features{}),
	} {
		t.Run(name, func(t *testing.T) { requireFlagRules(t, m.AVPs) })
	}
}

func TestErrorAnswerDiagnostic(t *testing.T) {
	ulr := must[*diameter.Message](t)(NewUpdateLocationRequest(mmeEnvelope, testULR()))
	air := must[*diameter.Message](t)(NewAuthenticationInformationRequest(mmeEnvelope, testAIR()))

	for _, tt := range []struct {
		name     string
		req      *diameter.Message
		err      *ResultError
		withDiag bool
	}{
		{"ULA unknown EPS subscription", ulr, &ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUnknownEPSSubscription), ErrorDiagnostic: ptr(ErrorDiagnosticNoGPRSDataSubscribed)}, true},
		{"AIA unknown EPS subscription", air, &ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUnknownEPSSubscription), ErrorDiagnostic: ptr(ErrorDiagnosticGPRSDataSubscribed)}, true},
		{"ULA roaming not allowed", ulr, &ResultError{Result: tgpp.Experimental(tgpp.ResultErrorRoamingNotAllowed), ErrorDiagnostic: ptr(ErrorDiagnosticODBAllAPN)}, true},
		{"AIA roaming not allowed", air, &ResultError{Result: tgpp.Experimental(tgpp.ResultErrorRoamingNotAllowed), ErrorDiagnostic: ptr(ErrorDiagnosticODBAllAPN)}, false},
		{"ODB diagnostic on 5420", ulr, &ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUnknownEPSSubscription), ErrorDiagnostic: ptr(ErrorDiagnosticODBVPLMNAPN)}, false},
		{"GPRS diagnostic on 5004", ulr, &ResultError{Result: tgpp.Experimental(tgpp.ResultErrorRoamingNotAllowed), ErrorDiagnostic: ptr(ErrorDiagnosticGPRSDataSubscribed)}, false},
		{"user unknown", ulr, &ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUserUnknown), ErrorDiagnostic: ptr(ErrorDiagnosticODBAllAPN)}, false},
		{"base 5004", ulr, &ResultError{Result: tgpp.Result{Code: diameter.ResultInvalidAVPValue}, ErrorDiagnostic: ptr(ErrorDiagnosticODBAllAPN)}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ans := roundTrip(t, NewErrorAnswer(tt.req, hssIdentity, tt.err, Features{List2: FeatureResetIDs}))
			requireNoVendorSpecificApplicationID(t, ans)

			if _, ok := ans.Find(AVPErrorDiagnostic, tgpp.VendorID); ok != tt.withDiag {
				t.Fatalf("Error-Diagnostic present = %v", ok)
			}

			_, err := ParseUpdateLocationAnswer(ans)

			var re *ResultError
			if !errors.As(err, &re) || re.Result != tt.err.Result || re.Features.List2 != FeatureResetIDs {
				t.Fatalf("err = %v", err)
			}

			if tt.withDiag && !reflect.DeepEqual(re.ErrorDiagnostic, tt.err.ErrorDiagnostic) {
				t.Fatalf("diagnostic = %v", re.ErrorDiagnostic)
			}
		})
	}
}

func TestErrorAnswerFromAVPError(t *testing.T) {
	clr := must[*diameter.Message](t)(NewCancelLocationRequest(hssEnvelope, CancelLocationRequest{IMSI: testIMSI}))
	features := Features{List1: FeatureODBAllAPN}

	ans := NewErrorAnswer(clr, mmeIdentity, tgpp.MissingAVP(AVPCancellationType, tgpp.VendorID, 4), features)

	var re *ResultError
	if _, err := ParseCancelLocationAnswer(ans); !errors.As(err, &re) || re.Code != diameter.ResultMissingAVP || re.Features != features {
		t.Fatalf("err = %v", err)
	}

	if _, ok := ans.Find(diameter.AVPFailedAVP, 0); !ok {
		t.Fatal("AVP error answer without Failed-AVP")
	}

	ans = NewErrorAnswer(clr, mmeIdentity, errors.New("boom"), features)
	if _, err := ParseCancelLocationAnswer(ans); !errors.As(err, &re) || re.Code != diameter.ResultUnableToComply || re.Features != features {
		t.Fatalf("plain error: err = %v", err)
	}
}

func TestResultErrorString(t *testing.T) {
	e := &ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUnknownEPSSubscription), ErrorDiagnostic: ptr(ErrorDiagnosticGPRSDataSubscribed)}

	want := "s6a: request failed with experimental result 5420 (vendor 10415) DIAMETER_ERROR_UNKNOWN_EPS_SUBSCRIPTION (GPRS_DATA_SUBSCRIBED)"
	if e.Error() != want {
		t.Fatalf("Error() = %q", e.Error())
	}

	if !tgpp.IsExperimental(e, tgpp.ResultErrorUnknownEPSSubscription) {
		t.Fatal("ResultError is not a tgpp result error")
	}
}

func TestMalformedAnswers(t *testing.T) {
	if _, err := ParseUpdateLocationAnswer(&diameter.Message{}); !errors.Is(err, ErrMalformedAnswer) {
		t.Fatalf("empty: err = %v", err)
	}

	req := request(CommandUpdateLocation, mmeEnvelope)
	ans := with(NewAnswer(req, hssIdentity, tgpp.Result{Code: 1001}, Features{}))

	if _, err := ParseUpdateLocationAnswer(ans); !errors.Is(err, ErrMalformedAnswer) {
		t.Fatalf("informational result: err = %v", err)
	}
}

func TestFeatures(t *testing.T) {
	f := Features{List1: FeatureODBAllAPN | FeatureODBHPLMNAPN | FeatureODBVPLMNAPN, List2: FeatureSMSInMME}
	req := request(CommandUpdateLocation, mmeEnvelope, f.avps()...)

	if got := featuresOf(roundTrip(t, req)); got != f {
		t.Fatalf("features = %+v", got)
	}

	if RequiresUnsupportedFeatures(req, Features{}) {
		t.Fatal("optional features reported as required")
	}

	required := request(CommandUpdateLocation, mmeEnvelope, tgpp.SupportedFeatures{
		VendorID: tgpp.VendorID, FeatureListID: FeatureListID2, FeatureList: uint32(FeatureResetIDs), Mandatory: true,
	}.AVP())

	if !RequiresUnsupportedFeatures(required, f) || RequiresUnsupportedFeatures(required, Features{List2: FeatureResetIDs}) {
		t.Fatal("required features mismatch")
	}

	if s := f.List1.String(); s != "ODB-all-APN|ODB-HPLMN-APN|ODB-VPLMN-APN" {
		t.Fatalf("String = %q", s)
	}

	if s := FeatureEthernetPDNTypeAPNs.String(); s != "Ethernet PDN Type APNs" {
		t.Fatalf("String = %q", s)
	}
}

func TestEnumStrings(t *testing.T) {
	for _, tt := range []struct {
		got, want string
	}{
		{CancellationSubscriptionWithdrawal.String(), "SUBSCRIPTION_WITHDRAWAL"},
		{CancellationType(9).String(), "CancellationType(9)"},
		{NetworkAccessMode(1).String(), "NetworkAccessMode(1)"},
		{PDNTypeIPv4v6.String(), "IPv4v6"},
		{PDNGWAllocationDynamic.String(), "DYNAMIC"},
		{ModifiedAddedAPNConfigurationsIncluded.String(), "MODIFIED_ADDED_APN_CONFIGURATIONS_INCLUDED"},
		{SubscriberStatusOperatorDeterminedBarring.String(), "OPERATOR_DETERMINED_BARRING"},
		{ErrorDiagnosticODBVPLMNAPN.String(), "ODB_VPLMN_APN"},
		{IMSVoiceOverPSSupported.String(), "SUPPORTED"},
		{UESRVCCSupported.String(), "UE-SRVCC-SUPPORTED"},
		{AlertReasonUEMemoryAvailable.String(), "UE_MEMORY_AVAILABLE"},
		{SMSRegistrationNoPreference.String(), "NO_PREFERENCE"},
		{DaylightSavingPlusTwoHours.String(), "PLUS_TWO_HOURS_ADJUSTMENT"},
		{NonIPDataDeliverySCEF.String(), "SCEF-BASED-DATA-DELIVERY"},
		{NonIPDataDeliveryMechanism(7).String(), "NonIPDataDeliveryMechanism(7)"},
		{(ARDWBEUTRANNotAllowed | 1<<31).String(), "WB_E_UTRAN_NOT_ALLOWED|0x80000000"},
		{NORFlags(1 << 4).String(), "RESERVED"},
		{DSRFlagServiceGapTimeWithdrawal.String(), "SERVICE_GAP_TIME_WITHDRAWAL"},
		{(HPLMNODBType1 | HPLMNODBType4).String(), "HPLMN_SPECIFIC_BARRING_TYPE_1|HPLMN_SPECIFIC_BARRING_TYPE_4"},
		{ODBAllPacketOrientedServices.String(), "ALL_PACKET_ORIENTED_SERVICES_BARRED"},
		{(ULAFlagSeparationIndication | ULAFlagMMERegisteredForSMS).String(), "SEPARATION_INDICATION|MME_REGISTERED_FOR_SMS"},
		{AIRFlagSendUEUsageType.String(), "SEND_UE_USAGE_TYPE"},
		{CLRFlagReattachRequired.String(), "REATTACH_REQUIRED"},
		{IDRFlagPCSCFRestorationRequest.String(), "P_CSCF_RESTORATION_REQUEST"},
		{IDAFlagNetworkNodeAreaRestricted.String(), "NETWORK_NODE_AREA_RESTRICTED"},
		{DSAFlagNetworkNodeAreaRestricted.String(), "NETWORK_NODE_AREA_RESTRICTED"},
		{PURFlagUEPurgedInSGSN.String(), "UE_PURGED_IN_SGSN"},
		{PUAFlagFreezePTMSI.String(), "FREEZE_P_TMSI"},
		{ULRFlags(0).String(), "0"},
	} {
		if tt.got != tt.want {
			t.Errorf("got %q, want %q", tt.got, tt.want)
		}
	}
}

func TestKnown(t *testing.T) {
	for name, tt := range map[string]struct {
		known, unknown interface{ Known() bool }
	}{
		"ULR-Flags":  {ULRFlagDisasterRoamingServiceIndication, ULRFlags(1 << 13)},
		"ULA-Flags":  {ULAFlagMMERegisteredForSMS, ULAFlags(1 << 2)},
		"AIR-Flags":  {AIRFlagSendUEUsageType, AIRFlags(1 << 2)},
		"CLR-Flags":  {CLRFlagS6aS6dIndicator, CLRFlags(1 << 2)},
		"IDR-Flags":  {IDRFlagPCSCFRestorationRequest, IDRFlags(1 << 9)},
		"IDA-Flags":  {IDAFlagNetworkNodeAreaRestricted, IDAFlags(1 << 1)},
		"DSA-Flags":  {DSAFlagNetworkNodeAreaRestricted, DSAFlags(1 << 1)},
		"DSR-Flags":  {DSRFlagPDNSubscriptionContextsWithdrawal, DSRFlagPDNSubscriptionContextsWithdrawal | DSRFlagPDPContextsWithdrawal},
		"PUR-Flags":  {PURFlagUEPurgedInMME, PURFlags(1 << 2)},
		"PUA-Flags":  {PUAFlagFreezeMTMSI, PUAFlags(1 << 2)},
		"NOR-Flags":  {NORFlagRemovalOfMMERegistrationForSMS, NORFlags(1 << 4)},
		"ODB":        {ODBAllOutgoingInternationalExceptHPLMNAndInterzonal, OperatorDeterminedBarring(1 << 9)},
		"HPLMN-ODB":  {HPLMNODBType4, HPLMNODB(1 << 4)},
		"ARD":        {ARDNREnhancedRedCapNotAllowed, AccessRestrictionData(1 << 31)},
		"PDN-Type":   {PDNTypeIPv4OrIPv6, PDNType(4)},
		"NAM":        {NetworkAccessPacketAndCircuit, NetworkAccessMode(1)},
		"Diagnostic": {ErrorDiagnosticODBHPLMNAPN, ErrorDiagnostic(5)},
		"Non-IP":     {NonIPDataDeliverySCEF, NonIPDataDeliveryMechanism(2)},
		"QoS":        {EPSSubscribedQoSProfile{QCI: 9, ARP: tgpp.DefaultAllocationRetentionPriority(1)}, EPSSubscribedQoSProfile{QCI: 1, ARP: tgpp.DefaultAllocationRetentionPriority(1)}},
		"QoS ARP":    {EPSSubscribedQoSProfile{QCI: 254, ARP: tgpp.DefaultAllocationRetentionPriority(15)}, EPSSubscribedQoSProfile{QCI: 9, ARP: tgpp.DefaultAllocationRetentionPriority(16)}},
		"AMBR":       {AMBR{UL: 1, DL: 4294968000}, AMBR{UL: 0, DL: 1}},
		"AMBR gap":   {AMBR{UL: 1<<32 - 1, DL: 1}, AMBR{UL: 1 << 32, DL: 1}},
	} {
		if !tt.known.Known() || tt.unknown.Known() {
			t.Errorf("%s: Known = %v, %v", name, tt.known.Known(), tt.unknown.Known())
		}
	}

	if !UEUsageType(128).OperatorSpecific() || UEUsageType(127).OperatorSpecific() {
		t.Fatal("UE-Usage-Type operator range")
	}
}
