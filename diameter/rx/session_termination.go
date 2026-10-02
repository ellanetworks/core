// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type SessionTerminationRequest struct {
	Cause              TerminationCause
	RequiredAccessInfo []RequiredAccessInfo
	Class              [][]byte
}

type SessionTerminationAnswer struct {
	Result              tgpp.Result
	ServingNetwork      ServingNetwork
	Location            UserLocation
	NetLocAccessSupport *NetLocAccessSupport
}

var strRules = commonRequestRules.With(diameter.Rules{
	{Code: diameter.AVPTerminationCause}:  {Required: true, MinLength: 4},
	vendorKey(AVPRequiredAccessInfo):      {Multiple: true},
	{Code: diameter.AVPClass}:             {Multiple: true},
	vendorKey(AVPAFApplicationIdentifier): {},
})

func NewSessionTerminationRequest(env tgpp.Envelope, r SessionTerminationRequest) (*diameter.Message, error) {
	if !r.Cause.valid() {
		return nil, invalidf("Termination-Cause %s", r.Cause)
	}

	required, err := enumListAVPs(AVPRequiredAccessInfo, 0, r.RequiredAccessInfo)
	if err != nil {
		return nil, err
	}

	avps := append([]diameter.AVP{diameter.Unsigned32(diameter.AVPTerminationCause, diameter.AVPFlagMandatory, 0, uint32(r.Cause))}, required...)

	return newRequest(env, CommandSessionTermination, append(avps, classAVPs(r.Class)...)...)
}

func CheckSessionTermination(req *diameter.Message) error {
	return checkRequest(strRules, req)
}

func ParseSessionTerminationRequest(req *diameter.Message) (SessionTerminationRequest, error) {
	if err := CheckSessionTermination(req); err != nil {
		return SessionTerminationRequest{}, err
	}

	cause, err := defaultEnum[TerminationCause](req.AVPs, diameter.AVPTerminationCause, 0)
	if err != nil {
		return SessionTerminationRequest{}, err
	}

	required, err := enumList[RequiredAccessInfo](req.AVPs, AVPRequiredAccessInfo, tgpp.VendorID)
	if err != nil {
		return SessionTerminationRequest{}, err
	}

	return SessionTerminationRequest{Cause: cause, RequiredAccessInfo: required, Class: classes(req.AVPs)}, nil
}

func NewSessionTerminationAnswer(req *diameter.Message, id diameter.Identity, a SessionTerminationAnswer) (*diameter.Message, error) {
	if !validEnum(a.NetLocAccessSupport) {
		return nil, invalidf("NetLoc-Access-Support %s", *a.NetLocAccessSupport)
	}

	location, err := a.Location.avps()
	if err != nil {
		return nil, err
	}

	serving, err := a.ServingNetwork.avps()
	if err != nil {
		return nil, err
	}

	ans, err := successAnswer(req, id, a.Result)
	if err != nil {
		return nil, err
	}

	ans.AVPs = append(ans.AVPs, location...)
	ans.AVPs = append(ans.AVPs, serving...)
	ans.AVPs = appendOptional(ans.AVPs, tgpp.AVPNetLocAccessSupport, 0, (*uint32)(a.NetLocAccessSupport))

	return ans, nil
}

func ParseSessionTerminationAnswer(ans *diameter.Message) (SessionTerminationAnswer, error) {
	result, err := parseResult(ans)
	if err != nil {
		return SessionTerminationAnswer{}, err
	}

	a := SessionTerminationAnswer{Result: result}

	if a.Location, err = parseUserLocation(ans.AVPs); err != nil {
		return SessionTerminationAnswer{}, malformedf("user location: %w", err)
	}

	if a.ServingNetwork, err = parseServingNetwork(ans.AVPs); err != nil {
		return SessionTerminationAnswer{}, malformedf("serving network: %w", err)
	}

	if a.NetLocAccessSupport, err = optionalEnum[NetLocAccessSupport](ans.AVPs, tgpp.AVPNetLocAccessSupport, tgpp.VendorID); err != nil {
		return SessionTerminationAnswer{}, malformedf("NetLoc-Access-Support: %w", err)
	}

	return a, nil
}
