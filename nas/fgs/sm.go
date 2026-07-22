// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/nas/common"
)

// SMMessageType is a 5GSM message type (TS 24.501 §9.7, table 9.7.2).
type SMMessageType uint8

const (
	MsgPDUSessionEstablishmentRequest   SMMessageType = 0xC1
	MsgPDUSessionEstablishmentAccept    SMMessageType = 0xC2
	MsgPDUSessionEstablishmentReject    SMMessageType = 0xC3
	MsgPDUSessionAuthenticationComplete SMMessageType = 0xC6
	MsgPDUSessionModificationRequest    SMMessageType = 0xC9
	MsgPDUSessionModificationReject     SMMessageType = 0xCA
	MsgPDUSessionModificationCommand    SMMessageType = 0xCB
	MsgPDUSessionModificationComplete   SMMessageType = 0xCC
	MsgPDUSessionModificationCmdReject  SMMessageType = 0xCD
	MsgPDUSessionReleaseRequest         SMMessageType = 0xD1
	MsgPDUSessionReleaseCommand         SMMessageType = 0xD3
	MsgPDUSessionReleaseComplete        SMMessageType = 0xD4
	Msg5GSMStatus                       SMMessageType = 0xD6
)

// ErrNot5GSM reports an extended protocol discriminator other than 5GSM.
var ErrNot5GSM = errors.New("nas/fgs: not a 5GSM message")

// PeekSMMessageType returns the 5GSM message type of a NAS message (the fourth
// octet) without consuming it, after checking the extended protocol
// discriminator.
func PeekSMMessageType(b []byte) (SMMessageType, error) {
	r := common.NewReader(b)

	epd, err := r.U8()
	if err != nil {
		return 0, err
	}

	if epd != EPD5GSM {
		return 0, fmt.Errorf("%w (EPD %#x)", ErrNot5GSM, epd)
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

	return SMMessageType(mt), nil
}

// writeSMHeader emits the 4-octet 5GSM header: extended protocol discriminator,
// PDU session identity, procedure transaction identity, message type
// (TS 24.501 §9.1.1).
func writeSMHeader(w *common.Writer, pduSessionID, pti uint8, mt SMMessageType) {
	w.U8(EPD5GSM)
	w.U8(pduSessionID)
	w.U8(pti)
	w.U8(uint8(mt))
}

func readSMHeader(r *common.Reader, want SMMessageType) (pduSessionID, pti uint8, err error) {
	epd, err := r.U8()
	if err != nil {
		return 0, 0, err
	}

	if epd != EPD5GSM {
		return 0, 0, fmt.Errorf("%w (EPD %#x)", ErrNot5GSM, epd)
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

	if SMMessageType(mt) != want {
		return 0, 0, fmt.Errorf("%w: got %#x, want %#x", ErrWrongMessageType, mt, uint8(want))
	}

	return pduSessionID, pti, nil
}
