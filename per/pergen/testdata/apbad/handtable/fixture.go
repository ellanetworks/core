// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fixture

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

type ID uint16

var profile = &ap.Profile{Name: "fixture", MaxProtocolIEs: 65535, MaxProtocolExtensions: 65535}

var tExt []ap.IESpec[ID, T]

type T struct {
	A   uint8                  `per:",range:0..3"`
	ext ap.Preserved[ID]       `per:",exttable:tExt"`
	add per.ExtensionAdditions `per:",extraw"`
}
