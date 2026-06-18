// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

type PreemptionCapability string

const (
	PreemptionCapabilityNotPreempt PreemptionCapability = "NOT_PREEMPT"
	PreemptionCapabilityMayPreempt PreemptionCapability = "MAY_PREEMPT"
)
