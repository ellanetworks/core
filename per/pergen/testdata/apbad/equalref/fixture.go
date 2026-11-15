// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fixture

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

type ID uint16

var profile = &ap.Profile{Name: "fixture", MaxProtocolIEs: 65535, MaxProtocolExtensions: 65535}

type Hand struct {
	byName map[string]uint8
}

func (h *Hand) MarshalPER(*per.Writer, per.Encoding) error { return nil }

func (h *Hand) UnmarshalPER(*per.Reader, per.Encoding) error { return nil }

type T struct {
	A    uint8 `per:",range:0..3"`
	Hand Hand
	ext  ap.Preserved[ID] `per:",exttable"`
}
