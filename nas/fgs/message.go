// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// MessageType is a 5GMM message type (TS 24.501 §9.7, table 9.7.1).
type MessageType uint8

// 5GMM message types (TS 24.501 §9.7, table 9.7.1).
const (
	MsgRegistrationRequest              MessageType = 0x41
	MsgRegistrationAccept               MessageType = 0x42
	MsgRegistrationComplete             MessageType = 0x43
	MsgRegistrationReject               MessageType = 0x44
	MsgDeregistrationRequestUEOrig      MessageType = 0x45
	MsgDeregistrationAcceptUEOrig       MessageType = 0x46
	MsgDeregistrationRequestUETerm      MessageType = 0x47
	MsgDeregistrationAcceptUETerm       MessageType = 0x48
	MsgServiceRequest                   MessageType = 0x4C
	MsgServiceReject                    MessageType = 0x4D
	MsgServiceAccept                    MessageType = 0x4E
	MsgControlPlaneServiceRequest       MessageType = 0x4F
	MsgNetworkSliceSpecificAuthCommand  MessageType = 0x50
	MsgNetworkSliceSpecificAuthComplete MessageType = 0x51
	MsgNetworkSliceSpecificAuthResult   MessageType = 0x52
	MsgConfigurationUpdateCommand       MessageType = 0x54
	MsgConfigurationUpdateComplete      MessageType = 0x55
	MsgAuthenticationRequest            MessageType = 0x56
	MsgAuthenticationResponse           MessageType = 0x57
	MsgAuthenticationReject             MessageType = 0x58
	MsgAuthenticationFailure            MessageType = 0x59
	MsgAuthenticationResult             MessageType = 0x5A
	MsgIdentityRequest                  MessageType = 0x5B
	MsgIdentityResponse                 MessageType = 0x5C
	MsgSecurityModeCommand              MessageType = 0x5D
	MsgSecurityModeComplete             MessageType = 0x5E
	MsgSecurityModeReject               MessageType = 0x5F
	MsgGMMStatus                        MessageType = 0x64
	MsgNotification                     MessageType = 0x65
	MsgNotificationResponse             MessageType = 0x66
	MsgULNASTransport                   MessageType = 0x67
	MsgDLNASTransport                   MessageType = 0x68
	MsgRelayKeyRequest                  MessageType = 0x69
	MsgRelayKeyAccept                   MessageType = 0x6A
	MsgRelayKeyReject                   MessageType = 0x6B
	MsgRelayAuthenticationRequest       MessageType = 0x6C
	MsgRelayAuthenticationResponse      MessageType = 0x6D
)

// ErrNotPlain reports a non-zero security-header type where a plain message was
// expected (the message must be unwrapped via Unprotect first).
var ErrNotPlain = fmt.Errorf("nas/fgs: %w", nas.ErrNotPlain)

// ErrWrongMessageType reports a message-type mismatch when parsing.
var ErrWrongMessageType = fmt.Errorf("nas/fgs: %w", nas.ErrWrongMessageType)

// PeekProtocolDiscriminator returns the extended protocol discriminator of a NAS
// message (EPD5GMM or EPD5GSM), so the receiver can route between the 5GMM and
// 5GSM handlers before calling the matching Peek/Parse function.
func PeekProtocolDiscriminator(b []byte) (ProtocolDiscriminator, error) {
	epd, err := nas.NewReader(b).U8()
	if err != nil {
		return 0, err
	}

	return ProtocolDiscriminator(epd), nil
}

// PeekMessageType returns the 5GMM message type of a plain NAS message without
// consuming it, after checking the extended protocol discriminator and that the
// message is not security protected.
func PeekMessageType(b []byte) (MessageType, error) {
	r := nas.NewReader(b)

	epd, err := r.U8()
	if err != nil {
		return 0, err
	}

	if ProtocolDiscriminator(epd) != EPD5GMM {
		return 0, fmt.Errorf("%w (EPD %#x)", ErrNotGMM, epd)
	}

	octet1, err := r.U8()
	if err != nil {
		return 0, err
	}

	if SecurityHeaderType(octet1&0x0F) != SHTPlain {
		return 0, ErrNotPlain
	}

	mt, err := r.U8()
	if err != nil {
		return 0, err
	}

	return MessageType(mt), nil
}

// writeGMMHeader emits the 3-octet plain 5GMM header: extended protocol
// discriminator, security-header-type-and-spare (plain), message type
// (TS 24.501 §9.1.1).
func writeGMMHeader(w *nas.Writer, mt MessageType) {
	w.U8(uint8(EPD5GMM))
	w.U8(uint8(SHTPlain))
	w.U8(uint8(mt))
}

func readGMMHeader(r *nas.Reader, want MessageType) error {
	if err := nas.CheckPDULen(r.Remaining()); err != nil {
		return err
	}

	epd, err := r.U8()
	if err != nil {
		return err
	}

	if ProtocolDiscriminator(epd) != EPD5GMM {
		return fmt.Errorf("%w (EPD %#x)", ErrNotGMM, epd)
	}

	octet1, err := r.U8()
	if err != nil {
		return err
	}

	if SecurityHeaderType(octet1&0x0F) != SHTPlain {
		return ErrNotPlain
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
