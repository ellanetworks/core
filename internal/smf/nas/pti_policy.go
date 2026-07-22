// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "github.com/ellanetworks/core/nas/fgs"

// PTI values (TS 24.501 §9.6, TS 24.007 §11.2.3.1a).
const (
	ptiUnassigned uint8 = 0x00
	ptiReserved   uint8 = 0xff
)

// PTIVerdict is the action the SMF must take for an inbound 5GSM message after
// applying the PTI-handling rules of TS 24.501 §7.3.1.
type PTIVerdict int

const (
	// PTIProcess: the PTI is acceptable; handle the message normally.
	PTIProcess PTIVerdict = iota
	// PTIIgnore: discard the message without responding (§7.3.1 d).
	PTIIgnore
	// PTIRespondStatus: answer with a 5GSM STATUS carrying the returned cause.
	PTIRespondStatus
)

// PolicePTI applies the network PTI-handling rules of TS 24.501 §7.3.1 to an
// inbound 5GSM message identified by its message type and PTI. ptiInUse reports
// whether the SMF has an outstanding procedure for that PTI on the PDU session;
// it is consulted only for the messages that complete or reject a procedure.
// When the verdict is PTIRespondStatus, the returned cause is the 5GSM cause for
// the STATUS message.
func PolicePTI(msgType fgs.SMMessageType, pti uint8, ptiInUse func(uint8) bool) (PTIVerdict, uint8) {
	// §7.3.1 d): a reserved PTI value is ignored regardless of message type.
	if pti == ptiReserved {
		return PTIIgnore, 0
	}

	switch msgType {
	// §7.3.1 c): a request carrying an unassigned PTI is invalid.
	case fgs.MsgPDUSessionEstablishmentRequest,
		fgs.MsgPDUSessionModificationRequest,
		fgs.MsgPDUSessionReleaseRequest:
		if pti == ptiUnassigned {
			return PTIRespondStatus, fgs.Cause5GSMInvalidPTIValue
		}

	// §7.3.1 a): a completion or command-reject whose PTI matches no procedure
	// in use is a mismatch.
	case fgs.MsgPDUSessionModificationComplete,
		fgs.MsgPDUSessionReleaseComplete,
		fgs.MsgPDUSessionModificationCmdReject:
		if !ptiInUse(pti) {
			return PTIRespondStatus, fgs.Cause5GSMPTIMismatch
		}

	// §7.3.1 b): an authentication complete must carry an unassigned PTI.
	case fgs.MsgPDUSessionAuthenticationComplete:
		if pti != ptiUnassigned {
			return PTIRespondStatus, fgs.Cause5GSMInvalidPTIValue
		}
	}

	return PTIProcess, 0
}
