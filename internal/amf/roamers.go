// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"slices"
	"strings"

	"github.com/ellanetworks/core/internal/roaming"
)

func (amf *AMF) Roamers() []roaming.LiveRoamer {
	amf.mu.RLock()
	ues := make([]*UeContext, 0, len(amf.UEs))

	for _, ue := range amf.UEs {
		ues = append(ues, ue)
	}

	amf.mu.RUnlock()

	out := make([]roaming.LiveRoamer, 0)

	for _, ue := range ues {
		if r, ok := amf.liveRoamer(ue); ok {
			out = append(out, r)
		}
	}

	slices.SortFunc(out, func(a, b roaming.LiveRoamer) int { return strings.Compare(a.IMSI, b.IMSI) })

	return out
}

func (amf *AMF) RoamerLoad() map[string]roaming.PartnerLoad {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	out := map[string]roaming.PartnerLoad{}

	for _, ue := range amf.UEs {
		ue.mu.Lock()
		v, registered, sessions := ue.visitor, ue.state == Registered, len(ue.SmContextList)
		ue.mu.Unlock()

		if v == nil || !registered {
			continue
		}

		load := out[v.PartnerName]
		load.Roamers++
		load.Sessions += sessions
		out[v.PartnerName] = load
	}

	return out
}

func (amf *AMF) liveRoamer(ue *UeContext) (roaming.LiveRoamer, bool) {
	supi := ue.Supi()
	if !supi.IsIMSI() {
		return roaming.LiveRoamer{}, false
	}

	v, ok := ue.Visitor()
	if !ok {
		return roaming.LiveRoamer{}, false
	}

	snap, pdus, found := amf.LookupSubscriber(supi)
	if !found || !snap.Registered {
		return roaming.LiveRoamer{}, false
	}

	imsi := supi.IMSI()

	r := roaming.LiveRoamer{
		IMSI:        imsi,
		PartnerID:   v.PartnerID,
		PartnerName: v.PartnerName,
		HomeMcc:     v.Home.PLMN.Mcc,
		HomeMnc:     v.Home.PLMN.Mnc,
		RAT:         roaming.RAT5G,
		Registered:  snap.Registered,
		Connected:   snap.Connected,
		Since:       v.Since,
		LastSeenAt:  snap.LastSeenAt,
		Sessions:    make([]roaming.LiveRoamerSession, 0, len(pdus)),
	}

	if seen, ok := amf.LastSeen(imsi); ok {
		r.Radio = seen.RadioName
	}

	for _, p := range pdus {
		r.Sessions = append(r.Sessions, roaming.LiveRoamerSession{
			ID:          p.PDUSessionID,
			DataNetwork: p.DNN,
			IPv4Address: p.PDUIPV4Address,
			IPv6Prefix:  p.PDUIPV6Prefix,
		})
	}

	slices.SortFunc(r.Sessions, func(a, b roaming.LiveRoamerSession) int { return int(a.ID) - int(b.ID) })

	return r, true
}
