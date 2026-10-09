// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"maps"
	"slices"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	smfNgap "github.com/ellanetworks/core/internal/smf/ngap"
	"go.uber.org/zap"
)

func (b *dedicatedBearer) movesTo(access AccessType) bool {
	if b.ebi == 0 || b.state != dedicatedActive || b.withdraw {
		return false
	}

	if access == Access4G {
		return b.ueHolds && b.modification == nil
	}

	return b.qfi != 0 && b.fiveGSQoS && len(b.rules) > 0
}

func (b *dedicatedBearer) ueView() pendingModification {
	if b.realignUE != nil {
		return *b.realignUE
	}

	return pendingModification{binding: b.binding, rules: b.rules, tft: b.tft, mbr: b.mbr, gbr: b.gbr}
}

func (b *dedicatedBearer) tftOn(t bearerTFT, access AccessType) bearerTFT {
	tft := maps.Clone(t)
	for k, f := range tft {
		if access == Access4G {
			f.Precedence = filterPrecedence(b.slot, f.ID)
		} else {
			f.Precedence = b.precedence[k.rule]
		}

		tft[k] = f
	}

	return tft
}

func (s *SMF) prepareFlowsLocked(ctx context.Context, sc *SMContext, to AccessType) {
	if sc.Access == to || sc.Tunnel == nil || len(sc.dedicated) == 0 {
		return
	}

	if to == Access4G {
		s.interruptFlowProcedureLocked(ctx, sc)
	}

	next := sc.Tunnel.dataPlane
	next.Bearers = slices.Clone(next.Bearers)

	for i, l := range next.Bearers {
		j := slices.IndexFunc(sc.dedicated, func(b *dedicatedBearer) bool { return b.slot == l.Slot })
		if j < 0 {
			continue
		}

		b := sc.dedicated[j]
		next.Bearers[i].TargetUplink = b.movesTo(to)

		if to == Access5G {
			next.Bearers[i].QFI = b.qfi
		}
	}

	if err := next.checkSDFCapacity(); err != nil {
		logger.From(ctx, logger.SmfLog).Warn("QoS flows not prepared for the target access; they are released at the move",
			logger.SUPI(sc.Supi.String()), zap.Stringer("to", to), zap.Error(err))

		return
	}

	if err := s.applyDataPlane(ctx, sc, next, sc.policyID()); err != nil {
		logger.From(ctx, logger.SmfLog).Warn("QoS flows not prepared for the target access; they are released at the move",
			logger.SUPI(sc.Supi.String()), zap.Stringer("to", to), zap.Error(err))
	}
}

func (s *SMF) clearTargetUplinkLocked(ctx context.Context, sc *SMContext) {
	if sc.Tunnel == nil || !slices.ContainsFunc(sc.Tunnel.Bearers, func(l bearerLeg) bool { return l.TargetUplink }) {
		return
	}

	next := sc.Tunnel.dataPlane
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
		mbr, gbr := bearerRates(v.rules)

		out = append(out, models.DedicatedBearerContext{
			EBI:       b.ebi,
			QCI:       v.binding.QCI,
			ARP:       v.binding.ARP,
			MBR:       mbr,
			GBR:       gbr,
			Filters:   epsFilters(b.slot, v.tft),
			SGW:       models.FTEID{TEID: teid, Addr: sc.Tunnel.N3IPv4},
			SGWN3IPv6: sc.Tunnel.N3IPv6,
		})
	}

	return out
}

func (sc *SMContext) handoverFlowsTo5GSLocked() ([]smfNgap.GBRQoSFlow, []uint8) {
	var (
		flows []smfNgap.GBRQoSFlow
		ebis  []uint8
	)

	for _, b := range sc.dedicated {
		if !b.movesTo(Access5G) || !sc.Tunnel.targetUplink(b.slot) {
			continue
		}

		flows = append(flows, gbrFlow(b, b.binding, b.mbr, b.gbr))
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

		leg := bearerLeg{Slot: b.slot, Rules: sc.legRulesLocked(b, ruleSet{b.tftOn(v.tft, to), v.rules})}
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
			v := b.ueView()
			b.binding, b.rules, b.tft, b.mbr, b.gbr = v.binding, v.rules, v.tft, v.mbr, v.gbr
		}

		b.tft = b.tftOn(b.tft, f.to)
		b.admitted, b.ueHolds, b.realignRAN, b.realignUE = false, f.to == Access5G, false, nil

		if m := b.modification; m != nil {
			b.modification = nil
			b.realignUE = &pendingModification{binding: m.binding, rules: m.rules, tft: b.tftOn(m.tft, f.to), mbr: m.mbr, gbr: m.gbr}
		}

		b.fiveGSQoS = true
		b.rejected = rejectedStep{}
		b.enb = models.FTEID{}
		b.modifyWaitingSince = time.Time{}
	}

	var (
		lost []PCCRule
		ebis []uint8
	)

	for _, b := range f.dropped {
		if b.state == dedicatedActive && !b.withdraw {
			lost = append(lost, b.rules...)
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

		if b.state == dedicatedActive && !b.withdraw {
			lost = append(lost, b.rules...)
		}

		b.ueHolds = false
		b.ebi = 0

		if b.admitted {
			s.loseFlowLocked(ctx, sc, b)
			continue
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
