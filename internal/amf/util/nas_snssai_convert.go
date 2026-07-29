// SPDX-FileCopyrightText: Ella Networks Inc.
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package util

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
)

// SnssaiToModels renders a decoded S-NSSAI as the model form.
func SnssaiToModels(s fgs.SNSSAI) *models.Snssai {
	out := models.Snssai{Sst: int32(s.SST)}
	if s.SD != nil {
		out.Sd = strings.ToUpper(hex.EncodeToString(s.SD[:]))
	}

	return &out
}

// SnssaiToNas builds the NAS S-NSSAI for a model S-NSSAI.
func SnssaiToNas(snssai models.Snssai) (fgs.SNSSAI, error) {
	out := fgs.SNSSAI{SST: uint8(snssai.Sst)}

	if snssai.Sd == "" {
		return out, nil
	}

	sd, err := hex.DecodeString(snssai.Sd)
	if err != nil {
		return fgs.SNSSAI{}, fmt.Errorf("error decoding snssai sd: %+v", err)
	}

	if len(sd) != 3 {
		return fgs.SNSSAI{}, fmt.Errorf("snssai sd is %d octets, want 3", len(sd))
	}

	out.SD = &[3]byte{sd[0], sd[1], sd[2]}

	return out, nil
}
