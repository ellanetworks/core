// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"bytes"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const (
	AVPMMEUserState                uint32 = 1497
	AVPUserState                   uint32 = 1499
	AVPMMELocationInformation      uint32 = 1600
	AVPEUTRANCellGlobalIdentity    uint32 = 1602
	AVPTrackingAreaIdentity        uint32 = 1603
	AVPAgeOfLocationInformation    uint32 = 1611
	eutranCellGlobalIdentityOctets        = 7
	trackingAreaIdentityOctets            = 5
)

func init() {
	for _, code := range []uint32{
		AVPEPSUserState, AVPEPSLocationInformation, AVPMMEUserState, AVPUserState,
		AVPMMELocationInformation, AVPEUTRANCellGlobalIdentity, AVPTrackingAreaIdentity, AVPAgeOfLocationInformation,
	} {
		mandatoryBit[code] = true
	}
}

type UserState uint32

const (
	UserStateDetached                       UserState = 0
	UserStateAttachedNotReachableForPaging  UserState = 1
	UserStateAttachedReachableForPaging     UserState = 2
	UserStateConnectedNotReachableForPaging UserState = 3
	UserStateConnectedReachableForPaging    UserState = 4
)

var userStateNames = tgpp.EnumNames{
	"DETACHED",
	"ATTACHED_NOT_REACHABLE_FOR_PAGING",
	"ATTACHED_REACHABLE_FOR_PAGING",
	"CONNECTED_NOT_REACHABLE_FOR_PAGING",
	"CONNECTED_REACHABLE_FOR_PAGING",
}

func (s UserState) String() string { return userStateNames.Name("UserState", uint32(s)) }

func (s UserState) Known() bool { return userStateNames.Has(uint32(s)) }

type MMELocationInformation struct {
	ECGI []byte
	TAI  []byte
	Age  *uint32
}

type EPSLocationInformation struct {
	MME *MMELocationInformation
}

var (
	epsLocationRules = diameter.Rules{
		vendorKey(AVPMMELocationInformation): {},
	}
	mmeLocationRules = diameter.Rules{
		vendorKey(AVPEUTRANCellGlobalIdentity): {MinLength: eutranCellGlobalIdentityOctets},
		vendorKey(AVPTrackingAreaIdentity):     {MinLength: trackingAreaIdentityOctets},
		vendorKey(AVPAgeOfLocationInformation): {MinLength: 4},
	}
	epsUserStateRules = diameter.Rules{
		vendorKey(AVPMMEUserState): {},
	}
	mmeUserStateRules = diameter.Rules{
		vendorKey(AVPUserState): {MinLength: 4},
	}
)

func (l EPSLocationInformation) avp() (diameter.AVP, error) {
	var inner []diameter.AVP

	if m := l.MME; m != nil {
		var mme []diameter.AVP

		if len(m.ECGI) > 0 {
			if len(m.ECGI) != eutranCellGlobalIdentityOctets {
				return diameter.AVP{}, invalidf("E-UTRAN-Cell-Global-Identity of %d octets", len(m.ECGI))
			}

			mme = append(mme, octetsAVP(AVPEUTRANCellGlobalIdentity, m.ECGI))
		}

		if len(m.TAI) > 0 {
			if len(m.TAI) != trackingAreaIdentityOctets {
				return diameter.AVP{}, invalidf("Tracking-Area-Identity of %d octets", len(m.TAI))
			}

			mme = append(mme, octetsAVP(AVPTrackingAreaIdentity, m.TAI))
		}

		if m.Age != nil {
			mme = append(mme, unsignedAVP(AVPAgeOfLocationInformation, *m.Age))
		}

		inner = append(inner, groupedAVP(AVPMMELocationInformation, mme...))
	}

	return groupedAVP(AVPEPSLocationInformation, inner...), nil
}

func (d decoder) epsLocationInformation(a diameter.AVP) (*EPSLocationInformation, error) {
	inner, err := d.group(a, epsLocationRules)
	if err != nil {
		return nil, err
	}

	var l EPSLocationInformation

	m, ok := diameter.Find(inner, AVPMMELocationInformation, tgpp.VendorID)
	if !ok {
		return &l, nil
	}

	fields, err := d.group(m, mmeLocationRules)
	if err != nil {
		return nil, diameter.WithinGrouped(a, err)
	}

	l.MME = &MMELocationInformation{}

	if ecgi, ok := diameter.Find(fields, AVPEUTRANCellGlobalIdentity, tgpp.VendorID); ok {
		if len(ecgi.Data) != eutranCellGlobalIdentityOctets {
			return nil, diameter.WithinGrouped(a, diameter.WithinGrouped(m, tgpp.InvalidAVP(ecgi)))
		}

		l.MME.ECGI = bytes.Clone(ecgi.Data)
	}

	if tai, ok := diameter.Find(fields, AVPTrackingAreaIdentity, tgpp.VendorID); ok {
		if len(tai.Data) != trackingAreaIdentityOctets {
			return nil, diameter.WithinGrouped(a, diameter.WithinGrouped(m, tgpp.InvalidAVP(tai)))
		}

		l.MME.TAI = bytes.Clone(tai.Data)
	}

	if age, ok, err := optionalUnsigned(fields, AVPAgeOfLocationInformation); err != nil {
		return nil, diameter.WithinGrouped(a, diameter.WithinGrouped(m, err))
	} else if ok {
		l.MME.Age = &age
	}

	return &l, nil
}

func userStateAVP(s UserState) (diameter.AVP, error) {
	if !s.Known() {
		return diameter.AVP{}, invalidf("User-State %d", s)
	}

	return groupedAVP(AVPEPSUserState, groupedAVP(AVPMMEUserState, unsignedAVP(AVPUserState, uint32(s)))), nil
}

func (d decoder) userState(a diameter.AVP) (*UserState, error) {
	inner, err := d.group(a, epsUserStateRules)
	if err != nil {
		return nil, err
	}

	m, ok := diameter.Find(inner, AVPMMEUserState, tgpp.VendorID)
	if !ok {
		return nil, nil
	}

	fields, err := d.group(m, mmeUserStateRules)
	if err != nil {
		return nil, diameter.WithinGrouped(a, err)
	}

	s, err := optionalEnum[UserState](fields, AVPUserState, d.strict)
	if err != nil {
		return nil, diameter.WithinGrouped(a, diameter.WithinGrouped(m, err))
	}

	return s, nil
}
