// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cx

import (
	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type LocationInfoRequest struct {
	PublicIdentity    string
	Originating       bool
	AuthorizationType AuthorizationType
	Features          Features
	FeaturesRequired  bool
}

type LocationInfo struct {
	Result                   tgpp.Result
	ServerName               string
	Capabilities             *ServerCapabilities
	WildcardedPublicIdentity string
	PSIDirectRouting         bool
	Features                 Features
}

var lirRules = commonRequestRules.With(diameter.Rules{
	{Code: AVPOriginatingRequest, VendorID: tgpp.VendorID}:    {},
	{Code: AVPPublicIdentity, VendorID: tgpp.VendorID}:        {Required: true},
	{Code: AVPUserAuthorizationType, VendorID: tgpp.VendorID}: {},
	{Code: AVPSessionPriority, VendorID: tgpp.VendorID}:       {},
})

func NewLocationInfoRequest(env tgpp.Envelope, r LocationInfoRequest) (*diameter.Message, error) {
	if r.AuthorizationType != AuthorizationRegistration && r.AuthorizationType != AuthorizationRegistrationAndCapabilities {
		return nil, invalidf("User-Authorization-Type %d in a location query", r.AuthorizationType)
	}

	public, err := publicIdentityAVP(r.PublicIdentity)
	if err != nil {
		return nil, err
	}

	var avps []diameter.AVP

	if r.Originating {
		avps = append(avps, vendorUnsigned(AVPOriginatingRequest, originating))
	}

	avps = append(avps, public)

	if r.AuthorizationType != AuthorizationRegistration {
		avps = append(avps, vendorUnsigned(AVPUserAuthorizationType, uint32(r.AuthorizationType)))
	}

	return newRequest(env, CommandLocationInfo, r.Features, r.FeaturesRequired, avps...)
}

func CheckLocationInfo(req *diameter.Message) error {
	return lirRules.Check(req)
}

func ParseLocationInfoRequest(req *diameter.Message) (LocationInfoRequest, error) {
	if err := CheckLocationInfo(req); err != nil {
		return LocationInfoRequest{}, err
	}

	public, err := requestPublicIdentity(req)
	if err != nil {
		return LocationInfoRequest{}, err
	}

	_, isOriginating, err := optionalUnsigned(req, AVPOriginatingRequest, originating)
	if err != nil {
		return LocationInfoRequest{}, err
	}

	authType, _, err := optionalUnsigned(req, AVPUserAuthorizationType, uint32(AuthorizationRegistrationAndCapabilities))
	if err != nil {
		return LocationInfoRequest{}, err
	}

	if AuthorizationType(authType) == AuthorizationDeregistration {
		a, _ := req.Find(AVPUserAuthorizationType, tgpp.VendorID)
		return LocationInfoRequest{}, tgpp.InvalidAVP(a)
	}

	return LocationInfoRequest{
		PublicIdentity:    public,
		Originating:       isOriginating,
		AuthorizationType: AuthorizationType(authType),
		Features:          featureList(req),
		FeaturesRequired:  featuresRequired(req),
	}, nil
}

func NewLocationInfoAnswer(req *diameter.Message, id diameter.Identity, l LocationInfo) (*diameter.Message, error) {
	result, err := successResult(l.Result)
	if err != nil {
		return nil, err
	}

	avps, err := selection(l.ServerName, l.Capabilities)
	if err != nil {
		return nil, err
	}

	if l.WildcardedPublicIdentity != "" {
		w, err := wildcardedIdentityAVP(l.WildcardedPublicIdentity)
		if err != nil {
			return nil, err
		}

		avps = append(avps, w)
	}

	if l.PSIDirectRouting {
		avps = append(avps, diameter.Unsigned32(AVPLIAFlags, 0, tgpp.VendorID, liaFlagPSIDirectRouting))
	}

	ans := NewAnswer(req, id, result, l.Features)
	ans.AVPs = append(ans.AVPs, avps...)

	return ans, nil
}

func ParseLocationInfoAnswer(ans *diameter.Message) (LocationInfo, error) {
	result, err := parseResult(ans)
	if err != nil {
		return LocationInfo{}, err
	}

	name, capabilities, err := parseSelection(ans)
	if err != nil {
		return LocationInfo{}, err
	}

	liaFlags, err := answerUnsigned(ans, AVPLIAFlags, "LIA-Flags")
	if err != nil {
		return LocationInfo{}, err
	}

	wildcarded, err := answerWildcardedIdentity(ans)
	if err != nil {
		return LocationInfo{}, err
	}

	return LocationInfo{
		Result:                   result,
		ServerName:               name,
		Capabilities:             capabilities,
		WildcardedPublicIdentity: wildcarded,
		PSIDirectRouting:         liaFlags&liaFlagPSIDirectRouting != 0,
		Features:                 featureList(ans),
	}, nil
}
