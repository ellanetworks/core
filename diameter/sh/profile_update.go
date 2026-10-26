// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sh

import (
	"bytes"
	"errors"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type ProfileUpdateRequest struct {
	User                     UserIdentity
	WildcardedPublicIdentity string
	WildcardedIMPU           string
	PrivateIdentity          string
	DataReferences           []DataReference
	UserData                 []byte
	Features                 Features
	FeaturesRequired         bool
}

type ProfileUpdate struct {
	Result                   tgpp.Result
	WildcardedPublicIdentity string
	WildcardedIMPU           string
	Features                 Features
}

type RepositoryDataID struct {
	ServiceIndication []byte
	SequenceNumber    uint32
}

type ProfileUpdateError struct {
	ResultError

	WildcardedPublicIdentity string
	WildcardedIMPU           string
	RepositoryDataID         *RepositoryDataID
	DataReference            *DataReference
}

func (e *ProfileUpdateError) Unwrap() error {
	return &e.ResultError
}

var purRules = commonRequestRules.With(diameter.Rules{
	{Code: AVPUserIdentity, VendorID: tgpp.VendorID}:             {Required: true},
	{Code: AVPWildcardedPublicIdentity, VendorID: tgpp.VendorID}: {},
	{Code: AVPWildcardedIMPU, VendorID: tgpp.VendorID}:           {},
	{Code: diameter.AVPUserName}:                                 {},
	{Code: AVPDataReference, VendorID: tgpp.VendorID}:            {Required: true, Multiple: true, MinLength: 4},
	{Code: AVPUserData, VendorID: tgpp.VendorID}:                 {Required: true},
})

func NewProfileUpdateRequest(env tgpp.Envelope, r ProfileUpdateRequest) (*diameter.Message, error) {
	switch {
	case len(r.DataReferences) == 0:
		return nil, invalidf("profile update without a data reference")
	case len(r.DataReferences) > 1 && !r.Features.updateEffEnhance():
		return nil, invalidf("profile update of %d data references without Update-Eff and Update-Eff-Enhance", len(r.DataReferences))
	case len(r.DataReferences) > 1 && !r.FeaturesRequired:
		return nil, invalidf("profile update of %d data references without requiring Update-Eff-Enhance", len(r.DataReferences))
	case len(r.UserData) == 0:
		return nil, invalidf("profile update without user data")
	}

	user, err := userIdentityAVP(r.User)
	if err != nil {
		return nil, err
	}

	wildcarded, err := wildcardedAVPs(r.WildcardedPublicIdentity, r.WildcardedIMPU)
	if err != nil {
		return nil, err
	}

	avps := append([]diameter.AVP{user}, wildcarded...)

	if r.PrivateIdentity != "" {
		avps = append(avps, diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, r.PrivateIdentity))
	}

	for _, ref := range r.DataReferences {
		a, err := dataReferenceAVP(ref)
		if err != nil {
			return nil, err
		}

		avps = append(avps, a)
	}

	avps = append(avps, diameter.OctetString(AVPUserData, diameter.AVPFlagMandatory, tgpp.VendorID, r.UserData))

	return newRequest(env, CommandProfileUpdate, r.Features, r.FeaturesRequired, avps...)
}

func CheckProfileUpdate(req *diameter.Message) error {
	return purRules.Check(req)
}

func ParseProfileUpdateRequest(req *diameter.Message) (ProfileUpdateRequest, error) {
	if err := CheckProfileUpdate(req); err != nil {
		return ProfileUpdateRequest{}, err
	}

	userAVP, _ := req.Find(AVPUserIdentity, tgpp.VendorID)

	user, err := parseUserIdentity(userAVP)
	if err != nil {
		return ProfileUpdateRequest{}, err
	}

	wildcardedPSI, wildcardedIMPU, err := requestWildcarded(req)
	if err != nil {
		return ProfileUpdateRequest{}, err
	}

	r := ProfileUpdateRequest{
		User:                     user,
		WildcardedPublicIdentity: wildcardedPSI,
		WildcardedIMPU:           wildcardedIMPU,
		Features:                 featureList(req),
		FeaturesRequired:         featuresRequired(req),
	}

	if name, ok := req.Find(diameter.AVPUserName, 0); ok {
		r.PrivateIdentity = name.UTF8String()
	}

	for i, a := range diameter.FindAll(req.AVPs, AVPDataReference, tgpp.VendorID) {
		if i > 0 && !r.Features.updateEffEnhance() {
			return ProfileUpdateRequest{}, diameter.NewAVPError(diameter.ResultAVPOccursTooManyTimes, a)
		}

		ref, err := parseDataReference(a)
		if err != nil {
			return ProfileUpdateRequest{}, err
		}

		r.DataReferences = append(r.DataReferences, ref)
	}

	data, _ := req.Find(AVPUserData, tgpp.VendorID)
	r.UserData = bytes.Clone(data.Data)

	return r, nil
}

func NewProfileUpdateAnswer(req *diameter.Message, id diameter.Identity, a ProfileUpdate) (*diameter.Message, error) {
	result, err := successResult(a.Result)
	if err != nil {
		return nil, err
	}

	wildcarded, err := wildcardedAVPs(a.WildcardedPublicIdentity, a.WildcardedIMPU)
	if err != nil {
		return nil, err
	}

	ans := NewAnswer(req, id, result, a.Features)
	ans.AVPs = append(ans.AVPs, wildcarded...)

	return ans, nil
}

func NewProfileUpdateErrorAnswer(req *diameter.Message, id diameter.Identity, e ProfileUpdateError) (*diameter.Message, error) {
	if err := errorResult(e.Result); err != nil {
		return nil, err
	}

	wildcarded, err := wildcardedAVPs(e.WildcardedPublicIdentity, e.WildcardedIMPU)
	if err != nil {
		return nil, err
	}

	ans := NewAnswer(req, id, e.Result, e.Features)
	ans.AVPs = append(ans.AVPs, wildcarded...)

	requested := featureList(req)

	if e.RepositoryDataID != nil {
		if !requested.updateEff() || !e.Features.updateEff() {
			return nil, invalidf("Repository-Data-ID without Update-Eff or Update-Eff-Enhance on both sides")
		}

		repository, err := repositoryDataIDAVP(*e.RepositoryDataID)
		if err != nil {
			return nil, err
		}

		ans.AVPs = append(ans.AVPs, repository)
	}

	if e.DataReference != nil {
		if !requested.updateEffEnhance() || !e.Features.updateEffEnhance() {
			return nil, invalidf("Data-Reference without Update-Eff-Enhance on both sides")
		}

		ref, err := dataReferenceAVP(*e.DataReference)
		if err != nil {
			return nil, err
		}

		ans.AVPs = append(ans.AVPs, ref)
	}

	return ans, nil
}

func ParseProfileUpdateAnswer(ans *diameter.Message) (ProfileUpdate, error) {
	result, err := parseResult(ans)

	var re *ResultError
	if errors.As(err, &re) {
		e := &ProfileUpdateError{ResultError: *re}
		e.WildcardedPublicIdentity, e.WildcardedIMPU, _ = answerWildcarded(ans)

		if a, ok := ans.Find(AVPRepositoryDataID, tgpp.VendorID); ok {
			if id, err := parseRepositoryDataID(a); err == nil {
				e.RepositoryDataID = &id
			}
		}

		if a, ok := ans.Find(AVPDataReference, tgpp.VendorID); ok {
			if ref, err := parseDataReference(a); err == nil {
				e.DataReference = &ref
			}
		}

		return ProfileUpdate{}, e
	}

	if err != nil {
		return ProfileUpdate{}, err
	}

	wildcardedPSI, wildcardedIMPU, err := answerWildcarded(ans)
	if err != nil {
		return ProfileUpdate{}, err
	}

	return ProfileUpdate{
		Result:                   result,
		WildcardedPublicIdentity: wildcardedPSI,
		WildcardedIMPU:           wildcardedIMPU,
		Features:                 featureList(ans),
	}, nil
}

func repositoryDataIDAVP(id RepositoryDataID) (diameter.AVP, error) {
	switch {
	case len(id.ServiceIndication) == 0:
		return diameter.AVP{}, invalidf("repository data ID without a service indication")
	case id.SequenceNumber > maxSequenceNumber:
		return diameter.AVP{}, invalidf("Sequence-Number %d", id.SequenceNumber)
	}

	return diameter.Grouped(AVPRepositoryDataID, 0, tgpp.VendorID,
		diameter.OctetString(AVPServiceIndication, diameter.AVPFlagMandatory, tgpp.VendorID, id.ServiceIndication),
		diameter.Unsigned32(AVPSequenceNumber, 0, tgpp.VendorID, id.SequenceNumber),
	), nil
}

func parseRepositoryDataID(a diameter.AVP) (RepositoryDataID, error) {
	inner, err := a.Grouped()
	if err != nil {
		return RepositoryDataID{}, malformedf("Repository-Data-ID: %w", err)
	}

	service, ok := diameter.Find(inner, AVPServiceIndication, tgpp.VendorID)
	if !ok || len(service.Data) == 0 {
		return RepositoryDataID{}, malformedf("Repository-Data-ID without a Service-Indication")
	}

	sequence, ok := diameter.Find(inner, AVPSequenceNumber, tgpp.VendorID)
	if !ok {
		return RepositoryDataID{}, malformedf("Repository-Data-ID without a Sequence-Number")
	}

	number, err := sequence.Unsigned32()
	if err != nil || number > maxSequenceNumber {
		return RepositoryDataID{}, malformedf("Repository-Data-ID Sequence-Number")
	}

	return RepositoryDataID{ServiceIndication: bytes.Clone(service.Data), SequenceNumber: number}, nil
}
