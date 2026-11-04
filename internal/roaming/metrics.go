// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/diameternode"
	"github.com/ellanetworks/core/internal/metrics"
)

func s6aResult(ans *diameter.Message, err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return metrics.PeerTimeout
	case err != nil || ans == nil:
		return metrics.PeerError
	}

	r, err := tgpp.ParseResult(ans)

	switch {
	case err != nil:
		return metrics.PeerError
	case r.Success():
		return metrics.PeerSuccess
	default:
		return metrics.PeerFailure
	}
}

func ConnectedS6aPartners(partners []db.RoamingPartner, peers []diameternode.PeerStatus) int {
	open := make(map[string]bool, len(peers))

	for _, p := range peers {
		if p.Role == diameternode.PeerRoleRoaming && p.State == diameter.PeerOpen {
			open[p.ID] = true
		}
	}

	n := 0

	for _, p := range partners {
		if p.DiameterPeerID != nil && open[DiameterPeerID(*p.DiameterPeerID)] {
			n++
		}
	}

	return n
}
