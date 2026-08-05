// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/nas/fgs"
)

type Snssai struct {
	Sst int32
	Sd  string
}

func (s Snssai) Equal(other Snssai) bool {
	return s.Sst == other.Sst && s.Sd == other.Sd
}

// NAS builds the S-NSSAI information element (TS 24.501 §9.11.2.8). Both
// accesses need it: 5GS carries the element itself, EPS carries its value part
// in the PCO S-NSSAI container (TS 24.008 §10.5.6.3).
func (s Snssai) NAS() (fgs.SNSSAI, error) {
	out := fgs.SNSSAI{SST: uint8(s.Sst)}

	if s.Sd == "" {
		return out, nil
	}

	sd, err := hex.DecodeString(s.Sd)
	if err != nil {
		return fgs.SNSSAI{}, fmt.Errorf("decode snssai sd: %w", err)
	}

	if len(sd) != 3 {
		return fgs.SNSSAI{}, fmt.Errorf("snssai sd is %d octets, want 3", len(sd))
	}

	out.SD = &[3]byte{sd[0], sd[1], sd[2]}

	return out, nil
}

// SnssaiFromNAS renders a decoded S-NSSAI as the model form.
func SnssaiFromNAS(s fgs.SNSSAI) *Snssai {
	out := Snssai{Sst: int32(s.SST)}
	if s.SD != nil {
		out.Sd = strings.ToUpper(hex.EncodeToString(s.SD[:]))
	}

	return &out
}
