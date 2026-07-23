// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/nas/common"
)

// GSMMessageType is a 5GSM message type (TS 24.501 §9.7, table 9.7.2).
type GSMMessageType uint8

const (
	MsgPDUSessionEstablishmentRequest   GSMMessageType = 0xC1
	MsgPDUSessionEstablishmentAccept    GSMMessageType = 0xC2
	MsgPDUSessionEstablishmentReject    GSMMessageType = 0xC3
	MsgPDUSessionAuthenticationComplete GSMMessageType = 0xC6
	MsgPDUSessionModificationRequest    GSMMessageType = 0xC9
	MsgPDUSessionModificationReject     GSMMessageType = 0xCA
	MsgPDUSessionModificationCommand    GSMMessageType = 0xCB
	MsgPDUSessionModificationComplete   GSMMessageType = 0xCC
	MsgPDUSessionModificationCmdReject  GSMMessageType = 0xCD
	MsgPDUSessionReleaseRequest         GSMMessageType = 0xD1
	MsgPDUSessionReleaseCommand         GSMMessageType = 0xD3
	MsgPDUSessionReleaseComplete        GSMMessageType = 0xD4
	MsgGSMStatus                        GSMMessageType = 0xD6
)

// ErrNotGSM reports an extended protocol discriminator other than 5GSM.
var ErrNotGSM = errors.New("nas/fgs: not a 5GSM message")

// PeekGSMMessageType returns the 5GSM message type of a NAS message (the fourth
// octet) without consuming it, after checking the extended protocol
// discriminator.
func PeekGSMMessageType(b []byte) (GSMMessageType, error) {
	r := common.NewReader(b)

	epd, err := r.U8()
	if err != nil {
		return 0, err
	}

	if epd != EPD5GSM {
		return 0, fmt.Errorf("%w (EPD %#x)", ErrNotGSM, epd)
	}

	if _, err := r.U8(); err != nil { // PDU session identity
		return 0, err
	}

	if _, err := r.U8(); err != nil { // procedure transaction identity
		return 0, err
	}

	mt, err := r.U8()
	if err != nil {
		return 0, err
	}

	return GSMMessageType(mt), nil
}

// writeGSMHeader emits the 4-octet 5GSM header: extended protocol discriminator,
// PDU session identity, procedure transaction identity, message type
// (TS 24.501 §9.1.1).
func writeGSMHeader(w *common.Writer, pduSessionID, pti uint8, mt GSMMessageType) {
	w.U8(EPD5GSM)
	w.U8(pduSessionID)
	w.U8(pti)
	w.U8(uint8(mt))
}

func readGSMHeader(r *common.Reader, want GSMMessageType) (pduSessionID, pti uint8, err error) {
	epd, err := r.U8()
	if err != nil {
		return 0, 0, err
	}

	if epd != EPD5GSM {
		return 0, 0, fmt.Errorf("%w (EPD %#x)", ErrNotGSM, epd)
	}

	pduSessionID, err = r.U8()
	if err != nil {
		return 0, 0, err
	}

	pti, err = r.U8()
	if err != nil {
		return 0, 0, err
	}

	mt, err := r.U8()
	if err != nil {
		return 0, 0, err
	}

	if GSMMessageType(mt) != want {
		return 0, 0, fmt.Errorf("%w: got %#x, want %#x", ErrWrongMessageType, mt, uint8(want))
	}

	return pduSessionID, pti, nil
}
