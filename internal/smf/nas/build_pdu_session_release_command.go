// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package nas

import (
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

// BuildGSMPDUSessionReleaseCommand builds a PDU Session Release Command
// (TS 24.501 clause 8.3.14). pti is the UE-allocated value for a UE-requested
// release or 0 ("no procedure transaction identity assigned") for a
// network-requested release; cause is the 5GSM release cause.
func BuildGSMPDUSessionReleaseCommand(pduSessionID, pti, cause uint8) ([]byte, error) {
	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionReleaseCommand)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionReleaseCommand = nasMessage.NewPDUSessionReleaseCommand(0x0)
	m.PDUSessionReleaseCommand.SetMessageType(nas.MsgTypePDUSessionReleaseCommand)
	m.PDUSessionReleaseCommand.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionReleaseCommand.SetPDUSessionID(pduSessionID)
	m.PDUSessionReleaseCommand.SetPTI(pti)
	m.PDUSessionReleaseCommand.SetCauseValue(cause)

	return m.PlainNasEncode()
}
