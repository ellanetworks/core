// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pcf

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/smf"
)

func (p *PCF) decide(ctx context.Context, c smf.PolicyContext) (*smf.PolicyDecision, error) {
	pol, err := p.store.GetSessionPolicy(ctx, c.Supi.IMSI(), c.Snssai.Sst, c.Snssai.Sd, c.Dnn)
	if err != nil {
		return nil, policyLookupError(err)
	}

	return &smf.PolicyDecision{
		PolicyID:    pol.ID,
		Var5qi:      c.Subscribed.Var5qi,
		Arp:         c.Subscribed.Arp,
		SessionAMBR: c.Subscribed.SessionAMBR,
	}, nil
}

func policyLookupError(err error) error {
	if errors.Is(err, db.ErrDataNetworkNotFound) {
		return fmt.Errorf("%w: %v", smf.ErrDNNNotFound, err)
	}

	if errors.Is(err, db.ErrDNNNotInSlice) {
		return fmt.Errorf("%w: %v", smf.ErrDNNNotInSlice, err)
	}

	if errors.Is(err, db.ErrNoMatchingPolicy) {
		return fmt.Errorf("%w: %v", smf.ErrNoPolicyMatch, err)
	}

	return fmt.Errorf("get session policy: %w", err)
}
