// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "github.com/ellanetworks/core/nas/fgs"

// BuildGSMPDUSessionEstablishmentReject builds a PDU SESSION ESTABLISHMENT
// REJECT (TS 24.501 §8.3.3) echoing the PTI of the request it rejects.
func BuildGSMPDUSessionEstablishmentReject(pduSessionID, pti, cause uint8) ([]byte, error) {
	return (&fgs.PDUSessionEstablishmentReject{PDUSessionID: pduSessionID, PTI: pti, Cause: cause}).Marshal()
}
