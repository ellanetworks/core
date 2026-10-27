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
	testResetID   = "\x01\x02"
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

func avpResult(t *testing.T, err error) uint32 {
	t.Helper()

	var avpErr *diameter.AVPError
	if !errors.As(err, &avpErr) {
		t.Fatalf("err = %v, want an AVP error", err)
	}

	return avpErr.ResultCode
}

func request(command uint32, env tgpp.Envelope, avps ...diameter.AVP) *diameter.Message {
	return &diameter.Message{
		Flags:         diameter.FlagRequest | diameter.FlagProxiable,
		CommandCode:   command,
		ApplicationID: ApplicationID,
		AVPs:          append(env.AVPs(), avps...),
	}
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

func encodeAVP(t *testing.T, a diameter.AVP) []byte {
	t.Helper()

	m := &diameter.Message{AVPs: []diameter.AVP{a}}

	b, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return b[20:]
}

func testSubscription() SubscriptionData {
	return SubscriptionData{
		SubscriberStatus:          ptr(SubscriberStatusServiceGranted),
		MSISDN:                    testMSISDN,
		NetworkAccessMode:         ptr(NetworkAccessOnlyPacket),
		OperatorDeterminedBarring: ptr(ODBRoamerAccessHPLMNAP),
		AccessRestrictionData:     ptr(ARDUTRANNotAllowed | ARDGERANNotAllowed),
		APNOIReplacement:          "mnc001.mcc001.gprs",
		ChargingCharacteristics:   "0800",
		AMBR:                      &AMBR{UplinkBPS: testUEAMBRUL, DownlinkBPS: testUEAMBRDL},
		APNConfigurationProfile: &APNConfigurationProfile{
			ContextIdentifier: 1,
			Included:          AllAPNConfigurationsIncluded,
			Configurations: []APNConfiguration{
				{
					ContextIdentifier: 1,
					PDNType:           PDNTypeIPv4,
					ServiceSelection:  "internet",
					QoS: &EPSSubscribedQoSProfile{
						QCI: 9, ARP: AllocationRetentionPriority{PriorityLevel: 8, PreemptionVulnerability: true},
					},
					VPLMNDynamicAddressAllowed: true,
					AMBR:                       &AMBR{UplinkBPS: testAPNAMBRUL, DownlinkBPS: testAPNAMBRDL},
					Interworking5GSIndicator:   true,
				},
				{
					ContextIdentifier:      2,
					PDNType:                PDNTypeIPv4v6,
					ServiceSelection:       "ims",
					ServedPartyIPAddresses: []netip.Addr{netip.MustParseAddr("10.45.0.2"), netip.MustParseAddr("2001:db8::")},
					QoS: &EPSSubscribedQoSProfile{
						QCI: 5, ARP: AllocationRetentionPriority{PriorityLevel: 1, PreemptionCapability: true},
					},
					PDNGateway: &MIP6AgentInfo{
						HomeAgentAddresses: []netip.Addr{netip.MustParseAddr("192.0.2.1")},
						HomeAgentHost:      "topon.s5.pgw.epc.mnc001.mcc001.3gppnetwork.org",
						HomeAgentRealm:     homeRealm,
					},
					PDNGWAllocationType:      ptr(PDNGWAllocationStatic),
					VisitedNetworkIdentifier: "mnc001.mcc001.3gppnetwork.org",
					APNOIReplacement:         "ims.mnc001.mcc001.gprs",
				},
				{
					ContextIdentifier: 3,
					PDNType:           PDNTypeIPv4OrIPv6,
					ServiceSelection:  WildcardAPN,
					SpecificAPNInfo: []SpecificAPNInfo{{
						ServiceSelection: "corp",
						PDNGateway:       MIP6AgentInfo{HomeAgentAddresses: []netip.Addr{netip.MustParseAddr("2001:db8::1")}},
					}},
				},
			},
		},
		RATFrequencySelectionPriorityID: 12,
		SubscribedPeriodicRAUTAUTimer:   testRAUTimer,
		UEUsageType:                     ptr(uint32(0)),
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

	if s := r.Flags.String(); s != "S6A_S6D_INDICATOR|INITIAL_ATTACH_INDICATOR" {
		t.Fatalf("String = %q", s)
	}
}

func TestEUTRANVectorEncoding(t *testing.T) {
	v := EUTRANVector{
		ItemNumber: 1,
		RAND:       bytes.Repeat([]byte{0x11}, 16),
		XRES:       bytes.Repeat([]byte{0x22}, 8),
		AUTN:       bytes.Repeat([]byte{0x33}, 16),
		KASME:      bytes.Repeat([]byte{0x44}, 32),
	}

	want := "00000586c0000094000028af" +
		"0000058bc0000010000028af00000001" +
		"000005a7c000001c000028af" + "11111111111111111111111111111111" +
		"000005a8c0000014000028af" + "2222222222222222" +
		"000005a9c000001c000028af" + "33333333333333333333333333333333" +
		"000005aac000002c000028af" + "4444444444444444444444444444444444444444444444444444444444444444"

	a := must[diameter.AVP](t)(v.avp())
	if got := encodeAVP(t, a); !bytes.Equal(got, mustHex(t, want)) {
		t.Fatalf("E-UTRAN-Vector =\n%x\nwant\n%s", got, want)
	}

	back, err := parseEUTRANVector(a)
	if err != nil || !reflect.DeepEqual(back, v) {
		t.Fatalf("parse = %+v, %v", back, err)
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
		{AVPUserID, false},
		{AVPHomogeneousSupportOfIMSVoiceOverPSSession, false},
		{AVPErrorDiagnostic, false},
		{AVPCLRFlags, false},
		{AVPPURFlags, false},
		{AVPSubscribedPeriodicRAUTAUTimer, false},
		{AVPResetID, false},
		{tgpp.AVPMSISDN, true},
		{tgpp.AVPQoSClassIdentifier, true},
		{tgpp.AVPAllocationRetentionPriority, false},
		{tgpp.AVPPriorityLevel, false},
		{tgpp.AVPMaxRequestedBandwidthUL, true},
		{tgpp.AVPExtendedMaxRequestedBWUL, false},
		{AVPServedPartyIPAddress, true},
	} {
		if got := avpFlags(tt.code)&diameter.AVPFlagMandatory != 0; got != tt.mandatory {
			t.Errorf("AVP %d: M = %v, want %v", tt.code, got, tt.mandatory)
		}
	}
}

func TestAMBREncoding(t *testing.T) {
	for _, tt := range []struct {
		bps, decoded uint64
		extended     bool
	}{
		{0, 0, false},
		{1<<32 - 1, 1<<32 - 1, false},
		{1 << 32, 4294968000, true},
		{6_000_000_000, 6_000_000_000, true},
		{6_000_000_999, 6_000_000_000, true},
	} {
		a := must[diameter.AVP](t)(AMBR{UplinkBPS: tt.bps, DownlinkBPS: 1}.avp())

		inner := must[[]diameter.AVP](t)(a.Grouped())
		if _, ok := diameter.Find(inner, tgpp.AVPExtendedMaxRequestedBWUL, tgpp.VendorID); ok != tt.extended {
			t.Errorf("%d: extended = %v", tt.bps, ok)
		}

		back := must[*AMBR](t)(parseAMBR(a))
		if back.UplinkBPS != tt.decoded || back.DownlinkBPS != 1 {
			t.Errorf("%d decodes to %+v", tt.bps, back)
		}
	}

	if _, err := (AMBR{UplinkBPS: 1 << 63}).avp(); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("huge AMBR err = %v", err)
	}
}

func TestErrorAnswerDiagnostic(t *testing.T) {
	ulr := must[*diameter.Message](t)(NewUpdateLocationRequest(mmeEnvelope, UpdateLocationRequest{
		IMSI: testIMSI, RATType: tgpp.RATEUTRAN, VisitedPLMN: visitedPLMN, Flags: ULRFlagS6aS6dIndicator,
	}))

	for _, tt := range []struct {
		name string
		err  *ResultError
	}{
		{"unknown EPS subscription", &ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUnknownEPSSubscription), ErrorDiagnostic: ptr(ErrorDiagnosticNoGPRSDataSubscribed)}},
		{"roaming not allowed", &ResultError{Result: tgpp.Experimental(tgpp.ResultErrorRoamingNotAllowed), ErrorDiagnostic: ptr(ErrorDiagnosticODBAllAPN)}},
		{"user unknown", &ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUserUnknown)}},
		{"RAT not allowed", &ResultError{Result: tgpp.Experimental(tgpp.ResultErrorRATNotAllowed)}},
		{"unable to comply", &ResultError{Result: tgpp.Result{Code: diameter.ResultUnableToComply}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ans := roundTrip(t, NewErrorAnswer(ulr, hssIdentity, tt.err))
			requireNoVendorSpecificApplicationID(t, ans)

			_, err := ParseUpdateLocationAnswer(ans)

			var re *ResultError
			if !errors.As(err, &re) || re.Result != tt.err.Result || !reflect.DeepEqual(re.ErrorDiagnostic, tt.err.ErrorDiagnostic) {
				t.Fatalf("err = %v", err)
			}
		})
	}

	clr := must[*diameter.Message](t)(NewCancelLocationRequest(hssEnvelope, CancelLocationRequest{IMSI: testIMSI}))

	ans := NewErrorAnswer(clr, mmeIdentity, &ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUserUnknown), ErrorDiagnostic: ptr(ErrorDiagnosticODBAllAPN)})
	if _, ok := ans.Find(AVPErrorDiagnostic, tgpp.VendorID); ok {
		t.Fatal("CLA carries Error-Diagnostic")
	}

	ans = NewErrorAnswer(clr, mmeIdentity, tgpp.MissingAVP(AVPCancellationType, tgpp.VendorID, 4))

	var re *ResultError
	if _, err := ParseCancelLocationAnswer(ans); !errors.As(err, &re) || re.Code != diameter.ResultMissingAVP {
		t.Fatalf("err = %v", err)
	}

	if _, ok := ans.Find(diameter.AVPFailedAVP, 0); !ok {
		t.Fatal("AVP error answer without Failed-AVP")
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
	ans := NewAnswer(req, hssIdentity, tgpp.Result{Code: diameter.ResultSuccess}, Features{})
	ans.AVPs = append(ans.AVPs, vendorGroup(AVPSubscriptionData, vendorU32(AVPSubscriberStatus, 7)))

	if _, err := ParseUpdateLocationAnswer(ans); !errors.Is(err, ErrMalformedAnswer) {
		t.Fatalf("bad Subscriber-Status: err = %v", err)
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
}

func TestEnumStrings(t *testing.T) {
	for _, tt := range []struct {
		got, want string
	}{
		{CancellationSubscriptionWithdrawal.String(), "SUBSCRIPTION_WITHDRAWAL"},
		{CancellationType(9).String(), "CancellationType(9)"},
		{NetworkAccessMode(1).String(), "NetworkAccessMode(1)"},
		{PDNTypeIPv4v6.String(), "IPv4v6"},
		{ErrorDiagnosticODBVPLMNAPN.String(), "ODB_VPLMN_APN"},
		{(ARDWBEUTRANNotAllowed | 1<<31).String(), "WB_E_UTRAN_NOT_ALLOWED|0x80000000"},
		{NORFlags(1 << 4).String(), "RESERVED"},
		{DSRFlagServiceGapTimeWithdrawal.String(), "SERVICE_GAP_TIME_WITHDRAWAL"},
	} {
		if tt.got != tt.want {
			t.Errorf("got %q, want %q", tt.got, tt.want)
		}
	}
}

func mustAddr(s string) netip.Addr { return netip.MustParseAddr(s) }
