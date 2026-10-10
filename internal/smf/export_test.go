// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"time"
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
	d, err := s.createAssociation(ctx, sc, PolicyContext{Supi: sc.Supi, PDUSessionID: sc.PDUSessionID, Dnn: sc.Dnn, Snssai: *sc.Snssai, Access: sc.Access})
	if err != nil {
		return err
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	sc.policyDecision = d
	sc.subscribedQoS = SubscribedQoS{Var5qi: d.Var5qi, Arp: d.Arp, SessionAMBR: d.SessionAMBR}

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
