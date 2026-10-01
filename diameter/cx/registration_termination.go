// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cx

import (
	"cmp"
	"errors"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type DeregistrationReason struct {
	Code ReasonCode
	Info string
}

type RegistrationTerminationRequest struct {
	PrivateIdentity          string
	AssociatedIdentities     []string
	PublicIdentities         []string
	Reason                   DeregistrationReason
	ReferenceLocationChanged bool
	Features                 Features
	FeaturesRequired         bool
}

type EmergencyIdentity struct {
	PrivateIdentity string
	PublicIdentity  string
}

type RegistrationTermination struct {
	Result               tgpp.Result
	AssociatedIdentities []string
	EmergencyIdentities  []EmergencyIdentity
	Features             Features
}

var rtrRules = commonRequestRules.With(diameter.Rules{
	{Code: diameter.AVPDestinationHost}:                      {Required: true},
	{Code: diameter.AVPUserName}:                             {Required: true},
	{Code: AVPAssociatedIdentities, VendorID: tgpp.VendorID}: {},
	{Code: AVPPublicIdentity, VendorID: tgpp.VendorID}:       {Multiple: true},
	{Code: AVPDeregistrationReason, VendorID: tgpp.VendorID}: {Required: true},
	{Code: AVPRTRFlags, VendorID: tgpp.VendorID}:             {},
})

func NewRegistrationTerminationRequest(env tgpp.Envelope, r RegistrationTerminationRequest) (*diameter.Message, error) {
	switch {
	case env.DestinationHost == "":
		return nil, invalidf("registration termination without a destination host")
	case r.PrivateIdentity == "":
		return nil, invalidf("registration termination without a private identity")
	case r.Reason.Code > ReasonRemoveSCSCF:
		return nil, invalidf("Reason-Code %d", r.Reason.Code)
	case r.Reason.Code == ReasonNewServerAssigned && len(r.PublicIdentities) == 0:
		return nil, invalidf("new server assigned without public identities")
	}

	if err := validIdentityList(r.AssociatedIdentities); err != nil {
		return nil, err
	}

	avps := []diameter.AVP{userName(r.PrivateIdentity)}

	if len(r.AssociatedIdentities) > 0 {
		avps = append(avps, identityList(AVPAssociatedIdentities, r.AssociatedIdentities))
	}

	for _, p := range r.PublicIdentities {
		a, err := publicIdentityAVP(p)
		if err != nil {
			return nil, err
		}

		avps = append(avps, a)
	}

	reason := []diameter.AVP{vendorUnsigned(AVPReasonCode, uint32(r.Reason.Code))}
	if r.Reason.Info != "" {
		reason = append(reason, vendorString(AVPReasonInfo, r.Reason.Info))
	}

	avps = append(avps, diameter.Grouped(AVPDeregistrationReason, diameter.AVPFlagMandatory, tgpp.VendorID, reason...))

	if r.ReferenceLocationChanged {
		avps = append(avps, diameter.Unsigned32(AVPRTRFlags, 0, tgpp.VendorID, rtrFlagReferenceLocationChanged))
	}

	return newRequest(env, CommandRegistrationTermination, r.Features, r.FeaturesRequired, avps...)
}

func CheckRegistrationTermination(req *diameter.Message) error {
	return rtrRules.Check(req)
}

func ParseRegistrationTerminationRequest(req *diameter.Message) (RegistrationTerminationRequest, error) {
	if err := CheckRegistrationTermination(req); err != nil {
		return RegistrationTerminationRequest{}, err
	}

	user, _ := req.Find(diameter.AVPUserName, 0)

	r := RegistrationTerminationRequest{PrivateIdentity: user.UTF8String(), Features: featureList(req), FeaturesRequired: featuresRequired(req)}

	if a, ok := req.Find(AVPAssociatedIdentities, tgpp.VendorID); ok {
		ids, err := parseIdentityList(a)
		if err != nil {
			return RegistrationTerminationRequest{}, tgpp.InvalidAVP(a)
		}

		r.AssociatedIdentities = ids
	}

	for _, p := range diameter.FindAll(req.AVPs, AVPPublicIdentity, tgpp.VendorID) {
		if !ValidPublicIdentity(p.UTF8String()) {
			return RegistrationTerminationRequest{}, tgpp.InvalidAVP(p)
		}

		r.PublicIdentities = append(r.PublicIdentities, p.UTF8String())
	}

	reasonAVP, _ := req.Find(AVPDeregistrationReason, tgpp.VendorID)

	reason, err := reasonAVP.Grouped()
	if err != nil {
		return RegistrationTerminationRequest{}, tgpp.InvalidAVP(reasonAVP)
	}

	code, ok := diameter.Find(reason, AVPReasonCode, tgpp.VendorID)
	if !ok {
		return RegistrationTerminationRequest{}, tgpp.MissingAVP(AVPReasonCode, tgpp.VendorID, 4)
	}

	reasonCode, err := tgpp.Unsigned32(code)
	if err != nil {
		return RegistrationTerminationRequest{}, err
	}

	if ReasonCode(reasonCode) > ReasonRemoveSCSCF {
		return RegistrationTerminationRequest{}, tgpp.InvalidAVP(code)
	}

	r.Reason.Code = ReasonCode(reasonCode)

	if info, ok := diameter.Find(reason, AVPReasonInfo, tgpp.VendorID); ok {
		r.Reason.Info = info.UTF8String()
	}

	if r.Reason.Code == ReasonNewServerAssigned && len(r.PublicIdentities) == 0 {
		return RegistrationTerminationRequest{}, tgpp.MissingAVP(AVPPublicIdentity, tgpp.VendorID, 0)
	}

	rtrFlags, err := optionalFlags(req, AVPRTRFlags)
	if err != nil {
		return RegistrationTerminationRequest{}, err
	}

	r.ReferenceLocationChanged = rtrFlags&rtrFlagReferenceLocationChanged != 0

	return r, nil
}

func NewRegistrationTerminationAnswer(req *diameter.Message, id diameter.Identity, a RegistrationTermination) (*diameter.Message, error) {
	result, err := successResult(a.Result)
	if err != nil {
		return nil, err
	}

	return registrationTerminationAnswer(req, id, result, a.Features, a.AssociatedIdentities, a.EmergencyIdentities)
}

func NewRegistrationTerminationErrorAnswer(req *diameter.Message, id diameter.Identity, e RegistrationTerminationError) (*diameter.Message, error) {
	if err := errorResult(e.Result); err != nil {
		return nil, err
	}

	return registrationTerminationAnswer(req, id, e.Result, e.Features, e.AssociatedIdentities, e.EmergencyIdentities)
}

func registrationTerminationAnswer(req *diameter.Message, id diameter.Identity, r tgpp.Result, features Features, associated []string,
	emergency []EmergencyIdentity,
) (*diameter.Message, error) {
	if err := validIdentityList(associated); err != nil {
		return nil, err
	}

	ans := NewAnswer(req, id, r, features)

	if len(associated) > 0 {
		ans.AVPs = append(ans.AVPs, identityList(AVPAssociatedIdentities, associated))
	}

	for _, e := range emergency {
		pair, err := identityPairAVP(AVPIdentityWithEmergencyRegistration, e.PrivateIdentity, e.PublicIdentity)
		if err != nil {
			return nil, err
		}

		ans.AVPs = append(ans.AVPs, pair)
	}

	return ans, nil
}

func ParseRegistrationTerminationAnswer(ans *diameter.Message) (RegistrationTermination, error) {
	result, err := parseResult(ans)
	associated, associatedErr := answerAssociatedIdentities(ans)
	emergency, emergencyErr := answerEmergencyIdentities(ans)

	var re *ResultError
	if errors.As(err, &re) {
		return RegistrationTermination{}, &RegistrationTerminationError{ResultError: *re, AssociatedIdentities: associated, EmergencyIdentities: emergency}
	}

	for _, e := range []error{err, associatedErr, emergencyErr} {
		if e != nil {
			return RegistrationTermination{}, e
		}
	}

	return RegistrationTermination{
		Result:               result,
		AssociatedIdentities: associated,
		EmergencyIdentities:  emergency,
		Features:             featureList(ans),
	}, nil
}

func answerAssociatedIdentities(ans *diameter.Message) ([]string, error) {
	ids, ok := ans.Find(AVPAssociatedIdentities, tgpp.VendorID)
	if !ok {
		return nil, nil
	}

	names, err := parseIdentityList(ids)
	if err != nil {
		return nil, malformedf("Associated-Identities: %w", err)
	}

	return names, nil
}

func answerEmergencyIdentities(ans *diameter.Message) ([]EmergencyIdentity, error) {
	var (
		pairs    []EmergencyIdentity
		firstErr error
	)

	for _, e := range diameter.FindAll(ans.AVPs, AVPIdentityWithEmergencyRegistration, tgpp.VendorID) {
		inner, err := e.Grouped()
		if err != nil {
			firstErr = cmp.Or(firstErr, malformedf("Identity-with-Emergency-Registration: %w", err))
			continue
		}

		user, _ := diameter.Find(inner, diameter.AVPUserName, 0)
		public, _ := diameter.Find(inner, AVPPublicIdentity, tgpp.VendorID)

		if user.UTF8String() == "" || !ValidPublicIdentity(public.UTF8String()) {
			firstErr = cmp.Or(firstErr, malformedf("Identity-with-Emergency-Registration without a valid identity pair"))
			continue
		}

		pairs = append(pairs, EmergencyIdentity{PrivateIdentity: user.UTF8String(), PublicIdentity: public.UTF8String()})
	}

	return pairs, firstErr
}
