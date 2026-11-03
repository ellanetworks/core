// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"slices"
	"strings"

	"github.com/ellanetworks/core/internal/roaming"
)

func (m *MME) Roamers() []roaming.LiveRoamer {
	m.mu.RLock()
	ues := make([]*UeContext, 0, len(m.UEs))

	for _, ue := range m.UEs {
		ues = append(ues, ue)
	}

	m.mu.RUnlock()

	out := make([]roaming.LiveRoamer, 0)

	for _, ue := range ues {
		if r, ok := m.liveRoamer(ue); ok {
			out = append(out, r)
		}
	}

	slices.SortFunc(out, func(a, b roaming.LiveRoamer) int { return strings.Compare(a.IMSI, b.IMSI) })

	return out
}

func (m *MME) RoamerLoad() map[string]roaming.PartnerLoad {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := map[string]roaming.PartnerLoad{}

	for _, ue := range m.UEs {
		if ue.EMMState() != EMMRegistered || ue.imsiOrEmpty() == "" {
			continue
		}

		v, ok := ue.Visitor()
		if !ok {
			continue
		}

		ue.mu.Lock()
		sessions := len(ue.Pdns)
		ue.mu.Unlock()

		load := out[v.PartnerName]
		load.Roamers++
		load.Sessions += sessions
		out[v.PartnerName] = load
	}

	return out
}

func (m *MME) liveRoamer(ue *UeContext) (roaming.LiveRoamer, bool) {
	imsi := ue.imsiOrEmpty()
	if imsi == "" {
		return roaming.LiveRoamer{}, false
	}

	v, ok := ue.Visitor()
	if !ok {
		return roaming.LiveRoamer{}, false
	}

	cs, found := m.LookupSubscriber(imsi)
	if !found || !cs.Registered {
		return roaming.LiveRoamer{}, false
	}

	r := roaming.LiveRoamer{
		IMSI:        imsi,
		PartnerID:   v.PartnerID,
		PartnerName: v.PartnerName,
		HomeMcc:     v.HomePLMN.MCC,
		HomeMnc:     v.HomePLMN.MNC,
		RAT:         roaming.RAT4G,
		Registered:  cs.Registered,
		Connected:   cs.Connected,
		Since:       v.Since,
		LastSeenAt:  cs.LastSeenAt,
		Sessions:    make([]roaming.LiveRoamerSession, 0, len(cs.Sessions)),
	}

	if seen, ok := m.LastSeen(imsi); ok {
		r.Radio = seen.RadioName
	}

	for _, s := range cs.Sessions {
		r.Sessions = append(r.Sessions, roaming.LiveRoamerSession{
			ID:          s.BearerID,
			DataNetwork: s.APN,
			IPv4Address: s.IPv4Address,
			IPv6Prefix:  s.IPv6Prefix,
		})
	}

	return r, true
}
