// SPDX-FileCopyrightText: Ella Networks Inc.
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package util

import (
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// TaiListToNas builds the TAI list IE value for the given TAIs
// (TS 24.501 §9.11.3.9).
func TaiListToNas(taiList []models.Tai) (fgs.TAIList, error) {
	tais := make([]fgs.TAI, 0, len(taiList))

	for _, tai := range taiList {
		if tai.PlmnID == nil {
			return nil, fmt.Errorf("tai has no PLMN ID")
		}

		tac, err := strconv.ParseUint(tai.Tac, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("failed to decode tac %q: %w", tai.Tac, err)
		}

		tais = append(tais, fgs.TAI{PLMN: nas.PLMN{MCC: tai.PlmnID.Mcc, MNC: tai.PlmnID.Mnc}, TAC: uint32(tac)})
	}

	return fgs.NewTAIList(tais...)
}
