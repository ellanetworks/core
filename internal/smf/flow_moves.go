// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	smfNgap "github.com/ellanetworks/core/internal/smf/ngap"
	"go.uber.org/zap"
)

func (b *dedicatedBearer) movesTo(access AccessType) bool {
	if b.ebi == 0 || b.state != dedicatedActive || !b.enforces() {
		return false
	}

	if access == Access4G {
		return b.ue != nil && b.pending == nil
	}

	return b.qfi != 0 && b.fiveGSQoS && len(b.installed.rules) > 0
}

func (b *dedicatedBearer) ueView() bearerConfig {
	if b.ue != nil {
		return *b.ue
	}

	return b.installed
}

func (b *dedicatedBearer) tftOn(t bearerTFT, access AccessType) bearerTFT {
	tft := maps.Clone(t)
	for k, f := range tft {
		if access == Access4G || k == blockingUplinkKey {
			f.Precedence = filterPrecedence(b.slot, f.ID)
		} else {
			f.Precedence = b.ruleIdentity[k.rule].precedence
		}

		tft[k] = f
	}

	return tft
}

func (s *SMF) prepareFlowsLocked(ctx context.Context, sc *SMContext, to AccessType) error {
	if sc.Access == to || sc.Tunnel == nil || len(sc.dedicated) == 0 && to != Access4G {
		return nil
	}

	next := sc.Tunnel.dataPlane
	next.TargetUplink = to == Access4G

	if len(sc.dedicated) > 0 {
		if to == Access4G {
			s.interruptFlowProcedureLocked(ctx, sc, false)
		}

		withFlows := next
		withFlows.Bearers = slices.Clone(next.Bearers)

		for i, l := range withFlows.Bearers {
			j := slices.IndexFunc(sc.dedicated, func(b *dedicatedBearer) bool { return b.slot == l.Slot })
			if j < 0 {
				continue
			}

			b := sc.dedicated[j]
			withFlows.Bearers[i].TargetUplink = b.movesTo(to)

			if to == Access5G {
				withFlows.Bearers[i].QFI = b.qfi
			}
		}

		if err := withFlows.checkSDFCapacity(); err != nil {
			logger.From(ctx, logger.SmfLog).Warn("QoS flows not prepared for the target access; they are released at the move",
				logger.SUPI(sc.Supi.String()), zap.Stringer("to", to), zap.Error(err))
		} else {
			next = withFlows
		}
	}

	if err := s.applyDataPlane(ctx, sc, next, sc.policyID()); err != nil {
		return fmt.Errorf("prepare the user plane for %s: %w", to, err)
	}

	return nil
}

func (s *SMF) clearTargetUplinkLocked(ctx context.Context, sc *SMContext) {
	if sc.Tunnel == nil || !sc.Tunnel.TargetUplink && !slices.ContainsFunc(sc.Tunnel.Bearers, func(l bearerLeg) bool { return l.TargetUplink }) {
		return
	}

	next := sc.Tunnel.dataPlane
	next.TargetUplink = false
	next.Bearers = slices.Clone(next.Bearers)

	for i := range next.Bearers {
		next.Bearers[i].TargetUplink = false
	}

	if err := s.applyDataPlane(ctx, sc, next, sc.policyID()); err != nil {
		logger.From(ctx, logger.SmfLog).Warn("target uplink of an abandoned move not removed", logger.SUPI(sc.Supi.String()), zap.Error(err))
	}

	s.reconcileAfter(sc.Ref, 0)
}

func (sc *SMContext) epsDedicatedLocked() []models.DedicatedBearerContext {
	var out []models.DedicatedBearerContext

	for _, b := range sc.dedicated {
		if !sc.heldOnEPSLocked(b) {
			continue
		}

		teid := sc.Tunnel.bearerTEID(b.slot)
		if teid == 0 {
			continue
		}

		v := b.ueView()

		out = append(out, models.DedicatedBearerContext{
			EBI:       b.ebi,
			QCI:       v.binding.QCI,
			ARP:       v.binding.ARP,
			MBR:       v.mbr,
			GBR:       v.gbr,
			Filters:   epsFilters(b.slot, v.tft),
			SGW:       models.FTEID{TEID: teid, Addr: sc.Tunnel.N3IPv4},
			SGWN3IPv6: sc.Tunnel.N3IPv6,
		})
	}

	return out
}

func (sc *SMContext) handoverFlowsTo5GSLocked() ([]smfNgap.QoSFlow, []uint8) {
	var (
		flows []smfNgap.QoSFlow
		ebis  []uint8
	)

	for _, b := range sc.dedicated {
		if !b.movesTo(Access5G) || !sc.Tunnel.targetUplink(b.slot) {
			continue
		}

		flows = append(flows, qosFlow(b, b.installed.binding, b.installed.mbr, b.installed.gbr))
		ebis = append(ebis, b.ebi)
	}

	return flows, ebis
}

func (sc *SMContext) flowEBIsLocked() []uint8 {
	var out []uint8

	for _, b := range sc.dedicated {
		if b.ebi != 0 {
			out = append(out, b.ebi)
		}
	}

	return out
}

type movingFlows struct {
	to      AccessType
	legs    []bearerLeg
	kept    []*dedicatedBearer
	dropped []*dedicatedBearer
}

func (sc *SMContext) movingFlowsLocked(to AccessType, prepared bool) movingFlows {
	out := movingFlows{to: to}

	for _, b := range sc.dedicated {
		if !b.movesTo(to) || prepared && !sc.Tunnel.targetUplink(b.slot) {
			out.dropped = append(out.dropped, b)
			continue
		}

		v := b.ueView()

		leg := bearerLeg{Slot: b.slot, FiveQI: v.binding.QCI, Rules: sc.legRulesLocked(b, ruleSet{b.tftOn(v.tft, to), v.rules})}
		if to == Access5G {
			leg.QFI = b.qfi
		}

		out.kept = append(out.kept, b)
		out.legs = append(out.legs, leg)
	}

	return out
}

func (s *SMF) adoptMovedFlowsLocked(ctx context.Context, sc *SMContext, f movingFlows, reconcile bool) {
	for _, b := range f.kept {
		if f.to == Access4G {
			b.installed = *b.ueView().clone()
		}

		b.installed.tft = b.tftOn(b.installed.tft, f.to)
		b.ue, b.ran = nil, nil

		if f.to == Access5G {
			b.ue = b.installed.clone()
		}

		if m := b.pending; m != nil {
			b.pending = nil
			b.ue = m.clone()
			b.ue.tft = b.tftOn(m.tft, f.to)
		}

		b.fiveGSQoS = true
		b.refused = nil
		b.deferredSince = time.Time{}
		b.pruneRuleIdentities()
	}

	var (
		lost []PCCRule
		ebis []uint8
	)

	for _, b := range f.dropped {
		if b.state == dedicatedActive {
			lost = append(lost, b.installed.rules...)
		}

		if b.ebi != 0 {
			ebis = append(ebis, b.ebi)
		}
	}

	if f.to == Access5G && len(ebis) > 0 {
		s.amf.ReleaseEPSBearerIdentities(sc.Supi, sc.PDUSessionID, sc.Ref, ebis)
	}

	sc.dedicated = slices.DeleteFunc(sc.dedicated, func(b *dedicatedBearer) bool { return slices.Contains(f.dropped, b) })

	if len(f.dropped) > 0 {
		logger.From(ctx, logger.SmfLog).Info("QoS flows that could not move released",
			logger.SUPI(sc.Supi.String()), zap.Stringer("to", f.to), zap.Int("flows", len(f.dropped)), zap.Int("lost_rules", len(lost)))
	}

	s.reportLostLocked(sc, lost)

	if reconcile {
		s.reconcileAfter(sc.Ref, 0)
	}
}

func (t *UPTunnel) targetUplink(slot uint8) bool {
	return slices.ContainsFunc(t.Bearers, func(l bearerLeg) bool { return l.Slot == slot && l.TargetUplink })
}

func (s *SMF) abandonTransfer(ctx context.Context, sc *SMContext, access AccessType) {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if sc.pending != nil && sc.pending.to == access {
		sc.abandonPendingLocked()
	}

	s.clearTargetUplinkLocked(ctx, sc)
	s.reconcileAfter(sc.Ref, 0)
}

func (sc *SMContext) heldOnEPSLocked(b *dedicatedBearer) bool {
	if sc.Access == Access4G {
		return b.ebi != 0 && b.state == dedicatedActive
	}

	return b.movesTo(Access4G) && sc.Tunnel.targetUplink(b.slot)
}

func (s *SMF) HandoverAdmittedFlowEBIs(ref string) []uint8 {
	sc := s.GetSession(ref)
	if sc == nil {
		return nil
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if sc.handoverAdmitted == nil {
		return nil
	}

	var out []uint8

	for _, b := range sc.dedicated {
		if b.movesTo(Access5G) && slices.Contains(*sc.handoverAdmitted, b.qfi) && sc.Tunnel.targetUplink(b.slot) {
			out = append(out, b.ebi)
		}
	}

	return out
}

func (s *SMF) ReleaseInactiveEPSBearers(ctx context.Context, ref string, ebis []uint8) {
	sc := s.GetSession(ref)
	if sc == nil {
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	var (
		lost []PCCRule
		gone []*dedicatedBearer
	)

	for _, b := range sc.dedicated {
		if b.ebi == 0 || !slices.Contains(ebis, b.ebi) {
			continue
		}

		b.ue = nil
		b.ebi = 0

		if b.ran != nil {
			lost = append(lost, s.loseFlowLocked(ctx, sc, b)...)
			continue
		}

		if b.state == dedicatedActive {
			lost = append(lost, b.installed.rules...)
		}

		gone = append(gone, b)
	}

	if len(lost)+len(gone) == 0 {
		return
	}

	logger.From(ctx, logger.SmfLog).Info("released the QoS flows of EPS bearers the UE deleted in EPS (TS 23.502 §4.11.1.3.3 step 14)",
		logger.SUPI(sc.Supi.String()), logger.PDUSessionID(sc.PDUSessionID), zap.Uint8s("ebis", ebis))

	if err := s.removeDedicatedLocked(ctx, sc, gone...); err != nil {
		logger.From(ctx, logger.SmfLog).Warn("QoS flow user plane not removed", logger.SUPI(sc.Supi.String()), zap.Error(err))
	}

	s.reportLostRulesLocked(sc, lost)
}
