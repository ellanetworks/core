// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cx

import (
	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type DeregistrationReason struct {
	Code uint32
	Info string
}

type RegistrationTerminationRequest struct {
	PrivateIdentity          string
	AssociatedIdentities     []string
	PublicIdentities         []string
	Reason                   DeregistrationReason
	ReferenceLocationChanged bool
	Features                 uint32
}

type EmergencyRegistration struct {
	PrivateIdentity string
	PublicIdentity  string
}

type RegistrationTermination struct {
	Result                 tgpp.Result
	AssociatedIdentities   []string
	EmergencyRegistrations []EmergencyRegistration
	Features               uint32
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
		return nil, invalid("registration termination without a destination host")
	case r.PrivateIdentity == "":
		return nil, invalid("registration termination without a private identity")
	case r.Reason.Code > ReasonRemoveSCSCF:
		return nil, invalid("Reason-Code %d", r.Reason.Code)
	case r.Reason.Code == ReasonNewServerAssigned && len(r.PublicIdentities) == 0:
		return nil, invalid("new server assigned without public identities")
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

	reason := []diameter.AVP{vendorUnsigned(AVPReasonCode, r.Reason.Code)}
	if r.Reason.Info != "" {
		reason = append(reason, vendorString(AVPReasonInfo, r.Reason.Info))
	}

	avps = append(avps, diameter.Grouped(AVPDeregistrationReason, diameter.AVPFlagMandatory, tgpp.VendorID, reason...))

	if r.ReferenceLocationChanged {
		avps = append(avps, diameter.Unsigned32(AVPRTRFlags, 0, tgpp.VendorID, RTRFlagReferenceLocationChanged))
	}

	return newRequest(env, CommandRegistrationTermination, r.Features, avps...), nil
}

func CheckRegistrationTermination(req *diameter.Message) error {
	return rtrRules.Check(req)
}

func ParseRegistrationTerminationRequest(req *diameter.Message) (RegistrationTerminationRequest, error) {
	if err := CheckRegistrationTermination(req); err != nil {
		return RegistrationTerminationRequest{}, err
	}

	user, _ := req.Find(diameter.AVPUserName, 0)

	r := RegistrationTerminationRequest{PrivateIdentity: user.UTF8String(), Features: features(req)}

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
		return RegistrationTerminationRequest{}, tgpp.MissingAVP(AVPReasonCode, tgpp.VendorID)
	}

	if r.Reason.Code, err = code.Unsigned32(); err != nil || r.Reason.Code > ReasonRemoveSCSCF {
		return RegistrationTerminationRequest{}, tgpp.InvalidAVP(reasonAVP)
	}

	if info, ok := diameter.Find(reason, AVPReasonInfo, tgpp.VendorID); ok {
		r.Reason.Info = info.UTF8String()
	}

	if r.Reason.Code == ReasonNewServerAssigned && len(r.PublicIdentities) == 0 {
		return RegistrationTerminationRequest{}, tgpp.MissingAVP(AVPPublicIdentity, tgpp.VendorID)
	}

	rtrFlags, err := flags(req, AVPRTRFlags)
	if err != nil {
		return RegistrationTerminationRequest{}, err
	}

	r.ReferenceLocationChanged = rtrFlags&RTRFlagReferenceLocationChanged != 0

	return r, nil
}

func NewRegistrationTerminationAnswer(req *diameter.Message, id diameter.Identity, a RegistrationTermination) (*diameter.Message, error) {
	if err := successResult(a.Result); err != nil {
		return nil, err
	}

	if err := validIdentityList(a.AssociatedIdentities); err != nil {
		return nil, err
	}

	ans := NewAnswer(req, id, a.Result, a.Features)

	if len(a.AssociatedIdentities) > 0 {
		ans.AVPs = append(ans.AVPs, identityList(AVPAssociatedIdentities, a.AssociatedIdentities))
	}

	for _, e := range a.EmergencyRegistrations {
		if e.PrivateIdentity == "" {
			return nil, invalid("emergency registration without a private identity")
		}

		public, err := publicIdentityAVP(e.PublicIdentity)
		if err != nil {
			return nil, err
		}

		ans.AVPs = append(ans.AVPs, diameter.Grouped(AVPIdentityWithEmergencyRegistration, 0, tgpp.VendorID, userName(e.PrivateIdentity), public))
	}

	return ans, nil
}

func ParseRegistrationTerminationAnswer(ans *diameter.Message) (RegistrationTermination, error) {
	result, err := parseResult(ans)
	if err != nil {
		return RegistrationTermination{}, err
	}

	a := RegistrationTermination{Result: result, Features: features(ans)}

	if ids, ok := ans.Find(AVPAssociatedIdentities, tgpp.VendorID); ok {
		if a.AssociatedIdentities, err = parseIdentityList(ids); err != nil {
			return RegistrationTermination{}, malformed("Associated-Identities: %v", err)
		}
	}

	for _, e := range diameter.FindAll(ans.AVPs, AVPIdentityWithEmergencyRegistration, tgpp.VendorID) {
		inner, err := e.Grouped()
		if err != nil {
			return RegistrationTermination{}, malformed("Identity-with-Emergency-Registration: %v", err)
		}

		user, hasUser := diameter.Find(inner, diameter.AVPUserName, 0)
		public, hasPublic := diameter.Find(inner, AVPPublicIdentity, tgpp.VendorID)

		if !hasUser || !hasPublic {
			return RegistrationTermination{}, malformed("Identity-with-Emergency-Registration without both identities")
		}

		a.EmergencyRegistrations = append(a.EmergencyRegistrations, EmergencyRegistration{
			PrivateIdentity: user.UTF8String(),
			PublicIdentity:  public.UTF8String(),
		})
	}

	return a, nil
}
