// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/nas/common"
)

// MessageType is an EMM message type (TS 24.301 §9.8, Table 9.8.1).
type MessageType uint8

const (
	MsgAttachRequest          MessageType = 0x41
	MsgAttachAccept           MessageType = 0x42
	MsgAttachComplete         MessageType = 0x43
	MsgAttachReject           MessageType = 0x44
	MsgDetachRequest          MessageType = 0x45
	MsgDetachAccept           MessageType = 0x46
	MsgAuthenticationRequest  MessageType = 0x52
	MsgAuthenticationResponse MessageType = 0x53
	MsgAuthenticationReject   MessageType = 0x54
	MsgIdentityRequest        MessageType = 0x55
	MsgIdentityResponse       MessageType = 0x56
	MsgAuthenticationFailure  MessageType = 0x5c
	MsgSecurityModeCommand    MessageType = 0x5d
	MsgSecurityModeComplete   MessageType = 0x5e
	MsgSecurityModeReject     MessageType = 0x5f
	MsgEMMStatus              MessageType = 0x60
)

// ErrNotPlain reports a non-zero security-header type where a plain message was
// expected (the message must be unwrapped via Unprotect first).
var ErrNotPlain = errors.New("nas/eps: message is security protected")

// ErrWrongMessageType reports a message-type mismatch when parsing.
var ErrWrongMessageType = errors.New("nas/eps: unexpected message type")

// PeekMessageType returns the EMM message type of a plain NAS message without
// consuming it, after checking the protocol discriminator and that the message
// is not security protected.
func PeekMessageType(b []byte) (MessageType, error) {
	r := common.NewReader(b)

	octet0, err := r.U8()
	if err != nil {
		return 0, err
	}

	if octet0&0x0F != PDEMM {
		return 0, fmt.Errorf("%w (PD %#x)", ErrNotEMM, octet0&0x0F)
	}

	if SecurityHeaderType(octet0>>4) != SHTPlain {
		return 0, ErrNotPlain
	}

	mt, err := r.U8()
	if err != nil {
		return 0, err
	}

	return MessageType(mt), nil
}

// ProtocolDiscriminator returns the protocol discriminator of a NAS message
// (PDEMM or PDESM), so the receiver can route between the EMM and ESM handlers
// before calling the matching Peek/Parse function.
func ProtocolDiscriminator(b []byte) (uint8, error) {
	octet0, err := common.NewReader(b).U8()
	if err != nil {
		return 0, err
	}

	return octet0 & 0x0F, nil
}

func writeEMMHeader(w *common.Writer, mt MessageType) {
	w.U8(uint8(SHTPlain)<<4 | PDEMM)
	w.U8(uint8(mt))
}

func readEMMHeader(r *common.Reader, want MessageType) error {
	octet0, err := r.U8()
	if err != nil {
		return err
	}

	if octet0&0x0F != PDEMM {
		return fmt.Errorf("%w (PD %#x)", ErrNotEMM, octet0&0x0F)
	}

	mt, err := r.U8()
	if err != nil {
		return err
	}

	if MessageType(mt) != want {
		return fmt.Errorf("%w: got %#x, want %#x", ErrWrongMessageType, mt, uint8(want))
	}

	return nil
}
