package gmm

import (
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
)

func handleConfigurationUpdateComplete(amfInstance *amf.AMF, ue *amf.AmfUe) error {
	if state := ue.GetState(); state != amf.Registered {
		return fmt.Errorf("state mismatch: receive Configuration Update Complete message in state %s", state)
	}

	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	if ue.T3555 != nil {
		ue.T3555.Stop()
		ue.T3555 = nil // clear the timer
	}

	amfInstance.FreeOldGuti(ue)

	return nil
}
