// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package util

import (
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/common"
)

// PlmnIDToNas encodes a PLMN identity into its 3-octet NAS/TBCD representation
// (TS 24.008). The codec is shared with the 4G stack via nas/common.
func PlmnIDToNas(plmnID models.PlmnID) ([]uint8, error) {
	b, err := common.EncodePLMN(plmnID.Mcc, plmnID.Mnc)
	if err != nil {
		return nil, err
	}

	return b[:], nil
}
