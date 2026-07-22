// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas_test

import (
	"testing"

	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func TestBuildGSM5GSMStatus_RoundTrip(t *testing.T) {
	const (
		pduSessionID uint8 = 5
		pti          uint8 = 9
	)

	encoded, err := smfNas.BuildGSM5GSMStatus(pduSessionID, pti, fgs.GSMCausePTIMismatch)
	if err != nil {
		t.Fatalf("BuildGSM5GSMStatus failed: %v", err)
	}

	m, err := fgs.ParseGSMStatus(encoded)
	if err != nil {
		t.Fatalf("ParseGSMStatus failed: %v", err)
	}

	if m.PDUSessionID != pduSessionID {
		t.Errorf("PDU session ID = %d, want %d", m.PDUSessionID, pduSessionID)
	}

	if m.PTI != pti {
		t.Errorf("PTI = %d, want %d", m.PTI, pti)
	}

	if m.Cause != fgs.GSMCausePTIMismatch {
		t.Errorf("cause = %d, want %d (#47 PTI mismatch)", m.Cause, fgs.GSMCausePTIMismatch)
	}
}
