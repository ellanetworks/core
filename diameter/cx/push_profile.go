// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cx

import (
	"bytes"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type AllowedWebRTCFunctions struct {
	AuthenticationFunctions []string
	WebServerFunctions      []string
}

type PushProfileRequest struct {
	PrivateIdentity  string
	UserData         []byte
	Charging         *ChargingInformation
	AllowedWebRTC    *AllowedWebRTCFunctions
	Features         Features
	RequiredFeatures Features
}

type PushProfile struct {
	Result   tgpp.Result
	Features Features
}

var pprRules = commonRequestRules.With(diameter.Rules{
	{Code: diameter.AVPDestinationHost}:                          {Required: true},
	{Code: diameter.AVPUserName}:                                 {Required: true},
	{Code: AVPUserData, VendorID: tgpp.VendorID}:                 {},
	{Code: AVPChargingInformation, VendorID: tgpp.VendorID}:      {},
	{Code: AVPSIPAuthDataItem, VendorID: tgpp.VendorID}:          {},
	{Code: AVPAllowedWAFWWSFIdentities, VendorID: tgpp.VendorID}: {},
})

func NewPushProfileRequest(env tgpp.Envelope, r PushProfileRequest) (*diameter.Message, error) {
	switch {
	case env.DestinationHost == "":
		return nil, invalid("profile push without a destination host")
	case r.PrivateIdentity == "":
		return nil, invalid("profile push without a private identity")
	case len(r.UserData) == 0 && r.Charging == nil && r.AllowedWebRTC == nil:
		return nil, invalid("profile push without user data, charging information or allowed WAF/WWSF identities")
	}

	avps := []diameter.AVP{userName(r.PrivateIdentity)}

	if len(r.UserData) > 0 {
		avps = append(avps, diameter.OctetString(AVPUserData, diameter.AVPFlagMandatory, tgpp.VendorID, r.UserData))
	}

	if r.Charging != nil {
		charging, err := chargingAVP(*r.Charging)
		if err != nil {
			return nil, err
		}

		avps = append(avps, charging)
	}

	if r.AllowedWebRTC != nil {
		allowed, err := allowedWebRTCAVP(*r.AllowedWebRTC)
		if err != nil {
			return nil, err
		}

		avps = append(avps, allowed)
	}

	return newRequest(env, CommandPushProfile, r.Features, r.RequiredFeatures, avps...)
}

func CheckPushProfile(req *diameter.Message) error {
	if err := pprRules.Check(req); err != nil {
		return err
	}

	for _, code := range []uint32{AVPUserData, AVPChargingInformation, AVPSIPAuthDataItem, AVPAllowedWAFWWSFIdentities} {
		if _, ok := req.Find(code, tgpp.VendorID); ok {
			return nil
		}
	}

	return tgpp.MissingAVP(AVPUserData, tgpp.VendorID)
}

func ParsePushProfileRequest(req *diameter.Message) (PushProfileRequest, error) {
	if err := CheckPushProfile(req); err != nil {
		return PushProfileRequest{}, err
	}

	user, _ := req.Find(diameter.AVPUserName, 0)

	r := PushProfileRequest{PrivateIdentity: user.UTF8String(), Features: featureList(req), RequiredFeatures: requiredFeatures(req)}

	if data, ok := req.Find(AVPUserData, tgpp.VendorID); ok {
		r.UserData = bytes.Clone(data.Data)
	}

	if c, ok := req.Find(AVPChargingInformation, tgpp.VendorID); ok {
		charging, err := parseCharging(c)
		if err != nil {
			return PushProfileRequest{}, tgpp.InvalidAVP(c)
		}

		r.Charging = charging
	}

	if a, ok := req.Find(AVPAllowedWAFWWSFIdentities, tgpp.VendorID); ok {
		allowed, err := parseAllowedWebRTC(a)
		if err != nil {
			return PushProfileRequest{}, tgpp.InvalidAVP(a)
		}

		r.AllowedWebRTC = allowed
	}

	return r, nil
}

func NewPushProfileAnswer(req *diameter.Message, id diameter.Identity, a PushProfile) (*diameter.Message, error) {
	result, err := successResult(a.Result)
	if err != nil {
		return nil, err
	}

	return NewAnswer(req, id, result, a.Features), nil
}

func ParsePushProfileAnswer(ans *diameter.Message) (PushProfile, error) {
	result, err := parseResult(ans)
	if err != nil {
		return PushProfile{}, err
	}

	return PushProfile{Result: result, Features: featureList(ans)}, nil
}

func allowedWebRTCAVP(a AllowedWebRTCFunctions) (diameter.AVP, error) {
	var inner []diameter.AVP

	for _, f := range []struct {
		code  uint32
		names []string
	}{
		{AVPWebRTCAuthenticationFunctionName, a.AuthenticationFunctions},
		{AVPWebRTCWebServerFunctionName, a.WebServerFunctions},
	} {
		for _, name := range f.names {
			if name == "" {
				return diameter.AVP{}, invalid("empty WAF or WWSF name")
			}

			inner = append(inner, diameter.UTF8String(f.code, 0, tgpp.VendorID, name))
		}
	}

	return diameter.Grouped(AVPAllowedWAFWWSFIdentities, 0, tgpp.VendorID, inner...), nil
}

func parseAllowedWebRTC(a diameter.AVP) (*AllowedWebRTCFunctions, error) {
	inner, err := a.Grouped()
	if err != nil {
		return nil, malformed("Allowed-WAF-WWSF-Identities: %w", err)
	}

	var allowed AllowedWebRTCFunctions

	for _, f := range []struct {
		code uint32
		dst  *[]string
	}{
		{AVPWebRTCAuthenticationFunctionName, &allowed.AuthenticationFunctions},
		{AVPWebRTCWebServerFunctionName, &allowed.WebServerFunctions},
	} {
		for _, name := range diameter.FindAll(inner, f.code, tgpp.VendorID) {
			if name.UTF8String() == "" {
				return nil, malformed("Allowed-WAF-WWSF-Identities with an empty name")
			}

			*f.dst = append(*f.dst, name.UTF8String())
		}
	}

	return &allowed, nil
}
