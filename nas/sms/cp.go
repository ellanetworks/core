// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sms

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// ProtocolDiscriminator is the SMS protocol discriminator (TS 24.007 table 11.2).
const ProtocolDiscriminator uint8 = 0x09

// Errors a CP or RP decode reports for a message the SMC or SMR entity answers
// with CP-ERROR or RP-ERROR (TS 24.011 §9).
var (
	ErrProtocolDiscriminator = errors.New("not an SMS protocol discriminator")
	ErrTransactionIdentifier = errors.New("invalid transaction identifier")
	ErrUnknownMessageType    = errors.New("unknown SMS message type")
)

const (
	maxTIValue     = 6
	cpHeaderLength = 2
)

// TransactionIdentifier is the CP transaction identifier (TS 24.007 §11.2.3.1.3).
// Flag is set on messages sent to the side that allocated Value.
type TransactionIdentifier struct {
	Value uint8
	Flag  bool
}

// Peer returns the identifier the other side uses for the same transaction.
func (t TransactionIdentifier) Peer() TransactionIdentifier {
	return TransactionIdentifier{Value: t.Value, Flag: !t.Flag}
}

// String renders the identifier as "value/flag".
func (t TransactionIdentifier) String() string {
	return fmt.Sprintf("%d/%d", t.Value, boolBit(t.Flag))
}

func (t TransactionIdentifier) octet() (uint8, error) {
	if t.Value > maxTIValue {
		return 0, fmt.Errorf("sms: TI value %d: %w", t.Value, ErrTransactionIdentifier)
	}

	return boolBit(t.Flag)<<7 | t.Value<<4 | ProtocolDiscriminator, nil
}

// CPMessageType is a CP message type (TS 24.011 table 8.1).
type CPMessageType uint8

// CP message types (TS 24.011 table 8.1).
const (
	CPMessageTypeData  CPMessageType = 0x01
	CPMessageTypeAck   CPMessageType = 0x04
	CPMessageTypeError CPMessageType = 0x10
)

var cpMessageTypeNames = map[CPMessageType]string{
	CPMessageTypeData:  "CP-DATA",
	CPMessageTypeAck:   "CP-ACK",
	CPMessageTypeError: "CP-ERROR",
}

// Name returns the value's spec description, or the empty string when the value
// is not one TS 24.011 assigns.
func (t CPMessageType) Name() string { return cpMessageTypeNames[t] }

func (t CPMessageType) String() string {
	if name, ok := cpMessageTypeNames[t]; ok {
		return name
	}

	return fmt.Sprintf("unknown CP message type (%#02x)", uint8(t))
}

// CPCause is a CP-Cause value (TS 24.011 table 8.2).
type CPCause uint8

// CP-Cause values (TS 24.011 table 8.2).
const (
	CPCauseNetworkFailure                CPCause = 17
	CPCauseCongestion                    CPCause = 22
	CPCauseInvalidTransactionIdentifier  CPCause = 81
	CPCauseSemanticallyIncorrectMessage  CPCause = 95
	CPCauseInvalidMandatoryInformation   CPCause = 96
	CPCauseMessageTypeNonExistent        CPCause = 97
	CPCauseMessageNotCompatibleWithState CPCause = 98
	CPCauseInformationElementNonExistent CPCause = 99
	CPCauseProtocolErrorUnspecified      CPCause = 111
)

var cpCauseNames = map[CPCause]string{
	CPCauseNetworkFailure:                "Network failure",
	CPCauseCongestion:                    "Congestion",
	CPCauseInvalidTransactionIdentifier:  "Invalid Transaction Identifier value",
	CPCauseSemanticallyIncorrectMessage:  "Semantically incorrect message",
	CPCauseInvalidMandatoryInformation:   "Invalid mandatory information",
	CPCauseMessageTypeNonExistent:        "Message type non-existent or not implemented",
	CPCauseMessageNotCompatibleWithState: "Message not compatible with the short message protocol state",
	CPCauseInformationElementNonExistent: "Information element non-existent or not implemented",
	CPCauseProtocolErrorUnspecified:      "Protocol error, unspecified",
}

// Name returns the value's spec description, or the empty string when the value
// is not one TS 24.011 assigns.
func (c CPCause) Name() string { return cpCauseNames[c] }

func (c CPCause) String() string {
	if name, ok := cpCauseNames[c]; ok {
		return name
	}

	return fmt.Sprintf("unknown CP cause (%d)", uint8(c))
}

// CPMessage is a CP-DATA, CP-ACK or CP-ERROR message (TS 24.011 §7.2).
type CPMessage interface {
	MessageType() CPMessageType
	TI() TransactionIdentifier
	AppendBinary(b []byte) ([]byte, error)
	MarshalBinary() ([]byte, error)
	isCPMessage()
}

// CPData is the CP-DATA message (TS 24.011 §7.2.1); UserData carries the RPDU.
type CPData struct {
	TransactionIdentifier TransactionIdentifier
	UserData              []byte

	Unrecognized []nas.RawIE
}

// CPAck is the CP-ACK message (TS 24.011 §7.2.2).
type CPAck struct {
	TransactionIdentifier TransactionIdentifier

	Unrecognized []nas.RawIE
}

// CPError is the CP-ERROR message (TS 24.011 §7.2.3).
type CPError struct {
	TransactionIdentifier TransactionIdentifier
	Cause                 CPCause

	Unrecognized []nas.RawIE
}

// AppendBinary encodes the message onto b.
func (m *CPData) AppendBinary(b []byte) ([]byte, error) {
	w, err := writeCPHeader(b, m.TransactionIdentifier, CPMessageTypeData)
	if err != nil {
		return b, err
	}

	w.LV(m.UserData)

	return finishIEs(w, b, m.Unrecognized)
}

// AppendBinary encodes the message onto b.
func (m *CPAck) AppendBinary(b []byte) ([]byte, error) {
	w, err := writeCPHeader(b, m.TransactionIdentifier, CPMessageTypeAck)
	if err != nil {
		return b, err
	}

	return finishIEs(w, b, m.Unrecognized)
}

// AppendBinary encodes the message onto b.
func (m *CPError) AppendBinary(b []byte) ([]byte, error) {
	w, err := writeCPHeader(b, m.TransactionIdentifier, CPMessageTypeError)
	if err != nil {
		return b, err
	}

	w.U8(uint8(m.Cause) & 0x7F)

	return finishIEs(w, b, m.Unrecognized)
}

func writeCPHeader(b []byte, ti TransactionIdentifier, mt CPMessageType) (*nas.Writer, error) {
	octet, err := ti.octet()
	if err != nil {
		return nil, err
	}

	w := nas.NewWriter(b)
	w.U8(octet)
	w.U8(uint8(mt))

	return w, nil
}

// CPHeader is the transaction identifier and message type every CP message
// starts with.
type CPHeader struct {
	TransactionIdentifier TransactionIdentifier
	MessageType           CPMessageType
}

// ParseCPHeader decodes the header of a CP message.
func ParseCPHeader(b []byte) (CPHeader, error) {
	if len(b) < cpHeaderLength {
		return CPHeader{}, &nas.Error{Op: "CP header", Offset: len(b), Err: nas.ErrTruncated}
	}

	if b[0]&0x0F != ProtocolDiscriminator {
		return CPHeader{}, fmt.Errorf("sms: protocol discriminator %#x: %w", b[0]&0x0F, ErrProtocolDiscriminator)
	}

	h := CPHeader{
		TransactionIdentifier: TransactionIdentifier{Value: b[0] >> 4 & 0x07, Flag: b[0]&0x80 != 0},
		MessageType:           CPMessageType(b[1]),
	}

	if h.TransactionIdentifier.Value > maxTIValue {
		return h, fmt.Errorf("sms: TI value %d: %w", h.TransactionIdentifier.Value, ErrTransactionIdentifier)
	}

	return h, nil
}

// ParseCP decodes a CP message. Octets after the defined fields are preserved
// in Unrecognized.
func ParseCP(b []byte) (CPMessage, error) {
	h, err := ParseCPHeader(b)
	if err != nil {
		return nil, err
	}

	r := nas.NewReader(b[cpHeaderLength:])

	var (
		out   CPMessage
		unrec *[]nas.RawIE
	)

	switch h.MessageType {
	case CPMessageTypeData:
		userData, err := r.LV()
		if err != nil {
			return nil, err
		}

		m := &CPData{TransactionIdentifier: h.TransactionIdentifier, UserData: userData}
		out, unrec = m, &m.Unrecognized
	case CPMessageTypeAck:
		m := &CPAck{TransactionIdentifier: h.TransactionIdentifier}
		out, unrec = m, &m.Unrecognized
	case CPMessageTypeError:
		cause, err := r.U8()
		if err != nil {
			return nil, err
		}

		m := &CPError{TransactionIdentifier: h.TransactionIdentifier, Cause: CPCause(cause & 0x7F)}
		out, unrec = m, &m.Unrecognized
	default:
		return nil, fmt.Errorf("sms: %s: %w", h.MessageType, ErrUnknownMessageType)
	}

	if *unrec, err = walkIEs(r, nil, nil); err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	return out, err
}

func boolBit(set bool) uint8 {
	if set {
		return 1
	}

	return 0
}
