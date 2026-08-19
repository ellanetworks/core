// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package ebpf

import "testing"

// TestClearNotifiedForSEID asserts that clearing a SEID removes all of its
// paging entries (any PdrID/QFI) and leaves other sessions' entries intact.
func TestClearNotifiedForSEID(t *testing.T) {
	obj := NewBpfObjects(false, false, false, 0, 0, 0, 0)

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

// Suppression is marked when a page fails and cleared when the UE returns; a
// policy change in between moves the PDR's QFI, and only the current one is
// knowable at the clear.
func TestClearNotifiedIsQFIAgnostic(t *testing.T) {
	obj := NewBpfObjects(false, false, false, 0, 0, 0, 0)

	oldQFI := DataNotification{LocalSEID: 1, PdrID: 2, QFI: 5}
	newQFI := DataNotification{LocalSEID: 1, PdrID: 2, QFI: 9}
	otherPDR := DataNotification{LocalSEID: 1, PdrID: 3, QFI: 5}
	otherSEID := DataNotification{LocalSEID: 2, PdrID: 2, QFI: 5}

	for _, d := range []DataNotification{oldQFI, newQFI, otherPDR, otherSEID} {
		obj.MarkNotified(d)
	}

	obj.ClearNotified(1, 2)

	if obj.IsAlreadyNotified(oldQFI) {
		t.Error("entry marked under the previous QFI survived the clear")
	}

	if obj.IsAlreadyNotified(newQFI) {
		t.Error("entry marked under the current QFI survived the clear")
	}

	if !obj.IsAlreadyNotified(otherPDR) {
		t.Error("another PDR of the same session was cleared")
	}

	if !obj.IsAlreadyNotified(otherSEID) {
		t.Error("another session's entry was cleared")
	}
}
