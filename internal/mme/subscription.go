// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/s1ap"
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

// Pre-emption is fixed at shall-not-trigger / not-pre-emptable, the same pair
// the 5G path encodes: a policy carries an ARP priority column and nothing else,
// so a bearer claiming it may displace another's radio resources would be
// claiming an authorization the profile never granted.
func BearerARP(priority byte) s1ap.AllocationAndRetentionPriority {
	return s1ap.AllocationAndRetentionPriority{
		PriorityLevel:           priority,
		PreemptionCapability:    s1ap.PreemptionShallNotTrigger,
		PreemptionVulnerability: s1ap.PreemptionNotPreemptable,
	}
}
