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
	return featureList(avps, vendorID, featureListID, false)
}

func RequiredFeatureList(avps []diameter.AVP, vendorID, featureListID uint32) uint32 {
	return featureList(avps, vendorID, featureListID, true)
}

func FeatureAVPs(vendorID, featureListID, features, required uint32) []diameter.AVP {
	var avps []diameter.AVP

	if required != 0 {
		avps = append(avps, SupportedFeatures{VendorID: vendorID, FeatureListID: featureListID, FeatureList: required, Mandatory: true}.AVP())
	}

	if optional := features &^ required; optional != 0 {
		avps = append(avps, SupportedFeatures{VendorID: vendorID, FeatureListID: featureListID, FeatureList: optional}.AVP())
	}

	return avps
}

func featureList(avps []diameter.AVP, vendorID, featureListID uint32, requiredOnly bool) uint32 {
	var list uint32

	for _, a := range diameter.FindAll(avps, AVPSupportedFeatures, VendorID) {
		f, err := ParseSupportedFeatures(a)
		if err == nil && f.VendorID == vendorID && f.FeatureListID == featureListID && (f.Mandatory || !requiredOnly) {
			list |= f.FeatureList
		}
	}

	return list
}
