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
