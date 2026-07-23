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
)

// SnssaiToModels decodes an S-NSSAI IE value (TS 24.501 §9.11.2.8): octet 1 is
// the SST, octets 2-4 (when present) are the SD.
func SnssaiToModels(v []byte) *models.Snssai {
	var out models.Snssai

	if len(v) == 0 {
		return &out
	}

	out.Sst = int32(v[0])

	if len(v) >= 4 {
		out.Sd = strings.ToUpper(hex.EncodeToString(v[1:4]))
	}

	return &out
}

func SnssaiToNas(snssai models.Snssai) ([]uint8, error) {
	var buf []uint8

	if snssai.Sd == "" {
		buf = append(buf, 0x01)
		buf = append(buf, uint8(snssai.Sst))
	} else {
		buf = append(buf, 0x04)
		buf = append(buf, uint8(snssai.Sst))

		byteArray, err := hex.DecodeString(snssai.Sd)
		if err != nil {
			return nil, fmt.Errorf("error decoding snssai sd: %+v", err)
		}

		buf = append(buf, byteArray...)
	}

	return buf, nil
}
