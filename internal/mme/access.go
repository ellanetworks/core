// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"
)

type Access struct {
	Allow4G bool
	Allow5G bool
}

func ResolveAccess(ctx context.Context, m *MME, imsi string) (Access, error) {
	am, err := m.subscriptions().AccessAndMobility(ctx, imsi)
	if err != nil {
		return Access{}, fmt.Errorf("get access and mobility subscription: %w", err)
	}

	return Access{Allow4G: am.Allow4G, Allow5G: am.Allow5G}, nil
}

func (ue *UeContext) SetAccess(a Access) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.allow5G = a.Allow5G
}

func (ue *UeContext) FiveGSInterworkingAllowed() bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.allow5G
}
