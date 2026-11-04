// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/udm"
)

// ErrUnknownAPN reports that the subscriber's profile has no policy bound to a
// data network with the requested APN, so the PDN connection cannot be
// authorised (TS 24.301 ESM cause #27).
var ErrUnknownAPN = models.ErrUnknownAPN

func (m *MME) subscriptions() *roaming.Subscriptions {
	return roaming.NewSubscriptions(udm.NewSubscriptions(m.Bearer, nil), m)
}

func SubscribedAPN(ctx context.Context, m *MME, imsi, requested string) (string, error) {
	apn, err := m.subscriptions().SelectAPN(ctx, imsi, requested)
	if err != nil {
		return "", fmt.Errorf("select the APN: %w", err)
	}

	return apn, nil
}

func SubscribedUEAMBR(ctx context.Context, m *MME, imsi string) (models.Ambr, error) {
	return m.subscriptions().UEAMBR(ctx, imsi)
}

func (m *MME) APNSubscription(ue *UeContext, apn string) (*models.APNSubscription, error) {
	v, ok := ue.Visitor()
	if !ok {
		return nil, nil
	}

	if v.Subscription == nil {
		return nil, fmt.Errorf("%w: %s", roaming.ErrNoHPLMNSubscription, ue.IMSI())
	}

	return roaming.APNSubscription(models.PlmnID{Mcc: v.HomePLMN.MCC, Mnc: v.HomePLMN.MNC}, v.Subscription, apn)
}
