// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import fgs "github.com/ellanetworks/core/nas/fgs"

// BuildGSMPDUSessionModificationReject builds a PDU SESSION MODIFICATION REJECT
// (TS 24.501 §8.3.8) echoing the PTI of the request it rejects.
func BuildGSMPDUSessionModificationReject(pduSessionID, pti, cause uint8) ([]byte, error) {
	return (&fgs.PDUSessionModificationReject{PDUSessionID: pduSessionID, PTI: pti, Cause: cause}).Marshal()
}
