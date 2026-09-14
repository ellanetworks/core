// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
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

	return smContext.Tunnel.ForwardingTEID
}
