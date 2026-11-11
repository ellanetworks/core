// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameternode

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/nas/fgs"
)

func DiameterRealm(mcc, mnc string) (string, error) {
	return plmnRealm("epc", mcc, mnc)
}

func IMSRealm(mcc, mnc string) (string, error) {
	return plmnRealm("ims", mcc, mnc)
}

func ServedRealms(mcc, mnc, homeDomain string) ([]string, error) {
	epc, err := DiameterRealm(mcc, mnc)
	if err != nil {
		return nil, err
	}

	ims, err := IMSRealm(mcc, mnc)
	if err != nil {
		return nil, err
	}

	realms := []string{epc, ims}

	if homeDomain != "" && !slices.ContainsFunc(realms, func(r string) bool { return strings.EqualFold(r, homeDomain) }) {
		realms = append(realms, strings.ToLower(homeDomain))
	}

	return realms, nil
}

func plmnRealm(domain, mcc, mnc string) (string, error) {
	if !isDigits(mcc, 3, 3) {
		return "", fmt.Errorf("invalid MCC %q", mcc)
	}

	if !isDigits(mnc, 2, 3) {
		return "", fmt.Errorf("invalid MNC %q", mnc)
	}

	if len(mnc) == 2 {
		mnc = "0" + mnc
	}

	return fmt.Sprintf("%s.mnc%s.mcc%s.3gppnetwork.org", domain, mnc, mcc), nil
}

func NodeSettingsOf(mcc, mnc string, amfRegionID uint8, amfSetID uint16, amfPointer uint8) NodeSettings {
	mapped := etsi.MapGUTI5GToEPS(fgs.GUTI{AMFRegionID: amfRegionID, AMFSetID: amfSetID, AMFPointer: amfPointer})

	return NodeSettings{MCC: mcc, MNC: mnc, MMEGroupID: mapped.MMEGroupID, MMECode: mapped.MMECode}
}

func MMEHost(realm string, groupID uint16, code uint8) string {
	return fmt.Sprintf("mmec%02x.mmegi%04x.mme.%s", code, groupID, realm)
}

func isDigits(s string, minLen, maxLen int) bool {
	if len(s) < minLen || len(s) > maxLen {
		return false
	}

	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
}
