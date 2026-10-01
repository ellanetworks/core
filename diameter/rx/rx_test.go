// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const (
	testIMSRealm = "ims.mnc001.mcc001.3gppnetwork.org"
	testEPCRealm = "epc.mnc001.mcc001.3gppnetwork.org"
	testPublic   = "sip:001010000000001@ims.mnc001.mcc001.3gppnetwork.org"
)

var (
	afIdentity   = diameter.Identity{OriginHost: "pcscf.ims.mnc001.mcc001.3gppnetwork.org", OriginRealm: testIMSRealm}
	pcrfIdentity = diameter.Identity{OriginHost: "pcrf.epc.mnc001.mcc001.3gppnetwork.org", OriginRealm: testEPCRealm}
	afEnvelope   = tgpp.Envelope{
		SessionID:        "pcscf.ims.mnc001.mcc001.3gppnetwork.org;1;1",
		Origin:           afIdentity,
		DestinationRealm: testEPCRealm,
	}
	pcrfEnvelope = tgpp.Envelope{
		SessionID:        afEnvelope.SessionID,
		Origin:           pcrfIdentity,
		DestinationHost:  afIdentity.OriginHost,
		DestinationRealm: testIMSRealm,
	}
)

func ptr[T any](v T) *T {
	return &v
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

func mustMessage(t testing.TB) func(*diameter.Message, error) *diameter.Message {
	return func(m *diameter.Message, err error) *diameter.Message {
		t.Helper()

		if err != nil {
			t.Fatal(err)
		}

		return m
	}
}

func avpError(t *testing.T, err error) *diameter.AVPError {
	t.Helper()

	var avpErr *diameter.AVPError
	if !errors.As(err, &avpErr) {
		t.Fatalf("err = %v, want an AVP error", err)
	}

	return avpErr
}

func request(command uint32, env tgpp.Envelope, avps ...diameter.AVP) *diameter.Message {
	return newRequest(env, command, avps...)
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

var mandatoryBit = map[diameter.AVPKey]bool{
	vendorKey(AVPAbortCause):                           true,
	vendorKey(AVPAccessNetworkChargingAddress):         true,
	vendorKey(AVPAccessNetworkChargingIdentifier):      true,
	vendorKey(AVPAccessNetworkChargingIdentifierValue): true,
	vendorKey(AVPAcceptableServiceInfo):                true,
	vendorKey(AVPAFApplicationIdentifier):              true,
	vendorKey(AVPAFChargingIdentifier):                 true,
	vendorKey(AVPCodecData):                            true,
	vendorKey(AVPFlowDescription):                      true,
	vendorKey(AVPFlowNumber):                           true,
	vendorKey(AVPFlows):                                true,
	vendorKey(AVPFlowStatus):                           true,
	vendorKey(AVPFlowUsage):                            true,
	vendorKey(AVPSpecificAction):                       true,
	vendorKey(AVPMaxRequestedBandwidthDL):              true,
	vendorKey(AVPMaxRequestedBandwidthUL):              true,
	vendorKey(AVPMediaComponentDescription):            true,
	vendorKey(AVPMediaComponentNumber):                 true,
	vendorKey(AVPMediaSubComponent):                    true,
	vendorKey(AVPMediaType):                            true,
	vendorKey(AVPRRBandwidth):                          true,
	vendorKey(AVPRSBandwidth):                          true,
	vendorKey(AVPSIPForkingIndication):                 true,
	vendorKey(AVPServiceURN):                           true,
	vendorKey(AVPServiceInfoStatus):                    true,
	vendorKey(AVPAFSignallingProtocol):                 false,
	vendorKey(AVPRxRequestType):                        false,
	vendorKey(AVPIPDomainID):                           false,
	vendorKey(AVPRetryInterval):                        false,
	vendorKey(tgpp.AVPFeatureListID):                   false,
	vendorKey(tgpp.AVPFeatureList):                     false,
	{Code: AVPFramedIPAddress}:                         true,
	{Code: AVPFramedIPv6Prefix}:                        true,
	{Code: AVPCalledStationID}:                         true,
	{Code: AVPSubscriptionID}:                          true,
	{Code: AVPSubscriptionIDType}:                      true,
	{Code: AVPSubscriptionIDData}:                      true,
	{Code: diameter.AVPTerminationCause}:               true,
	{Code: diameter.AVPAuthApplicationID}:              true,
	{Code: diameter.AVPAuthSessionState}:               true,
	{Code: diameter.AVPVendorID}:                       true,
}

var groupedAVPs = map[diameter.AVPKey]bool{
	vendorKey(AVPAccessNetworkChargingIdentifier): true,
	vendorKey(AVPAcceptableServiceInfo):           true,
	vendorKey(AVPFlows):                           true,
	vendorKey(AVPMediaComponentDescription):       true,
	vendorKey(AVPMediaSubComponent):               true,
	vendorKey(tgpp.AVPSupportedFeatures):          true,
	{Code: AVPSubscriptionID}:                     true,
}

var baseAVPs = map[uint32]bool{
	diameter.AVPSessionID: true, diameter.AVPOriginHost: true, diameter.AVPOriginRealm: true,
	diameter.AVPDestinationHost: true, diameter.AVPDestinationRealm: true, diameter.AVPResultCode: true,
	diameter.AVPExperimentalResult: true,
}

func checkFlags(t *testing.T, avps []diameter.AVP, requiredFeatures bool) {
	t.Helper()

	for _, a := range avps {
		key := diameter.AVPKey{Code: a.Code, VendorID: a.VendorID}
		if a.VendorID == 0 && baseAVPs[a.Code] {
			continue
		}

		mandatory, known := mandatoryBit[key]
		if key == vendorKey(tgpp.AVPSupportedFeatures) {
			mandatory, known = requiredFeatures && a.Flags&diameter.AVPFlagMandatory != 0, true
		}

		if !known {
			t.Errorf("AVP %d (vendor %d) not in the flag table", a.Code, a.VendorID)
		}

		if (a.Flags&diameter.AVPFlagMandatory != 0) != mandatory {
			t.Errorf("AVP %d M bit = %v, want %v", a.Code, a.Flags&diameter.AVPFlagMandatory != 0, mandatory)
		}

		if (a.Flags&diameter.AVPFlagVendor != 0) != (a.VendorID != 0) {
			t.Errorf("AVP %d V bit = %v with vendor %d", a.Code, a.Flags&diameter.AVPFlagVendor != 0, a.VendorID)
		}

		if groupedAVPs[key] {
			inner, err := a.Grouped()
			if err != nil {
				t.Fatal(err)
			}

			checkFlags(t, inner, false)
		}
	}
}

func fullMediaComponent() MediaComponent {
	return MediaComponent{
		Number: 1,
		SubComponents: []MediaSubComponent{
			{
				FlowNumber: 1,
				FlowDescriptions: []string{
					"permit out 17 from 10.4.128.21 30000 to 192.168.101.4 1234",
					"permit in 17 from 192.168.101.4 1234 to 10.4.128.21 30000",
				},
				FlowStatus:              ptr(FlowEnabled),
				MaxRequestedBandwidthUL: ptr(uint32(41000)),
				MaxRequestedBandwidthDL: ptr(uint32(41000)),
			},
			{
				FlowNumber: 2,
				FlowDescriptions: []string{
					"permit out 17 from 10.4.128.21 30001 to 192.168.101.4 1235",
					"permit in 17 from 192.168.101.4 1235 to 10.4.128.21 30001",
				},
				FlowUsage: FlowUsageRTCP,
			},
		},
		AFApplicationIdentifier: "IMS Services",
		Type:                    ptr(MediaAudio),
		MaxRequestedBandwidthUL: ptr(uint32(41000)),
		MaxRequestedBandwidthDL: ptr(uint32(0)),
		FlowStatus:              ptr(FlowEnabled),
		RSBandwidth:             ptr(uint32(512)),
		RRBandwidth:             ptr(uint32(1537)),
		CodecData: []CodecData{
			{Direction: CodecUplink, Kind: CodecOffer, SDP: "m=audio 1234 RTP/AVP 104 96\r\na=rtpmap:104 AMR-WB/16000/1\r\n"},
			{Direction: CodecDownlink, Kind: CodecAnswer, SDP: "m=audio 30000 RTP/AVP 104\r\n"},
		},
	}
}

func signallingComponent() MediaComponent {
	return MediaComponent{
		Number: 0,
		SubComponents: []MediaSubComponent{{
			FlowNumber: 1,
			FlowDescriptions: []string{
				"permit out 17 from 10.4.128.21 5060 to 192.168.101.4 5060",
				"permit in 17 from 192.168.101.4 5060 to 10.4.128.21 5060",
			},
			FlowStatus:         ptr(FlowEnabled),
			FlowUsage:          FlowUsageAFSignalling,
			SignallingProtocol: SignallingSIP,
		}},
	}
}

func fullAARequest() AARequest {
	return AARequest{
		AFApplicationIdentifier: "IMS Services",
		MediaComponents:         []MediaComponent{fullMediaComponent(), signallingComponent()},
		ServiceInfoStatus:       ServiceInfoPreliminary,
		AFChargingIdentifier:    "icid-1",
		SIPForkingIndication:    ForkingSeveralDialogues,
		SpecificActions:         []SpecificAction{ActionIndicationOfReleaseOfBearer, ActionIPCANChange},
		SubscriptionIDs:         []SubscriptionID{{Type: SubscriptionSIPURI, Data: testPublic}, {Type: SubscriptionE164, Data: "15551230002"}},
		FramedIPAddress:         netip.MustParseAddr("192.168.101.4"),
		FramedIPv6Address:       netip.MustParseAddr("2001:db8::4"),
		CalledStationID:         "ims",
		IPDomainID:              []byte("domain-a"),
		ServiceURN:              "sos",
		RequestType:             ptr(RequestInitial),
		Features:                FeatureRel8 | FeatureRel9 | FeatureProvAFSignalFlow | FeaturePCSCFRestorationEnhancement,
		RequiredFeatures:        FeatureRel8 | FeatureRel9,
	}
}

func TestRequestsCarryTheRxHeader(t *testing.T) {
	must := mustMessage(t)

	for name, m := range map[string]*diameter.Message{
		"AAR": must(NewAARequest(afEnvelope, fullAARequest())),
		"STR": must(NewSessionTerminationRequest(afEnvelope, SessionTerminationRequest{Cause: TerminationLogout})),
		"RAR": must(NewReAuthRequest(pcrfEnvelope, ReAuthRequest{
			SpecificActions: []SpecificAction{ActionChargingCorrelationExchange, ActionIndicationOfReleaseOfBearer},
			AccessNetworkChargingIdentifiers: []AccessNetworkChargingIdentifier{{
				Value: []byte{1, 2, 3, 4}, Flows: []Flows{{MediaComponentNumber: 1, FlowNumbers: []uint32{1, 2}}},
			}},
			AccessNetworkChargingAddress: netip.MustParseAddr("10.0.0.1"),
			Flows:                        []Flows{{MediaComponentNumber: 1}},
			SubscriptionIDs:              []SubscriptionID{{Type: SubscriptionIMSI, Data: "001010000000001"}},
			AbortCause:                   ptr(AbortBearerReleased),
		})),
		"ASR": must(NewAbortSessionRequest(pcrfEnvelope, AbortSessionRequest{Cause: AbortInsufficientBearerResources})),
	} {
		t.Run(name, func(t *testing.T) {
			if m.ApplicationID != ApplicationID || m.Flags != diameter.FlagRequest|diameter.FlagProxiable || m.AVPs[0].Code != diameter.AVPSessionID {
				t.Fatalf("header = %+v", m)
			}

			app, ok := m.Find(diameter.AVPAuthApplicationID, 0)
			if v, _ := app.Unsigned32(); !ok || v != ApplicationID {
				t.Fatalf("Auth-Application-Id = %+v", app)
			}

			for _, absent := range []uint32{diameter.AVPVendorSpecificApplicationID, diameter.AVPAuthSessionState} {
				if _, ok := m.Find(absent, 0); ok {
					t.Fatalf("AVP %d sent", absent)
				}
			}

			checkFlags(t, m.AVPs, true)
		})
	}
}

func TestAnswersFollowTheFlagTable(t *testing.T) {
	must := mustMessage(t)
	aar := request(CommandAA, afEnvelope)

	for name, m := range map[string]*diameter.Message{
		"AAA": must(NewAAAnswer(aar, pcrfIdentity, AAAnswer{
			AccessNetworkChargingIdentifiers: []AccessNetworkChargingIdentifier{{Value: []byte{1}, Flows: []Flows{{MediaComponentNumber: 1, FlowNumbers: []uint32{1}}}}},
			AccessNetworkChargingAddress:     netip.MustParseAddr("2001:db8::1"),
			SubscriptionIDs:                  []SubscriptionID{{Type: SubscriptionIMSI, Data: "001010000000001"}},
			Features:                         FeatureRel8 | FeatureCHEM,
		})),
		"AAA error": must(NewAAErrorAnswer(aar, pcrfIdentity, AAError{
			ResultError: ResultError{Result: tgpp.Experimental(tgpp.ResultRequestedServiceTemporarilyNotAuthorized)},
			AcceptableServiceInfo: &AcceptableServiceInfo{
				MediaComponents:         []MediaBandwidth{{MediaComponentNumber: 1, MaxRequestedBandwidthUL: ptr(uint32(1)), MaxRequestedBandwidthDL: ptr(uint32(2))}},
				MaxRequestedBandwidthUL: ptr(uint32(3)), MaxRequestedBandwidthDL: ptr(uint32(4)),
			},
			RetryInterval: 30 * time.Second,
			Features:      FeatureRel8,
		})),
		"RAA": must(NewReAuthAnswer(request(CommandReAuth, pcrfEnvelope), afIdentity, ReAuthAnswer{})),
		"STA": must(NewSessionTerminationAnswer(request(CommandSessionTermination, afEnvelope), pcrfIdentity, SessionTerminationAnswer{})),
		"ASA": must(NewAbortSessionAnswer(request(CommandAbortSession, pcrfEnvelope), afIdentity, AbortSessionAnswer{})),
	} {
		t.Run(name, func(t *testing.T) {
			checkFlags(t, m.AVPs, false)
		})
	}
}

func TestAnswersCarryTheRxHeader(t *testing.T) {
	ok := tgpp.Result{Code: diameter.ResultSuccess}

	for _, command := range []uint32{CommandAA, CommandReAuth, CommandSessionTermination, CommandAbortSession} {
		ans := NewAnswer(request(command, afEnvelope), pcrfIdentity, ok)

		_, hasApp := ans.Find(diameter.AVPAuthApplicationID, 0)
		if hasApp != (command == CommandAA) {
			t.Errorf("command %d: Auth-Application-Id present = %v", command, hasApp)
		}

		if _, ok := ans.Find(diameter.AVPAuthSessionState, 0); ok {
			t.Errorf("command %d: Auth-Session-State without one in the request", command)
		}

		if ans.ApplicationID != ApplicationID || ans.CommandCode != command || ans.IsRequest() {
			t.Errorf("command %d: header = %+v", command, ans)
		}
	}
}

func TestAAAnswerEchoesAuthSessionState(t *testing.T) {
	for state, echoed := range map[uint32]bool{
		diameter.AuthSessionStateMaintained:        true,
		diameter.AuthSessionStateNoStateMaintained: true,
		7: false,
	} {
		req := request(CommandAA, afEnvelope, diameter.Unsigned32(diameter.AVPAuthSessionState, diameter.AVPFlagMandatory, 0, state))

		a, ok := NewAnswer(req, pcrfIdentity, tgpp.Result{Code: diameter.ResultSuccess}).Find(diameter.AVPAuthSessionState, 0)
		if v, _ := a.Unsigned32(); ok != echoed || (echoed && v != state) {
			t.Errorf("state %d: echoed %+v", state, a)
		}
	}
}

func TestErrorAnswer(t *testing.T) {
	req := request(CommandAA, afEnvelope)
	ans := NewErrorAnswer(req, pcrfIdentity, tgpp.MissingAVP(AVPMediaComponentNumber, tgpp.VendorID))

	if _, ok := ans.Find(diameter.AVPFailedAVP, 0); !ok {
		t.Fatal("no Failed-AVP")
	}

	if _, ok := ans.Find(diameter.AVPAuthApplicationID, 0); !ok {
		t.Fatal("AAA without Auth-Application-Id")
	}

	_, err := ParseAAAnswer(ans)

	var re *ResultError
	if !errors.As(err, &re) || re.Code != diameter.ResultMissingAVP || re.Error() != "rx: request failed with result 5005 DIAMETER_MISSING_AVP" {
		t.Fatalf("err = %v", err)
	}

	generic := NewErrorAnswer(request(CommandSessionTermination, afEnvelope), pcrfIdentity, errors.New("boom"))
	if r, _ := tgpp.ParseResult(generic); r.Code != diameter.ResultUnableToComply {
		t.Fatalf("generic error = %+v", r)
	}

	wrapped := NewErrorAnswer(req, pcrfIdentity, &AAError{ResultError: ResultError{Result: tgpp.Experimental(tgpp.ResultIPCANSessionNotAvailable)}})
	if r, _ := tgpp.ParseResult(wrapped); !r.IsExperimental(tgpp.ResultIPCANSessionNotAvailable) {
		t.Fatalf("AA error = %+v", r)
	}

	for name, parse := range map[string]func(*diameter.Message) error{
		"AAA": func(m *diameter.Message) error { _, err := ParseAAAnswer(m); return err },
		"RAA": func(m *diameter.Message) error { _, err := ParseReAuthAnswer(m); return err },
		"STA": func(m *diameter.Message) error { _, err := ParseSessionTerminationAnswer(m); return err },
		"ASA": func(m *diameter.Message) error { _, err := ParseAbortSessionAnswer(m); return err },
	} {
		if err := parse(&diameter.Message{}); !errors.Is(err, ErrMalformedAnswer) {
			t.Errorf("%s: empty answer = %v", name, err)
		}
	}
}

func TestSuccessBuildersRejectErrors(t *testing.T) {
	failure := tgpp.Result{Code: diameter.ResultUnknownSessionID}

	for name, build := range map[string]func() (*diameter.Message, error){
		"AAA": func() (*diameter.Message, error) {
			return NewAAAnswer(request(CommandAA, afEnvelope), pcrfIdentity, AAAnswer{Result: failure})
		},
		"RAA": func() (*diameter.Message, error) {
			return NewReAuthAnswer(request(CommandReAuth, pcrfEnvelope), afIdentity, ReAuthAnswer{Result: failure})
		},
		"STA": func() (*diameter.Message, error) {
			return NewSessionTerminationAnswer(request(CommandSessionTermination, afEnvelope), pcrfIdentity, SessionTerminationAnswer{Result: failure})
		},
		"ASA": func() (*diameter.Message, error) {
			return NewAbortSessionAnswer(request(CommandAbortSession, pcrfEnvelope), afIdentity, AbortSessionAnswer{Result: failure})
		},
		"AAA error": func() (*diameter.Message, error) {
			return NewAAErrorAnswer(request(CommandAA, afEnvelope), pcrfIdentity, AAError{ResultError: ResultError{Result: tgpp.Result{Code: diameter.ResultSuccess}}})
		},
	} {
		if _, err := build(); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestResultErrorsCarryTheResult(t *testing.T) {
	for name, tc := range map[string]struct {
		command uint32
		parse   func(*diameter.Message) error
	}{
		"RAA": {CommandReAuth, func(m *diameter.Message) error { _, err := ParseReAuthAnswer(m); return err }},
		"STA": {CommandSessionTermination, func(m *diameter.Message) error { _, err := ParseSessionTerminationAnswer(m); return err }},
		"ASA": {CommandAbortSession, func(m *diameter.Message) error { _, err := ParseAbortSessionAnswer(m); return err }},
	} {
		ans := NewAnswer(request(tc.command, pcrfEnvelope), afIdentity, tgpp.Result{Code: diameter.ResultUnknownSessionID})

		var re *ResultError
		if err := tc.parse(roundTrip(t, ans)); !errors.As(err, &re) || re.Code != diameter.ResultUnknownSessionID {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestRxResultNames(t *testing.T) {
	for r, want := range map[tgpp.Result]string{
		tgpp.Experimental(tgpp.ResultIPCANSessionNotAvailable):                 "experimental result 5065 (vendor 10415) IP-CAN_SESSION_NOT_AVAILABLE",
		tgpp.Experimental(tgpp.ResultRequestedServiceTemporarilyNotAuthorized): "experimental result 4261 (vendor 10415) REQUESTED_SERVICE_TEMPORARILY_NOT_AUTHORIZED",
		tgpp.Experimental(tgpp.ResultFilterRestrictions):                       "experimental result 5062 (vendor 10415) FILTER_RESTRICTIONS",
		{Code: diameter.ResultUnknownSessionID}:                                "result 5002 DIAMETER_UNKNOWN_SESSION_ID",
	} {
		if got := r.String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}
}

func TestRequiredFeatures(t *testing.T) {
	for name, r := range map[string]AARequest{
		"both lists required": {
			Features:         FeatureRel8 | FeatureNetLoc | FeaturePCSCFRestorationEnhancement,
			RequiredFeatures: FeatureRel8 | FeatureNetLoc | FeaturePCSCFRestorationEnhancement,
		},
		"list 2 optional": {
			Features:         FeatureRel8 | FeaturePCSCFRestorationEnhancement,
			RequiredFeatures: FeatureRel8,
		},
		"partly required": {
			Features:         FeatureRel8 | FeatureRel9 | FeatureExtendedMaxRequestedBWNR,
			RequiredFeatures: FeatureRel8,
		},
		"none required": {Features: FeatureRel8},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := NewAARequest(afEnvelope, r)
			if err != nil {
				t.Fatal(err)
			}

			for _, a := range diameter.FindAll(req.AVPs, tgpp.AVPSupportedFeatures, tgpp.VendorID) {
				f, err := tgpp.ParseSupportedFeatures(a)
				if err != nil {
					t.Fatal(err)
				}

				required := listFeatures(f.FeatureListID, f.FeatureList)&^r.RequiredFeatures == 0
				if f.Mandatory != required {
					t.Errorf("Supported-Features %+v, M bit = %v", f, f.Mandatory)
				}
			}

			got, err := ParseAARequest(roundTrip(t, req))
			if err != nil || got.Features != r.Features || got.RequiredFeatures != r.RequiredFeatures {
				t.Fatalf("features = %s, required %s, %v", got.Features, got.RequiredFeatures, err)
			}
		})
	}

	_, err := NewAARequest(afEnvelope, AARequest{Features: FeatureRel8, RequiredFeatures: FeatureRel9})
	if !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("required but not advertised = %v", err)
	}
}

func TestAnswerFeaturesAreNeverMandatory(t *testing.T) {
	req := mustMessage(t)(NewAARequest(afEnvelope, AARequest{Features: FeatureRel8, RequiredFeatures: FeatureRel8}))

	ans, err := NewAAAnswer(req, pcrfIdentity, AAAnswer{Features: FeatureRel8 | FeaturePCSCFRestorationEnhancement})
	if err != nil {
		t.Fatal(err)
	}

	features := diameter.FindAll(ans.AVPs, tgpp.AVPSupportedFeatures, tgpp.VendorID)
	if len(features) != 2 {
		t.Fatalf("answer has %d Supported-Features", len(features))
	}

	for _, a := range features {
		if a.Flags&diameter.AVPFlagMandatory != 0 {
			t.Fatalf("answer Supported-Features = %+v", a)
		}
	}

	got, err := ParseAAAnswer(ans)
	if err != nil || got.Features != FeatureRel8|FeaturePCSCFRestorationEnhancement {
		t.Fatalf("answer features = %s, %v", got.Features, err)
	}
}
