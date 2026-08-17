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
	sub, err := m.Bearer.GetSubscriber(ctx, imsi)
	if err != nil {
		return Access{}, fmt.Errorf("get subscriber: %w", err)
	}

	profile, err := m.Bearer.GetProfileByID(ctx, sub.ProfileID)
	if err != nil {
		return Access{}, fmt.Errorf("get profile: %w", err)
	}

	return Access{Allow4G: profile.Allow4G, Allow5G: profile.Allow5G}, nil
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
