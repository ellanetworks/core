// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import "fmt"

func forwardDownlinkNAS(gnb *GnodeB, amfUEID, ranUEID int64, message []byte, perSession [][]byte) error {
	if message == nil && len(perSession) == 0 {
		return nil
	}

	ue, err := gnb.LoadUE(ranUEID)
	if err != nil {
		return fmt.Errorf("cannot find UE to deliver a downlink NAS message to: %w", err)
	}

	if message != nil {
		if err := ue.SendDownlinkNAS(message, amfUEID, ranUEID); err != nil {
			return fmt.Errorf("could not deliver NAS-PDU to UE: %w", err)
		}
	}

	for _, nasPDU := range perSession {
		if err := ue.SendDownlinkNAS(nasPDU, amfUEID, ranUEID); err != nil {
			return fmt.Errorf("could not deliver PDU session NAS-PDU to UE: %w", err)
		}
	}

	return nil
}
