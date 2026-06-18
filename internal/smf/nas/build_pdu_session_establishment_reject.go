// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

func BuildGSMPDUSessionEstablishmentReject(pduSessionID uint8, pti uint8, cause uint8) ([]byte, error) {
	m := nas.NewMessage()
	m.GsmMessage = nas.NewGsmMessage()
	m.GsmHeader.SetMessageType(nas.MsgTypePDUSessionEstablishmentReject)
	m.GsmHeader.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionEstablishmentReject = nasMessage.NewPDUSessionEstablishmentReject(0x0)
	m.PDUSessionEstablishmentReject.SetMessageType(nas.MsgTypePDUSessionEstablishmentReject)
	m.PDUSessionEstablishmentReject.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSSessionManagementMessage)
	m.PDUSessionEstablishmentReject.SetPDUSessionID(pduSessionID)
	m.PDUSessionEstablishmentReject.SetCauseValue(cause)
	m.PDUSessionEstablishmentReject.SetPTI(pti)

	return m.PlainNasEncode()
}
