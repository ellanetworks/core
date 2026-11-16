// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "github.com/ellanetworks/core/per/ap"

func diagnoseItems[T any](d *ap.Diagnosis[ProtocolIEID], items []T) bool {
	notComprehended := false

	for i := range items {
		if diagnoseOptional(d, &items[i]) {
			notComprehended = true
		}
	}

	return notComprehended
}

func diagnoseOptional[T any](d *ap.Diagnosis[ProtocolIEID], v *T) bool {
	if v == nil {
		return false
	}

	dv, ok := any(v).(ap.Diagnosable[ProtocolIEID])

	return ok && dv.Diagnose(d)
}
