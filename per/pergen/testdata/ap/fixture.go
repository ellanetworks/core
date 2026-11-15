// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fixture

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

type ID uint16

const IDThing ID = 7

var profile = &ap.Profile{Name: "fixture", MaxProtocolIEs: 65535, MaxProtocolExtensions: 65535}

type Kind uint8

var enumKind = per.Enum[Kind]{Root: []string{"a", "b"}, Extension: []string{"c"}}

type Inner struct {
	K     Kind
	Thing *Kind                  `per:",extie:IDThing,crit:ignore"`
	ext   ap.Preserved[ID]       `per:",exttable"`
	add   per.ExtensionAdditions `per:",extraw"`
}

type Choice struct {
	In    *Inner                    `per:",choice:0"`
	Typed *Inner                    `per:",choice:1,extie:IDThing,crit:reject"`
	Raw   *ap.SingleContainer[ID]   `per:",choice:1,extie"`
	Ext   *per.ExtensionAlternative `per:",extalt"`
	crit  ap.WireCriticality        `per:",choicecrit"`
}

type List []Inner

type Container struct {
	Typed *Inner                  `per:",choice:0,extie:IDThing,crit:ignore"`
	Raw   *ap.SingleContainer[ID] `per:",choice:0,extie"`
	crit  ap.WireCriticality      `per:",choicecrit"`
}
