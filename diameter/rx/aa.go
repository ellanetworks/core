// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"bytes"
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
	NoStateMaintained       bool
	Features                Features
	RequiredFeatures        Features
}

type AAAnswer struct {
	Result                           tgpp.Result
	AccessNetworkChargingIdentifiers []AccessNetworkChargingIdentifier
	AccessNetworkChargingAddress     netip.Addr
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
	{Code: AVPSubscriptionID}:                              {Multiple: true},
	vendorKey(tgpp.AVPSupportedFeatures):                   {Multiple: true},
	{Code: avpReservationPriority, VendorID: etsiVendorID}: {},
	{Code: AVPFramedIPAddress}:                             {},
	{Code: AVPFramedIPv6Prefix}:                            {},
	{Code: AVPCalledStationID}:                             {},
	vendorKey(AVPServiceURN):                               {},
	vendorKey(AVPSponsoredConnectivityData):                {},
	vendorKey(AVPMPSIdentifier):                            {},
	vendorKey(AVPGCSIdentifier):                            {},
	vendorKey(AVPMCPTTIdentifier):                          {},
	vendorKey(AVPMCVideoIdentifier):                        {},
	vendorKey(AVPIMSContentIdentifier):                     {},
	vendorKey(AVPIMSContentType):                           {},
	vendorKey(avpCallingPartyAddress):                      {Multiple: true},
	vendorKey(AVPCalleeInformation):                        {},
	vendorKey(AVPRxRequestType):                            {},
	vendorKey(AVPRequiredAccessInfo):                       {Multiple: true},
	vendorKey(AVPAFRequestedData):                          {},
	vendorKey(avpReferenceID):                              {},
	vendorKey(AVPPreemptionControlInfo):                    {},
	vendorKey(AVPMPSAction):                                {},
	{Code: diameter.AVPAuthorizationLifetime}:              {},
	{Code: diameter.AVPAuthGracePeriod}:                    {},
	{Code: diameter.AVPSessionTimeout}:                     {},
})

func NewAARequest(env tgpp.Envelope, r AARequest) (*diameter.Message, error) {
	switch {
	case r.ServiceInfoStatus > maxServiceInfoStatus:
		return nil, invalid("Service-Info-Status %d", uint32(r.ServiceInfoStatus))
	case r.SIPForkingIndication > maxSIPForkingIndication:
		return nil, invalid("SIP-Forking-Indication %d", uint32(r.SIPForkingIndication))
	case r.RequestType != nil && *r.RequestType > maxRequestType:
		return nil, invalid("Rx-Request-Type %d", uint32(*r.RequestType))
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

	features, err := featureAVPs(r.Features, r.RequiredFeatures)
	if err != nil {
		return nil, err
	}

	avps = append(avps, features...)

	binding, err := bindingAVPs(r.FramedIPAddress, r.FramedIPv6Address)
	if err != nil {
		return nil, err
	}

	avps = append(avps, binding...)

	if r.CalledStationID != "" {
		avps = append(avps, diameter.UTF8String(AVPCalledStationID, diameter.AVPFlagMandatory, 0, r.CalledStationID))
	}

	if r.ServiceURN != "" {
		avps = append(avps, vendorOctets(AVPServiceURN, []byte(r.ServiceURN)))
	}

	if r.RequestType != nil {
		avps = append(avps, diameter.Unsigned32(AVPRxRequestType, 0, tgpp.VendorID, uint32(*r.RequestType)))
	}

	return newRequest(env, CommandAA, avps...), nil
}

func CheckAA(req *diameter.Message) error {
	return checkRequest(aarRules, req)
}

func ParseAARequest(req *diameter.Message) (AARequest, error) {
	if err := CheckAA(req); err != nil {
		return AARequest{}, err
	}

	r := AARequest{Features: featureList(req), RequiredFeatures: requiredFeatures(req)}

	var err error

	if r.IPDomainID, err = optionalOctets(req, AVPIPDomainID, tgpp.VendorID); err != nil {
		return AARequest{}, err
	}

	state, hasState, err := enum(req.AVPs, diameter.AVPAuthSessionState, 0, upTo(diameter.AuthSessionStateNoStateMaintained))
	if err != nil {
		return AARequest{}, err
	}

	r.NoStateMaintained = hasState && state == diameter.AuthSessionStateNoStateMaintained

	for _, f := range []struct {
		code, vendorID uint32
		dst            *string
	}{
		{AVPAFApplicationIdentifier, tgpp.VendorID, &r.AFApplicationIdentifier},
		{AVPAFChargingIdentifier, tgpp.VendorID, &r.AFChargingIdentifier},
		{AVPCalledStationID, 0, &r.CalledStationID},
		{AVPServiceURN, tgpp.VendorID, &r.ServiceURN},
	} {
		v, err := optionalOctets(req, f.code, f.vendorID)
		if err != nil {
			return AARequest{}, err
		}

		*f.dst = string(v)
	}

	for _, a := range diameter.FindAll(req.AVPs, AVPMediaComponentDescription, tgpp.VendorID) {
		c, err := parseMediaComponent(a)
		if err != nil {
			return AARequest{}, err
		}

		r.MediaComponents = append(r.MediaComponents, c)
	}

	if r.ServiceInfoStatus, _, err = enum(req.AVPs, AVPServiceInfoStatus, tgpp.VendorID, upTo(maxServiceInfoStatus)); err != nil {
		return AARequest{}, err
	}

	if r.SIPForkingIndication, _, err = enum(req.AVPs, AVPSIPForkingIndication, tgpp.VendorID, upTo(maxSIPForkingIndication)); err != nil {
		return AARequest{}, err
	}

	if r.SpecificActions, err = specificActions(req); err != nil {
		return AARequest{}, err
	}

	if r.SubscriptionIDs, err = subscriptionIDs(req.AVPs); err != nil {
		return AARequest{}, err
	}

	if r.FramedIPAddress, r.FramedIPv6Address, err = binding(req); err != nil {
		return AARequest{}, err
	}

	if r.RequestType, err = optionalEnum(req.AVPs, AVPRxRequestType, upTo(maxRequestType)); err != nil {
		return AARequest{}, err
	}

	return r, nil
}

func NewAAAnswer(req *diameter.Message, id diameter.Identity, a AAAnswer) (*diameter.Message, error) {
	result, err := successResult(a.Result)
	if err != nil {
		return nil, err
	}

	features, err := featureAVPs(a.Features, 0)
	if err != nil {
		return nil, err
	}

	ans := NewAnswer(req, id, result)

	for _, c := range a.AccessNetworkChargingIdentifiers {
		charging, err := chargingIdentifierAVP(c)
		if err != nil {
			return nil, err
		}

		ans.AVPs = append(ans.AVPs, charging)
	}

	if a.AccessNetworkChargingAddress.IsValid() {
		ans.AVPs = append(ans.AVPs, diameter.Address(AVPAccessNetworkChargingAddress, diameter.AVPFlagMandatory, tgpp.VendorID,
			a.AccessNetworkChargingAddress))
	}

	ans.AVPs = append(ans.AVPs, features...)

	subscriptions, err := subscriptionIDAVPs(a.SubscriptionIDs)
	if err != nil {
		return nil, err
	}

	ans.AVPs = append(ans.AVPs, subscriptions...)

	return ans, nil
}

func NewAAErrorAnswer(req *diameter.Message, id diameter.Identity, e AAError) (*diameter.Message, error) {
	if err := errorResult(e.Result); err != nil {
		return nil, err
	}

	features, err := featureAVPs(e.Features, 0)
	if err != nil {
		return nil, err
	}

	ans := NewAnswer(req, id, e.Result)

	if e.AcceptableServiceInfo != nil {
		ans.AVPs = append(ans.AVPs, acceptableServiceInfoAVP(*e.AcceptableServiceInfo))
	}

	ans.AVPs = append(ans.AVPs, features...)

	if e.RetryInterval != 0 {
		seconds := e.RetryInterval / time.Second
		if e.RetryInterval%time.Second != 0 || seconds < 0 || seconds > math.MaxUint32 {
			return nil, invalid("retry interval %s", e.RetryInterval)
		}

		ans.AVPs = append(ans.AVPs, diameter.Unsigned32(AVPRetryInterval, 0, tgpp.VendorID, uint32(seconds)))
	}

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

		if retry, ok := ans.Find(AVPRetryInterval, tgpp.VendorID); ok {
			if seconds, err := retry.Unsigned32(); err == nil {
				e.RetryInterval = time.Duration(seconds) * time.Second
			}
		}

		return AAAnswer{}, e
	}

	if err != nil {
		return AAAnswer{}, err
	}

	a := AAAnswer{Result: result, Features: featureList(ans)}

	for _, c := range diameter.FindAll(ans.AVPs, AVPAccessNetworkChargingIdentifier, tgpp.VendorID) {
		charging, err := parseChargingIdentifier(c)
		if err != nil {
			return AAAnswer{}, malformed("Access-Network-Charging-Identifier: %w", err)
		}

		a.AccessNetworkChargingIdentifiers = append(a.AccessNetworkChargingIdentifiers, charging)
	}

	if address, ok := ans.Find(AVPAccessNetworkChargingAddress, tgpp.VendorID); ok {
		if a.AccessNetworkChargingAddress, err = address.Address(); err != nil {
			return AAAnswer{}, malformed("Access-Network-Charging-Address: %w", err)
		}
	}

	if a.SubscriptionIDs, err = subscriptionIDs(ans.AVPs); err != nil {
		return AAAnswer{}, malformed("Subscription-Id: %w", err)
	}

	return a, nil
}

func optionalOctets(m *diameter.Message, code, vendorID uint32) ([]byte, error) {
	a, ok := m.Find(code, vendorID)
	if !ok {
		return nil, nil
	}

	if len(a.Data) == 0 {
		return nil, tgpp.InvalidAVP(a)
	}

	return bytes.Clone(a.Data), nil
}

func specificActionAVPs(actions []SpecificAction) ([]diameter.AVP, error) {
	avps := make([]diameter.AVP, 0, len(actions))

	for _, a := range actions {
		if a > maxSpecificAction {
			return nil, invalid("Specific-Action %d", uint32(a))
		}

		avps = append(avps, vendorUnsigned(AVPSpecificAction, uint32(a)))
	}

	return avps, nil
}

func specificActions(m *diameter.Message) ([]SpecificAction, error) {
	var actions []SpecificAction

	for _, a := range diameter.FindAll(m.AVPs, AVPSpecificAction, tgpp.VendorID) {
		v, err := a.Unsigned32()
		if err != nil || SpecificAction(v) > maxSpecificAction {
			return nil, tgpp.InvalidAVP(a)
		}

		actions = append(actions, SpecificAction(v))
	}

	return actions, nil
}

func subscriptionIDAVPs(ids []SubscriptionID) ([]diameter.AVP, error) {
	avps := make([]diameter.AVP, 0, len(ids))

	for _, s := range ids {
		a, err := subscriptionIDAVP(s)
		if err != nil {
			return nil, err
		}

		avps = append(avps, a)
	}

	return avps, nil
}

func subscriptionIDs(avps []diameter.AVP) ([]SubscriptionID, error) {
	var ids []SubscriptionID

	for _, a := range diameter.FindAll(avps, AVPSubscriptionID, 0) {
		s, err := parseSubscriptionID(a)
		if err != nil {
			return nil, err
		}

		ids = append(ids, s)
	}

	return ids, nil
}

func bindingAVPs(v4, v6 netip.Addr) ([]diameter.AVP, error) {
	var avps []diameter.AVP

	if v4.IsValid() {
		if !validFramedIPAddress(v4) {
			return nil, invalid("Framed-IP-Address %s", v4)
		}

		avps = append(avps, diameter.OctetString(AVPFramedIPAddress, diameter.AVPFlagMandatory, 0, v4.AsSlice()))
	}

	if v6.IsValid() {
		if !v6.Is6() || v6.Is4In6() || v6.Zone() != "" {
			return nil, invalid("Framed-IPv6-Prefix %s", v6)
		}

		data := append([]byte{0, framedIPv6PrefixLength}, v6.AsSlice()...)
		avps = append(avps, diameter.OctetString(AVPFramedIPv6Prefix, diameter.AVPFlagMandatory, 0, data))
	}

	return avps, nil
}

func binding(m *diameter.Message) (netip.Addr, netip.Addr, error) {
	var v4, v6 netip.Addr

	if a, ok := m.Find(AVPFramedIPAddress, 0); ok {
		addr, ok := netip.AddrFromSlice(a.Data)
		if !ok || !addr.Is4() || !validFramedIPAddress(addr) {
			return netip.Addr{}, netip.Addr{}, tgpp.InvalidAVP(a)
		}

		v4 = addr
	}

	if a, ok := m.Find(AVPFramedIPv6Prefix, 0); ok {
		if len(a.Data) != framedIPv6PrefixOctets || a.Data[1] != framedIPv6PrefixLength {
			return netip.Addr{}, netip.Addr{}, tgpp.InvalidAVP(a)
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
