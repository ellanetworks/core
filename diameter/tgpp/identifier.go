// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tgpp

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/diameter"
)

var ErrInvalidUserIdentifier = errors.New("tgpp: invalid User-Identifier")

type UserIdentifier struct {
	IMSI   string
	MSISDN string
}

func NewUserIdentifier(u UserIdentifier) (diameter.AVP, error) {
	if u.IMSI == "" && u.MSISDN == "" {
		return diameter.AVP{}, fmt.Errorf("%w: no IMSI or MSISDN", ErrInvalidUserIdentifier)
	}

	var inner []diameter.AVP

	if u.IMSI != "" {
		if !ValidIMSI(u.IMSI) {
			return diameter.AVP{}, fmt.Errorf("%w: IMSI %q", ErrInvalidUserIdentifier, u.IMSI)
		}

		inner = append(inner, diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, u.IMSI))
	}

	if u.MSISDN != "" {
		msisdn, err := EncodeE164(u.MSISDN)
		if err != nil {
			return diameter.AVP{}, fmt.Errorf("%w: MSISDN: %w", ErrInvalidUserIdentifier, err)
		}

		inner = append(inner, diameter.OctetString(AVPMSISDN, diameter.AVPFlagMandatory, VendorID, msisdn))
	}

	return diameter.Grouped(AVPUserIdentifier, diameter.AVPFlagMandatory, VendorID, inner...), nil
}

func ParseUserIdentifier(a diameter.AVP) (UserIdentifier, error) {
	inner, err := a.Grouped()
	if err != nil {
		return UserIdentifier{}, fmt.Errorf("%w: %w", ErrInvalidUserIdentifier, err)
	}

	var u UserIdentifier

	if userName, ok := diameter.Find(inner, diameter.AVPUserName, 0); ok {
		u.IMSI = userName.UTF8String()

		if !ValidIMSI(u.IMSI) {
			return UserIdentifier{}, fmt.Errorf("%w: IMSI %q", ErrInvalidUserIdentifier, u.IMSI)
		}
	}

	if msisdn, ok := diameter.Find(inner, AVPMSISDN, VendorID); ok {
		if u.MSISDN, err = DecodeE164(msisdn.Data); err != nil {
			return UserIdentifier{}, fmt.Errorf("%w: MSISDN: %w", ErrInvalidUserIdentifier, err)
		}
	}

	if u.IMSI == "" && u.MSISDN == "" {
		return UserIdentifier{}, fmt.Errorf("%w: no IMSI or MSISDN", ErrInvalidUserIdentifier)
	}

	return u, nil
}
