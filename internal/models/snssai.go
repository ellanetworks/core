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

// The SD an operator provisions is free-form text; the SD decoded from NAS or
// NGAP is always the full three octets (TS 23.003 §28.4.2). Comparing them
// literally makes "0A0B0C" fail against the "0a0b0c" a UE signals, so every
// comparison goes through here. Short values are right-padded, matching
// ParseSDString on the operator API: "0a0b" is 0x0a0b00, not 0x000a0b, so it
// does not match a UE signalling "0a0b0c". A value with no canonical form is
// returned unchanged, so it still fails to match rather than matching wrongly.
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
