// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// MessageType is an EMM message type (TS 24.301).
type MessageType uint8

// EMM message types (TS 24.301 §9.8, table 9.8.1).
const (
	MsgAttachRequest               MessageType = 0x41
	MsgAttachAccept                MessageType = 0x42
	MsgAttachComplete              MessageType = 0x43
	MsgAttachReject                MessageType = 0x44
	MsgDetachRequest               MessageType = 0x45
	MsgDetachAccept                MessageType = 0x46
	MsgTrackingAreaUpdateRequest   MessageType = 0x48
	MsgTrackingAreaUpdateAccept    MessageType = 0x49
	MsgTrackingAreaUpdateComplete  MessageType = 0x4a
	MsgTrackingAreaUpdateReject    MessageType = 0x4b
	MsgExtendedServiceRequest      MessageType = 0x4c
	MsgControlPlaneServiceRequest  MessageType = 0x4d
	MsgServiceReject               MessageType = 0x4e
	MsgServiceAccept               MessageType = 0x4f
	MsgGUTIReallocationCommand     MessageType = 0x50
	MsgGUTIReallocationComplete    MessageType = 0x51
	MsgAuthenticationRequest       MessageType = 0x52
	MsgAuthenticationResponse      MessageType = 0x53
	MsgAuthenticationReject        MessageType = 0x54
	MsgIdentityRequest             MessageType = 0x55
	MsgIdentityResponse            MessageType = 0x56
	MsgAuthenticationFailure       MessageType = 0x5c
	MsgSecurityModeCommand         MessageType = 0x5d
	MsgSecurityModeComplete        MessageType = 0x5e
	MsgSecurityModeReject          MessageType = 0x5f
	MsgEMMStatus                   MessageType = 0x60
	MsgEMMInformation              MessageType = 0x61
	MsgDownlinkNASTransport        MessageType = 0x62
	MsgUplinkNASTransport          MessageType = 0x63
	MsgCSServiceNotification       MessageType = 0x64
	MsgDownlinkGenericNASTransport MessageType = 0x68
	MsgUplinkGenericNASTransport   MessageType = 0x69
)

// ErrNotPlain reports a non-zero security-header type where a plain message was
// expected (the message must be unwrapped via Unprotect first).
var ErrNotPlain = fmt.Errorf("nas/eps: %w", nas.ErrNotPlain)

// ErrWrongMessageType reports a message-type mismatch when parsing.
var ErrWrongMessageType = fmt.Errorf("nas/eps: %w", nas.ErrWrongMessageType)

// PeekMessageType returns the EMM message type of a plain NAS message without
// consuming it, after checking the protocol discriminator and that the message
// is not security protected.
func PeekMessageType(b []byte) (MessageType, error) {
	r := nas.NewReader(b)

	octet0, err := r.U8()
	if err != nil {
		return 0, err
	}

	if ProtocolDiscriminator(octet0&0x0F) != PDEMM {
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

// PeekKeySetIdentifier returns the NAS key set identifier of a plain EMM message
// that carries it in the high half of octet 3, which the ATTACH REQUEST, the
// TRACKING AREA UPDATE REQUEST and the DETACH REQUEST all do (TS 24.301
// §8.2.4, §8.2.29, §8.2.11). It reads the identifier alone, so a receiver can
// name the security context a message cites before it has a context to decode
// the rest with.
func PeekKeySetIdentifier(b []byte) (nas.KeySetIdentifier, error) {
	if _, err := PeekMessageType(b); err != nil {
		return nas.KeySetIdentifier{}, err
	}

	if len(b) < 3 {
		return nas.KeySetIdentifier{}, fmt.Errorf("nas/eps: message is %d octets, too short for its NAS key set identifier", len(b))
	}

	return nas.ParseKeySetIdentifier(b[2] >> 4), nil
}

// PeekProtocolDiscriminator returns the protocol discriminator of a NAS message
// (PDEMM or PDESM), so the receiver can route between the EMM and ESM handlers
// before calling the matching Peek/Parse function.
func PeekProtocolDiscriminator(b []byte) (ProtocolDiscriminator, error) {
	octet0, err := nas.NewReader(b).U8()
	if err != nil {
		return 0, err
	}

	return ProtocolDiscriminator(octet0 & 0x0F), nil
}

func writeEMMHeader(w *nas.Writer, mt MessageType) {
	w.U8(uint8(SHTPlain)<<4 | uint8(PDEMM))
	w.U8(uint8(mt))
}

func readEMMHeader(r *nas.Reader, want MessageType) error {
	if err := nas.CheckPDULen(r.Remaining()); err != nil {
		return err
	}

	octet0, err := r.U8()
	if err != nil {
		return err
	}

	if ProtocolDiscriminator(octet0&0x0F) != PDEMM {
		return fmt.Errorf("%w (PD %#x)", ErrNotEMM, octet0&0x0F)
	}

	// EPS packs the security header type into the same octet as the protocol
	// discriminator (TS 24.301 §9.3.1), so a protected message reaches here with a
	// matching PD and its first MAC octet would otherwise be read as the message
	// type. Unwrap it with ParseSecurityProtectedMessage first.
	if SecurityHeaderType(octet0>>4) != SHTPlain {
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
