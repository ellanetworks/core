// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"fmt"
)

func handleConfigurationUpdateComplete(amfInstance *AMF, ue *UeContext, integrityVerified bool) error {
	if state := ue.GetState(); state != Registered {
		return fmt.Errorf("state mismatch: receive Configuration Update Complete message in state %s", state)
	}

	if !integrityVerified {
		return fmt.Errorf("NAS message integrity check failed")
	}

	if conn := ue.NasConn(); conn != nil && conn.T3555 != nil {
		conn.T3555.Stop()
		conn.T3555 = nil
	}

	amfInstance.FreeOldGuti(ue)

	return nil
}
