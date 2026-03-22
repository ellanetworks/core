package gmm

import (
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf"
)

func handleConfigurationUpdateComplete(amf *amfContext.AMF, ue *amfContext.AmfUe) error {
	if ue.State != amfContext.Registered {
		return fmt.Errorf("state mismatch: receive Configuration Update Complete message in state %s", ue.State)
	}

	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	if ue.T3555 != nil {
		ue.T3555.Stop()
		ue.T3555 = nil // clear the timer
	}

	amf.FreeOldGuti(ue)

	return nil
}
