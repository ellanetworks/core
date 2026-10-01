// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"errors"
	"math"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const (
	framedIPv6PrefixLength = 128
	framedIPv6PrefixOctets = 2 + 16
)

type AARequest struct {
	AFApplicationIdentifier string
	MediaComponents         []MediaComponent
	ServiceInfoStatus       ServiceInfoStatus
	AFChargingIdentifier    string
	SIPForkingIndication    SIPForkingIndication
	SpecificActions         []SpecificAction
	SubscriptionIDs         []SubscriptionID
	FramedIPAddress         netip.Addr
	FramedIPv6Address       netip.Addr
	CalledStationID         string
	IPDomainID              []byte
	ServiceURN              string
	RequestType             *RequestType
	RequiredAccessInfo      []RequiredAccessInfo
	NoStateMaintained       bool
	Features                Features
	FeaturesRequired        bool
}

type AAAnswer struct {
	Result                           tgpp.Result
	AccessNetworkChargingIdentifiers []AccessNetworkChargingIdentifier
	AccessNetworkChargingAddress     netip.Addr
	AccessNetwork                    AccessNetwork
	ServingNetwork                   ServingNetwork
	NetLocAccessSupport              *NetLocAccessSupport
	Flows                            []Flows
	SubscriptionIDs                  []SubscriptionID
	Features                         Features
}

type AAError struct {
	ResultError

	AcceptableServiceInfo *AcceptableServiceInfo
	RetryInterval         time.Duration
	Features              Features
}

func (e *AAError) Unwrap() error {
	return &e.ResultError
}

var aarRules = commonRequestRules.With(diameter.Rules{
	vendorKey(AVPIPDomainID):                               {},
	vendorKey(AVPAFApplicationIdentifier):                  {},
	vendorKey(AVPMediaComponentDescription):                {Multiple: true},
	vendorKey(AVPServiceInfoStatus):                        {},
	vendorKey(AVPAFChargingIdentifier):                     {},
	vendorKey(AVPSIPForkingIndication):                     {},
	vendorKey(AVPSpecificAction):                           {Multiple: true},
	{Code: diameter.AVPSubscriptionID}:                     {Multiple: true},
	vendorKey(tgpp.AVPSupportedFeatures):                   {Multiple: true},
	{Code: avpReservationPriority, VendorID: etsiVendorID}: {},
	{Code: diameter.AVPFramedIPAddress}:                    {},
	{Code: diameter.AVPFramedIPv6Prefix}:                   {},
	{Code: diameter.AVPCalledStationID}:                    {},
	vendorKey(AVPServiceURN):                               {},
	vendorKey(avpSponsoredConnectivity):                    {},
	vendorKey(avpMPSIdentifier):                            {},
	vendorKey(avpGCSIdentifier):                            {},
	vendorKey(avpMCPTTIdentifier):                          {},
	vendorKey(avpMCVideoIdentifier):                        {},
	vendorKey(avpIMSContentIdentifier):                     {},
	vendorKey(avpIMSContentType):                           {},
	vendorKey(avpCallingPartyAddress):                      {Multiple: true},
	vendorKey(avpCalleeInformation):                        {},
	vendorKey(AVPRxRequestType):                            {},
	vendorKey(AVPRequiredAccessInfo):                       {Multiple: true},
	vendorKey(avpAFRequestedData):                          {},
	vendorKey(avpReferenceID):                              {},
	vendorKey(avpPreemptionControlInfo):                    {},
	vendorKey(avpMPSAction):                                {},
	{Code: diameter.AVPAuthorizationLifetime}:              {},
	{Code: diameter.AVPAuthGracePeriod}:                    {},
	{Code: diameter.AVPSessionTimeout}:                     {},
})

func NewAARequest(env tgpp.Envelope, r AARequest) (*diameter.Message, error) {
	switch {
	case !r.ServiceInfoStatus.valid():
		return nil, invalidf("Service-Info-Status %s", r.ServiceInfoStatus)
	case !r.SIPForkingIndication.valid():
		return nil, invalidf("SIP-Forking-Indication %s", r.SIPForkingIndication)
	case !validEnum(r.RequestType):
		return nil, invalidf("Rx-Request-Type %s", *r.RequestType)
	case restorationWithState(r.RequestType, r.NoStateMaintained):
		return nil, invalidf("P-CSCF restoration without NO_STATE_MAINTAINED")
	case r.FeaturesRequired && r.Features == 0:
		return nil, invalidf("features required without any feature")
	}

	var avps []diameter.AVP

	if len(r.IPDomainID) > 0 {
		avps = append(avps, diameter.OctetString(AVPIPDomainID, 0, tgpp.VendorID, r.IPDomainID))
	}

	if r.NoStateMaintained {
		avps = append(avps, diameter.Unsigned32(diameter.AVPAuthSessionState, diameter.AVPFlagMandatory, 0, diameter.AuthSessionStateNoStateMaintained))
	}

	if r.AFApplicationIdentifier != "" {
		avps = append(avps, vendorOctets(AVPAFApplicationIdentifier, []byte(r.AFApplicationIdentifier)))
	}

	for _, c := range r.MediaComponents {
		a, err := mediaComponentAVP(c)
		if err != nil {
			return nil, err
		}

		avps = append(avps, a)
	}

	if r.ServiceInfoStatus != ServiceInfoFinal {
		avps = append(avps, vendorUnsigned(AVPServiceInfoStatus, uint32(r.ServiceInfoStatus)))
	}

	if r.AFChargingIdentifier != "" {
		avps = append(avps, vendorOctets(AVPAFChargingIdentifier, []byte(r.AFChargingIdentifier)))
	}

	if r.SIPForkingIndication != ForkingSingleDialogue {
		avps = append(avps, vendorUnsigned(AVPSIPForkingIndication, uint32(r.SIPForkingIndication)))
	}

	actions, err := specificActionAVPs(r.SpecificActions)
	if err != nil {
		return nil, err
	}

	avps = append(avps, actions...)

	subscriptions, err := subscriptionIDAVPs(r.SubscriptionIDs)
	if err != nil {
		return nil, err
	}

	avps = append(avps, subscriptions...)
	avps = append(avps, featureAVPs(r.Features, r.FeaturesRequired)...)

	binding, err := bindingAVPs(r.FramedIPAddress, r.FramedIPv6Address)
	if err != nil {
		return nil, err
	}

	avps = append(avps, binding...)

	if r.CalledStationID != "" {
		avps = append(avps, diameter.UTF8String(diameter.AVPCalledStationID, diameter.AVPFlagMandatory, 0, r.CalledStationID))
	}

	if r.ServiceURN != "" {
		avps = append(avps, vendorOctets(AVPServiceURN, []byte(r.ServiceURN)))
	}

	if r.RequestType != nil {
		avps = append(avps, diameter.Unsigned32(AVPRxRequestType, 0, tgpp.VendorID, uint32(*r.RequestType)))
	}

	required, err := enumListAVPs(AVPRequiredAccessInfo, 0, r.RequiredAccessInfo)
	if err != nil {
		return nil, err
	}

	avps = append(avps, required...)

	return newRequest(env, CommandAA, avps...)
}

func CheckAA(req *diameter.Message) error {
	return checkRequest(aarRules, req)
}

func ParseAARequest(req *diameter.Message) (AARequest, error) {
	if err := CheckAA(req); err != nil {
		return AARequest{}, err
	}

	r := AARequest{Features: featureList(req), FeaturesRequired: featuresRequired(req)}

	var err error

	if r.IPDomainID, err = optionalOctets(req.AVPs, AVPIPDomainID, tgpp.VendorID); err != nil {
		return AARequest{}, err
	}

	state, err := optionalUint32(req.AVPs, diameter.AVPAuthSessionState, 0)
	if err != nil {
		return AARequest{}, err
	}

	if state != nil {
		if *state > diameter.AuthSessionStateNoStateMaintained {
			a, _ := req.Find(diameter.AVPAuthSessionState, 0)
			return AARequest{}, tgpp.InvalidAVP(a)
		}

		r.NoStateMaintained = *state == diameter.AuthSessionStateNoStateMaintained
	}

	for _, f := range []struct {
		code, vendorID uint32
		dst            *string
	}{
		{AVPAFApplicationIdentifier, tgpp.VendorID, &r.AFApplicationIdentifier},
		{AVPAFChargingIdentifier, tgpp.VendorID, &r.AFChargingIdentifier},
		{diameter.AVPCalledStationID, 0, &r.CalledStationID},
		{AVPServiceURN, tgpp.VendorID, &r.ServiceURN},
	} {
		if *f.dst, err = optionalString(req.AVPs, f.code, f.vendorID); err != nil {
			return AARequest{}, err
		}
	}

	for _, a := range diameter.FindAll(req.AVPs, AVPMediaComponentDescription, tgpp.VendorID) {
		c, err := parseMediaComponent(a)
		if err != nil {
			return AARequest{}, err
		}

		r.MediaComponents = append(r.MediaComponents, c)
	}

	if r.ServiceInfoStatus, err = defaultEnum[ServiceInfoStatus](req.AVPs, AVPServiceInfoStatus, tgpp.VendorID); err != nil {
		return AARequest{}, err
	}

	if r.SIPForkingIndication, err = defaultEnum[SIPForkingIndication](req.AVPs, AVPSIPForkingIndication, tgpp.VendorID); err != nil {
		return AARequest{}, err
	}

	if r.SpecificActions, err = specificActions(req.AVPs, true); err != nil {
		return AARequest{}, err
	}

	if r.SubscriptionIDs, err = subscriptionIDs(req.AVPs); err != nil {
		return AARequest{}, err
	}

	if r.FramedIPAddress, r.FramedIPv6Address, err = binding(req); err != nil {
		return AARequest{}, err
	}

	if r.RequestType, err = optionalEnum[RequestType](req.AVPs, AVPRxRequestType, tgpp.VendorID); err != nil {
		return AARequest{}, err
	}

	if r.RequiredAccessInfo, err = enumList[RequiredAccessInfo](req.AVPs, AVPRequiredAccessInfo, tgpp.VendorID); err != nil {
		return AARequest{}, err
	}

	if restorationWithState(r.RequestType, r.NoStateMaintained) {
		return AARequest{}, diameter.NewAVPError(diameter.ResultMissingAVP,
			diameter.Unsigned32(diameter.AVPAuthSessionState, diameter.AVPFlagMandatory, 0, diameter.AuthSessionStateNoStateMaintained))
	}

	return r, nil
}

func NewAAAnswer(req *diameter.Message, id diameter.Identity, a AAAnswer) (*diameter.Message, error) {
	result, err := successResult(a.Result, CommandAA)
	if err != nil {
		return nil, err
	}

	if !validEnum(a.NetLocAccessSupport) {
		return nil, invalidf("NetLoc-Access-Support %s", *a.NetLocAccessSupport)
	}

	charging, err := chargingIdentifierAVPs(a.AccessNetworkChargingIdentifiers)
	if err != nil {
		return nil, err
	}

	var avps []diameter.AVP

	if a.AccessNetworkChargingAddress.IsValid() {
		address, err := addressAVP(AVPAccessNetworkChargingAddress, diameter.AVPFlagMandatory, a.AccessNetworkChargingAddress)
		if err != nil {
			return nil, err
		}

		avps = append(avps, address)
	}

	access, err := a.AccessNetwork.avps()
	if err != nil {
		return nil, err
	}

	avps = append(avps, access...)
	avps = appendOptional(avps, tgpp.AVPNetLocAccessSupport, 0, (*uint32)(a.NetLocAccessSupport))

	flows, err := flowsAVPs(a.Flows)
	if err != nil {
		return nil, err
	}

	avps = append(avps, flows...)

	subscriptions, err := subscriptionIDAVPs(a.SubscriptionIDs)
	if err != nil {
		return nil, err
	}

	avps = append(avps, subscriptions...)

	serving, err := a.ServingNetwork.avps()
	if err != nil {
		return nil, err
	}

	avps = append(avps, serving...)

	ans := NewAnswer(req, id, result, a.Features)
	ans.AVPs = append(ans.AVPs, charging...)
	ans.AVPs = append(ans.AVPs, avps...)

	return ans, nil
}

func NewAAErrorAnswer(req *diameter.Message, id diameter.Identity, e AAError) (*diameter.Message, error) {
	if err := errorResult(e.Result); err != nil {
		return nil, err
	}

	var retry []diameter.AVP

	if e.RetryInterval != 0 {
		seconds := e.RetryInterval / time.Second
		if e.RetryInterval%time.Second != 0 || seconds < 0 || seconds > math.MaxUint32 {
			return nil, invalidf("retry interval %s", e.RetryInterval)
		}

		retry = append(retry, diameter.Unsigned32(AVPRetryInterval, 0, tgpp.VendorID, uint32(seconds)))
	}

	var acceptable []diameter.AVP

	if e.AcceptableServiceInfo != nil {
		a, err := acceptableServiceInfoAVP(*e.AcceptableServiceInfo)
		if err != nil {
			return nil, err
		}

		acceptable = append(acceptable, a)
	}

	ans := NewAnswer(req, id, e.Result, e.Features)
	ans.AVPs = append(ans.AVPs, acceptable...)
	ans.AVPs = append(ans.AVPs, retry...)

	return ans, nil
}

func ParseAAAnswer(ans *diameter.Message) (AAAnswer, error) {
	result, err := parseResult(ans)

	var re *ResultError
	if errors.As(err, &re) {
		e := &AAError{ResultError: *re, Features: featureList(ans)}

		if a, ok := ans.Find(AVPAcceptableServiceInfo, tgpp.VendorID); ok {
			e.AcceptableServiceInfo, _ = parseAcceptableServiceInfo(a)
		}

		if seconds, err := optionalUint32(ans.AVPs, AVPRetryInterval, tgpp.VendorID); err == nil && seconds != nil {
			e.RetryInterval = time.Duration(*seconds) * time.Second
		}

		return AAAnswer{}, e
	}

	if err != nil {
		return AAAnswer{}, err
	}

	a := AAAnswer{Result: result, Features: featureList(ans)}

	if a.AccessNetworkChargingIdentifiers, err = chargingIdentifiers(ans.AVPs); err != nil {
		return AAAnswer{}, malformedf("Access-Network-Charging-Identifier: %w", err)
	}

	if a.AccessNetworkChargingAddress, err = optionalAddress(ans.AVPs, AVPAccessNetworkChargingAddress); err != nil {
		return AAAnswer{}, malformedf("Access-Network-Charging-Address: %w", err)
	}

	if a.SubscriptionIDs, err = subscriptionIDs(ans.AVPs); err != nil {
		return AAAnswer{}, malformedf("Subscription-Id: %w", err)
	}

	if a.AccessNetwork, err = parseAccessNetwork(ans.AVPs); err != nil {
		return AAAnswer{}, malformedf("access network: %w", err)
	}

	if a.ServingNetwork, err = parseServingNetwork(ans.AVPs); err != nil {
		return AAAnswer{}, malformedf("serving network: %w", err)
	}

	if a.NetLocAccessSupport, err = optionalEnum[NetLocAccessSupport](ans.AVPs, tgpp.AVPNetLocAccessSupport, tgpp.VendorID); err != nil {
		return AAAnswer{}, malformedf("NetLoc-Access-Support: %w", err)
	}

	if a.Flows, err = flowsList(ans.AVPs); err != nil {
		return AAAnswer{}, malformedf("Flows: %w", err)
	}

	return a, nil
}

func restorationWithState(t *RequestType, noState bool) bool {
	return t != nil && *t == RequestPCSCFRestoration && !noState
}

func bindingAVPs(v4, v6 netip.Addr) ([]diameter.AVP, error) {
	var avps []diameter.AVP

	if v4.IsValid() {
		if !validFramedIPAddress(v4) {
			return nil, invalidf("Framed-IP-Address %s", v4)
		}

		avps = append(avps, diameter.OctetString(diameter.AVPFramedIPAddress, diameter.AVPFlagMandatory, 0, v4.AsSlice()))
	}

	if v6.IsValid() {
		if !v6.Is6() || v6.Is4In6() || v6.Zone() != "" {
			return nil, invalidf("Framed-IPv6-Prefix %s", v6)
		}

		data := append([]byte{0, framedIPv6PrefixLength}, v6.AsSlice()...)
		avps = append(avps, diameter.OctetString(diameter.AVPFramedIPv6Prefix, diameter.AVPFlagMandatory, 0, data))
	}

	return avps, nil
}

func binding(m *diameter.Message) (netip.Addr, netip.Addr, error) {
	var v4, v6 netip.Addr

	if a, ok := m.Find(diameter.AVPFramedIPAddress, 0); ok {
		if len(a.Data) != 4 {
			return netip.Addr{}, netip.Addr{}, tgpp.InvalidLength(a)
		}

		v4 = netip.AddrFrom4([4]byte(a.Data))
		if !validFramedIPAddress(v4) {
			return netip.Addr{}, netip.Addr{}, tgpp.InvalidAVP(a)
		}
	}

	if a, ok := m.Find(diameter.AVPFramedIPv6Prefix, 0); ok {
		switch {
		case len(a.Data) < 2:
			return netip.Addr{}, netip.Addr{}, tgpp.InvalidLength(a)
		case a.Data[1] != framedIPv6PrefixLength:
			return netip.Addr{}, netip.Addr{}, tgpp.InvalidAVP(a)
		case len(a.Data) != framedIPv6PrefixOctets:
			return netip.Addr{}, netip.Addr{}, tgpp.InvalidLength(a)
		}

		v6 = netip.AddrFrom16([16]byte(a.Data[2:]))
		if v6.Is4In6() {
			return netip.Addr{}, netip.Addr{}, tgpp.InvalidAVP(a)
		}
	}

	return v4, v6, nil
}

func validFramedIPAddress(a netip.Addr) bool {
	return a.Is4() && a != netip.AddrFrom4([4]byte{0xff, 0xff, 0xff, 0xff}) && a != netip.AddrFrom4([4]byte{0xff, 0xff, 0xff, 0xfe})
}
