// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

type PendingUPF map[string]bool

func (pendingUPF PendingUPF) IsEmpty() bool {
	if len(pendingUPF) == 0 {
		return true
	} else {
		return false
	}
}
