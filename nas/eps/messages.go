// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// Message is a NAS message of the EPS control plane. Only the message types in
// this package implement it, so a type switch over them is complete: a message
// type this package does not model arrives as an [UnknownMessage].
type Message interface {
	// AppendBinary appends the message's encoding to b.
	AppendBinary(b []byte) ([]byte, error)
	// MarshalBinary encodes the message.
	MarshalBinary() ([]byte, error)

	isMessage()
}

// EMMMessage is an EPS mobility management message (TS 24.301 §8.2).
//
// [ServiceRequest] is the one EMM message that does not implement it: TS 24.301
// §8.2.25 gives it no message identity, so it has no message type to report.
type EMMMessage interface {
	Message

	MessageType() MessageType
}

// ESMMessage is an EPS session management message (TS 24.301 §8.3).
//
// Every one of them carries the same header: the EPS bearer it concerns and the
// transaction it belongs to (TS 24.007 §11.2.3.1.1), so both are readable
// without a type switch.
type ESMMessage interface {
	Message

	MessageType() ESMMessageType
	// BearerIdentity is the EPS bearer the message concerns.
	BearerIdentity() EPSBearerIdentity
	// TransactionIdentity is the procedure transaction the message belongs to.
	TransactionIdentity() nas.ProcedureTransactionIdentity
}

// UnknownMessage is a message whose type this package does not model. It keeps
// the header fields a receiver needs to answer with an EMM or ESM STATUS
// (TS 24.301 §7.6), and re-encodes to the octets it arrived as.
type UnknownMessage struct {
	// PD is the protocol discriminator, which says whether Type names an EMM or
	// an ESM message.
	PD ProtocolDiscriminator
	// Type is the message type octet.
	Type uint8
	// EPSBearerIdentity and PTI are the ESM header of the message, which an ESM
	// STATUS answering it carries (TS 24.301 §9.3.2, §9.4). Both are zero for an
	// EMM message, whose header has neither.
	EPSBearerIdentity EPSBearerIdentity
	PTI               nas.ProcedureTransactionIdentity
	// Raw is the whole message, header included.
	Raw []byte
}

// AppendBinary appends the message exactly as it arrived.
func (m *UnknownMessage) AppendBinary(b []byte) ([]byte, error) {
	if len(m.Raw) == 0 {
		return b, fmt.Errorf("nas/eps: unknown message has no octets to encode")
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
//	msg, err := eps.ParseMessage(pdu, nas.DirectionUplink)
//	if err != nil && !nas.SoftOnly(err) {
//	    return err
//	}
//
//	switch m := msg.(type) {
//	case *eps.AttachRequest:
//	    handleAttachRequest(m)
//	case *eps.ServiceRequest:
//	    handleServiceRequest(m)
//	}
//
// dir is the direction the message travelled in. TS 24.301 table 9.8.1 gives
// DETACH REQUEST one message type for both directions, where TS 24.501 assigns
// the two directions their own types, so this generation needs the direction to
// tell [DetachRequestUE] from [DetachRequestNetwork]; every other message type
// belongs to one direction and decodes the same either way.
//
// A message type this package does not model is not a hard failure: it returns
// an [*UnknownMessage] together with [nas.ErrUnknownMessageType], which
// [nas.SoftOnly] reports as soft, so the caller keeps what it needs to answer
// with a STATUS. A security-protected message is rejected with [ErrNotPlain];
// unwrap it with [Unprotect] first. A SERVICE REQUEST, which TS 24.301 §8.2.25
// frames as its own security header type rather than as a plain message, is
// decoded here rather than rejected.
func ParseMessage(b []byte, dir nas.Direction) (Message, error) {
	if err := nas.CheckPDULen(len(b)); err != nil {
		return nil, err
	}

	pd, err := PeekProtocolDiscriminator(b)
	if err != nil {
		return nil, err
	}

	switch pd {
	case PDEMM:
		return parseEMMMessage(b, dir)
	case PDESM:
		return parseESMMessage(b)
	default:
		return nil, fmt.Errorf("%w (PD %#02x)", nas.ErrUnknownProtocolDiscriminator, uint8(pd))
	}
}

func parseEMMMessage(b []byte, dir nas.Direction) (Message, error) {
	sht, err := PeekSecurityHeaderType(b)
	if err != nil {
		return nil, err
	}

	// The SERVICE REQUEST is not a plain message: its security header type is its
	// message identity (TS 24.301 §9.3.1, §8.2.25).
	if sht == SHTServiceRequest {
		return messageOrNil(ParseServiceRequest(b))
	}

	mt, err := PeekMessageType(b)
	if err != nil {
		return nil, err
	}

	if mt == MsgDetachRequest {
		return parseDetachRequest(b, dir)
	}

	parse, ok := emmParsers[mt]
	if !ok {
		return unknownMessage(PDEMM, uint8(mt), b)
	}

	return parse(b)
}

// parseDetachRequest picks the variant the direction names: TS 24.301 §8.2.11.1
// defines the UE-originating message and §8.2.11.2 the network-originating one,
// both under message type 0x45.
func parseDetachRequest(b []byte, dir nas.Direction) (Message, error) {
	if dir == nas.DirectionDownlink {
		return messageOrNil(ParseDetachRequestNetwork(b))
	}

	return messageOrNil(ParseDetachRequestUE(b))
}

func parseESMMessage(b []byte) (Message, error) {
	mt, err := PeekESMMessageType(b)
	if err != nil {
		return nil, err
	}

	parse, ok := esmParsers[mt]
	if !ok {
		return unknownESMMessage(mt, b)
	}

	return parse(b)
}

// unknownMessage keeps an unmodelled message and reports it as the soft error
// TS 24.301 §7.6 expects a receiver to answer with a STATUS.
func unknownMessage(pd ProtocolDiscriminator, mt uint8, b []byte) (Message, error) {
	m := &UnknownMessage{PD: pd, Type: mt, Raw: bytes.Clone(b)}

	return m, fmt.Errorf("nas/eps: %w %#02x", nas.ErrUnknownMessageType, mt)
}

// unknownESMMessage keeps an unmodelled ESM message together with the header a
// receiver needs to answer it: TS 24.301 §7.4 has the network return an ESM
// STATUS, and §8.3.15 gives that STATUS the EPS bearer identity and procedure
// transaction identity of the message it answers.
func unknownESMMessage(mt ESMMessageType, b []byte) (Message, error) {
	// The header is present: PeekESMMessageType read past it to reach mt.
	hdr, err := ParseESMHeader(b)
	if err != nil {
		return nil, err
	}

	m := &UnknownMessage{
		PD:                PDESM,
		Type:              uint8(mt),
		EPSBearerIdentity: hdr.EPSBearerIdentity,
		PTI:               hdr.PTI,
		Raw:               bytes.Clone(b),
	}

	return m, fmt.Errorf("nas/eps: %w %#02x", nas.ErrUnknownMessageType, uint8(mt))
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

// emmParser adapts a message parser to the dispatch table. It maps a nil message
func emmParser[T interface {
	comparable
	EMMMessage
}](parse func([]byte) (T, error),
) func([]byte) (Message, error) {
	return func(b []byte) (Message, error) { return messageOrNil(parse(b)) }
}

// esmParser adapts an ESM message parser to the dispatch table.
func esmParser[T interface {
	comparable
	ESMMessage
}](parse func([]byte) (T, error),
) func([]byte) (Message, error) {
	return func(b []byte) (Message, error) { return messageOrNil(parse(b)) }
}

// emmParsers dispatches an EMM message type to its parser. DETACH REQUEST is
// absent: it needs the direction, which parseDetachRequest applies.
var emmParsers = map[MessageType]func([]byte) (Message, error){
	MsgAttachAccept:               emmParser(ParseAttachAccept),
	MsgAttachComplete:             emmParser(ParseAttachComplete),
	MsgAttachReject:               emmParser(ParseAttachReject),
	MsgAttachRequest:              emmParser(ParseAttachRequest),
	MsgAuthenticationFailure:      emmParser(ParseAuthenticationFailure),
	MsgAuthenticationReject:       emmParser(ParseAuthenticationReject),
	MsgAuthenticationRequest:      emmParser(ParseAuthenticationRequest),
	MsgAuthenticationResponse:     emmParser(ParseAuthenticationResponse),
	MsgDetachAccept:               emmParser(ParseDetachAccept),
	MsgEMMInformation:             emmParser(ParseEMMInformation),
	MsgEMMStatus:                  emmParser(ParseEMMStatus),
	MsgGUTIReallocationCommand:    emmParser(ParseGUTIReallocationCommand),
	MsgGUTIReallocationComplete:   emmParser(ParseGUTIReallocationComplete),
	MsgIdentityRequest:            emmParser(ParseIdentityRequest),
	MsgIdentityResponse:           emmParser(ParseIdentityResponse),
	MsgSecurityModeCommand:        emmParser(ParseSecurityModeCommand),
	MsgSecurityModeComplete:       emmParser(ParseSecurityModeComplete),
	MsgSecurityModeReject:         emmParser(ParseSecurityModeReject),
	MsgServiceAccept:              emmParser(ParseServiceAccept),
	MsgServiceReject:              emmParser(ParseServiceReject),
	MsgTrackingAreaUpdateAccept:   emmParser(ParseTrackingAreaUpdateAccept),
	MsgTrackingAreaUpdateComplete: emmParser(ParseTrackingAreaUpdateComplete),
	MsgTrackingAreaUpdateReject:   emmParser(ParseTrackingAreaUpdateReject),
	MsgTrackingAreaUpdateRequest:  emmParser(ParseTrackingAreaUpdateRequest),
}

// esmParsers dispatches an ESM message type to its parser.
var esmParsers = map[ESMMessageType]func([]byte) (Message, error){
	MsgActivateDefaultEPSBearerContextAccept:  esmParser(ParseActivateDefaultEPSBearerContextAccept),
	MsgActivateDefaultEPSBearerContextReject:  esmParser(ParseActivateDefaultEPSBearerContextReject),
	MsgActivateDefaultEPSBearerContextRequest: esmParser(ParseActivateDefaultEPSBearerContextRequest),
	MsgBearerResourceAllocationReject:         esmParser(ParseBearerResourceAllocationReject),
	MsgBearerResourceAllocationRequest:        esmParser(ParseBearerResourceAllocationRequest),
	MsgBearerResourceModificationReject:       esmParser(ParseBearerResourceModificationReject),
	MsgBearerResourceModificationRequest:      esmParser(ParseBearerResourceModificationRequest),
	MsgDeactivateEPSBearerContextAccept:       esmParser(ParseDeactivateEPSBearerContextAccept),
	MsgDeactivateEPSBearerContextRequest:      esmParser(ParseDeactivateEPSBearerContextRequest),
	MsgESMInformationRequest:                  esmParser(ParseESMInformationRequest),
	MsgESMInformationResponse:                 esmParser(ParseESMInformationResponse),
	MsgESMStatus:                              esmParser(ParseESMStatus),
	MsgModifyEPSBearerContextAccept:           esmParser(ParseModifyEPSBearerContextAccept),
	MsgModifyEPSBearerContextReject:           esmParser(ParseModifyEPSBearerContextReject),
	MsgModifyEPSBearerContextRequest:          esmParser(ParseModifyEPSBearerContextRequest),
	MsgPDNConnectivityReject:                  esmParser(ParsePDNConnectivityReject),
	MsgPDNConnectivityRequest:                 esmParser(ParsePDNConnectivityRequest),
	MsgPDNDisconnectReject:                    esmParser(ParsePDNDisconnectReject),
	MsgPDNDisconnectRequest:                   esmParser(ParsePDNDisconnectRequest),
}
