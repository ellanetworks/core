// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package testutil

import (
	"fmt"

	coresctp "github.com/ellanetworks/core/internal/sctp"
	"github.com/ishidawataru/sctp"
)

func ValidateSCTP(info *sctp.SndRcvInfo, expectedPPID uint32, expectedStreamID uint16) error {
	if info == nil {
		return fmt.Errorf("missing SCTP SndRcvInfo")
	}

	if info.PPID != coresctp.PPIDWireOrder(expectedPPID) {
		return fmt.Errorf("ppid=%d want %d (NGAP)", info.PPID, coresctp.PPIDWireOrder(expectedPPID))
	}

	if info.Stream != expectedStreamID {
		return fmt.Errorf("stream=%d want %d (non-UE signalling)", info.Stream, expectedStreamID)
	}

	return nil
}
