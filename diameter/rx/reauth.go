// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"net/netip"
	"slices"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type ReAuthRequest struct {
	SpecificActions                  []SpecificAction
	AccessNetworkChargingIdentifiers []AccessNetworkChargingIdentifier
	AccessNetworkChargingAddress     netip.Addr
	Flows                            []Flows
	SubscriptionIDs                  []SubscriptionID
	AbortCause                       *AbortCause
}

type ReAuthAnswer struct {
	Result tgpp.Result
}

var rarRules = commonRequestRules.With(diameter.Rules{
	{Code: diameter.AVPDestinationHost}:           {Required: true},
	vendorKey(AVPSpecificAction):                  {Required: true, Multiple: true, MinLength: 4},
	vendorKey(AVPAccessNetworkChargingIdentifier): {Multiple: true},
	vendorKey(AVPAccessNetworkChargingAddress):    {},
	vendorKey(avpANGWAddress):                     {Multiple: true},
	vendorKey(avpANTrusted):                       {},
	vendorKey(AVPFlows):                           {Multiple: true},
	{Code: AVPSubscriptionID}:                     {Multiple: true},
	vendorKey(AVPAbortCause):                      {},
	vendorKey(avpIPCANType):                       {},
	vendorKey(AVPMAInformation):                   {},
	vendorKey(avpNetLocAccessSupport):             {},
	vendorKey(avpRATType):                         {},
	vendorKey(AVPSponsoredConnectivityData):       {},
	vendorKey(avp3GPPUserLocationInfo):            {},
	vendorKey(avpUserLocationInfoTime):            {},
	vendorKey(avp3GPPMSTimeZone):                  {},
	vendorKey(AVPServingSatelliteIdentity):        {},
	vendorKey(avpRANNASReleaseCause):              {Multiple: true},
	vendorKey(AVP5GSRANNASReleaseCause):           {Multiple: true},
	vendorKey(avp3GPPSGSNMCCMNC):                  {},
	vendorKey(AVPNID):                             {},
	vendorKey(avpTWANIdentifier):                  {},
	vendorKey(avpTCPSourcePort):                   {},
	vendorKey(avpUDPSourcePort):                   {},
	vendorKey(avpUELocalIPAddress):                {},
	vendorKey(AVPWirelineUserLocationInfo):        {},
	vendorKey(AVPPCSessionRecoveryStatus):         {},
	{Code: diameter.AVPClass}:                     {Multiple: true},
	{Code: diameter.AVPReAuthRequestType}:         {},
})

func NewReAuthRequest(env tgpp.Envelope, r ReAuthRequest) (*diameter.Message, error) {
	switch {
	case env.DestinationHost == "":
		return nil, invalid("re-auth without a destination host")
	case len(r.SpecificActions) == 0:
		return nil, invalid("re-auth without a specific action")
	case r.AbortCause != nil && *r.AbortCause > maxAbortCause:
		return nil, invalid("Abort-Cause %d", uint32(*r.AbortCause))
	case slices.Contains(r.SpecificActions, ActionChargingCorrelationExchange) && len(r.AccessNetworkChargingIdentifiers) == 0:
		return nil, invalid("charging correlation exchange without an access network charging identifier")
	}

	avps, err := specificActionAVPs(r.SpecificActions)
	if err != nil {
		return nil, err
	}

	for _, c := range r.AccessNetworkChargingIdentifiers {
		charging, err := chargingIdentifierAVP(c)
		if err != nil {
			return nil, err
		}

		avps = append(avps, charging)
	}

	if r.AccessNetworkChargingAddress.IsValid() {
		avps = append(avps, diameter.Address(AVPAccessNetworkChargingAddress, diameter.AVPFlagMandatory, tgpp.VendorID, r.AccessNetworkChargingAddress))
	}

	for _, f := range r.Flows {
		avps = append(avps, flowsAVP(f))
	}

	subscriptions, err := subscriptionIDAVPs(r.SubscriptionIDs)
	if err != nil {
		return nil, err
	}

	avps = append(avps, subscriptions...)

	if r.AbortCause != nil {
		avps = append(avps, vendorUnsigned(AVPAbortCause, uint32(*r.AbortCause)))
	}

	return newRequest(env, CommandReAuth, avps...), nil
}

func CheckReAuth(req *diameter.Message) error {
	return checkRequest(rarRules, req)
}

func ParseReAuthRequest(req *diameter.Message) (ReAuthRequest, error) {
	if err := CheckReAuth(req); err != nil {
		return ReAuthRequest{}, err
	}

	var (
		r   ReAuthRequest
		err error
	)

	if r.SpecificActions, err = specificActions(req); err != nil {
		return ReAuthRequest{}, err
	}

	for _, c := range diameter.FindAll(req.AVPs, AVPAccessNetworkChargingIdentifier, tgpp.VendorID) {
		charging, err := parseChargingIdentifier(c)
		if err != nil {
			return ReAuthRequest{}, err
		}

		r.AccessNetworkChargingIdentifiers = append(r.AccessNetworkChargingIdentifiers, charging)
	}

	if slices.Contains(r.SpecificActions, ActionChargingCorrelationExchange) && len(r.AccessNetworkChargingIdentifiers) == 0 {
		return ReAuthRequest{}, tgpp.MissingAVP(AVPAccessNetworkChargingIdentifier, tgpp.VendorID)
	}

	if a, ok := req.Find(AVPAccessNetworkChargingAddress, tgpp.VendorID); ok {
		if r.AccessNetworkChargingAddress, err = a.Address(); err != nil {
			return ReAuthRequest{}, tgpp.InvalidAVP(a)
		}
	}

	if r.Flows, err = flowsList(req.AVPs); err != nil {
		return ReAuthRequest{}, err
	}

	if r.SubscriptionIDs, err = subscriptionIDs(req.AVPs); err != nil {
		return ReAuthRequest{}, err
	}

	if r.AbortCause, err = optionalEnum(req.AVPs, AVPAbortCause, upTo(maxAbortCause)); err != nil {
		return ReAuthRequest{}, err
	}

	return r, nil
}

func NewReAuthAnswer(req *diameter.Message, id diameter.Identity, a ReAuthAnswer) (*diameter.Message, error) {
	return successAnswer(req, id, a.Result)
}

func ParseReAuthAnswer(ans *diameter.Message) (ReAuthAnswer, error) {
	result, err := parseResult(ans)
	if err != nil {
		return ReAuthAnswer{}, err
	}

	return ReAuthAnswer{Result: result}, nil
}
