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
}

func (f SupportedFeatures) AVP() diameter.AVP {
	return diameter.Grouped(AVPSupportedFeatures, 0, VendorID,
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

	var f SupportedFeatures

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

	for _, a := range diameter.FindAll(avps, AVPSupportedFeatures, VendorID) {
		f, err := ParseSupportedFeatures(a)
		if err == nil && f.VendorID == vendorID && f.FeatureListID == featureListID {
			list |= f.FeatureList
		}
	}

	return list
}
