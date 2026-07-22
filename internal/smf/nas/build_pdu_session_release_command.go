// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "github.com/ellanetworks/core/nas/fgs"

// BuildGSMPDUSessionReleaseCommand builds a PDU SESSION RELEASE COMMAND
// (TS 24.501 §8.3.14). pti is the UE-allocated value for a UE-requested release
// or 0 ("no procedure transaction identity assigned") for a network-requested
// release; cause is the 5GSM release cause.
func BuildGSMPDUSessionReleaseCommand(pduSessionID, pti, cause uint8) ([]byte, error) {
	return (&fgs.PDUSessionReleaseCommand{PDUSessionID: pduSessionID, PTI: pti, Cause: cause}).Marshal()
}
