// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package scenarios

import (
	"context"
	"fmt"
	"os"
	"time"
)

const (
	HAHandoverMarker  = "HA_HANDOVER_DONE"
	haHandoverMaxHold = 3 * time.Minute
)

func HAHandoverDone(ctx context.Context, imsi, wantCore, sourceCore, targetCore, ueIP string, targetRANUEID int64) error {
	if sourceCore != wantCore {
		return fmt.Errorf("the UE registered through the core at %q, want the first core %q", sourceCore, wantCore)
	}

	if targetCore != sourceCore {
		return fmt.Errorf("the target radio reached the UE's core at %q, want the core at %q that holds the UE context", targetCore, sourceCore)
	}

	fmt.Printf("%s imsi=%s ue_ip=%s ran_ue_id=%d core=%s\n", HAHandoverMarker, imsi, ueIP, targetRANUEID, sourceCore)

	_ = os.Stdout.Sync()

	select {
	case <-ctx.Done():
	case <-time.After(haHandoverMaxHold):
	}

	return nil
}
