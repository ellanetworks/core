// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package nas

import (
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

// BuildGSMPDUSessionModificationReject builds a PDU Session Modification Reject
// (TS 24.501 clause 8.3.8) echoing the PTI of the request it rejects.
func BuildGSMPDUSessionModificationReject(pduSessionID uint8, pti uint8, cause uint8) ([]byte, error) {
	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionModificationReject)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionModificationReject = nasMessage.NewPDUSessionModificationReject(0x0)
	m.PDUSessionModificationReject.SetMessageType(nas.MsgTypePDUSessionModificationReject)
	m.PDUSessionModificationReject.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionModificationReject.SetPDUSessionID(pduSessionID)
	m.PDUSessionModificationReject.SetCauseValue(cause)
	m.PDUSessionModificationReject.SetPTI(pti)

	return m.PlainNasEncode()
}
