// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import (
	"encoding/hex"
	"strings"
)

type Snssai struct {
	Sst int32
	Sd  string
}

func (s Snssai) Equal(other Snssai) bool {
	return s.Sst == other.Sst && NormalizeSD(s.Sd) == NormalizeSD(other.Sd)
}

func NormalizeSD(sd string) string {
	if sd == "" {
		return ""
	}

	b, err := hex.DecodeString(sd)
	if err != nil || len(b) == 0 || len(b) > 3 {
		return sd
	}

	var full [3]byte

	copy(full[:], b)

	return strings.ToLower(hex.EncodeToString(full[:]))
}
