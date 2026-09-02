// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package utils

import (
	"encoding/hex"
	"slices"

	"github.com/ellanetworks/core/nas"
)

// RawIE is a NAS information element the decoder does not model, carried through
// so a capture shows the element arrived rather than dropping it silently. It
// mirrors what the NGAP and S1AP decoders render for an unmodelled IE, with the
// identifier NAS puts on the element itself rather than on a wrapper.
type RawIE struct {
	IEI  uint8  `json:"iei"`
	Name string `json:"name,omitempty"`
	Hex  string `json:"hex"`
}

// RawIEs renders the elements a NAS message preserved but the decoder does not
// model. It returns nil for none, so the field stays out of the JSON.
func RawIEs(raw []nas.RawIE) []RawIE {
	return RawIEsExcept(raw)
}

// RawIEsExcept is RawIEs for a builder that already renders some preserved
// elements itself: those identifiers are named here so an element is reported
// once, either as its own field or as a raw one, never both.
func RawIEsExcept(raw []nas.RawIE, handled ...uint8) []RawIE {
	var out []RawIE

	for _, r := range raw {
		if slices.Contains(handled, r.IEI) {
			continue
		}

		out = append(out, RawIE{IEI: r.IEI, Name: r.Name, Hex: hex.EncodeToString(r.Value)})
	}

	return out
}
