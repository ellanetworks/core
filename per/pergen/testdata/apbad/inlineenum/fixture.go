// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fixture

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

type ID uint16

var profile = &apimpl.Profile{Name: "fixture", MaxProtocolIEs: 65535, MaxProtocolExtensions: 65535}

type E uint8

type T struct {
	A   E                      `per:"ENUMERATED,range:0..1,..."`
	ext apimpl.Preserved[ID]       `per:",exttable"`
	add per.ExtensionAdditions `per:",extraw"`
}
