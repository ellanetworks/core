// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cx

import (
	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type UserAuthorizationRequest struct {
	PrivateIdentity       string
	PublicIdentity        string
	VisitedNetwork        string
	AuthorizationType     uint32
	EmergencyRegistration bool
	Features              uint32
}

type UserAuthorization struct {
	Result       tgpp.Result
	ServerName   string
	Capabilities *ServerCapabilities
	Features     uint32
}

var commonRequestRules = diameter.BaseRequestRules().With(diameter.Rules{
	{Code: tgpp.AVPSupportedFeatures, VendorID: tgpp.VendorID}: {Multiple: true},
	{Code: avpOCSupportedFeatures}:                             {},
})

var uarRules = commonRequestRules.With(diameter.Rules{
	{Code: diameter.AVPUserName}:                                 {Required: true},
	{Code: AVPPublicIdentity, VendorID: tgpp.VendorID}:           {Required: true},
	{Code: AVPVisitedNetworkIdentifier, VendorID: tgpp.VendorID}: {Required: true},
	{Code: AVPUserAuthorizationType, VendorID: tgpp.VendorID}:    {},
	{Code: AVPUARFlags, VendorID: tgpp.VendorID}:                 {},
})

func NewUserAuthorizationRequest(env tgpp.Envelope, r UserAuthorizationRequest) (*diameter.Message, error) {
	switch {
	case r.PrivateIdentity == "":
		return nil, invalid("user authorization without a private identity")
	case r.VisitedNetwork == "":
		return nil, invalid("user authorization without a visited network")
	case r.AuthorizationType > AuthorizationRegistrationAndCapabilities:
		return nil, invalid("User-Authorization-Type %d", r.AuthorizationType)
	}

	public, err := publicIdentityAVP(r.PublicIdentity)
	if err != nil {
		return nil, err
	}

	avps := []diameter.AVP{
		userName(r.PrivateIdentity),
		public,
		diameter.OctetString(AVPVisitedNetworkIdentifier, diameter.AVPFlagMandatory, tgpp.VendorID, []byte(r.VisitedNetwork)),
	}

	if r.AuthorizationType != AuthorizationRegistration {
		avps = append(avps, vendorUnsigned(AVPUserAuthorizationType, r.AuthorizationType))
	}

	if r.EmergencyRegistration {
		avps = append(avps, diameter.Unsigned32(AVPUARFlags, 0, tgpp.VendorID, UARFlagEmergencyRegistration))
	}

	return newRequest(env, CommandUserAuthorization, r.Features, avps...), nil
}

func CheckUserAuthorization(req *diameter.Message) error {
	return uarRules.Check(req)
}

func ParseUserAuthorizationRequest(req *diameter.Message) (UserAuthorizationRequest, error) {
	if err := CheckUserAuthorization(req); err != nil {
		return UserAuthorizationRequest{}, err
	}

	public, err := requestPublicIdentity(req)
	if err != nil {
		return UserAuthorizationRequest{}, err
	}

	authType, _, err := optionalUnsigned(req, AVPUserAuthorizationType, AuthorizationRegistrationAndCapabilities)
	if err != nil {
		return UserAuthorizationRequest{}, err
	}

	uarFlags, err := flags(req, AVPUARFlags)
	if err != nil {
		return UserAuthorizationRequest{}, err
	}

	visited, _ := req.Find(AVPVisitedNetworkIdentifier, tgpp.VendorID)
	user, _ := req.Find(diameter.AVPUserName, 0)

	return UserAuthorizationRequest{
		PrivateIdentity:       user.UTF8String(),
		PublicIdentity:        public,
		VisitedNetwork:        string(visited.Data),
		AuthorizationType:     authType,
		EmergencyRegistration: uarFlags&UARFlagEmergencyRegistration != 0,
		Features:              features(req),
	}, nil
}

func NewUserAuthorizationAnswer(req *diameter.Message, id diameter.Identity, a UserAuthorization) (*diameter.Message, error) {
	if err := successResult(a.Result); err != nil {
		return nil, err
	}

	if a.ServerName != "" && a.Capabilities != nil {
		return nil, invalid("user authorization answer with both a server name and capabilities")
	}

	avps, err := selection(a.ServerName, a.Capabilities)
	if err != nil {
		return nil, err
	}

	ans := NewAnswer(req, id, a.Result, a.Features)
	ans.AVPs = append(ans.AVPs, avps...)

	return ans, nil
}

func ParseUserAuthorizationAnswer(ans *diameter.Message) (UserAuthorization, error) {
	result, err := parseResult(ans)
	if err != nil {
		return UserAuthorization{}, err
	}

	name, capabilities, err := parseSelection(ans)
	if err != nil {
		return UserAuthorization{}, err
	}

	return UserAuthorization{Result: result, ServerName: name, Capabilities: capabilities, Features: features(ans)}, nil
}

func selection(serverName string, capabilities *ServerCapabilities) ([]diameter.AVP, error) {
	var avps []diameter.AVP

	if serverName != "" {
		a, err := serverNameAVP(serverName)
		if err != nil {
			return nil, err
		}

		avps = append(avps, a)
	}

	if capabilities != nil {
		a, err := capabilitiesAVP(*capabilities)
		if err != nil {
			return nil, err
		}

		avps = append(avps, a)
	}

	return avps, nil
}

func parseSelection(ans *diameter.Message) (string, *ServerCapabilities, error) {
	name := answerString(ans, AVPServerName, tgpp.VendorID)

	a, ok := ans.Find(AVPServerCapabilities, tgpp.VendorID)
	if !ok {
		return name, nil, nil
	}

	capabilities, err := parseCapabilities(a)
	if err != nil {
		return "", nil, malformed("Server-Capabilities: %v", err)
	}

	return name, capabilities, nil
}
