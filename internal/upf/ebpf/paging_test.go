// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package ebpf

import "testing"

// TestClearNotifiedForSEID asserts that clearing a SEID removes all of its
// paging entries (any PdrID/QFI) and leaves other sessions' entries intact.
func TestClearNotifiedForSEID(t *testing.T) {
	obj := NewBpfObjects(false, false, 0, 0, 0, 0)

	a1 := DataNotification{LocalSEID: 1, PdrID: 2, QFI: 5}
	a2 := DataNotification{LocalSEID: 1, PdrID: 3, QFI: 9}
	other := DataNotification{LocalSEID: 2, PdrID: 2, QFI: 5}

	obj.MarkNotified(a1)
	obj.MarkNotified(a2)
	obj.MarkNotified(other)

	obj.ClearNotifiedForSEID(1)

	if obj.IsAlreadyNotified(a1) || obj.IsAlreadyNotified(a2) {
		t.Fatal("entries for SEID 1 should be cleared")
	}

	if !obj.IsAlreadyNotified(other) {
		t.Fatal("entry for SEID 2 should remain")
	}
}
