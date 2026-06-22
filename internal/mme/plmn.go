// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

// encodePLMN encodes an MCC/MNC pair into the 3-octet TBCD PLMN identity
// (TS 23.003).
func encodePLMN(plmn models.PlmnID) (s1ap.PLMNIdentity, error) {
	mcc := strings.Split(plmn.Mcc, "")
	mnc := strings.Split(plmn.Mnc, "")

	if len(mcc) != 3 || (len(mnc) != 2 && len(mnc) != 3) {
		return s1ap.PLMNIdentity{}, fmt.Errorf("mme: invalid PLMN mcc=%q mnc=%q", plmn.Mcc, plmn.Mnc)
	}

	var hexString string
	if len(mnc) == 2 {
		hexString = mcc[1] + mcc[0] + "f" + mcc[2] + mnc[1] + mnc[0]
	} else {
		hexString = mcc[1] + mcc[0] + mnc[0] + mcc[2] + mnc[2] + mnc[1]
	}

	b, err := hex.DecodeString(hexString)
	if err != nil {
		return s1ap.PLMNIdentity{}, fmt.Errorf("mme: encode PLMN: %w", err)
	}

	var out s1ap.PLMNIdentity

	copy(out[:], b)

	return out, nil
}

// decodePLMN decodes a 3-octet TBCD PLMN identity into its MCC/MNC pair, the
// inverse of encodePLMN. The high nibble of octet 2 holds the third MNC digit and
// is 0xf for a 2-digit MNC, whose two digits then occupy octet 3 (TS 23.003).
func decodePLMN(p s1ap.PLMNIdentity) models.PlmnID {
	mcc := fmt.Sprintf("%x%x%x", p[0]&0x0f, p[0]>>4, p[1]&0x0f)

	if p[1]>>4 == 0x0f {
		return models.PlmnID{Mcc: mcc, Mnc: fmt.Sprintf("%x%x", p[2]&0x0f, p[2]>>4)}
	}

	return models.PlmnID{Mcc: mcc, Mnc: fmt.Sprintf("%x%x%x", p[1]>>4, p[2]&0x0f, p[2]>>4)}
}
