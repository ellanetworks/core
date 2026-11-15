// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fixture

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

type ID uint16

var profile = &ap.Profile{Name: "fixture", MaxProtocolIEs: 65535, MaxProtocolExtensions: 65535}

const IDX ID = 1

type S struct {
	A uint8 `per:",range:0..3"`
}

type T struct {
	A *S `per:",choice:0"`
	B *S `per:",choice:1,extie:IDX,crit:reject"`
}

var _ per.Encoding
