// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"maps"
	"slices"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

func (b *dedicatedBearer) movesToEPS() bool {
	return b.ebi != 0 && b.state == dedicatedActive && b.ueHolds && !b.withdraw && b.modification == nil
}

func (b *dedicatedBearer) epsTFT() bearerTFT {
	tft := maps.Clone(b.tft)
	for k, f := range tft {
		f.Precedence = filterPrecedence(b.slot, f.ID)
		tft[k] = f
	}

	return tft
}

func (s *SMF) prepareFlowsForEPSLocked(ctx context.Context, sc *SMContext) {
	if sc.Access != Access5G || len(sc.dedicated) == 0 {
		return
	}

	s.interruptFlowProcedureLocked(ctx, sc)

	next := sc.Tunnel.dataPlane
	next.Bearers = slices.Clone(next.Bearers)

	for i, l := range next.Bearers {
		if j := slices.IndexFunc(sc.dedicated, func(b *dedicatedBearer) bool { return b.slot == l.Slot }); j >= 0 {
			next.Bearers[i].TargetUplink = sc.dedicated[j].movesToEPS()
		}
	}

	if err := next.checkSDFCapacity(); err != nil {
		logger.From(ctx, logger.SmfLog).Warn("QoS flows not prepared for EPS; they are released at the move",
			logger.SUPI(sc.Supi.String()), zap.Error(err))

		return
	}

	if err := s.applyDataPlane(ctx, sc, next, sc.policyID()); err != nil {
		logger.From(ctx, logger.SmfLog).Warn("QoS flows not prepared for EPS; they are released at the move",
			logger.SUPI(sc.Supi.String()), zap.Error(err))
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

		mbr, gbr := bearerRates(b.rules)

		out = append(out, models.DedicatedBearerContext{
			EBI:       b.ebi,
			QCI:       b.binding.QCI,
			ARP:       b.binding.ARP,
			MBR:       mbr,
			GBR:       gbr,
			Filters:   epsFilters(b.slot, b.tft),
			SGW:       models.FTEID{TEID: teid, Addr: sc.Tunnel.N3IPv4},
			SGWN3IPv6: sc.Tunnel.N3IPv6,
		})
	}

	return out
}

type flowsOnEPS struct {
	legs    []bearerLeg
	kept    []*dedicatedBearer
	dropped []*dedicatedBearer
}

func (sc *SMContext) flowsOnEPSLocked(prepared bool) flowsOnEPS {
	var out flowsOnEPS

	for _, b := range sc.dedicated {
		if !b.movesToEPS() || prepared && !sc.Tunnel.targetUplink(b.slot) {
			out.dropped = append(out.dropped, b)
			continue
		}

		out.kept = append(out.kept, b)
		out.legs = append(out.legs, bearerLeg{Slot: b.slot, Rules: sc.legRulesLocked(b, ruleSet{b.epsTFT(), b.rules})})
	}

	return out
}

func (s *SMF) adoptFlowsOnEPSLocked(ctx context.Context, sc *SMContext, f flowsOnEPS) {
	for _, b := range f.kept {
		b.tft = b.epsTFT()
		b.admitted, b.ueHolds, b.realignRAN, b.realignUE = false, false, false, nil
		b.fiveGSQoS = true
		b.rejected = rejectedStep{}
		b.enb = models.FTEID{}
	}

	var lost []PCCRule

	for _, b := range f.dropped {
		if b.state == dedicatedActive && !b.withdraw {
			lost = append(lost, b.rules...)
		}
	}

	sc.dedicated = slices.DeleteFunc(sc.dedicated, func(b *dedicatedBearer) bool { return slices.Contains(f.dropped, b) })

	if len(f.dropped) > 0 {
		logger.From(ctx, logger.SmfLog).Info("QoS flows not moved to EPS released",
			logger.SUPI(sc.Supi.String()), zap.Int("flows", len(f.dropped)), zap.Int("lost_rules", len(lost)))
	}

	s.reportLostRulesLocked(sc, lost)
}

func (t *UPTunnel) targetUplink(slot uint8) bool {
	return slices.ContainsFunc(t.Bearers, func(l bearerLeg) bool { return l.Slot == slot && l.TargetUplink })
}

func (s *SMF) abandonTransferToEPS(ctx context.Context, sc *SMContext) {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if sc.pending != nil && sc.pending.to == Access4G {
		sc.abandonPendingLocked()
	}

	s.clearTargetUplinkLocked(ctx, sc)
}

func (sc *SMContext) heldOnEPSLocked(b *dedicatedBearer) bool {
	if sc.Access == Access4G {
		return b.ebi != 0 && b.state == dedicatedActive
	}

	return b.movesToEPS() && sc.Tunnel.targetUplink(b.slot)
}
