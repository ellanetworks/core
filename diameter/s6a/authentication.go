// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"bytes"
	"fmt"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const (
	randLen    = 16
	autnLen    = 16
	autsLen    = 14
	kasmeLen   = 32
	keyLen     = 16
	sresLen    = 4
	kcLen      = 8
	minXRESLen = 4
	maxXRESLen = 16
)

type AuthenticationInformationRequest struct {
	IMSI        string
	Features    Features
	EUTRAN      *RequestedAuthenticationInfo
	UTRANGERAN  *RequestedAuthenticationInfo
	VisitedPLMN tgpp.PLMN
	Flags       AIRFlags
}

type Resync struct {
	RAND []byte
	AUTS []byte
}

type RequestedAuthenticationInfo struct {
	NumberOfVectors            uint32
	ImmediateResponsePreferred bool
	Resync                     *Resync
}

type AuthenticationInformation struct {
	HSS         diameter.Identity
	Features    Features
	EUTRAN      []EUTRANVector
	UTRAN       []UTRANVector
	GERAN       []GERANVector
	UEUsageType *UEUsageType
}

type EUTRANVector struct {
	ItemNumber uint32
	RAND       []byte
	XRES       []byte
	AUTN       []byte
	KASME      []byte
}

type UTRANVector struct {
	ItemNumber uint32
	RAND       []byte
	XRES       []byte
	AUTN       []byte
	CK         []byte
	IK         []byte
}

type GERANVector struct {
	ItemNumber uint32
	RAND       []byte
	SRES       []byte
	Kc         []byte
}

var (
	airRules = requestRules(false, userNameRule.With(diameter.Rules{
		vendorKey(AVPRequestedEUTRANAuthenticationInfo):     {},
		vendorKey(AVPRequestedUTRANGERANAuthenticationInfo): {},
		vendorKey(AVPVisitedPLMNID):                         {Required: true, MinLength: 3},
		vendorKey(AVPAIRFlags):                              {},
	}))
	requestedAuthenticationInfoRules = diameter.Rules{
		vendorKey(AVPNumberOfRequestedVectors):   {},
		vendorKey(AVPImmediateResponsePreferred): {},
		vendorKey(AVPReSynchronizationInfo):      {},
	}
)

func NewAuthenticationInformationRequest(env tgpp.Envelope, r AuthenticationInformationRequest) (*diameter.Message, error) {
	userName, err := userNameAVP(r.IMSI)
	if err != nil {
		return nil, err
	}

	switch {
	case r.EUTRAN == nil && r.UTRANGERAN == nil:
		return nil, invalidf("authentication request without requested authentication info")
	case r.EUTRAN != nil && r.UTRANGERAN != nil && r.EUTRAN.Resync != nil && r.UTRANGERAN.Resync != nil:
		return nil, invalidf("resynchronisation information in both requested authentication infos")
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

	if avps, err = appendFlags(avps, AVPAIRFlags, "AIR-Flags", r.Flags); err != nil {
		return nil, err
	}

	return newRequest(env, CommandAuthenticationInformation, false, avps...)
}

func CheckAuthenticationInformation(req *diameter.Message) error {
	if err := airRules.Check(req); err != nil {
		return err
	}

	_, eutran := req.Find(AVPRequestedEUTRANAuthenticationInfo, tgpp.VendorID)
	if _, utran := req.Find(AVPRequestedUTRANGERANAuthenticationInfo, tgpp.VendorID); !eutran && !utran {
		return tgpp.MissingAVP(AVPRequestedEUTRANAuthenticationInfo, tgpp.VendorID, 0)
	}

	return nil
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
			if *info.dst, err = requestDecoder.requestedAuthenticationInfo(a); err != nil {
				return AuthenticationInformationRequest{}, err
			}
		}
	}

	if r.EUTRAN != nil && r.UTRANGERAN != nil && r.EUTRAN.Resync != nil && r.UTRANGERAN.Resync != nil {
		return AuthenticationInformationRequest{}, &diameter.AVPError{ResultCode: diameter.ResultUnableToComply}
	}

	if r.VisitedPLMN, err = requestVisitedPLMN(req); err != nil {
		return AuthenticationInformationRequest{}, err
	}

	if r.Flags, err = requestFlags(req.AVPs, AVPAIRFlags, airFlagsDefined); err != nil {
		return AuthenticationInformationRequest{}, err
	}

	return r, nil
}

func (r RequestedAuthenticationInfo) avp(code uint32) (diameter.AVP, error) {
	var inner []diameter.AVP

	if r.NumberOfVectors != 0 {
		inner = append(inner, unsignedAVP(AVPNumberOfRequestedVectors, r.NumberOfVectors))
	}

	if r.ImmediateResponsePreferred {
		inner = append(inner, unsignedAVP(AVPImmediateResponsePreferred, 1))
	}

	if r.Resync != nil {
		if len(r.Resync.RAND) != randLen || len(r.Resync.AUTS) != autsLen {
			return diameter.AVP{}, invalidf("resynchronisation with a %d-octet RAND and a %d-octet AUTS", len(r.Resync.RAND), len(r.Resync.AUTS))
		}

		inner = append(inner, octetsAVP(AVPReSynchronizationInfo, append(bytes.Clone(r.Resync.RAND), r.Resync.AUTS...)))
	}

	return groupedAVP(code, inner...), nil
}

func (d decoder) requestedAuthenticationInfo(a diameter.AVP) (*RequestedAuthenticationInfo, error) {
	inner, err := d.group(a, requestedAuthenticationInfoRules)
	if err != nil {
		return nil, err
	}

	var r RequestedAuthenticationInfo

	if r.NumberOfVectors, _, err = optionalUnsigned(inner, AVPNumberOfRequestedVectors); err != nil {
		return nil, diameter.WithinGrouped(a, err)
	}

	if _, ok := diameter.Find(inner, AVPImmediateResponsePreferred, tgpp.VendorID); ok {
		r.ImmediateResponsePreferred = true
	}

	if resync, ok := diameter.Find(inner, AVPReSynchronizationInfo, tgpp.VendorID); ok {
		if len(resync.Data) != randLen+autsLen {
			return nil, diameter.WithinGrouped(a, tgpp.InvalidAVP(resync))
		}

		r.Resync = &Resync{RAND: bytes.Clone(resync.Data[:randLen]), AUTS: bytes.Clone(resync.Data[randLen:])}
	}

	return &r, nil
}

func (a AuthenticationInformation) vectors() int {
	return len(a.EUTRAN) + len(a.UTRAN) + len(a.GERAN)
}

func NewAuthenticationInformationAnswer(req *diameter.Message, id diameter.Identity, a AuthenticationInformation) (*diameter.Message, error) {
	if a.vectors() == 0 && a.UEUsageType == nil {
		return nil, invalidf("authentication answer without vectors or UE-Usage-Type")
	}

	type vector struct {
		code, item uint32
		fields     []vectorField
	}

	var all []vector

	for i := range a.EUTRAN {
		all = append(all, vector{AVPEUTRANVector, a.EUTRAN[i].ItemNumber, a.EUTRAN[i].fields()})
	}

	for i := range a.UTRAN {
		all = append(all, vector{AVPUTRANVector, a.UTRAN[i].ItemNumber, a.UTRAN[i].fields()})
	}

	for i := range a.GERAN {
		all = append(all, vector{AVPGERANVector, a.GERAN[i].ItemNumber, a.GERAN[i].fields()})
	}

	vectors := make([]diameter.AVP, 0, len(all))

	for _, v := range all {
		if len(all) > 1 && v.item == 0 {
			return nil, invalidf("%d authentication vectors without an Item-Number in each", len(all))
		}

		avp, err := encodeVector(v.code, v.item, v.fields)
		if err != nil {
			return nil, err
		}

		vectors = append(vectors, avp)
	}

	ans := newSuccessAnswer(req, id, a.Features)

	if len(vectors) > 0 {
		ans.AVPs = append(ans.AVPs, groupedAVP(AVPAuthenticationInfo, vectors...))
	}

	if a.UEUsageType != nil {
		ans.AVPs = append(ans.AVPs, a.UEUsageType.avp())
	}

	return ans, nil
}

func ParseAuthenticationInformationAnswer(ans *diameter.Message) (AuthenticationInformation, error) {
	features, err := parseAnswer(ans)
	if err != nil {
		return AuthenticationInformation{}, err
	}

	a := AuthenticationInformation{HSS: origin(ans), Features: features}

	if info, ok := ans.Find(AVPAuthenticationInfo, tgpp.VendorID); ok {
		if err := a.parseVectors(info); err != nil {
			return AuthenticationInformation{}, malformed(err)
		}
	}

	if usage, ok := ans.Find(AVPUEUsageType, tgpp.VendorID); ok {
		u, err := parseUEUsageType(usage)
		if err != nil {
			return AuthenticationInformation{}, malformed(err)
		}

		a.UEUsageType = &u
	}

	if a.vectors() == 0 && a.UEUsageType == nil {
		return AuthenticationInformation{}, malformedf("success without vectors or UE-Usage-Type")
	}

	return a, nil
}

func (a *AuthenticationInformation) parseVectors(info diameter.AVP) error {
	inner, err := info.Grouped()
	if err != nil {
		return err
	}

	for _, avp := range inner {
		if avp.VendorID != tgpp.VendorID {
			continue
		}

		switch avp.Code {
		case AVPEUTRANVector:
			var v EUTRANVector
			if err := decodeVector(avp, &v.ItemNumber, v.fields()); err != nil {
				return err
			}

			a.EUTRAN = append(a.EUTRAN, v)
		case AVPUTRANVector:
			var v UTRANVector
			if err := decodeVector(avp, &v.ItemNumber, v.fields()); err != nil {
				return err
			}

			a.UTRAN = append(a.UTRAN, v)
		case AVPGERANVector:
			var v GERANVector
			if err := decodeVector(avp, &v.ItemNumber, v.fields()); err != nil {
				return err
			}

			a.GERAN = append(a.GERAN, v)
		}
	}

	return nil
}

type vectorField struct {
	code           uint32
	name           string
	value          *[]byte
	minLen, maxLen int
}

func encodeVector(code, itemNumber uint32, fields []vectorField) (diameter.AVP, error) {
	var inner []diameter.AVP

	if itemNumber != 0 {
		inner = append(inner, unsignedAVP(AVPItemNumber, itemNumber))
	}

	for _, f := range fields {
		if len(*f.value) < f.minLen || len(*f.value) > f.maxLen {
			return diameter.AVP{}, invalidf("authentication vector with a %d-octet %s", len(*f.value), f.name)
		}

		inner = append(inner, octetsAVP(f.code, *f.value))
	}

	return groupedAVP(code, inner...), nil
}

func decodeVector(a diameter.AVP, itemNumber *uint32, fields []vectorField) error {
	inner, err := a.Grouped()
	if err != nil {
		return fmt.Errorf("vector %d: %w", a.Code, err)
	}

	if *itemNumber, _, err = optionalUnsigned(inner, AVPItemNumber); err != nil {
		return err
	}

	for _, f := range fields {
		v, ok := diameter.Find(inner, f.code, tgpp.VendorID)
		if !ok {
			return fmt.Errorf("vector without %s", f.name)
		}

		if len(v.Data) < f.minLen || len(v.Data) > f.maxLen {
			return fmt.Errorf("vector with a %d-octet %s", len(v.Data), f.name)
		}

		*f.value = bytes.Clone(v.Data)
	}

	return nil
}

func (v *EUTRANVector) fields() []vectorField {
	return []vectorField{
		{AVPRAND, "RAND", &v.RAND, randLen, randLen},
		{AVPXRES, "XRES", &v.XRES, minXRESLen, maxXRESLen},
		{AVPAUTN, "AUTN", &v.AUTN, autnLen, autnLen},
		{AVPKASME, "KASME", &v.KASME, kasmeLen, kasmeLen},
	}
}

func (v *UTRANVector) fields() []vectorField {
	return []vectorField{
		{AVPRAND, "RAND", &v.RAND, randLen, randLen},
		{AVPXRES, "XRES", &v.XRES, minXRESLen, maxXRESLen},
		{AVPAUTN, "AUTN", &v.AUTN, autnLen, autnLen},
		{tgpp.AVPConfidentialityKey, "CK", &v.CK, keyLen, keyLen},
		{tgpp.AVPIntegrityKey, "IK", &v.IK, keyLen, keyLen},
	}
}

func (v *GERANVector) fields() []vectorField {
	return []vectorField{
		{AVPRAND, "RAND", &v.RAND, randLen, randLen},
		{AVPSRES, "SRES", &v.SRES, sresLen, sresLen},
		{AVPKc, "Kc", &v.Kc, kcLen, kcLen},
	}
}
