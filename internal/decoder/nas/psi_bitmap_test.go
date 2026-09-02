// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"

	"github.com/ellanetworks/core/nas/fgs"
)

func bitmapWith(set ...int) *fgs.PSIBitmap {
	b := &fgs.PSIBitmap{}
	for _, i := range set {
		b.PSI[i] = true
	}

	return b
}

// TS 24.501 §9.11.3.44 and §9.11.3.42 both make PSI(0) spare, so no session 0
// may be reported.
func TestPSIBitmapsSkipTheSpareIndex(t *testing.T) {
	status := decodePDUSessionStatus(bitmapWith(1))
	if len(status) != 15 || status[0].PDUSessionID != 1 {
		t.Fatalf("status listing starts at %d with %d entries, want 1 and 15", status[0].PDUSessionID, len(status))
	}

	result := decodePDUSessionReactivationResult(bitmapWith(1))
	if len(result) != 15 || result[0].PDUSessionID != 1 {
		t.Fatalf("result listing starts at %d with %d entries, want 1 and 15", result[0].PDUSessionID, len(result))
	}
}

// The two bitmaps share a shape but not a polarity: a set bit means "not
// inactive" for the status and "establishment was not successful" for the
// reactivation result (TS 24.501 §9.11.3.42).
func TestReactivationResultReportsFailureNotActivity(t *testing.T) {
	set := bitmapWith(5)

	status := decodePDUSessionStatus(set)
	if !status[4].Active {
		t.Error("a set status bit should report the session as not inactive")
	}

	result := decodePDUSessionReactivationResult(set)
	if !result[4].EstablishmentFailed {
		t.Error("a set reactivation bit should report establishment as failed")
	}

	if result[3].EstablishmentFailed {
		t.Error("a clear reactivation bit should not report a failure")
	}
}
