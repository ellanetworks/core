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
	AccessNetwork                    AccessNetwork
	Flows                            []Flows
	SubscriptionIDs                  []SubscriptionID
	AbortCause                       *AbortCause
	NetLocAccessSupport              *NetLocAccessSupport
	ServingNetwork                   ServingNetwork
	Location                         UserLocation
	PCSessionRecoveryStatus          *PCSessionRecoveryStatus
}

type ReAuthAnswer struct {
	Result tgpp.Result
}

var rarRules = commonRequestRules.With(accessNetworkRules).With(servingNetworkRules).With(userLocationRules).With(diameter.Rules{
	{Code: diameter.AVPDestinationHost}:           {Required: true},
	vendorKey(AVPSpecificAction):                  {Required: true, Multiple: true, MinLength: 4},
	vendorKey(AVPAccessNetworkChargingIdentifier): {Multiple: true},
	vendorKey(AVPAccessNetworkChargingAddress):    {},
	vendorKey(AVPFlows):                           {Multiple: true},
	{Code: diameter.AVPSubscriptionID}:            {Multiple: true},
	vendorKey(AVPAbortCause):                      {},
	vendorKey(avpMAInformation):                   {},
	vendorKey(tgpp.AVPNetLocAccessSupport):        {},
	vendorKey(avpSponsoredConnectivity):           {},
	vendorKey(AVPPCSessionRecoveryStatus):         {},
	{Code: diameter.AVPClass}:                     {Multiple: true},
	{Code: diameter.AVPReAuthRequestType}:         {},
})

type missingAVP struct {
	code      uint32
	minLength int
}

func (r ReAuthRequest) missing() (missingAVP, bool) {
	for _, req := range []struct {
		action  SpecificAction
		present bool
		avp     missingAVP
	}{
		{ActionChargingCorrelationExchange, len(r.AccessNetworkChargingIdentifiers) > 0, missingAVP{AVPAccessNetworkChargingIdentifier, 0}},
		{ActionIPCANChange, r.AccessNetwork.IPCANType != nil, missingAVP{tgpp.AVPIPCANType, 4}},
		{ActionPLMNChange, r.ServingNetwork.PLMN != "", missingAVP{tgpp.AVP3GPPSGSNMCCMNC, 5}},
		{ActionCNHealthMonitor, r.PCSessionRecoveryStatus != nil, missingAVP{AVPPCSessionRecoveryStatus, 4}},
	} {
		if slices.Contains(r.SpecificActions, req.action) && !req.present {
			return req.avp, true
		}
	}

	return missingAVP{}, false
}

func NewReAuthRequest(env tgpp.Envelope, r ReAuthRequest) (*diameter.Message, error) {
	switch {
	case env.DestinationHost == "":
		return nil, invalidf("re-auth without a destination host")
	case len(r.SpecificActions) == 0:
		return nil, invalidf("re-auth without a specific action")
	case !validEnum(r.AbortCause):
		return nil, invalidf("Abort-Cause %s", *r.AbortCause)
	case !validEnum(r.NetLocAccessSupport):
		return nil, invalidf("NetLoc-Access-Support %s", *r.NetLocAccessSupport)
	case !validEnum(r.PCSessionRecoveryStatus):
		return nil, invalidf("PC-Session-Recovery-Status %s", *r.PCSessionRecoveryStatus)
	}

	if m, ok := r.missing(); ok {
		return nil, invalidf("re-auth for %v without AVP %d", r.SpecificActions, m.code)
	}

	avps, err := specificActionAVPs(r.SpecificActions)
	if err != nil {
		return nil, err
	}

	charging, err := chargingIdentifierAVPs(r.AccessNetworkChargingIdentifiers)
	if err != nil {
		return nil, err
	}

	avps = append(avps, charging...)

	if r.AccessNetworkChargingAddress.IsValid() {
		address, err := addressAVP(AVPAccessNetworkChargingAddress, diameter.AVPFlagMandatory, r.AccessNetworkChargingAddress)
		if err != nil {
			return nil, err
		}

		avps = append(avps, address)
	}

	access, err := r.AccessNetwork.avps()
	if err != nil {
		return nil, err
	}

	avps = append(avps, access...)

	flows, err := flowsAVPs(r.Flows)
	if err != nil {
		return nil, err
	}

	avps = append(avps, flows...)

	subscriptions, err := subscriptionIDAVPs(r.SubscriptionIDs)
	if err != nil {
		return nil, err
	}

	avps = append(avps, subscriptions...)
	avps = appendOptional(avps, AVPAbortCause, diameter.AVPFlagMandatory, (*uint32)(r.AbortCause))
	avps = appendOptional(avps, tgpp.AVPNetLocAccessSupport, 0, (*uint32)(r.NetLocAccessSupport))

	location, err := r.Location.avps()
	if err != nil {
		return nil, err
	}

	avps = append(avps, location...)

	serving, err := r.ServingNetwork.avps()
	if err != nil {
		return nil, err
	}

	avps = append(avps, serving...)
	avps = appendOptional(avps, AVPPCSessionRecoveryStatus, 0, (*uint32)(r.PCSessionRecoveryStatus))

	return newRequest(env, CommandReAuth, avps...)
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

	if r.SpecificActions, err = specificActions(req.AVPs, false); err != nil {
		return ReAuthRequest{}, err
	}

	if r.AccessNetworkChargingIdentifiers, err = chargingIdentifiers(req.AVPs); err != nil {
		return ReAuthRequest{}, err
	}

	if r.AccessNetworkChargingAddress, err = optionalAddress(req.AVPs, AVPAccessNetworkChargingAddress); err != nil {
		return ReAuthRequest{}, err
	}

	if r.AccessNetwork, err = parseAccessNetwork(req.AVPs); err != nil {
		return ReAuthRequest{}, err
	}

	if r.Flows, err = flowsList(req.AVPs); err != nil {
		return ReAuthRequest{}, err
	}

	if r.SubscriptionIDs, err = subscriptionIDs(req.AVPs); err != nil {
		return ReAuthRequest{}, err
	}

	if r.AbortCause, err = optionalEnum[AbortCause](req.AVPs, AVPAbortCause, tgpp.VendorID); err != nil {
		return ReAuthRequest{}, err
	}

	if r.NetLocAccessSupport, err = optionalEnum[NetLocAccessSupport](req.AVPs, tgpp.AVPNetLocAccessSupport, tgpp.VendorID); err != nil {
		return ReAuthRequest{}, err
	}

	if r.ServingNetwork, err = parseServingNetwork(req.AVPs); err != nil {
		return ReAuthRequest{}, err
	}

	if r.Location, err = parseUserLocation(req.AVPs); err != nil {
		return ReAuthRequest{}, err
	}

	if r.PCSessionRecoveryStatus, err = optionalEnum[PCSessionRecoveryStatus](req.AVPs, AVPPCSessionRecoveryStatus, tgpp.VendorID); err != nil {
		return ReAuthRequest{}, err
	}

	if m, ok := r.missing(); ok {
		return ReAuthRequest{}, tgpp.MissingAVP(m.code, tgpp.VendorID, m.minLength)
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
