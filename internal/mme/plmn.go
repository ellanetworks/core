// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/s1ap"
)

// EncodePLMN encodes an MCC/MNC pair into the 3-octet TBCD PLMN identity
// (TS 23.003).
func EncodePLMN(plmn models.PlmnID) (s1ap.PLMNIdentity, error) {
	b, err := nascommon.EncodePLMN(plmn.Mcc, plmn.Mnc)
	if err != nil {
		return s1ap.PLMNIdentity{}, fmt.Errorf("mme: encode PLMN mcc=%q mnc=%q: %w", plmn.Mcc, plmn.Mnc, err)
	}

	return s1ap.PLMNIdentity(b), nil
}

// decodePLMN decodes a 3-octet TBCD PLMN identity into its MCC/MNC pair
// (TS 23.003).
func decodePLMN(p s1ap.PLMNIdentity) models.PlmnID {
	mcc, mnc := nascommon.DecodePLMN([3]byte(p))

	return models.PlmnID{Mcc: mcc, Mnc: mnc}
}
