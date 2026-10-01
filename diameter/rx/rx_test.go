// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"errors"
	"fmt"
	"net/netip"
	"reflect"
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
	m, err := newRequest(env, command, avps...)
	if err != nil {
		panic(err)
	}

	return m
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
	vendorKey(AVPMinRequestedBandwidthUL):              false,
	vendorKey(AVPMinRequestedBandwidthDL):              false,
	vendorKey(AVPMaxSupportedBandwidthUL):              false,
	vendorKey(AVPMaxSupportedBandwidthDL):              false,
	vendorKey(AVPMinDesiredBandwidthUL):                false,
	vendorKey(AVPMinDesiredBandwidthDL):                false,
	vendorKey(AVPExtendedMaxRequestedBWUL):             false,
	vendorKey(AVPExtendedMaxRequestedBWDL):             false,
	vendorKey(AVPExtendedMaxSupportedBWUL):             false,
	vendorKey(AVPExtendedMaxSupportedBWDL):             false,
	vendorKey(AVPExtendedMinDesiredBWUL):               false,
	vendorKey(AVPExtendedMinDesiredBWDL):               false,
	vendorKey(AVPExtendedMinRequestedBWUL):             false,
	vendorKey(AVPExtendedMinRequestedBWDL):             false,
	vendorKey(AVPContentVersion):                       false,
	vendorKey(AVPMediaComponentStatus):                 false,
	vendorKey(AVPRequiredAccessInfo):                   false,
	vendorKey(AVPNID):                                  false,
	vendorKey(AVPServingSatelliteIdentity):             false,
	vendorKey(AVPPCSessionRecoveryStatus):              false,
	vendorKey(tgpp.AVPIPCANType):                       true,
	vendorKey(tgpp.AVPRATType):                         false,
	vendorKey(tgpp.AVPANGWAddress):                     false,
	vendorKey(tgpp.AVPANTrusted):                       false,
	vendorKey(tgpp.AVP3GPPSGSNMCCMNC):                  false,
	vendorKey(tgpp.AVP3GPPUserLocationInfo):            false,
	vendorKey(tgpp.AVP3GPPMSTimeZone):                  false,
	vendorKey(tgpp.AVPTWANIdentifier):                  false,
	vendorKey(tgpp.AVPUELocalIPAddress):                false,
	vendorKey(tgpp.AVPUDPSourcePort):                   false,
	vendorKey(tgpp.AVPTCPSourcePort):                   false,
	vendorKey(tgpp.AVPUserLocationInfoTime):            false,
	vendorKey(tgpp.AVPRANNASReleaseCause):              false,
	vendorKey(tgpp.AVPNetLocAccessSupport):             false,
	{Code: diameter.AVPFinalUnitAction}:                true,
	vendorKey(tgpp.AVPFeatureListID):                   false,
	vendorKey(tgpp.AVPFeatureList):                     false,
	{Code: diameter.AVPFramedIPAddress}:                true,
	{Code: diameter.AVPFramedIPv6Prefix}:               true,
	{Code: diameter.AVPCalledStationID}:                true,
	{Code: diameter.AVPSubscriptionID}:                 true,
	{Code: diameter.AVPSubscriptionIDType}:             true,
	{Code: diameter.AVPSubscriptionIDData}:             true,
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
	{Code: diameter.AVPSubscriptionID}:            true,
}

var baseAVPs = map[uint32]bool{
	diameter.AVPSessionID: true, diameter.AVPOriginHost: true, diameter.AVPOriginRealm: true,
	diameter.AVPDestinationHost: true, diameter.AVPDestinationRealm: true, diameter.AVPResultCode: true,
	diameter.AVPExperimentalResult: true,
}

func checkFlags(t *testing.T, avps []diameter.AVP, featuresMandatory bool) {
	t.Helper()

	for _, a := range avps {
		key := diameter.AVPKey{Code: a.Code, VendorID: a.VendorID}
		if a.VendorID == 0 && baseAVPs[a.Code] {
			continue
		}

		mandatory, known := mandatoryBit[key]
		if key == vendorKey(tgpp.AVPSupportedFeatures) {
			mandatory, known = featuresMandatory, true
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

func fullAccessNetwork() AccessNetwork {
	return AccessNetwork{
		IPCANType:     ptr(IPCANNon3GPPEPS),
		RATType:       ptr(RATWLAN),
		ANTrusted:     ptr(ANTrustedUntrusted),
		ANGWAddresses: []netip.Addr{netip.MustParseAddr("10.0.0.9"), netip.MustParseAddr("2001:db8::9")},
	}
}

func fullUserLocation() UserLocation {
	return UserLocation{
		UserLocationInfo:         []byte{0x82, 0x00, 0xf1, 0x10, 0x00, 0x01, 0x00, 0xf1, 0x10, 0x00, 0x00, 0x00, 0x01},
		UserLocationInfoTime:     time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
		MSTimeZone:               []byte{0x40, 0x00},
		TWANIdentifier:           []byte("twan"),
		UELocalIPAddress:         netip.MustParseAddr("192.0.2.7"),
		UDPSourcePort:            4500,
		TCPSourcePort:            443,
		ServingSatelliteIdentity: []byte("sat-1"),
		RANNASReleaseCauses:      [][]byte{{0x10, 0x01}, {0x20, 0x02}},
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
				FlowStatus:              ptr(FlowStatusEnabled),
				MaxRequestedBandwidthUL: ptr(Bandwidth(41000)),
				MaxRequestedBandwidthDL: ptr(Bandwidth(41000)),
			},
			{
				FlowNumber: 2,
				FlowDescriptions: []string{
					"permit out 17 from 10.4.128.21 30001 to 192.168.101.4 1235",
					"permit in 17 from 192.168.101.4 1235 to 10.4.128.21 30001",
				},
				FlowUsage: ptr(FlowUsageRTCP),
			},
		},
		AFApplicationIdentifier: "IMS Services",
		Type:                    ptr(MediaAudio),
		MaxRequestedBandwidthUL: ptr(Bandwidth(41000)),
		MaxRequestedBandwidthDL: ptr(Bandwidth(0)),
		MinRequestedBandwidthUL: ptr(Bandwidth(24000)),
		MinRequestedBandwidthDL: ptr(Bandwidth(5_000_000_000)),
		MaxSupportedBandwidthUL: ptr(Bandwidth(64000)),
		MaxSupportedBandwidthDL: ptr(Bandwidth(64000)),
		MinDesiredBandwidthUL:   ptr(Bandwidth(12000)),
		MinDesiredBandwidthDL:   ptr(Bandwidth(12000)),
		ContentVersion:          ptr(uint64(7)),
		FlowStatus:              ptr(FlowStatusEnabled),
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
			FlowStatus:         ptr(FlowStatusEnabled),
			FlowUsage:          ptr(FlowUsageAFSignalling),
			SignallingProtocol: SignallingProtocolSIP,
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
		SubscriptionIDs:         []SubscriptionID{{Type: SubscriptionIDSIPURI, Data: testPublic}, {Type: SubscriptionIDE164, Data: "15551230002"}},
		FramedIPAddress:         netip.MustParseAddr("192.168.101.4"),
		FramedIPv6Address:       netip.MustParseAddr("2001:db8::4"),
		CalledStationID:         "ims",
		IPDomainID:              []byte("domain-a"),
		ServiceURN:              "sos",
		RequestType:             ptr(RequestInitial),
		RequiredAccessInfo:      []RequiredAccessInfo{RequiredUserLocation, RequiredMSTimeZone},
		Features:                FeatureRel8 | FeatureRel9 | FeatureProvAFSignalFlow | FeaturePCSCFRestorationEnhancement,
		FeaturesRequired:        true,
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
			SubscriptionIDs:              []SubscriptionID{{Type: SubscriptionIDIMSI, Data: "001010000000001"}},
			AbortCause:                   ptr(AbortBearerReleased),
			AccessNetwork:                fullAccessNetwork(),
			NetLocAccessSupport:          ptr(NetLocAccessNotSupported),
			ServingNetwork:               ServingNetwork{PLMN: "00101", NID: []byte{1}},
			Location:                     fullUserLocation(),
			PCSessionRecoveryStatus:      ptr(SessionRestorationTriggered),
		})),
		"STR with access info": must(NewSessionTerminationRequest(afEnvelope, SessionTerminationRequest{
			Cause: TerminationLogout, RequiredAccessInfo: []RequiredAccessInfo{RequiredUserLocation},
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
	aar := aaRequest(allFeatures)

	for name, m := range map[string]*diameter.Message{
		"AAA": must(NewAAAnswer(aar, pcrfIdentity, AAAnswer{
			AccessNetworkChargingIdentifiers: []AccessNetworkChargingIdentifier{{Value: []byte{1}, Flows: []Flows{{MediaComponentNumber: 1, FlowNumbers: []uint32{1}}}}},
			AccessNetworkChargingAddress:     netip.MustParseAddr("2001:db8::1"),
			SubscriptionIDs:                  []SubscriptionID{{Type: SubscriptionIDIMSI, Data: "001010000000001"}},
			Features:                         FeatureRel8 | FeatureCHEM,
			AccessNetwork:                    fullAccessNetwork(),
			ServingNetwork:                   ServingNetwork{PLMN: "00101", NID: []byte{1}},
			NetLocAccessSupport:              ptr(NetLocAccessNotSupported),
			Flows: []Flows{{
				MediaComponentNumber: 1, ContentVersions: []uint64{1}, FinalUnitAction: ptr(FinalUnitRedirect),
				MediaComponentStatus: ptr(MediaComponentActive),
			}},
		})),
		"AAA error": must(NewAAErrorAnswer(aar, pcrfIdentity, AAError{
			ResultError: ResultError{Result: tgpp.Experimental(tgpp.ResultRequestedServiceTemporarilyNotAuthorized)},
			AcceptableServiceInfo: &AcceptableServiceInfo{
				MediaComponents:         []MediaBandwidth{{MediaComponentNumber: 1, MaxRequestedBandwidthUL: ptr(Bandwidth(1)), MaxRequestedBandwidthDL: ptr(Bandwidth(2))}},
				MaxRequestedBandwidthUL: ptr(Bandwidth(3)), MaxRequestedBandwidthDL: ptr(Bandwidth(4)),
			},
			RetryInterval: 30 * time.Second,
			Features:      FeatureRel8,
		})),
		"RAA": must(NewReAuthAnswer(request(CommandReAuth, pcrfEnvelope), afIdentity, ReAuthAnswer{})),
		"STA": must(NewSessionTerminationAnswer(request(CommandSessionTermination, afEnvelope), pcrfIdentity, SessionTerminationAnswer{
			ServingNetwork: ServingNetwork{PLMN: "00101"}, Location: fullUserLocation(), NetLocAccessSupport: ptr(NetLocAccessNotSupported),
		})),
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
		ans := NewAnswer(request(command, afEnvelope), pcrfIdentity, ok, 0)

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

		a, ok := NewAnswer(req, pcrfIdentity, tgpp.Result{Code: diameter.ResultSuccess}, 0).Find(diameter.AVPAuthSessionState, 0)
		if v, _ := a.Unsigned32(); ok != echoed || (echoed && v != state) {
			t.Errorf("state %d: echoed %+v", state, a)
		}
	}
}

const allFeatures = Features(1<<64 - 1)

func aaRequest(features Features) *diameter.Message {
	return request(CommandAA, afEnvelope, featureAVPs(features, false)...)
}

func TestErrorAnswer(t *testing.T) {
	req := aaRequest(FeatureRel8 | FeatureRel9)
	ans := NewErrorAnswer(req, pcrfIdentity, tgpp.MissingAVP(AVPMediaComponentNumber, tgpp.VendorID, 4), FeatureRel8|FeatureCHEM)

	if _, ok := ans.Find(diameter.AVPFailedAVP, 0); !ok {
		t.Fatal("no Failed-AVP")
	}

	if _, ok := ans.Find(diameter.AVPAuthApplicationID, 0); !ok {
		t.Fatal("AAA without Auth-Application-Id")
	}

	_, err := ParseAAAnswer(ans)

	var aaErr *AAError
	if !errors.As(err, &aaErr) || aaErr.Code != diameter.ResultMissingAVP || aaErr.Features != FeatureRel8 ||
		aaErr.Error() != "rx: request failed with result 5005 DIAMETER_MISSING_AVP" {
		t.Fatalf("err = %v", err)
	}

	generic := NewErrorAnswer(request(CommandSessionTermination, afEnvelope), pcrfIdentity, errors.New("boom"), FeatureRel8)
	if r, _ := tgpp.ParseResult(generic); r.Code != diameter.ResultUnableToComply {
		t.Fatalf("generic error = %+v", r)
	}

	if _, ok := generic.Find(tgpp.AVPSupportedFeatures, tgpp.VendorID); ok {
		t.Fatal("STA with Supported-Features")
	}

	for name, parse := range map[string]func(*diameter.Message) error{
		"AAA": func(m *diameter.Message) error { _, err := ParseAAAnswer(m); return err },
		"RAA": func(m *diameter.Message) error { _, err := ParseReAuthAnswer(m); return err },
		"STA": func(m *diameter.Message) error { _, err := ParseSessionTerminationAnswer(m); return err },
		"ASA": func(m *diameter.Message) error { _, err := ParseAbortSessionAnswer(m); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := parse(&diameter.Message{}); !errors.Is(err, ErrMalformedAnswer) {
				t.Errorf("empty answer = %v", err)
			}

			informational := &diameter.Message{AVPs: []diameter.AVP{diameter.Unsigned32(diameter.AVPResultCode, diameter.AVPFlagMandatory, 0, 1001)}}
			if err := parse(informational); !errors.Is(err, ErrMalformedAnswer) {
				t.Errorf("informational answer = %v", err)
			}
		})
	}
}

func TestErrorAnswerKeepsAAErrorData(t *testing.T) {
	req := aaRequest(FeatureRel8)
	e := AAError{
		ResultError:           ResultError{Result: tgpp.Experimental(tgpp.ResultRequestedServiceTemporarilyNotAuthorized)},
		AcceptableServiceInfo: &AcceptableServiceInfo{MaxRequestedBandwidthUL: ptr(Bandwidth(64000))},
		RetryInterval:         30 * time.Second,
		Features:              FeatureRel8,
	}

	_, err := ParseAAAnswer(NewErrorAnswer(req, pcrfIdentity, fmt.Errorf("wrapped: %w", &e), 0))

	var got *AAError
	if !errors.As(err, &got) || !reflect.DeepEqual(*got, e) {
		t.Fatalf("parsed %+v (%v)", got, err)
	}

	invalid := NewErrorAnswer(req, pcrfIdentity, &AAError{RetryInterval: time.Millisecond}, 0)
	if r, _ := tgpp.ParseResult(invalid); r.Code != diameter.ResultUnableToComply {
		t.Fatalf("invalid AA error = %+v", r)
	}
}

func TestExperimentalResultsOnlyWhereAllowed(t *testing.T) {
	failure := tgpp.Experimental(tgpp.ResultInvalidServiceInformation)
	success := tgpp.Experimental(2001)

	for command, allowed := range map[uint32]bool{
		CommandAA: true, CommandReAuth: true, CommandSessionTermination: false, CommandAbortSession: false,
	} {
		req := request(command, pcrfEnvelope)

		for r, base := range map[tgpp.Result]uint32{failure: diameter.ResultUnableToComply, success: diameter.ResultSuccess} {
			got, err := tgpp.ParseResult(NewAnswer(req, afIdentity, r, 0))
			if err != nil || (allowed && got != r) || (!allowed && got != tgpp.Result{Code: base}) {
				t.Errorf("command %d, %s: answered %s, %v", command, r, got, err)
			}
		}
	}

	if _, err := NewSessionTerminationAnswer(request(CommandSessionTermination, afEnvelope), pcrfIdentity,
		SessionTerminationAnswer{Result: success}); !errors.Is(err, ErrInvalidMessage) {
		t.Errorf("experimental STA = %v", err)
	}

	if _, err := NewAbortSessionAnswer(request(CommandAbortSession, pcrfEnvelope), afIdentity,
		AbortSessionAnswer{Result: success}); !errors.Is(err, ErrInvalidMessage) {
		t.Errorf("experimental ASA = %v", err)
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
		"AAA error with success": func() (*diameter.Message, error) {
			return NewAAErrorAnswer(request(CommandAA, afEnvelope), pcrfIdentity, AAError{ResultError: ResultError{Result: tgpp.Result{Code: diameter.ResultSuccess}}})
		},
		"zero AAA error": func() (*diameter.Message, error) {
			return NewAAErrorAnswer(request(CommandAA, afEnvelope), pcrfIdentity, AAError{})
		},
		"informational AAA error": func() (*diameter.Message, error) {
			return NewAAErrorAnswer(request(CommandAA, afEnvelope), pcrfIdentity, AAError{ResultError: ResultError{Result: tgpp.Result{Code: 1001}}})
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := build(); !errors.Is(err, ErrInvalidMessage) {
				t.Errorf("err = %v", err)
			}
		})
	}
}

func TestRequestsNeedAnEnvelope(t *testing.T) {
	for name, env := range map[string]tgpp.Envelope{
		"no Session-Id":        {Origin: afIdentity, DestinationRealm: testEPCRealm},
		"no origin host":       {SessionID: "s;1", Origin: diameter.Identity{OriginRealm: testIMSRealm}, DestinationRealm: testEPCRealm},
		"no origin realm":      {SessionID: "s;1", Origin: diameter.Identity{OriginHost: "pcscf"}, DestinationRealm: testEPCRealm},
		"no destination realm": {SessionID: "s;1", Origin: afIdentity},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewAARequest(env, AARequest{}); !errors.Is(err, ErrInvalidMessage) {
				t.Errorf("AAR: err = %v", err)
			}

			if _, err := NewSessionTerminationRequest(env, SessionTerminationRequest{Cause: TerminationLogout}); !errors.Is(err, ErrInvalidMessage) {
				t.Errorf("STR: err = %v", err)
			}
		})
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
		t.Run(name, func(t *testing.T) {
			ans := NewAnswer(request(tc.command, pcrfEnvelope), afIdentity, tgpp.Result{Code: diameter.ResultUnknownSessionID}, 0)

			var re *ResultError
			if err := tc.parse(roundTrip(t, ans)); !errors.As(err, &re) || re.Code != diameter.ResultUnknownSessionID {
				t.Errorf("err = %v", err)
			}
		})
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

func TestFeaturesRequired(t *testing.T) {
	for name, r := range map[string]AARequest{
		"required":            {Features: FeatureRel8 | FeatureNetLoc | FeaturePCSCFRestorationEnhancement, FeaturesRequired: true},
		"advertised":          {Features: FeatureRel8 | FeaturePCSCFRestorationEnhancement},
		"one list required":   {Features: FeatureRel8 | FeatureRel9, FeaturesRequired: true},
		"one list advertised": {Features: FeatureCHEM},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := NewAARequest(afEnvelope, r)
			if err != nil {
				t.Fatal(err)
			}

			lists := map[uint32]bool{}

			for _, a := range diameter.FindAll(req.AVPs, tgpp.AVPSupportedFeatures, tgpp.VendorID) {
				f, err := tgpp.ParseSupportedFeatures(a)
				if err != nil || f.Mandatory != r.FeaturesRequired || lists[f.FeatureListID] {
					t.Fatalf("Supported-Features %+v, %v", f, err)
				}

				lists[f.FeatureListID] = true
			}

			got, err := ParseAARequest(roundTrip(t, req))
			if err != nil || got.Features != r.Features || got.FeaturesRequired != r.FeaturesRequired {
				t.Fatalf("features = %s, required %v, %v", got.Features, got.FeaturesRequired, err)
			}
		})
	}

	if _, err := NewAARequest(afEnvelope, AARequest{FeaturesRequired: true}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("required without features = %v", err)
	}
}

func TestAAAnswerAdvertisesCommonFeatures(t *testing.T) {
	pcrf := FeatureRel8 | FeatureRel9 | FeaturePCSCFRestorationEnhancement

	for name, tc := range map[string]struct {
		af, want Features
	}{
		"AF without features": {0, 0},
		"common subset":       {FeatureRel8 | FeatureNetLoc | FeaturePCSCFRestorationEnhancement, FeatureRel8 | FeaturePCSCFRestorationEnhancement},
		"nothing in common":   {FeatureNetLoc, 0},
	} {
		t.Run(name, func(t *testing.T) {
			req := mustMessage(t)(NewAARequest(afEnvelope, AARequest{Features: tc.af, FeaturesRequired: tc.af != 0}))

			ans, err := NewAAAnswer(req, pcrfIdentity, AAAnswer{Features: pcrf})
			if err != nil {
				t.Fatal(err)
			}

			for _, a := range diameter.FindAll(ans.AVPs, tgpp.AVPSupportedFeatures, tgpp.VendorID) {
				if a.Flags&diameter.AVPFlagMandatory != 0 {
					t.Fatalf("answer Supported-Features = %+v", a)
				}
			}

			got, err := ParseAAAnswer(ans)
			if err != nil || got.Features != tc.want {
				t.Fatalf("answer features = %s, %v", got.Features, err)
			}
		})
	}
}
