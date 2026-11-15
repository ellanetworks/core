// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ap

import "fmt"

type Criticality uint8

const (
	CriticalityReject Criticality = iota
	CriticalityIgnore
	CriticalityNotify
)

const criticalityCount = 3

var criticalityNames = [criticalityCount]string{"reject", "ignore", "notify"}

func (c Criticality) String() string {
	if int(c) < len(criticalityNames) {
		return criticalityNames[c]
	}

	return fmt.Sprintf("Criticality(%d)", uint8(c))
}

type Presence uint8

const (
	PresenceOptional Presence = iota
	PresenceConditional
	PresenceMandatory
)

func (c Criticality) Name() string {
	if int(c) < len(criticalityNames) {
		return criticalityNames[c]
	}

	return ""
}
