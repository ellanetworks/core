// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

type BearerView struct {
	ID      uint8
	Default bool
	QCI     uint8
	ARP     models.Arp
	MBR     models.Ambr
	GBR     models.Ambr
	Rules   []string
	Traffic *Traffic
}

type SessionView struct {
	EstablishedAt time.Time
	ReadAt        time.Time
	Traffic       *Traffic
	Bearers       []BearerView
}

type Traffic struct {
	UplinkVolume    uint64
	DownlinkVolume  uint64
	UplinkPackets   uint64
	DownlinkPackets uint64
	LastPacket      time.Time
}

func (t *Traffic) add(r models.UsageReport) {
	t.UplinkVolume += r.UplinkVolume
	t.DownlinkVolume += r.DownlinkVolume
	t.UplinkPackets += r.UplinkPackets
	t.DownlinkPackets += r.DownlinkPackets

	if r.LastPacket.After(t.LastPacket) {
		t.LastPacket = r.LastPacket
	}
}

type sessionTraffic struct {
	session   Traffic
	onDefault Traffic
	dedicated map[*dedicatedBearer]Traffic
}

func (sc *SMContext) dedicatedBySlot(slot uint8) *dedicatedBearer {
	for _, b := range sc.dedicated {
		if b.slot == slot {
			return b
		}
	}

	return nil
}

func (s *SMF) recordSessionTraffic(_ context.Context, sc *SMContext, usage []urrUsage) {
	for _, u := range usage {
		sc.traffic.add(u.UsageReport)

		if !u.Charging.Dedicated {
			sc.defaultTraffic.add(u.UsageReport)
			continue
		}

		if b := sc.dedicatedBySlot(u.Charging.Slot); b != nil {
			b.traffic.add(u.UsageReport)
		}
	}
}

func ruleIDs(rules []PCCRule) []string {
	ids := make([]string, 0, len(rules))
	for _, r := range rules {
		ids = append(ids, r.ID)
	}

	slices.Sort(ids)

	return ids
}

func (s *SMF) SessionView(ctx context.Context, ref string) (SessionView, bool) {
	sc := s.GetSession(ref)
	if sc == nil {
		return SessionView{}, false
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	live, ok := s.liveTrafficLocked(ctx, sc)

	v := SessionView{EstablishedAt: sc.EstablishedAt}

	if !ok {
		v.Bearers = sc.bearersLocked(nil)
		return v, true
	}

	v.ReadAt = time.Now()
	v.Traffic = &live.session
	v.Bearers = sc.bearersLocked(&live)

	return v, true
}

func (s *SMF) liveTrafficLocked(ctx context.Context, sc *SMContext) (sessionTraffic, bool) {
	live := sessionTraffic{
		session:   sc.traffic,
		onDefault: sc.defaultTraffic,
		dedicated: make(map[*dedicatedBearer]Traffic, len(sc.dedicated)),
	}

	for _, b := range sc.dedicated {
		live.dedicated[b] = b.traffic
	}

	if sc.PFCPContext == nil || !sc.PFCPContext.Established {
		return live, true
	}

	reports, err := s.upf.PeekUsage(ctx, sc.PFCPContext.SEID)
	if err != nil {
		logger.From(ctx, logger.SmfLog).Debug("could not read the session's live traffic counters",
			logger.SEID(sc.PFCPContext.SEID), logger.SUPI(sc.Supi.String()), zap.Error(err))

		return sessionTraffic{}, false
	}

	for _, u := range sc.dataPlane().chargedUsage(reports) {
		live.session.add(u.UsageReport)

		if !u.Charging.Dedicated {
			live.onDefault.add(u.UsageReport)
			continue
		}

		if b := sc.dedicatedBySlot(u.Charging.Slot); b != nil {
			t := live.dedicated[b]
			t.add(u.UsageReport)
			live.dedicated[b] = t
		}
	}

	return live, true
}

func (sc *SMContext) bearersLocked(live *sessionTraffic) []BearerView {
	binding, ok := sc.defaultBinding()
	if !ok {
		return nil
	}

	def := BearerView{ID: sc.EBI, Default: true, QCI: binding.QCI, ARP: binding.ARP, Rules: []string{}}
	if sc.Access == Access5G {
		def.ID = sc.PolicyData.QosData.QFI
	}

	if sc.onDefault != nil {
		def.Rules = ruleIDs(sc.onDefault.installed.rules)
	}

	if live != nil {
		def.Traffic = &live.onDefault
	}

	var dedicated []BearerView

	for _, b := range sc.dedicated {
		if b.state != dedicatedActive || !b.enforces() {
			continue
		}

		v := BearerView{ID: b.ebi, QCI: b.installed.binding.QCI, ARP: fullARP(b.installed.binding.ARP), MBR: b.installed.mbr, GBR: b.installed.gbr, Rules: ruleIDs(b.installed.rules)}
		if sc.Access == Access5G {
			v.ID = b.qfi
		}

		if live != nil {
			t := live.dedicated[b]
			v.Traffic = &t
		}

		dedicated = append(dedicated, v)
	}

	slices.SortFunc(dedicated, func(a, b BearerView) int { return cmp.Compare(a.ID, b.ID) })

	return append([]BearerView{def}, dedicated...)
}
