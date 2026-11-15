// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fixture

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

type ID uint16

var profile = &apimpl.Profile{Name: "fixture", MaxProtocolIEs: 65535, MaxProtocolExtensions: 65535}

type T struct {
	A   uint8            `per:",range:0..3"`
	_   [0]struct{}      `per:"extseq"`
	ext apimpl.Preserved[ID] `per:",exttable"`
}

var _ per.Encoding
