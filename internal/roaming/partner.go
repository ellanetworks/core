// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/diameternode"
)

func PartnerOfIMSI(partners []db.RoamingPartner, imsi string) (db.RoamingPartner, db.RoamingPLMN, bool) {
	for _, p := range partners {
		if plmn, ok := p.PLMNOfIMSI(imsi); ok {
			return p, plmn, true
		}
	}

	return db.RoamingPartner{}, db.RoamingPLMN{}, false
}

func DiameterPeerID(id string) string {
	return diameternode.PeerRoleRoaming + "-" + id
}

func DiameterPeers(peers []db.RoamingDiameterPeer, partners []db.RoamingPartner) ([]diameternode.PeerConfig, error) {
	routes := make(map[string][]diameter.Route)

	for _, p := range partners {
		if p.DiameterPeerID == nil {
			continue
		}

		for _, plmn := range p.PLMNs {
			realm, err := diameternode.DiameterRealm(plmn.Mcc, plmn.Mnc)
			if err != nil {
				return nil, fmt.Errorf("roaming partner %s: %w", p.Name, err)
			}

			routes[*p.DiameterPeerID] = append(routes[*p.DiameterPeerID], diameter.Route{Realm: realm, Application: s6a.ApplicationID})
		}
	}

	for _, r := range routes {
		slices.SortFunc(r, func(a, b diameter.Route) int { return strings.Compare(a.Realm, b.Realm) })
	}

	out := make([]diameternode.PeerConfig, 0, len(peers))

	for _, p := range peers {
		addr, err := netip.ParseAddr(p.Address)
		if err != nil {
			return nil, fmt.Errorf("invalid roaming Diameter peer address %q: %w", p.Address, err)
		}

		if p.Port < 1 || p.Port > 65535 {
			return nil, fmt.Errorf("invalid roaming Diameter peer port %d", p.Port)
		}

		out = append(out, diameternode.PeerConfig{
			ID:           DiameterPeerID(p.ID),
			Role:         diameternode.PeerRoleRoaming,
			Host:         p.DiameterIdentity,
			Address:      netip.AddrPortFrom(addr, uint16(p.Port)),
			Applications: diameternode.RoamingApplications,
			Routes:       routes[p.ID],
		})
	}

	return out, nil
}

func plmnOf(p db.RoamingPLMN) tgpp.PLMN {
	return tgpp.PLMN{MCC: p.Mcc, MNC: p.Mnc}
}
