// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package nas

import (
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

// BuildGSM5GSMStatus builds a 5GSM STATUS message (TS 24.501 clause 8.3.13)
// reporting an erroneous condition on a PDU session, echoing the PTI of the
// triggering message.
func BuildGSM5GSMStatus(pduSessionID uint8, pti uint8, cause uint8) ([]byte, error) {
	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypeStatus5GSM)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.Status5GSM = nasMessage.NewStatus5GSM(0x0)
	m.Status5GSM.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.Status5GSM.SetPDUSessionID(pduSessionID)
	m.Status5GSM.SetPTI(pti)
	m.Status5GSM.SetMessageType(nas.MsgTypeStatus5GSM)
	m.Status5GSM.SetCauseValue(cause)

	return m.PlainNasEncode()
}
