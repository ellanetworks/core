// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
)

// ErrUnknownAPN reports that the subscriber's profile has no policy bound to a
// data network with the requested APN, so the PDN connection cannot be
// authorised (TS 24.301 ESM cause #27).
var ErrUnknownAPN = models.ErrUnknownAPN

func (m *MME) subscriptions() *udm.Subscriptions {
	return udm.NewSubscriptions(m.Bearer, nil)
}

func SubscribedAPN(ctx context.Context, m *MME, imsi, requested string) (string, error) {
	sm, err := m.subscriptions().SessionManagement(ctx, imsi)
	if err != nil {
		return "", fmt.Errorf("get session management subscription: %w", err)
	}

	if requested == "" {
		c, ok := sm.DefaultAPN()
		if !ok {
			return "", fmt.Errorf("subscriber %s has no default APN", imsi)
		}

		return c.DNN, nil
	}

	if _, ok := sm.ForAPN(requested); !ok {
		return "", ErrUnknownAPN
	}

	return requested, nil
}

func SubscribedUEAMBR(ctx context.Context, m *MME, imsi string) (models.Ambr, error) {
	return m.subscriptions().UEAMBR(ctx, imsi)
}
