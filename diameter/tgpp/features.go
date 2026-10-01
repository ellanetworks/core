// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tgpp

import (
	"fmt"

	"github.com/ellanetworks/core/diameter"
)

type SupportedFeatures struct {
	VendorID      uint32
	FeatureListID uint32
	FeatureList   uint32
	Mandatory     bool
}

func (f SupportedFeatures) AVP() diameter.AVP {
	var flags uint8
	if f.Mandatory {
		flags = diameter.AVPFlagMandatory
	}

	return diameter.Grouped(AVPSupportedFeatures, flags, VendorID,
		diameter.Unsigned32(diameter.AVPVendorID, diameter.AVPFlagMandatory, 0, f.VendorID),
		diameter.Unsigned32(AVPFeatureListID, 0, VendorID, f.FeatureListID),
		diameter.Unsigned32(AVPFeatureList, 0, VendorID, f.FeatureList),
	)
}

func ParseSupportedFeatures(a diameter.AVP) (SupportedFeatures, error) {
	inner, err := a.Grouped()
	if err != nil {
		return SupportedFeatures{}, fmt.Errorf("Supported-Features: %w", err)
	}

	f := SupportedFeatures{Mandatory: a.Flags&diameter.AVPFlagMandatory != 0}

	for _, field := range []struct {
		code, vendorID uint32
		name           string
		dst            *uint32
	}{
		{diameter.AVPVendorID, 0, "Vendor-Id", &f.VendorID},
		{AVPFeatureListID, VendorID, "Feature-List-ID", &f.FeatureListID},
		{AVPFeatureList, VendorID, "Feature-List", &f.FeatureList},
	} {
		v, ok := diameter.Find(inner, field.code, field.vendorID)
		if !ok {
			return SupportedFeatures{}, fmt.Errorf("Supported-Features without %s", field.name)
		}

		if *field.dst, err = v.Unsigned32(); err != nil {
			return SupportedFeatures{}, fmt.Errorf("Supported-Features %s: %w", field.name, err)
		}
	}

	return f, nil
}

func FeatureList(avps []diameter.AVP, vendorID, featureListID uint32) uint32 {
	var list uint32

	for _, f := range supportedFeatures(avps, vendorID) {
		if f.FeatureListID == featureListID {
			list |= f.FeatureList
		}
	}

	return list
}

func FeaturesMandatory(avps []diameter.AVP, vendorID uint32) bool {
	for _, f := range supportedFeatures(avps, vendorID) {
		if f.Mandatory {
			return true
		}
	}

	return false
}

func supportedFeatures(avps []diameter.AVP, vendorID uint32) []SupportedFeatures {
	var out []SupportedFeatures

	for _, a := range diameter.FindAll(avps, AVPSupportedFeatures, VendorID) {
		if f, err := ParseSupportedFeatures(a); err == nil && f.VendorID == vendorID {
			out = append(out, f)
		}
	}

	return out
}

func UnsupportedRequiredFeatures(avps []diameter.AVP, supported ...SupportedFeatures) bool {
	for _, a := range diameter.FindAll(avps, AVPSupportedFeatures, VendorID) {
		f, err := ParseSupportedFeatures(a)
		if err != nil {
			if a.Flags&diameter.AVPFlagMandatory != 0 {
				return true
			}

			continue
		}

		if f.Mandatory && f.FeatureList&^supportedList(supported, f.VendorID, f.FeatureListID) != 0 {
			return true
		}
	}

	return false
}

func supportedList(supported []SupportedFeatures, vendorID, featureListID uint32) uint32 {
	var list uint32

	for _, s := range supported {
		if s.VendorID == vendorID && s.FeatureListID == featureListID {
			list |= s.FeatureList
		}
	}

	return list
}
