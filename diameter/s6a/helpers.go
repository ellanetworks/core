// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type known interface {
	~uint32
	Known() bool
}

type decoder struct {
	strict bool
}

var (
	requestDecoder = decoder{strict: true}
	answerDecoder  = decoder{}
)

func (d decoder) group(a diameter.AVP, rules diameter.Rules) ([]diameter.AVP, error) {
	if d.strict {
		return rules.CheckGrouped(a)
	}

	return a.Grouped()
}

func optionalUnsigned(avps []diameter.AVP, code uint32) (uint32, bool, error) {
	a, ok := diameter.Find(avps, code, tgpp.VendorID)
	if !ok {
		return 0, false, nil
	}

	v, err := tgpp.Unsigned32(a)
	if err != nil {
		return 0, false, err
	}

	return v, true, nil
}

func requiredUnsigned(avps []diameter.AVP, code uint32) (uint32, error) {
	v, ok, err := optionalUnsigned(avps, code)
	if err == nil && !ok {
		err = tgpp.MissingAVP(code, tgpp.VendorID, 4)
	}

	return v, err
}

func optionalEnum[T known](avps []diameter.AVP, code uint32, strict bool) (*T, error) {
	a, ok := diameter.Find(avps, code, tgpp.VendorID)
	if !ok {
		return nil, nil
	}

	v, err := tgpp.Unsigned32(a)
	if err != nil {
		return nil, err
	}

	if strict && !T(v).Known() {
		return nil, tgpp.InvalidAVP(a)
	}

	e := T(v)

	return &e, nil
}

func requiredEnum[T known](avps []diameter.AVP, code uint32, strict bool) (T, error) {
	v, err := optionalEnum[T](avps, code, strict)
	if err == nil && v == nil {
		err = tgpp.MissingAVP(code, tgpp.VendorID, 4)
	}

	if err != nil {
		return 0, err
	}

	return *v, nil
}

func requestFlags[T ~uint32](avps []diameter.AVP, code uint32, defined T) (T, error) {
	v, _, err := optionalUnsigned(avps, code)

	return T(v) & defined, err
}

func answerFlags[T ~uint32](avps []diameter.AVP, code uint32) (T, error) {
	v, _, err := optionalUnsigned(avps, code)

	return T(v), err
}

func optionalIndicator(avps []diameter.AVP, code, set uint32) (bool, error) {
	v, ok, err := optionalUnsigned(avps, code)

	return ok && v == set, err
}

func optionalString(avps []diameter.AVP, code, vendorID uint32) (string, error) {
	a, ok := diameter.Find(avps, code, vendorID)
	if !ok {
		return "", nil
	}

	if len(a.Data) == 0 {
		return "", tgpp.InvalidAVP(a)
	}

	return a.UTF8String(), nil
}

func optionalE164(avps []diameter.AVP, code uint32) (string, error) {
	a, ok := diameter.Find(avps, code, tgpp.VendorID)
	if !ok {
		return "", nil
	}

	number, err := tgpp.DecodeE164(a.Data)
	if err != nil {
		return "", tgpp.InvalidAVP(a)
	}

	return number, nil
}

func appendE164(avps []diameter.AVP, code uint32, name, number string) ([]diameter.AVP, error) {
	if number == "" {
		return avps, nil
	}

	b, err := tgpp.EncodeE164(number)
	if err != nil {
		return nil, invalidf("%s: %w", name, err)
	}

	return append(avps, octetsAVP(code, b)), nil
}

func appendEnum[T known](avps []diameter.AVP, code uint32, name string, v *T) ([]diameter.AVP, error) {
	if v == nil {
		return avps, nil
	}

	if !(*v).Known() {
		return nil, invalidf("%s %d", name, uint32(*v))
	}

	return append(avps, unsignedAVP(code, uint32(*v))), nil
}

func appendFlags[T known](avps []diameter.AVP, code uint32, name string, v T) ([]diameter.AVP, error) {
	if !v.Known() {
		return nil, invalidf("%s with undefined bits %#x", name, uint32(v))
	}

	if v == 0 {
		return avps, nil
	}

	return append(avps, unsignedAVP(code, uint32(v))), nil
}

func appendString(avps []diameter.AVP, code uint32, v string) []diameter.AVP {
	if v == "" {
		return avps
	}

	return append(avps, stringAVP(code, v))
}

func appendIndicator(avps []diameter.AVP, code uint32, set bool, value uint32) []diameter.AVP {
	if !set {
		return avps
	}

	return append(avps, unsignedAVP(code, value))
}
