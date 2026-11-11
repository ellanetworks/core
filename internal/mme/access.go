// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/udm"
)

type Access struct {
	Allow4G            bool
	FiveGSInterworking bool
}

func ResolveAccess(ctx context.Context, m *MME, imsi string) (Access, error) {
	am, err := udm.NewSubscriptions(m.Bearer, nil).AccessAndMobility(ctx, imsi)
	if err != nil {
		return Access{}, fmt.Errorf("get access and mobility subscription: %w", err)
	}

	n26, err := m.N26Allowed(ctx, imsi)
	if err != nil {
		return Access{}, err
	}

	fiveGS := am.Allow5G && n26

	if am.Visitor {
		home, ok := m.HPLMNSubscription(imsi)
		fiveGS = fiveGS && (!ok || !home.FiveGCNotAllowed())
	}

	return Access{Allow4G: am.Allow4G, FiveGSInterworking: fiveGS}, nil
}

func IsOwnSubscriber(ctx context.Context, m *MME, imsi string) (bool, error) {
	return udm.NewSubscriptions(m.Bearer, nil).IsOwnSubscriber(ctx, imsi)
}

func (m *MME) N26Allowed(ctx context.Context, imsi string) (bool, error) {
	if m.N26 != nil {
		return m.N26.Allowed(ctx, imsi)
	}

	return IsOwnSubscriber(ctx, m, imsi)
}

func (m *MME) FiveGSInterworkingSubscribed(ue *UeContext, apn string) bool {
	if !ue.IsVisitor() {
		return true
	}

	home, _ := m.HPLMNSubscription(ue.IMSI())

	return roaming.FiveGSInterworkingSubscribed(home, apn)
}

func (ue *UeContext) SetAccess(a Access) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.allow5G = a.FiveGSInterworking
}

func (ue *UeContext) FiveGSInterworkingAllowed() bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.allow5G
}
