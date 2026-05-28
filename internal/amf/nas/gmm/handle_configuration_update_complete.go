package gmm

import (
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
)

func handleConfigurationUpdateComplete(amfInstance *amf.AMF, ue *amf.AmfUe, macFailed bool) error {
	if state := ue.GetState(); state != amf.Registered {
		return fmt.Errorf("state mismatch: receive Configuration Update Complete message in state %s", state)
	}

	if macFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	if conn := ue.NasConn(); conn != nil && conn.T3555 != nil {
		conn.T3555.Stop()
		conn.T3555 = nil
	}

	amfInstance.FreeOldGuti(ue)

	return nil
}
