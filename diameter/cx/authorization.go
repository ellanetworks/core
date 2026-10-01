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
	AuthorizationType     AuthorizationType
	EmergencyRegistration bool
	Features              Features
}

type UserAuthorization struct {
	Result       tgpp.Result
	ServerName   string
	Capabilities *ServerCapabilities
	Features     Features
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
		avps = append(avps, vendorUnsigned(AVPUserAuthorizationType, uint32(r.AuthorizationType)))
	}

	if r.EmergencyRegistration {
		avps = append(avps, diameter.Unsigned32(AVPUARFlags, 0, tgpp.VendorID, uarFlagEmergencyRegistration))
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

	authType, _, err := optionalUnsigned(req, AVPUserAuthorizationType, uint32(AuthorizationRegistrationAndCapabilities))
	if err != nil {
		return UserAuthorizationRequest{}, err
	}

	uarFlags, err := optionalFlags(req, AVPUARFlags)
	if err != nil {
		return UserAuthorizationRequest{}, err
	}

	visited, _ := req.Find(AVPVisitedNetworkIdentifier, tgpp.VendorID)
	user, _ := req.Find(diameter.AVPUserName, 0)

	return UserAuthorizationRequest{
		PrivateIdentity:       user.UTF8String(),
		PublicIdentity:        public,
		VisitedNetwork:        string(visited.Data),
		AuthorizationType:     AuthorizationType(authType),
		EmergencyRegistration: uarFlags&uarFlagEmergencyRegistration != 0,
		Features:              featureList(req),
	}, nil
}

func NewUserAuthorizationAnswer(req *diameter.Message, id diameter.Identity, a UserAuthorization) (*diameter.Message, error) {
	result, err := successResult(a.Result)
	if err != nil {
		return nil, err
	}

	avps, err := selection(a.ServerName, a.Capabilities)
	if err != nil {
		return nil, err
	}

	ans := NewAnswer(req, id, result, a.Features)
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

	return UserAuthorization{Result: result, ServerName: name, Capabilities: capabilities, Features: featureList(ans)}, nil
}

func selection(serverName string, capabilities *ServerCapabilities) ([]diameter.AVP, error) {
	if serverName != "" && capabilities != nil {
		return nil, invalid("answer with both a server name and capabilities")
	}

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
	name, err := answerServerName(ans)
	if err != nil {
		return "", nil, err
	}

	a, ok := ans.Find(AVPServerCapabilities, tgpp.VendorID)
	if !ok {
		return name, nil, nil
	}

	if name != "" {
		return "", nil, malformed("both Server-Name and Server-Capabilities")
	}

	capabilities, err := parseCapabilities(a)
	if err != nil {
		return "", nil, malformed("Server-Capabilities: %w", err)
	}

	return name, capabilities, nil
}
