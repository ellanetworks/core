// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// Message is a NAS message of the 5GS control plane. Only the message types in
// this package implement it, so a type switch over them is complete: a message
// type this package does not model arrives as an [UnknownMessage].
type Message interface {
	// AppendBinary appends the message's encoding to b.
	AppendBinary(b []byte) ([]byte, error)
	// MarshalBinary encodes the message.
	MarshalBinary() ([]byte, error)

	isMessage()
}

// GMMMessage is a 5GS mobility management message (TS 24.501 §8.2).
type GMMMessage interface {
	Message

	MessageType() MessageType
}

// GSMMessage is a 5GS session management message (TS 24.501 §8.3).
//
// Every one of them carries the same header: the PDU session it concerns and the
// transaction it belongs to (TS 24.501 §9.1.1), so both are readable without a
// type switch.
type GSMMessage interface {
	Message

	MessageType() GSMMessageType
	// SessionIdentity is the PDU session the message concerns.
	SessionIdentity() PDUSessionID
	// TransactionIdentity is the procedure transaction the message belongs to.
	TransactionIdentity() nas.ProcedureTransactionIdentity
}

// UnknownMessage is a message whose type this package does not model. It keeps
// the header fields a receiver needs to answer with a 5GMM or 5GSM STATUS
// (TS 24.501 §7.6), and re-encodes to the octets it arrived as.
type UnknownMessage struct {
	// PD is the extended protocol discriminator, which says whether Type names a
	// 5GMM or a 5GSM message.
	PD ProtocolDiscriminator
	// Type is the message type octet.
	Type uint8
	// Raw is the whole message, header included.
	Raw []byte
}

// AppendBinary appends the message exactly as it arrived.
func (m *UnknownMessage) AppendBinary(b []byte) ([]byte, error) {
	if len(m.Raw) == 0 {
		return b, fmt.Errorf("nas/fgs: unknown message has no octets to encode")
	}

	if err := nas.CheckPDULen(len(m.Raw)); err != nil {
		return b, err
	}

	return append(b, m.Raw...), nil
}

// MarshalBinary encodes the message.
func (m *UnknownMessage) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

func (m *UnknownMessage) isMessage() {}

func (m *UnknownMessage) String() string {
	return fmt.Sprintf("unknown %s message (type %#02x, %d octets)", m.PD, m.Type, len(m.Raw))
}

// ParseMessage decodes any plain NAS message of this generation, so a receiver
// can dispatch on the concrete type rather than peek at the message type and
// call the matching parser:
//
//	msg, err := fgs.ParseMessage(pdu)
//	if err != nil && !nas.SoftOnly(err) {
//	    return err
//	}
//
//	switch m := msg.(type) {
//	case *fgs.RegistrationRequest:
//	    handleRegistrationRequest(m)
//	case *fgs.ULNASTransport:
//	    handleULNASTransport(m)
//	}
//
// A message type this package does not model is not a hard failure: it returns
// an [*UnknownMessage] together with [nas.ErrUnknownMessageType], which
// [nas.SoftOnly] reports as soft, so the caller keeps what it needs to answer
// with a STATUS. A security-protected message is rejected with [ErrNotPlain];
// unwrap it with [Unprotect] first.
func ParseMessage(b []byte) (Message, error) {
	if err := nas.CheckPDULen(len(b)); err != nil {
		return nil, err
	}

	epd, err := PeekProtocolDiscriminator(b)
	if err != nil {
		return nil, err
	}

	switch epd {
	case EPD5GMM:
		return parseGMMMessage(b)
	case EPD5GSM:
		return parseGSMMessage(b)
	default:
		return nil, fmt.Errorf("%w (EPD %#02x)", nas.ErrUnknownProtocolDiscriminator, uint8(epd))
	}
}

func parseGMMMessage(b []byte) (Message, error) {
	mt, err := PeekMessageType(b)
	if err != nil {
		return nil, err
	}

	parse, ok := gmmParsers[mt]
	if !ok {
		return unknownMessage(EPD5GMM, uint8(mt), b)
	}

	return parse(b)
}

func parseGSMMessage(b []byte) (Message, error) {
	mt, err := PeekGSMMessageType(b)
	if err != nil {
		return nil, err
	}

	parse, ok := gsmParsers[mt]
	if !ok {
		return unknownMessage(EPD5GSM, uint8(mt), b)
	}

	return parse(b)
}

// unknownMessage keeps an unmodelled message and reports it as the soft error
// TS 24.501 §7.6 expects a receiver to answer with a STATUS.
func unknownMessage(pd ProtocolDiscriminator, mt uint8, b []byte) (Message, error) {
	m := &UnknownMessage{PD: pd, Type: mt, Raw: bytes.Clone(b)}

	return m, fmt.Errorf("nas/fgs: %w %#02x", nas.ErrUnknownMessageType, mt)
}

// messageOrNil maps a nil message to a nil interface, so a hard failure never
// returns a non-nil Message holding a nil pointer.
func messageOrNil[T interface {
	comparable
	Message
}](m T, err error) (Message, error) {
	var zero T
	if m == zero {
		return nil, err
	}

	return m, err
}

// gmmParser adapts a message parser to the dispatch table. It maps a nil message
func gmmParser[T interface {
	comparable
	GMMMessage
}](parse func([]byte) (T, error),
) func([]byte) (Message, error) {
	return func(b []byte) (Message, error) { return messageOrNil(parse(b)) }
}

// gsmParser adapts a 5GSM message parser to the dispatch table.
func gsmParser[T interface {
	comparable
	GSMMessage
}](parse func([]byte) (T, error),
) func([]byte) (Message, error) {
	return func(b []byte) (Message, error) { return messageOrNil(parse(b)) }
}

// gmmParsers dispatches a 5GMM message type to its parser.
var gmmParsers = map[MessageType]func([]byte) (Message, error){
	MsgAuthenticationFailure:       gmmParser(ParseAuthenticationFailure),
	MsgAuthenticationReject:        gmmParser(ParseAuthenticationReject),
	MsgAuthenticationRequest:       gmmParser(ParseAuthenticationRequest),
	MsgAuthenticationResponse:      gmmParser(ParseAuthenticationResponse),
	MsgConfigurationUpdateCommand:  gmmParser(ParseConfigurationUpdateCommand),
	MsgConfigurationUpdateComplete: gmmParser(ParseConfigurationUpdateComplete),
	MsgDLNASTransport:              gmmParser(ParseDLNASTransport),
	MsgDeregistrationAcceptUEOrig:  gmmParser(ParseDeregistrationAcceptUEOriginating),
	MsgDeregistrationAcceptUETerm:  gmmParser(ParseDeregistrationAcceptUETerminated),
	MsgDeregistrationRequestUEOrig: gmmParser(ParseDeregistrationRequestUEOriginating),
	MsgDeregistrationRequestUETerm: gmmParser(ParseDeregistrationRequestUETerminated),
	MsgGMMStatus:                   gmmParser(ParseGMMStatus),
	MsgIdentityRequest:             gmmParser(ParseIdentityRequest),
	MsgIdentityResponse:            gmmParser(ParseIdentityResponse),
	MsgNotificationResponse:        gmmParser(ParseNotificationResponse),
	MsgRegistrationAccept:          gmmParser(ParseRegistrationAccept),
	MsgRegistrationComplete:        gmmParser(ParseRegistrationComplete),
	MsgRegistrationReject:          gmmParser(ParseRegistrationReject),
	MsgRegistrationRequest:         gmmParser(ParseRegistrationRequest),
	MsgSecurityModeCommand:         gmmParser(ParseSecurityModeCommand),
	MsgSecurityModeComplete:        gmmParser(ParseSecurityModeComplete),
	MsgSecurityModeReject:          gmmParser(ParseSecurityModeReject),
	MsgServiceAccept:               gmmParser(ParseServiceAccept),
	MsgServiceReject:               gmmParser(ParseServiceReject),
	MsgServiceRequest:              gmmParser(ParseServiceRequest),
	MsgULNASTransport:              gmmParser(ParseULNASTransport),
}

// gsmParsers dispatches a 5GSM message type to its parser.
var gsmParsers = map[GSMMessageType]func([]byte) (Message, error){
	MsgGSMStatus:                        gsmParser(ParseGSMStatus),
	MsgPDUSessionEstablishmentAccept:    gsmParser(ParsePDUSessionEstablishmentAccept),
	MsgPDUSessionEstablishmentReject:    gsmParser(ParsePDUSessionEstablishmentReject),
	MsgPDUSessionEstablishmentRequest:   gsmParser(ParsePDUSessionEstablishmentRequest),
	MsgPDUSessionModificationCmdReject:  gsmParser(ParsePDUSessionModificationCommandReject),
	MsgPDUSessionModificationCommand:    gsmParser(ParsePDUSessionModificationCommand),
	MsgPDUSessionModificationComplete:   gsmParser(ParsePDUSessionModificationComplete),
	MsgPDUSessionModificationReject:     gsmParser(ParsePDUSessionModificationReject),
	MsgPDUSessionAuthenticationComplete: gsmParser(ParsePDUSessionAuthenticationComplete),
	MsgPDUSessionModificationRequest:    gsmParser(ParsePDUSessionModificationRequest),
	MsgPDUSessionReleaseCommand:         gsmParser(ParsePDUSessionReleaseCommand),
	MsgPDUSessionReleaseComplete:        gsmParser(ParsePDUSessionReleaseComplete),
	MsgPDUSessionReleaseRequest:         gsmParser(ParsePDUSessionReleaseRequest),
}
