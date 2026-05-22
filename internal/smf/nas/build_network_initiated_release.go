// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package nas

import (
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

// BuildNetworkInitiatedPDUSessionReleaseCommand constructs a NAS PDU Session
// Release Command for a network-initiated release (TS 24.501 clause 8.3.12).
//
// The cause value indicates why the network is releasing the session.
// Common values:
//   - nasMessage.Cause5GSMReactivationRequested (0x27 / #39):
//     Tells the UE to immediately re-establish the PDU session.
//     Used when the subscriber's slice assignment has changed.
func BuildNetworkInitiatedPDUSessionReleaseCommand(pduSessionID uint8, cause uint8) ([]byte, error) {
	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionReleaseCommand)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionReleaseCommand = nasMessage.NewPDUSessionReleaseCommand(0x0)
	m.PDUSessionReleaseCommand.SetMessageType(nas.MsgTypePDUSessionReleaseCommand)
	m.PDUSessionReleaseCommand.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionReleaseCommand.SetPDUSessionID(pduSessionID)
	m.PDUSessionReleaseCommand.SetPTI(0) // Network-initiated: no PTI
	m.PDUSessionReleaseCommand.SetCauseValue(cause)

	return m.PlainNasEncode()
}
