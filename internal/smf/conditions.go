// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"cmp"
	"slices"

	"github.com/ellanetworks/core/internal/models"
)

type TriggerType string

const (
	TriggerQoSChange          TriggerType = "QOS_CHANGE"
	TriggerSessionAMBRChange  TriggerType = "SESSION_AMBR_CHANGE"
	TriggerRATChange          TriggerType = "RAT_CHANGE"
	TriggerServingNodeChange  TriggerType = "SERVING_NODE_CHANGE"
	TriggerUserLocationChange TriggerType = "USER_LOCATION_CHANGE"
	TriggerTAIChange          TriggerType = "TAI_CHANGE"
	TriggerECGIChange         TriggerType = "ECGI_CHANGE"
	TriggerUETimeZoneChange   TriggerType = "UE_TIMEZONE_CHANGE"
	TriggerPLMNChange         TriggerType = "PLMN_CHANGE"
)

func (t TriggerType) fromUEInfo() bool {
	return t != TriggerQoSChange && t != TriggerSessionAMBRChange
}

type FlowQoS struct {
	FiveQI int32
	ARP    models.Arp
	MBR    models.Ambr
	GBR    models.Ambr
}

func (q FlowQoS) Equal(o FlowQoS) bool {
	return q.FiveQI == o.FiveQI && q.ARP == o.ARP && q.MBR.Equal(o.MBR) && q.GBR.Equal(o.GBR)
}

type ChargingFlow struct {
	Key        ChargingKey
	ChargingID uint32
	QoS        FlowQoS
}

func (f ChargingFlow) same(o ChargingFlow) bool {
	return f.Key == o.Key && f.ChargingID == o.ChargingID
}

type ChargingConditions struct {
	DefaultQoS     FlowQoS
	SessionAMBR    models.Ambr
	RATType        models.RATType
	Location       models.UserLocation
	ServingNode    models.ServingNode
	ServingNetwork models.PlmnID
	TimeZone       string
	Flows          []ChargingFlow
}

func (c *ChargingConditions) adoptUEInfo(ue models.SessionUEInfo) {
	c.RATType = ue.RATType
	c.Location = ue.Location
	c.ServingNode = ue.ServingNode
	c.ServingNetwork = ue.ServingNetwork
	c.TimeZone = ue.TimeZone
}

func (sc *SMContext) chargingConditionsLocked() ChargingConditions {
	var c ChargingConditions

	c.adoptUEInfo(sc.UEInfo())

	if p := sc.PolicyData; p != nil {
		c.DefaultQoS.FiveQI = p.QosData.Var5qi
		c.SessionAMBR = p.Ambr

		if p.QosData.Arp != nil {
			c.DefaultQoS.ARP = *p.QosData.Arp
		}
	}

	d := sc.dataPlane()

	if d.DefaultURR != 0 {
		c.Flows = append(c.Flows, ChargingFlow{Key: ChargingKey{RatingGroup: d.RatingGroup}, ChargingID: sc.ChargingID, QoS: c.DefaultQoS})
	}

	for _, leg := range d.Bearers {
		b := sc.dedicatedInSlot(leg.Slot)

		for _, r := range leg.Rules {
			if r.URRID == 0 {
				continue
			}

			f := ChargingFlow{Key: d.ruleCharging(leg, r)}

			if b != nil {
				cfg := b.chargedConfig()
				f.ChargingID = b.chargingID
				f.QoS = FlowQoS{FiveQI: int32(cfg.binding.QCI), ARP: cfg.binding.ARP, MBR: cfg.mbr, GBR: cfg.gbr}
			}

			if !slices.ContainsFunc(c.Flows, f.same) {
				c.Flows = append(c.Flows, f)
			}
		}
	}

	slices.SortFunc(c.Flows, func(a, b ChargingFlow) int {
		return cmp.Or(
			compareBool(a.Key.Dedicated, b.Key.Dedicated),
			cmp.Compare(a.Key.Slot, b.Key.Slot),
			cmp.Compare(a.Key.RatingGroup, b.Key.RatingGroup),
			cmp.Compare(a.ChargingID, b.ChargingID),
		)
	})

	return c
}

func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	default:
		return -1
	}
}

func (sc *SMContext) dedicatedInSlot(slot uint8) *dedicatedBearer {
	for _, b := range sc.dedicated {
		if b.slot == slot {
			return b
		}
	}

	return nil
}

func (b *dedicatedBearer) chargedConfig() bearerConfig {
	if b.enforces() || b.pending == nil {
		return b.installed
	}

	return *b.pending
}

func conditionTriggers(prev, next ChargingConditions) []TriggerType {
	var t []TriggerType

	if !prev.DefaultQoS.Equal(next.DefaultQoS) || flowQoSChanged(prev.Flows, next.Flows) {
		t = append(t, TriggerQoSChange)
	}

	if !prev.SessionAMBR.Equal(next.SessionAMBR) {
		t = append(t, TriggerSessionAMBRChange)
	}

	if prev.RATType != next.RATType {
		t = append(t, TriggerRATChange)
	}

	if !prev.ServingNode.Equal(next.ServingNode) {
		t = append(t, TriggerServingNodeChange)
	}

	if !prev.Location.SameCell(next.Location) {
		t = append(t, TriggerUserLocationChange)

		if !prev.Location.SameTAI(next.Location) {
			t = append(t, TriggerTAIChange)
		}

		if next.Location.EutraLocation != nil && !prev.Location.SameECGI(next.Location) {
			t = append(t, TriggerECGIChange)
		}
	}

	if prev.TimeZone != next.TimeZone {
		t = append(t, TriggerUETimeZoneChange)
	}

	if !prev.ServingNetwork.Equal(next.ServingNetwork) {
		t = append(t, TriggerPLMNChange)
	}

	return t
}

func flowQoSChanged(prev, next []ChargingFlow) bool {
	for _, p := range prev {
		i := slices.IndexFunc(next, p.same)
		if i >= 0 && !next[i].QoS.Equal(p.QoS) {
			return true
		}
	}

	return false
}

func flowChanges(prev, next []ChargingFlow) (started, terminated []ChargingFlow) {
	for _, n := range next {
		if !slices.ContainsFunc(prev, n.same) {
			started = append(started, n)
		}
	}

	for _, p := range prev {
		if !slices.ContainsFunc(next, p.same) {
			terminated = append(terminated, p)
		}
	}

	return started, terminated
}

func lastOfRatingGroup(f ChargingFlow, remaining []ChargingFlow) bool {
	return !slices.ContainsFunc(remaining, func(o ChargingFlow) bool { return o.Key.RatingGroup == f.Key.RatingGroup })
}
