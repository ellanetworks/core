package gmm

import (
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
)

func handleConfigurationUpdateComplete(ue *context.AmfUe) error {
	if ue.State != context.Registered {
		return fmt.Errorf("state mismatch: receive Configuration Update Complete message in state %s", ue.State)
	}

	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	if ue.T3555 != nil {
		ue.T3555.Stop()
		ue.T3555 = nil // clear the timer
	}

	ue.FreeOldGuti()

	return nil
}
