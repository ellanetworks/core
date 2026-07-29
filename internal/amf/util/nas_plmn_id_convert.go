// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package util

import (
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
)

// PlmnIDToNas encodes a PLMN identity into its 3-octet NAS/TBCD representation
// (TS 24.008).
func PlmnIDToNas(plmnID models.PlmnID) ([]uint8, error) {
	b, err := nas.PLMN{MCC: plmnID.Mcc, MNC: plmnID.Mnc}.Octets()
	if err != nil {
		return nil, err
	}

	return b[:], nil
}
