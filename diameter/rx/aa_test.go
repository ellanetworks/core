// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"errors"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func TestAARequestRoundTrip(t *testing.T) {
	for name, r := range map[string]AARequest{
		"full": fullAARequest(),
		"signalling path": {
			MediaComponents: []MediaComponent{{Number: 0, SubComponents: []MediaSubComponent{{FlowNumber: 0, FlowUsage: ptr(FlowUsageAFSignalling)}}}},
			SpecificActions: []SpecificAction{ActionIndicationOfLossOfBearer, ActionIndicationOfReleaseOfBearer},
			FramedIPAddress: netip.MustParseAddr("192.168.101.4"),
		},
		"P-CSCF restoration": {
			SubscriptionIDs:   []SubscriptionID{{Type: SubscriptionIDIMSI, Data: "001010000000001"}},
			CalledStationID:   "ims",
			RequestType:       ptr(RequestPCSCFRestoration),
			NoStateMaintained: true,
			Features:          FeaturePCSCFRestorationEnhancement,
		},
		"update":  {RequestType: ptr(RequestUpdate), MediaComponents: []MediaComponent{{Number: 1, FlowStatus: ptr(FlowStatusRemoved)}}},
		"minimal": {},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := NewAARequest(afEnvelope, r)
			if err != nil {
				t.Fatal(err)
			}

			got, err := ParseAARequest(roundTrip(t, req))
			if err != nil || !reflect.DeepEqual(got, r) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}
		})
	}
}

func TestAARequestEncoding(t *testing.T) {
	req := mustMessage(t)(NewAARequest(afEnvelope, AARequest{
		FramedIPAddress:   netip.MustParseAddr("192.168.101.4"),
		FramedIPv6Address: netip.MustParseAddr("2001:db8::4"),
		NoStateMaintained: true,
		MediaComponents: []MediaComponent{{
			Number:    1,
			CodecData: []CodecData{{Direction: CodecDownlink, Kind: CodecDescription, SDP: "m=video 0 RTP/AVP 96"}},
		}},
	}))

	v4, _ := req.Find(diameter.AVPFramedIPAddress, 0)
	if string(v4.Data) != "\xc0\xa8\x65\x04" {
		t.Errorf("Framed-IP-Address = %x", v4.Data)
	}

	v6, _ := req.Find(diameter.AVPFramedIPv6Prefix, 0)
	if want := append([]byte{0, 128}, netip.MustParseAddr("2001:db8::4").AsSlice()...); string(v6.Data) != string(want) {
		t.Errorf("Framed-IPv6-Prefix = %x", v6.Data)
	}

	state, _ := req.Find(diameter.AVPAuthSessionState, 0)
	if v, _ := state.Unsigned32(); v != diameter.AuthSessionStateNoStateMaintained {
		t.Errorf("Auth-Session-State = %d", v)
	}

	mcd, _ := req.Find(AVPMediaComponentDescription, tgpp.VendorID)
	inner, _ := mcd.Grouped()

	codec, _ := diameter.Find(inner, AVPCodecData, tgpp.VendorID)
	if string(codec.Data) != "downlink\ndescription\nm=video 0 RTP/AVP 96" {
		t.Errorf("Codec-Data = %q", codec.Data)
	}

	for _, absent := range []uint32{AVPServiceInfoStatus, AVPSIPForkingIndication, AVPRxRequestType} {
		if _, ok := req.Find(absent, tgpp.VendorID); ok {
			t.Errorf("default AVP %d sent", absent)
		}
	}
}

func TestAARequestValidation(t *testing.T) {
	for name, mutate := range map[string]func(*AARequest){
		"Service-Info-Status":          func(r *AARequest) { r.ServiceInfoStatus = 2 },
		"SIP-Forking-Indication":       func(r *AARequest) { r.SIPForkingIndication = 2 },
		"Rx-Request-Type":              func(r *AARequest) { r.RequestType = ptr(RequestType(3)) },
		"Specific-Action":              func(r *AARequest) { r.SpecificActions = []SpecificAction{22} },
		"Subscription-Id-Type":         func(r *AARequest) { r.SubscriptionIDs = []SubscriptionID{{Type: 5, Data: "x"}} },
		"empty Subscription-Id-Data":   func(r *AARequest) { r.SubscriptionIDs = []SubscriptionID{{Type: SubscriptionIDIMSI}} },
		"IPv6 as Framed-IP-Address":    func(r *AARequest) { r.FramedIPAddress = netip.MustParseAddr("2001:db8::1") },
		"negotiated Framed-IP-Address": func(r *AARequest) { r.FramedIPAddress = netip.MustParseAddr("255.255.255.255") },
		"assigned Framed-IP-Address":   func(r *AARequest) { r.FramedIPAddress = netip.MustParseAddr("255.255.255.254") },
		"IPv4 as Framed-IPv6-Prefix":   func(r *AARequest) { r.FramedIPv6Address = netip.MustParseAddr("10.0.0.1") },
		"mapped Framed-IPv6-Prefix":    func(r *AARequest) { r.FramedIPv6Address = netip.MustParseAddr("::ffff:10.0.0.1") },
		"Media-Type":                   func(r *AARequest) { r.MediaComponents[0].Type = ptr(MediaType(7)) },
		"component Flow-Status":        func(r *AARequest) { r.MediaComponents[0].FlowStatus = ptr(FlowStatus(5)) },
		"three Codec-Data": func(r *AARequest) {
			r.MediaComponents[0].CodecData = append(r.MediaComponents[0].CodecData, CodecData{})
		},
		"Codec-Data direction":      func(r *AARequest) { r.MediaComponents[0].CodecData[0].Direction = 2 },
		"Codec-Data kind":           func(r *AARequest) { r.MediaComponents[0].CodecData[0].Kind = 3 },
		"sub-component Flow-Status": func(r *AARequest) { r.MediaComponents[0].SubComponents[0].FlowStatus = ptr(FlowStatus(5)) },
		"Flow-Usage":                func(r *AARequest) { r.MediaComponents[0].SubComponents[0].FlowUsage = ptr(FlowUsage(3)) },
		"empty Flow-Description":    func(r *AARequest) { r.MediaComponents[0].SubComponents[0].FlowDescriptions = []string{""} },
		"AF-Signalling-Protocol":    func(r *AARequest) { r.MediaComponents[1].SubComponents[0].SignallingProtocol = 2 },
		"signalling protocol on media": func(r *AARequest) {
			r.MediaComponents[0].SubComponents[0].SignallingProtocol = SignallingProtocolSIP
		},
		"required without features": func(r *AARequest) { r.Features = 0 },
		"void Specific-Action":      func(r *AARequest) { r.SpecificActions = []SpecificAction{5} },
		"zero Specific-Action":      func(r *AARequest) { r.SpecificActions = []SpecificAction{0} },
		"restoration with state":    func(r *AARequest) { r.RequestType = ptr(RequestPCSCFRestoration) },
		"signalling without usage": func(r *AARequest) {
			r.MediaComponents[1].SubComponents[0].FlowUsage = nil
		},
	} {
		r := fullAARequest()
		mutate(&r)

		t.Run(name, func(t *testing.T) {
			if _, err := NewAARequest(afEnvelope, r); !errors.Is(err, ErrInvalidMessage) {
				t.Errorf("err = %v", err)
			}
		})
	}
}

func TestParseAARequestErrors(t *testing.T) {
	base := mustMessage(t)(NewAARequest(afEnvelope, AARequest{FramedIPAddress: netip.MustParseAddr("192.168.101.4")}))

	if _, err := ParseAARequest(base); err != nil {
		t.Fatalf("base request: %v", err)
	}

	component := func(avps ...diameter.AVP) diameter.AVP {
		return vendorGrouped(AVPMediaComponentDescription, append([]diameter.AVP{vendorUnsigned(AVPMediaComponentNumber, 1)}, avps...)...)
	}
	sub := func(avps ...diameter.AVP) diameter.AVP {
		return component(vendorGrouped(AVPMediaSubComponent, append([]diameter.AVP{vendorUnsigned(AVPFlowNumber, 1)}, avps...)...))
	}
	codec := func(s string) diameter.AVP { return vendorOctets(AVPCodecData, []byte(s)) }

	for name, tc := range map[string]struct {
		msg    *diameter.Message
		result uint32
	}{
		"no Auth-Application-Id": {without(base, diameter.AVPAuthApplicationID, 0), diameter.ResultMissingAVP},
		"other Auth-Application-Id": {
			with(without(base, diameter.AVPAuthApplicationID, 0), diameter.Unsigned32(diameter.AVPAuthApplicationID, diameter.AVPFlagMandatory, 0, 16777238)),
			diameter.ResultInvalidAVPValue,
		},
		"no Destination-Realm":  {without(base, diameter.AVPDestinationRealm, 0), diameter.ResultMissingAVP},
		"unknown mandatory AVP": {with(base, vendorUnsigned(999, 1)), diameter.ResultAVPUnsupported},
		"two Framed-IP-Address": {
			with(base, diameter.OctetString(diameter.AVPFramedIPAddress, diameter.AVPFlagMandatory, 0, []byte{10, 0, 0, 1})),
			diameter.ResultAVPOccursTooManyTimes,
		},
		"short Framed-IP-Address": {
			with(without(base, diameter.AVPFramedIPAddress, 0), diameter.OctetString(diameter.AVPFramedIPAddress, diameter.AVPFlagMandatory, 0, []byte{10, 0, 0})),
			diameter.ResultInvalidAVPLength,
		},
		"negotiated Framed-IP-Address": {
			with(without(base, diameter.AVPFramedIPAddress, 0), diameter.OctetString(diameter.AVPFramedIPAddress, diameter.AVPFlagMandatory, 0, []byte{255, 255, 255, 255})),
			diameter.ResultInvalidAVPValue,
		},
		"/64 Framed-IPv6-Prefix": {
			with(base, diameter.OctetString(diameter.AVPFramedIPv6Prefix, diameter.AVPFlagMandatory, 0, []byte{0, 64, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 1})),
			diameter.ResultInvalidAVPValue,
		},
		"padded /64 Framed-IPv6-Prefix": {
			with(base, diameter.OctetString(diameter.AVPFramedIPv6Prefix, diameter.AVPFlagMandatory, 0, append([]byte{0, 64}, make([]byte, 16)...))),
			diameter.ResultInvalidAVPValue,
		},
		"Auth-Session-State": {
			with(base, diameter.Unsigned32(diameter.AVPAuthSessionState, diameter.AVPFlagMandatory, 0, 2)),
			diameter.ResultInvalidAVPValue,
		},
		"Rx-Request-Type":        {with(base, diameter.Unsigned32(AVPRxRequestType, 0, tgpp.VendorID, 3)), diameter.ResultInvalidAVPValue},
		"Service-Info-Status":    {with(base, vendorUnsigned(AVPServiceInfoStatus, 2)), diameter.ResultInvalidAVPValue},
		"SIP-Forking-Indication": {with(base, vendorUnsigned(AVPSIPForkingIndication, 2)), diameter.ResultInvalidAVPValue},
		"Specific-Action":        {with(base, vendorUnsigned(AVPSpecificAction, 22)), diameter.ResultInvalidAVPValue},
		"empty AF-Application-Identifier": {
			with(base, vendorOctets(AVPAFApplicationIdentifier, nil)), diameter.ResultInvalidAVPValue,
		},
		"Subscription-Id without data": {
			with(base, diameter.Grouped(diameter.AVPSubscriptionID, diameter.AVPFlagMandatory, 0,
				diameter.Unsigned32(diameter.AVPSubscriptionIDType, diameter.AVPFlagMandatory, 0, 1))),
			diameter.ResultMissingAVP,
		},
		"Subscription-Id-Type": {
			with(base, diameter.Grouped(diameter.AVPSubscriptionID, diameter.AVPFlagMandatory, 0,
				diameter.Unsigned32(diameter.AVPSubscriptionIDType, diameter.AVPFlagMandatory, 0, 5),
				diameter.UTF8String(diameter.AVPSubscriptionIDData, diameter.AVPFlagMandatory, 0, "x"))),
			diameter.ResultInvalidAVPValue,
		},
		"Media-Component-Description not grouped": {with(base, vendorOctets(AVPMediaComponentDescription, []byte{1})), diameter.ResultInvalidAVPValue},
		"no Media-Component-Number":               {with(base, vendorGrouped(AVPMediaComponentDescription)), diameter.ResultMissingAVP},
		"two Media-Component-Number":              {with(base, component(vendorUnsigned(AVPMediaComponentNumber, 2))), diameter.ResultAVPOccursTooManyTimes},
		"unknown mandatory component AVP":         {with(base, component(vendorUnsigned(999, 1))), diameter.ResultAVPUnsupported},
		"Media-Type":                              {with(base, component(vendorUnsigned(AVPMediaType, 7))), diameter.ResultInvalidAVPValue},
		"short bandwidth":                         {with(base, component(vendorOctets(AVPRRBandwidth, []byte{1}))), diameter.ResultInvalidAVPLength},
		"three Codec-Data": {
			with(base, component(codec("uplink\noffer\n"), codec("downlink\nanswer\n"), codec("uplink\nanswer\n"))),
			diameter.ResultAVPOccursTooManyTimes,
		},
		"Codec-Data without kind": {with(base, component(codec("uplink\n"))), diameter.ResultInvalidAVPValue},
		"Codec-Data direction":    {with(base, component(codec("sideways\noffer\n"))), diameter.ResultInvalidAVPValue},
		"Codec-Data kind":         {with(base, component(codec("uplink\nrequest\n"))), diameter.ResultInvalidAVPValue},
		"no Flow-Number":          {with(base, component(vendorGrouped(AVPMediaSubComponent))), diameter.ResultMissingAVP},
		"empty Flow-Description":  {with(base, sub(vendorOctets(AVPFlowDescription, nil))), diameter.ResultInvalidAVPValue},
		"Flow-Usage":              {with(base, sub(vendorUnsigned(AVPFlowUsage, 3))), diameter.ResultInvalidAVPValue},
		"Flow-Status":             {with(base, sub(vendorUnsigned(AVPFlowStatus, 5))), diameter.ResultInvalidAVPValue},
		"signalling protocol on media": {
			with(base, sub(diameter.Unsigned32(AVPAFSignallingProtocol, 0, tgpp.VendorID, 1))),
			diameter.ResultInvalidAVPValue,
		},
		"signalling protocol without usage": {
			with(base, sub(vendorUnsigned(AVPFlowUsage, uint32(FlowUsageRTCP)), diameter.Unsigned32(AVPAFSignallingProtocol, 0, tgpp.VendorID, 1))),
			diameter.ResultInvalidAVPValue,
		},
		"AF-Signalling-Protocol": {
			with(base, sub(vendorUnsigned(AVPFlowUsage, uint32(FlowUsageAFSignalling)), diameter.Unsigned32(AVPAFSignallingProtocol, 0, tgpp.VendorID, 2))),
			diameter.ResultInvalidAVPValue,
		},
		"short sub-component bandwidth": {with(base, sub(vendorOctets(AVPMaxRequestedBandwidthDL, []byte{1}))), diameter.ResultInvalidAVPLength},
		"two Flow-Number":               {with(base, sub(vendorOctets(AVPFlowNumber, []byte{0, 0, 0, 0, 1}))), diameter.ResultAVPOccursTooManyTimes},
		"long Flow-Number": {
			with(base, component(vendorGrouped(AVPMediaSubComponent, vendorOctets(AVPFlowNumber, make([]byte, 8))))),
			diameter.ResultInvalidAVPLength,
		},
		"component Flow-Status":                     {with(base, component(vendorUnsigned(AVPFlowStatus, 5))), diameter.ResultInvalidAVPValue},
		"empty component AF-Application-Identifier": {with(base, component(vendorOctets(AVPAFApplicationIdentifier, nil))), diameter.ResultInvalidAVPValue},
		"empty IP-Domain-Id":                        {with(base, diameter.OctetString(AVPIPDomainID, 0, tgpp.VendorID, nil)), diameter.ResultInvalidAVPValue},
		"short Framed-IPv6-Prefix": {
			with(base, diameter.OctetString(diameter.AVPFramedIPv6Prefix, diameter.AVPFlagMandatory, 0, []byte{0})),
			diameter.ResultInvalidAVPLength,
		},
		"truncated /128 Framed-IPv6-Prefix": {
			with(base, diameter.OctetString(diameter.AVPFramedIPv6Prefix, diameter.AVPFlagMandatory, 0, append([]byte{0, 128}, make([]byte, 8)...))),
			diameter.ResultInvalidAVPLength,
		},
		"mapped Framed-IPv6-Prefix": {
			with(base, diameter.OctetString(diameter.AVPFramedIPv6Prefix, diameter.AVPFlagMandatory, 0,
				append([]byte{0, 128}, netip.MustParseAddr("::ffff:10.0.0.1").AsSlice()...))),
			diameter.ResultInvalidAVPValue,
		},
		"short Specific-Action": {with(base, vendorOctets(AVPSpecificAction, []byte{1})), diameter.ResultInvalidAVPLength},
		"restoration with state": {
			with(base, diameter.Unsigned32(AVPRxRequestType, 0, tgpp.VendorID, uint32(RequestPCSCFRestoration))),
			diameter.ResultMissingAVP,
		},
		"short Auth-Application-Id": {
			with(without(base, diameter.AVPAuthApplicationID, 0), diameter.OctetString(diameter.AVPAuthApplicationID, diameter.AVPFlagMandatory, 0, []byte{1})),
			diameter.ResultInvalidAVPLength,
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseAARequest(tc.msg)
			if got := avpError(t, err).ResultCode; got != tc.result {
				t.Fatalf("result = %d, want %d (%v)", got, tc.result, err)
			}
		})
	}
}

func TestParseAARequestToleratesPeerAVPs(t *testing.T) {
	base := mustMessage(t)(NewAARequest(afEnvelope, AARequest{FramedIPAddress: netip.MustParseAddr("192.168.101.4")}))

	extended := with(base,
		diameter.Grouped(diameter.AVPVendorSpecificApplicationID, diameter.AVPFlagMandatory, 0,
			diameter.Unsigned32(diameter.AVPVendorID, diameter.AVPFlagMandatory, 0, tgpp.VendorID),
			diameter.Unsigned32(diameter.AVPAuthApplicationID, diameter.AVPFlagMandatory, 0, ApplicationID)),
		diameter.Unsigned32(diameter.AVPAuthorizationLifetime, diameter.AVPFlagMandatory, 0, 7200),
		diameter.Unsigned32(diameter.AVPAuthGracePeriod, diameter.AVPFlagMandatory, 0, 0),
		diameter.Unsigned32(diameter.AVPSessionTimeout, diameter.AVPFlagMandatory, 0, 60),
		diameter.Unsigned32(avpReservationPriority, diameter.AVPFlagVendor, etsiVendorID, 0),
		diameter.Unsigned32(diameter.AVPAuthSessionState, diameter.AVPFlagMandatory, 0, diameter.AuthSessionStateMaintained),
		vendorUnsigned(AVPSpecificAction, 5),
		vendorGrouped(AVPMediaComponentDescription,
			vendorUnsigned(AVPMediaComponentNumber, 1),
			vendorGrouped(AVPMediaSubComponent,
				vendorUnsigned(AVPFlowNumber, 1),
				vendorOctets(AVPFlowDescription, []byte("permit out 17 from 10.0.0.1 1000 to 10.0.0.2 2000")),
				vendorOctets(AVPFlowDescription, []byte("permit in 17 from 10.0.0.2 2000 to 10.0.0.1 1000")),
				vendorOctets(AVPFlowDescription, []byte("permit out 17 from 10.0.0.1 1001 to 10.0.0.2 2001")),
				vendorOctets(AVPFlowDescription, []byte("permit in 17 from 10.0.0.2 2001 to 10.0.0.1 1001")),
				vendorUnsigned(AVPFlowUsage, uint32(FlowUsageNoInformation)),
			),
			diameter.Unsigned32(AVPMinRequestedBandwidthUL, 0, tgpp.VendorID, 41000),
			vendorOctets(AVPCodecData, []byte("uplink\noffer\n\x00")),
		),
	)

	r, err := ParseAARequest(extended)
	if err != nil {
		t.Fatal(err)
	}

	if r.SpecificActions != nil || r.NoStateMaintained {
		t.Errorf("parsed %+v", r)
	}

	if sub := r.MediaComponents[0].SubComponents[0]; len(sub.FlowDescriptions) != 4 {
		t.Errorf("flow descriptions = %q", sub.FlowDescriptions)
	}

	if codec := r.MediaComponents[0].CodecData[0]; codec.SDP != "\x00" {
		t.Errorf("Codec-Data = %+v", codec)
	}

	rebuilt, err := NewAARequest(afEnvelope, r)
	if err != nil {
		t.Fatal(err)
	}

	if again, err := ParseAARequest(rebuilt); err != nil || !reflect.DeepEqual(again, r) {
		t.Fatalf("rebuilt = %+v, %v", again, err)
	}
}

func TestParseAARequestCopiesOctets(t *testing.T) {
	req := roundTrip(t, mustMessage(t)(NewAARequest(afEnvelope, AARequest{IPDomainID: []byte("domain")})))

	r, err := ParseAARequest(req)
	if err != nil {
		t.Fatal(err)
	}

	r.IPDomainID[0] = 'X'

	if a, _ := req.Find(AVPIPDomainID, tgpp.VendorID); string(a.Data) != "domain" {
		t.Fatalf("message changed to %q", a.Data)
	}
}

func TestAAAnswerRoundTrip(t *testing.T) {
	req := aaRequest(allFeatures)

	for name, a := range map[string]AAAnswer{
		"bare": {Result: tgpp.Result{Code: diameter.ResultSuccess}},
		"full": {
			Result: tgpp.Result{Code: diameter.ResultSuccess},
			AccessNetworkChargingIdentifiers: []AccessNetworkChargingIdentifier{
				{Value: []byte{0, 0, 0, 1}},
				{Value: []byte{0, 0, 0, 2}, Flows: []Flows{{MediaComponentNumber: 1, FlowNumbers: []uint32{1, 2}}, {MediaComponentNumber: 2}}},
			},
			AccessNetworkChargingAddress: netip.MustParseAddr("10.0.0.1"),
			SubscriptionIDs:              []SubscriptionID{{Type: SubscriptionIDIMSI, Data: "001010000000001"}},
			Class:                        [][]byte{[]byte("pcrf-state-1"), {0xff, 0x00}},
			Features:                     FeatureRel8 | FeatureRel9 | FeatureCHEM,
		},
		"IPv6 charging address": {
			Result: tgpp.Result{Code: diameter.ResultSuccess}, AccessNetworkChargingAddress: netip.MustParseAddr("2001:db8::1"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			ans, err := NewAAAnswer(req, pcrfIdentity, a)
			if err != nil {
				t.Fatal(err)
			}

			got, err := ParseAAAnswer(roundTrip(t, ans))
			if err != nil || !reflect.DeepEqual(got, a) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}
		})
	}

	ans := mustMessage(t)(NewAAAnswer(req, pcrfIdentity, AAAnswer{}))
	if got, err := ParseAAAnswer(ans); err != nil || got.Result.Code != diameter.ResultSuccess {
		t.Fatalf("zero result = %+v, %v", got, err)
	}
}

func TestAAAnswerValidation(t *testing.T) {
	req := request(CommandAA, afEnvelope)

	for name, a := range map[string]AAAnswer{
		"zoned charging address":    {AccessNetworkChargingAddress: netip.MustParseAddr("fe80::1%eth0")},
		"empty charging identifier": {AccessNetworkChargingIdentifiers: []AccessNetworkChargingIdentifier{{}}},
		"Subscription-Id-Type":      {SubscriptionIDs: []SubscriptionID{{Type: 9, Data: "x"}}},
	} {
		if _, err := NewAAAnswer(req, pcrfIdentity, a); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseAAAnswerMalformed(t *testing.T) {
	ok := NewAnswer(request(CommandAA, afEnvelope), pcrfIdentity, tgpp.Result{Code: diameter.ResultSuccess}, 0)

	for name, ans := range map[string]*diameter.Message{
		"charging identifier without value": with(ok, vendorGrouped(AVPAccessNetworkChargingIdentifier)),
		"charging flows without number":     with(ok, vendorGrouped(AVPAccessNetworkChargingIdentifier, vendorOctets(AVPAccessNetworkChargingIdentifierValue, []byte{1}), vendorGrouped(AVPFlows))),
		"charging address":                  with(ok, vendorOctets(AVPAccessNetworkChargingAddress, []byte{0, 1, 10})),
		"Subscription-Id":                   with(ok, diameter.Grouped(diameter.AVPSubscriptionID, diameter.AVPFlagMandatory, 0)),
	} {
		if _, err := ParseAAAnswer(ans); !errors.Is(err, ErrMalformedAnswer) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestAAErrorRoundTrip(t *testing.T) {
	req := aaRequest(allFeatures)

	for name, e := range map[string]AAError{
		"service not authorized": {
			ResultError: ResultError{Result: tgpp.Experimental(tgpp.ResultRequestedServiceNotAuthorized)},
			AcceptableServiceInfo: &AcceptableServiceInfo{
				MediaComponents: []MediaBandwidth{
					{MediaComponentNumber: 1, MaxRequestedBandwidthUL: ptr(Bandwidth(24000)), MaxRequestedBandwidthDL: ptr(Bandwidth(24000))},
					{MediaComponentNumber: 2},
				},
			},
			Features: FeatureRel8,
		},
		"temporarily not authorized": {
			ResultError:           ResultError{Result: tgpp.Experimental(tgpp.ResultRequestedServiceTemporarilyNotAuthorized)},
			AcceptableServiceInfo: &AcceptableServiceInfo{MaxRequestedBandwidthUL: ptr(Bandwidth(64000)), MaxRequestedBandwidthDL: ptr(Bandwidth(128000))},
			RetryInterval:         90 * time.Second,
		},
		"session binding": {ResultError: ResultError{Result: tgpp.Experimental(tgpp.ResultIPCANSessionNotAvailable)}},
		"unknown session": {ResultError: ResultError{Result: tgpp.Result{Code: diameter.ResultUnknownSessionID}}},
	} {
		t.Run(name, func(t *testing.T) {
			ans, err := NewAAErrorAnswer(req, pcrfIdentity, e)
			if err != nil {
				t.Fatal(err)
			}

			if _, ok := ans.Find(diameter.AVPAuthApplicationID, 0); !ok {
				t.Fatal("no Auth-Application-Id")
			}

			_, err = ParseAAAnswer(roundTrip(t, ans))

			var got *AAError
			if !errors.As(err, &got) || !reflect.DeepEqual(*got, e) {
				t.Fatalf("parsed %+v (%v)", got, err)
			}

			var re *ResultError
			if r, ok := tgpp.ResultOf(err); !errors.As(err, &re) || !ok || r != e.Result {
				t.Fatalf("ResultOf = %+v, %v", r, ok)
			}
		})
	}
}

func TestAAErrorValidation(t *testing.T) {
	req := request(CommandAA, afEnvelope)
	failure := ResultError{Result: tgpp.Experimental(tgpp.ResultRequestedServiceTemporarilyNotAuthorized)}

	for name, e := range map[string]AAError{
		"success":                   {ResultError: ResultError{Result: tgpp.Result{Code: diameter.ResultSuccess}}},
		"sub-second retry interval": {ResultError: failure, RetryInterval: 1500 * time.Millisecond},
		"negative retry interval":   {ResultError: failure, RetryInterval: -time.Second},
		"huge retry interval":       {ResultError: failure, RetryInterval: (1 << 32) * time.Second},
	} {
		if _, err := NewAAErrorAnswer(req, pcrfIdentity, e); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestAAErrorDropsInvalidExtras(t *testing.T) {
	ans := with(NewAnswer(request(CommandAA, afEnvelope), pcrfIdentity, tgpp.Experimental(tgpp.ResultRequestedServiceNotAuthorized), 0),
		vendorGrouped(AVPAcceptableServiceInfo, vendorGrouped(AVPMediaComponentDescription)),
		diameter.OctetString(AVPRetryInterval, 0, tgpp.VendorID, []byte{1}),
	)

	_, err := ParseAAAnswer(ans)

	var e *AAError
	if !errors.As(err, &e) || e.AcceptableServiceInfo != nil || e.RetryInterval != 0 {
		t.Fatalf("err = %#v", err)
	}
}

func TestParseAARequestIgnoresVoidActions(t *testing.T) {
	req := request(CommandAA, afEnvelope,
		vendorUnsigned(AVPSpecificAction, 0),
		vendorUnsigned(AVPSpecificAction, uint32(ActionIndicationOfLossOfBearer)),
		vendorUnsigned(AVPSpecificAction, 5),
	)

	r, err := ParseAARequest(req)
	if err != nil || !reflect.DeepEqual(r.SpecificActions, []SpecificAction{ActionIndicationOfLossOfBearer}) {
		t.Fatalf("Specific-Action = %v, %v", r.SpecificActions, err)
	}
}

func TestCodecDataKinds(t *testing.T) {
	for _, d := range []CodecData{
		{Direction: CodecUplink, Kind: CodecOffer, SDP: "m=audio 1 RTP/AVP 0"},
		{Direction: CodecDownlink, Kind: CodecAnswer},
		{Direction: CodecUplink, Kind: CodecDescription, SDP: "m=video 0 RTP/AVP 96\nb=AS:64"},
	} {
		t.Run(d.Direction.String()+" "+d.Kind.String(), func(t *testing.T) {
			a, err := codecDataAVP(d)
			if err != nil {
				t.Fatal(err)
			}

			if got, err := parseCodecData(a); err != nil || got != d {
				t.Fatalf("parseCodecData = %+v, %v", got, err)
			}
		})
	}
}

func TestParseAcceptableServiceInfoErrors(t *testing.T) {
	for name, a := range map[string]diameter.AVP{
		"not grouped":         vendorOctets(AVPAcceptableServiceInfo, []byte{1}),
		"bad media component": vendorGrouped(AVPAcceptableServiceInfo, vendorGrouped(AVPMediaComponentDescription)),
		"short UL":            vendorGrouped(AVPAcceptableServiceInfo, vendorOctets(AVPMaxRequestedBandwidthUL, []byte{1})),
		"short DL":            vendorGrouped(AVPAcceptableServiceInfo, vendorOctets(AVPMaxRequestedBandwidthDL, []byte{1})),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseAcceptableServiceInfo(a); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}
