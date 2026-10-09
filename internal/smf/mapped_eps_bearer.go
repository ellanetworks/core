// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"
	"slices"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func epsFilters(slot uint8, tft bearerTFT) []models.SDFFilter {
	filters := tft.list()
	for i := range filters {
		filters[i].Precedence = filterPrecedence(slot, filters[i].ID)
	}

	return filters
}

func mappedEPSQoS(qci uint8, mbr, gbr models.Ambr) (fgs.EPSParameter, error) {
	qos, err := eps.GBREPSQoS(qci, eps.EPSQoSBitRates{
		MaxUplinkKbps:          mbr.Uplink.Kbps(),
		MaxDownlinkKbps:        mbr.Downlink.Kbps(),
		GuaranteedUplinkKbps:   gbr.Uplink.Kbps(),
		GuaranteedDownlinkKbps: gbr.Downlink.Kbps(),
	})
	if err != nil {
		return fgs.EPSParameter{}, fmt.Errorf("mapped EPS QoS: %w", err)
	}

	raw, err := qos.MarshalBinary()
	if err != nil {
		return fgs.EPSParameter{}, fmt.Errorf("mapped EPS QoS: %w", err)
	}

	return fgs.EPSParameter{Identifier: fgs.EPSParameterMappedEPSQoS, Contents: raw}, nil
}

func mappedTFT(t eps.TrafficFlowTemplate) (fgs.EPSParameter, error) {
	raw, err := t.MarshalBinary()
	if err != nil {
		return fgs.EPSParameter{}, fmt.Errorf("mapped traffic flow template: %w", err)
	}

	return fgs.EPSParameter{Identifier: fgs.EPSParameterTrafficFlowTemplate, Contents: raw}, nil
}

func (b *dedicatedBearer) mappedCreate(qci uint8, mbr, gbr models.Ambr, tft bearerTFT) (fgs.MappedEPSBearerContexts, error) {
	if b.ebi == 0 {
		return nil, nil
	}

	qos, err := mappedEPSQoS(qci, mbr, gbr)
	if err != nil {
		return nil, err
	}

	filters, err := mappedTFT(models.EPSTFT(eps.TFTCreate, epsFilters(b.slot, tft)))
	if err != nil {
		return nil, err
	}

	return fgs.MappedEPSBearerContexts{{
		EPSBearerIdentity: b.ebi,
		Operation:         fgs.MappedEPSBearerOpCreate,
		EBit:              true,
		Parameters:        []fgs.EPSParameter{qos, filters},
	}}, nil
}

func (b *dedicatedBearer) mappedModify(qci uint8, mbr, gbr models.Ambr, ratesChanged bool, from, to bearerTFT) (fgs.MappedEPSBearerContexts, error) {
	if b.ebi == 0 {
		return nil, nil
	}

	before := epsFilters(b.slot, from)

	var (
		added, replaced []models.SDFFilter
		deleted         []uint8
	)

	for _, f := range epsFilters(b.slot, to) {
		switch {
		case !slices.ContainsFunc(before, func(o models.SDFFilter) bool { return o.ID == f.ID }):
			added = append(added, f)
		case !slices.Contains(before, f):
			replaced = append(replaced, f)
		}
	}

	for _, f := range before {
		if !slices.ContainsFunc(to.list(), func(n models.SDFFilter) bool { return n.ID == f.ID }) {
			deleted = append(deleted, f.ID)
		}
	}

	var params []fgs.EPSParameter

	if ratesChanged {
		qos, err := mappedEPSQoS(qci, mbr, gbr)
		if err != nil {
			return nil, err
		}

		params = append(params, qos)
	}

	var out fgs.MappedEPSBearerContexts

	for _, step := range []struct {
		op      eps.TFTOperation
		filters []models.SDFFilter
	}{{eps.TFTReplaceFilters, replaced}, {eps.TFTAddFilters, added}} {
		if len(step.filters) == 0 {
			continue
		}

		p, err := mappedTFT(models.EPSTFT(step.op, step.filters))
		if err != nil {
			return nil, err
		}

		out = append(out, fgs.MappedEPSBearerContext{EPSBearerIdentity: b.ebi, Operation: fgs.MappedEPSBearerOpModify, Parameters: append(params, p)})
		params = nil
	}

	if len(params) > 0 {
		out = append(out, fgs.MappedEPSBearerContext{EPSBearerIdentity: b.ebi, Operation: fgs.MappedEPSBearerOpModify, Parameters: params})
	}

	if len(deleted) > 0 {
		p, err := mappedTFT(eps.TrafficFlowTemplate{Operation: eps.TFTDeleteFilters, DeleteIdentifiers: deleted})
		if err != nil {
			return nil, err
		}

		out = append(out, fgs.MappedEPSBearerContext{EPSBearerIdentity: b.ebi, Operation: fgs.MappedEPSBearerOpModify, Parameters: []fgs.EPSParameter{p}})
	}

	return out, nil
}

func (b *dedicatedBearer) mappedDelete() fgs.MappedEPSBearerContexts {
	if b.ebi == 0 {
		return nil
	}

	return fgs.MappedEPSBearerContexts{{EPSBearerIdentity: b.ebi, Operation: fgs.MappedEPSBearerOpDelete}}
}

func (sc *SMContext) interworksWith5GS() bool {
	return sc.PDUSessionID != 0 && sc.Snssai != nil
}

func (sc *SMContext) mappedFiveGSQoSLocked(b *dedicatedBearer, prevRules []PCCRule, prevTFT bearerTFT, rules []PCCRule, tft bearerTFT, describe bool) ([]nas.PCOContainer, error) {
	if !sc.interworksWith5GS() {
		return nil, nil
	}

	if b.qfi == 0 {
		qfi, ok := sc.freeQFI()
		if !ok {
			return nil, fmt.Errorf("no free QFI")
		}

		b.qfi = qfi
	}

	for _, r := range rules {
		if !sc.allocateRuleIdentity(b, r.ID) {
			return nil, fmt.Errorf("no free QoS rule identifier or precedence")
		}
	}

	var flows fgs.QoSFlowDescriptions

	if describe {
		op := fgs.QoSFlowOpModify
		if prevRules == nil {
			op = fgs.QoSFlowOpCreate
		}

		mbr, gbr := bearerRates(rules)

		desc, err := b.qosFlowDescription(b.binding.QCI, op, mbr, gbr)
		if err != nil {
			return nil, err
		}

		flows = append(flows, desc)
	}

	return qosFlowContainers(b.qosRuleOps(prevRules, prevTFT, rules, tft), flows)
}

func qosFlowContainers(rules fgs.QoSRules, flows fgs.QoSFlowDescriptions) ([]nas.PCOContainer, error) {
	var out []nas.PCOContainer

	if len(rules) > 0 {
		raw, err := rules.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("encode the QoS rules: %w", err)
		}

		c, err := nas.NewQoSRulesContainer(raw, false)
		if err != nil {
			return nil, err
		}

		out = append(out, c)
	}

	if len(flows) > 0 {
		raw, err := flows.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("encode the QoS flow descriptions: %w", err)
		}

		c, err := nas.NewQoSFlowDescriptionsContainer(raw, false)
		if err != nil {
			return nil, err
		}

		out = append(out, c)
	}

	return out, nil
}

func (sc *SMContext) requestedMappedDeletions(req *fgs.PDUSessionModificationRequest) ([]*dedicatedBearer, bool) {
	if req.Cause == nil || sc.Access != Access5G || len(req.MappedEPSBearerContexts) == 0 || len(req.RequestedQoSRules)+len(req.RequestedQoSFlows) > 0 {
		return nil, false
	}

	var out []*dedicatedBearer

	for _, c := range req.MappedEPSBearerContexts {
		i := slices.IndexFunc(sc.dedicated, func(b *dedicatedBearer) bool { return b.ebi == c.EPSBearerIdentity })
		if c.Operation != fgs.MappedEPSBearerOpDelete || i < 0 {
			return nil, false
		}

		if !slices.Contains(out, sc.dedicated[i]) {
			out = append(out, sc.dedicated[i])
		}
	}

	return out, true
}

func (s *SMF) acceptMappedDeletionLocked(ctx context.Context, sc *SMContext, deleted []*dedicatedBearer, pti uint8) (*UpdateResult, error) {
	var (
		mapped fgs.MappedEPSBearerContexts
		flows  fgs.QoSFlowDescriptions
		ebis   []uint8
	)

	for _, b := range deleted {
		mapped = append(mapped, b.mappedDelete()...)
		ebis = append(ebis, b.ebi)
		b.ebi = 0

		if !b.ueHolds {
			continue
		}

		desc, err := b.qosFlowDescription(b.binding.QCI, fgs.QoSFlowOpModify, b.mbr, b.gbr)
		if err != nil {
			return nil, fmt.Errorf("QoS flow %d without its EPS bearer: %w", b.qfi, err)
		}

		flows = append(flows, desc)
	}

	s.amf.ReleaseEPSBearerIdentities(sc.Supi, sc.PDUSessionID, sc.Ref, ebis)

	n1, err := smfNas.BuildQoSFlowsModificationCommand(sc.PDUSessionID, pti, nil, flows, mapped)
	if err != nil {
		return nil, fmt.Errorf("build PDU Session Modification Command (N1): %w", err)
	}

	logger.From(ctx, logger.SmfLog).Info("accepted the UE's deletion of mapped EPS bearer contexts; the QoS flows stay in 5GS (TS 24.501 §6.4.2.2)",
		logger.SUPI(sc.Supi.String()), logger.PDUSessionID(sc.PDUSessionID), zap.Uint8s("ebis", ebis))

	sc.MarkPTIInUse(pti)

	supi, pduSessionID := sc.Supi, sc.PDUSessionID

	s.armRetransmit(ctx, sc, s.timerT3591(),
		func(ctx context.Context) error { return s.amf.ModifyN1N2(ctx, supi, pduSessionID, n1, nil) },
		func(ctx context.Context, sc *SMContext) {
			sc.ClearPTIInUse(pti)
			s.reconcileAfter(sc.Ref, 0)
		})

	return &UpdateResult{N1Msg: n1}, nil
}
