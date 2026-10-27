// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type AuthenticationInformationRequest struct {
	IMSI        string
	Features    Features
	EUTRAN      *RequestedAuthenticationInfo
	UTRANGERAN  *RequestedAuthenticationInfo
	VisitedPLMN tgpp.PLMN
	Flags       AIRFlags
}

type AuthenticationInformation struct {
	Features    Features
	EUTRAN      []EUTRANVector
	UTRAN       []UTRANVector
	GERAN       []GERANVector
	UEUsageType *uint32
}

var airRules = requestRules(false, userNameRule.With(diameter.Rules{
	vendorKey(AVPRequestedEUTRANAuthenticationInfo):     {},
	vendorKey(AVPRequestedUTRANGERANAuthenticationInfo): {},
	vendorKey(AVPVisitedPLMNID):                         {Required: true, MinLength: 3},
	vendorKey(AVPAIRFlags):                              {},
}))

func NewAuthenticationInformationRequest(env tgpp.Envelope, r AuthenticationInformationRequest) (*diameter.Message, error) {
	userName, err := userNameAVP(r.IMSI)
	if err != nil {
		return nil, err
	}

	avps := append([]diameter.AVP{userName}, r.Features.avps()...)

	for _, info := range []struct {
		code uint32
		req  *RequestedAuthenticationInfo
	}{
		{AVPRequestedEUTRANAuthenticationInfo, r.EUTRAN},
		{AVPRequestedUTRANGERANAuthenticationInfo, r.UTRANGERAN},
	} {
		if info.req == nil {
			continue
		}

		a, err := info.req.avp(info.code)
		if err != nil {
			return nil, err
		}

		avps = append(avps, a)
	}

	visited, err := visitedPLMNAVP(r.VisitedPLMN)
	if err != nil {
		return nil, err
	}

	avps = append(avps, visited)

	if avps, err = appendFlags(avps, AVPAIRFlags, "AIR-Flags", r.Flags, airFlagsDefined); err != nil {
		return nil, err
	}

	return newRequest(env, CommandAuthenticationInformation, false, avps...)
}

func CheckAuthenticationInformation(req *diameter.Message) error {
	return airRules.Check(req)
}

func ParseAuthenticationInformationRequest(req *diameter.Message) (AuthenticationInformationRequest, error) {
	if err := CheckAuthenticationInformation(req); err != nil {
		return AuthenticationInformationRequest{}, err
	}

	var (
		r   AuthenticationInformationRequest
		err error
	)

	if r.IMSI, err = requestIMSI(req); err != nil {
		return AuthenticationInformationRequest{}, err
	}

	r.Features = featuresOf(req)

	for _, info := range []struct {
		code uint32
		dst  **RequestedAuthenticationInfo
	}{
		{AVPRequestedEUTRANAuthenticationInfo, &r.EUTRAN},
		{AVPRequestedUTRANGERANAuthenticationInfo, &r.UTRANGERAN},
	} {
		if a, ok := req.Find(info.code, tgpp.VendorID); ok {
			if *info.dst, err = parseRequestedAuthenticationInfo(a); err != nil {
				return AuthenticationInformationRequest{}, err
			}
		}
	}

	if r.VisitedPLMN, err = requestVisitedPLMN(req); err != nil {
		return AuthenticationInformationRequest{}, err
	}

	if r.Flags, err = optionalFlags(req.AVPs, AVPAIRFlags, airFlagsDefined); err != nil {
		return AuthenticationInformationRequest{}, err
	}

	return r, nil
}

func NewAuthenticationInformationAnswer(req *diameter.Message, id diameter.Identity, a AuthenticationInformation) (*diameter.Message, error) {
	if len(a.EUTRAN)+len(a.UTRAN)+len(a.GERAN) == 0 {
		return nil, invalidf("authentication answer without vectors")
	}

	if a.UEUsageType != nil && *a.UEUsageType > maxUEUsage {
		return nil, invalidf("UE-Usage-Type %d", *a.UEUsageType)
	}

	var vectors []diameter.AVP

	for _, v := range a.EUTRAN {
		avp, err := v.avp()
		if err != nil {
			return nil, err
		}

		vectors = append(vectors, avp)
	}

	for _, v := range a.UTRAN {
		avp, err := v.avp()
		if err != nil {
			return nil, err
		}

		vectors = append(vectors, avp)
	}

	for _, v := range a.GERAN {
		avp, err := v.avp()
		if err != nil {
			return nil, err
		}

		vectors = append(vectors, avp)
	}

	ans := newSuccessAnswer(req, id, a.Features)
	ans.AVPs = append(ans.AVPs, groupedAVP(AVPAuthenticationInfo, vectors...))

	if a.UEUsageType != nil {
		ans.AVPs = append(ans.AVPs, unsignedAVP(AVPUEUsageType, *a.UEUsageType))
	}

	return ans, nil
}

func ParseAuthenticationInformationAnswer(ans *diameter.Message) (AuthenticationInformation, error) {
	features, err := parseAnswer(ans)
	if err != nil {
		return AuthenticationInformation{}, err
	}

	a := AuthenticationInformation{Features: features}

	info, ok := ans.Find(AVPAuthenticationInfo, tgpp.VendorID)
	if !ok {
		return AuthenticationInformation{}, malformedf("no Authentication-Info")
	}

	inner, err := info.Grouped()
	if err != nil {
		return AuthenticationInformation{}, malformed(err)
	}

	for _, avp := range inner {
		if avp.VendorID != tgpp.VendorID {
			continue
		}

		switch avp.Code {
		case AVPEUTRANVector:
			v, err := parseEUTRANVector(avp)
			if err != nil {
				return AuthenticationInformation{}, malformed(err)
			}

			a.EUTRAN = append(a.EUTRAN, v)
		case AVPUTRANVector:
			v, err := parseUTRANVector(avp)
			if err != nil {
				return AuthenticationInformation{}, malformed(err)
			}

			a.UTRAN = append(a.UTRAN, v)
		case AVPGERANVector:
			v, err := parseGERANVector(avp)
			if err != nil {
				return AuthenticationInformation{}, malformed(err)
			}

			a.GERAN = append(a.GERAN, v)
		}
	}

	if len(a.EUTRAN)+len(a.UTRAN)+len(a.GERAN) == 0 {
		return AuthenticationInformation{}, malformedf("Authentication-Info without vectors")
	}

	if usage, ok, err := optionalUnsigned(ans.AVPs, AVPUEUsageType); err != nil {
		return AuthenticationInformation{}, malformed(err)
	} else if ok {
		a.UEUsageType = &usage
	}

	return a, nil
}
