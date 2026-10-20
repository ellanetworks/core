// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/models"
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

func (s *SMF) TakeUsageForTest(ctx context.Context, sc *SMContext) { s.takeUsageLocked(ctx, sc) }

func (s *SMF) ApplyUsageForTest(ctx context.Context, sc *SMContext, report models.UsageReport, now time.Time) {
	s.applyUsage(ctx, sc, report, now)
}
