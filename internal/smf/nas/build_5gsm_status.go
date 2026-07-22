// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import fgs "github.com/ellanetworks/core/nas/fgs"

// BuildGSM5GSMStatus builds a 5GSM STATUS message (TS 24.501 §8.3.16) reporting
// an erroneous condition on a PDU session, echoing the PTI of the triggering
// message.
func BuildGSM5GSMStatus(pduSessionID, pti, cause uint8) ([]byte, error) {
	return (&fgs.Status5GSM{PDUSessionID: pduSessionID, PTI: pti, Cause: cause}).Marshal()
}
