// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// NotificationResponse is the NOTIFICATION RESPONSE message (TS 24.501 §8.2.24):
// an optional PDU session status listing the UE's active PDU sessions.
type NotificationResponse struct {
	PDUSessionStatus []byte // optional (IEI 0x50)
}

var notificationResponseIEs = []common.OptionalIE{
	{IEI: ieiPDUSessionStatus, Format: common.IETLV},
}

// ParseNotificationResponse decodes a NOTIFICATION RESPONSE message.
func ParseNotificationResponse(b []byte) (*NotificationResponse, error) {
	r := common.NewReader(b)

	if err := readGMMHeader(r, MsgNotificationResponse); err != nil {
		return nil, err
	}

	out := &NotificationResponse{}

	if _, err := common.WalkOptionalIEs(r, notificationResponseIEs, func(iei uint8, value []byte) error {
		if iei == ieiPDUSessionStatus {
			out.PDUSessionStatus = value
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return out, nil
}
