// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/roaming"
)

func (m *MME) LocationPrivacy(_ context.Context, supi etsi.SUPI) error {
	ue, ok := m.LookupUeBySupi(supi)
	if !ok {
		return nil
	}

	v, ok := ue.Visitor()
	if !ok {
		return nil
	}

	return roaming.CheckLCSPrivacy4G(v.Subscription)
}
