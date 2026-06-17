// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package models

type PreemptionCapability string

const (
	PreemptionCapabilityNotPreempt PreemptionCapability = "NOT_PREEMPT"
	PreemptionCapabilityMayPreempt PreemptionCapability = "MAY_PREEMPT"
)
