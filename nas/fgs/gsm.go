// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// GSMMessageType is a 5GSM message type (TS 24.501 §9.7, table 9.7.2).
type GSMMessageType uint8

// 5GSM message types (TS 24.501 §9.7, table 9.7.2).
const (
	MsgPDUSessionEstablishmentRequest   GSMMessageType = 0xC1
	MsgPDUSessionEstablishmentAccept    GSMMessageType = 0xC2
	MsgPDUSessionEstablishmentReject    GSMMessageType = 0xC3
	MsgPDUSessionAuthenticationCommand  GSMMessageType = 0xC5
	MsgPDUSessionAuthenticationComplete GSMMessageType = 0xC6
	MsgPDUSessionAuthenticationResult   GSMMessageType = 0xC7
	MsgPDUSessionModificationRequest    GSMMessageType = 0xC9
	MsgPDUSessionModificationReject     GSMMessageType = 0xCA
	MsgPDUSessionModificationCommand    GSMMessageType = 0xCB
	MsgPDUSessionModificationComplete   GSMMessageType = 0xCC
	MsgPDUSessionModificationCmdReject  GSMMessageType = 0xCD
	MsgPDUSessionReleaseRequest         GSMMessageType = 0xD1
	MsgPDUSessionReleaseReject          GSMMessageType = 0xD2
	MsgPDUSessionReleaseCommand         GSMMessageType = 0xD3
	MsgPDUSessionReleaseComplete        GSMMessageType = 0xD4
	MsgGSMStatus                        GSMMessageType = 0xD6
	MsgServiceLevelAuthCommand          GSMMessageType = 0xD8
	MsgServiceLevelAuthComplete         GSMMessageType = 0xD9
	MsgRemoteUEReport                   GSMMessageType = 0xDA
	MsgRemoteUEReportResponse           GSMMessageType = 0xDB
)

// ErrNotGSM reports an extended protocol discriminator other than 5GSM.
var ErrNotGSM = errors.New("nas/fgs: not a 5GSM message")

// PeekGSMMessageType returns the 5GSM message type of a NAS message (the fourth
// octet) without consuming it, after checking the extended protocol
// discriminator.
func PeekGSMMessageType(b []byte) (GSMMessageType, error) {
	r := nas.NewReader(b)

	epd, err := r.U8()
	if err != nil {
		return 0, err
	}

	if ProtocolDiscriminator(epd) != EPD5GSM {
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
func writeGSMHeader(w *nas.Writer, pduSessionID PDUSessionID, pti nas.ProcedureTransactionIdentity, mt GSMMessageType) {
	w.U8(uint8(EPD5GSM))
	w.U8(uint8(pduSessionID))
	w.U8(uint8(pti))
	w.U8(uint8(mt))
}

func readGSMHeader(r *nas.Reader, want GSMMessageType) (pduSessionID PDUSessionID, pti nas.ProcedureTransactionIdentity, err error) {
	if err := nas.CheckPDULen(r.Remaining()); err != nil {
		return 0, 0, err
	}

	epd, err := r.U8()
	if err != nil {
		return 0, 0, err
	}

	if ProtocolDiscriminator(epd) != EPD5GSM {
		return 0, 0, fmt.Errorf("%w (EPD %#x)", ErrNotGSM, epd)
	}

	rawSessionID, err := r.U8()
	if err != nil {
		return 0, 0, err
	}

	pduSessionID = PDUSessionID(rawSessionID)

	rawPTI, err := r.U8()
	if err != nil {
		return 0, 0, err
	}

	pti = nas.ProcedureTransactionIdentity(rawPTI)

	mt, err := r.U8()
	if err != nil {
		return 0, 0, err
	}

	if GSMMessageType(mt) != want {
		return 0, 0, fmt.Errorf("%w: got %#x, want %#x", ErrWrongMessageType, mt, uint8(want))
	}

	return pduSessionID, pti, nil
}
