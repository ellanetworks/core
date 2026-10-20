// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/models"
	dto "github.com/prometheus/client_model/go"
)

func (smContext *SMContext) TransferPendingForTest() bool { return smContext.pending != nil }

func (smContext *SMContext) HandoverTargetANForTest() *AnchorBinding {
	return smContext.handoverTargetAN
}

func SetTransferSupervisionForTest(d time.Duration) func() {
	prev := transferSupervision
	transferSupervision = d

	return func() { transferSupervision = prev }
}

func SetIndirectForwardingDurationForTest(d time.Duration) func() {
	prev := indirectForwardingDuration
	indirectForwardingDuration = d

	return func() { indirectForwardingDuration = prev }
}

func (smContext *SMContext) N2ReleasedForTest() bool {
	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	return smContext.n2Released
}

func (smContext *SMContext) ForwardingTEIDForTest() uint32 {
	if smContext.Tunnel == nil {
		return 0
	}

	return smContext.Tunnel.forwardingTEID()
}

func (s *SMF) AssociateForTest(ctx context.Context, sc *SMContext) error {
	subscribed := SubscribedQoS{Var5qi: sc.PolicyData.QosData.Var5qi, SessionAMBR: sc.PolicyData.Ambr}
	if sc.PolicyData.QosData.Arp != nil {
		subscribed.Arp = sc.PolicyData.QosData.Arp.PriorityLevel
	}

	d, err := s.createAssociation(ctx, sc, PolicyContext{Supi: sc.Supi, PDUSessionID: sc.PDUSessionID, Dnn: sc.Dnn, Snssai: *sc.Snssai, Access: sc.Access, Subscribed: subscribed})
	if err != nil {
		return err
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	sc.policyDecision = d
	sc.subscribedQoS = subscribed

	return nil
}

func PolicyRevisionForTest(sc *SMContext) uint64 {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if sc.policyDecision == nil {
		return 0
	}

	return sc.policyDecision.Revision
}

func (s *SMF) SetDedicatedAwaitLimitForTest(d time.Duration) { s.dedicatedAwaitLimit = d }

func (s *SMF) CollectUsageForTest(ctx context.Context) { s.collectUsage(ctx) }

func (s *SMF) FlushUsageForTest(ctx context.Context) { s.flushUsage(ctx) }

func (s *SMF) TakeUsageForTest(ctx context.Context, sc *SMContext, trigger models.UsageReportTrigger) {
	s.takeUsageLocked(ctx, sc, trigger)
}

func (s *SMF) DeliverUsageForTest(ctx context.Context, sc *SMContext, reports ...models.UsageReport) {
	s.deliverUsage(ctx, sc, sc.dataPlane().chargedUsage(reports))
}

type ChargedUsage struct {
	Dedicated   bool
	Slot        uint8
	RatingGroup uint32
	Report      models.UsageReport
}

func (s *SMF) ObserveUsageForTest() func() []ChargedUsage {
	var (
		mu   sync.Mutex
		seen []ChargedUsage
	)

	s.usageConsumers = append(s.usageConsumers, func(_ context.Context, _ *SMContext, usage []urrUsage) {
		mu.Lock()
		defer mu.Unlock()

		for _, u := range usage {
			seen = append(seen, ChargedUsage{Dedicated: u.Charging.Dedicated, Slot: u.Charging.Slot, RatingGroup: u.Charging.RatingGroup, Report: u.UsageReport})
		}
	})

	return func() []ChargedUsage {
		mu.Lock()
		defer mu.Unlock()

		return slices.Clone(seen)
	}
}

func (smContext *SMContext) ReleaseForTest() models.SessionReleaseRequest {
	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if smContext.release == nil {
		return models.SessionReleaseRequest{}
	}

	return *smContext.release
}

func SessionReleasesForTest(rat string, reason models.SessionReleaseReason) float64 {
	var m dto.Metric
	if err := sessionReleases.WithLabelValues(rat, reason.String()).Write(&m); err != nil {
		return -1
	}

	return m.GetCounter().GetValue()
}

func (s *SMF) RefreshTimeZoneForTest(now time.Time) { s.refreshTimeZone(now) }
