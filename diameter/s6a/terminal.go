// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"bytes"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const (
	minIMEIDigits = 14
	maxIMEIDigits = 15
	svnDigits     = 2
)

type TerminalInformation struct {
	IMEI            string
	SoftwareVersion string
	MEID            []byte
}

var terminalRules = diameter.Rules{
	vendorKey(AVPIMEI):            {},
	vendorKey(AVP3GPP2MEID):       {},
	vendorKey(AVPSoftwareVersion): {},
}

func (t TerminalInformation) avp() (diameter.AVP, error) {
	switch {
	case t.IMEI == "" && t.SoftwareVersion == "" && len(t.MEID) == 0:
		return diameter.AVP{}, invalidf("empty terminal information")
	case t.IMEI != "" && !tgpp.Digits(t.IMEI, minIMEIDigits, maxIMEIDigits):
		return diameter.AVP{}, invalidf("IMEI %q", t.IMEI)
	case t.SoftwareVersion != "" && !tgpp.Digits(t.SoftwareVersion, svnDigits, svnDigits):
		return diameter.AVP{}, invalidf("software version %q", t.SoftwareVersion)
	}

	inner := appendString(nil, AVPIMEI, t.IMEI)

	if len(t.MEID) > 0 {
		inner = append(inner, octetsAVP(AVP3GPP2MEID, t.MEID))
	}

	inner = appendString(inner, AVPSoftwareVersion, t.SoftwareVersion)

	return groupedAVP(AVPTerminalInformation, inner...), nil
}

func (d decoder) terminal(a diameter.AVP) (*TerminalInformation, error) {
	inner, err := d.group(a, terminalRules)
	if err != nil {
		return nil, err
	}

	var t TerminalInformation

	if imei, ok := diameter.Find(inner, AVPIMEI, tgpp.VendorID); ok {
		if t.IMEI = imei.UTF8String(); !tgpp.Digits(t.IMEI, minIMEIDigits, maxIMEIDigits) {
			return nil, diameter.WithinGrouped(a, tgpp.InvalidAVP(imei))
		}
	}

	if sv, ok := diameter.Find(inner, AVPSoftwareVersion, tgpp.VendorID); ok {
		if t.SoftwareVersion = sv.UTF8String(); !tgpp.Digits(t.SoftwareVersion, svnDigits, svnDigits) {
			return nil, diameter.WithinGrouped(a, tgpp.InvalidAVP(sv))
		}
	}

	if meid, ok := diameter.Find(inner, AVP3GPP2MEID, tgpp.VendorID); ok {
		t.MEID = bytes.Clone(meid.Data)
	}

	if t.IMEI == "" && t.SoftwareVersion == "" && len(t.MEID) == 0 {
		return nil, nil
	}

	return &t, nil
}
