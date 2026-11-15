// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fixture

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

type ID uint16

var profile = &apimpl.Profile{Name: "fixture", MaxProtocolIEs: 65535, MaxProtocolExtensions: 65535}

var tExt []apimpl.IESpec[ID, T]

type T struct {
	A   uint8                  `per:",range:0..3"`
	ext apimpl.Preserved[ID]       `per:",exttable:tExt"`
	add per.ExtensionAdditions `per:",extraw"`
}
