// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package models

type Arp struct {
	// nullable true shall not be used for this attribute
	PriorityLevel int32
	PreemptCap    PreemptionCapability
	PreemptVuln   PreemptionVulnerability
}
