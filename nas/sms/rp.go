// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sms

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/nas"
)

const (
	ieiRPUserData  = 0x41
	rpHeaderLength = 2
	maxRPUserData  = 232
	maxDiagnostic  = 1
)

// MessageTypeIndicator is the RP message type indicator (TS 24.011 table 8.3),
// whose value names both the message and its direction.
type MessageTypeIndicator uint8

// Message type indicators (TS 24.011 table 8.3).
const (
	MTIDataMSToNetwork  MessageTypeIndicator = 0
	MTIDataNetworkToMS  MessageTypeIndicator = 1
	MTIAckMSToNetwork   MessageTypeIndicator = 2
	MTIAckNetworkToMS   MessageTypeIndicator = 3
	MTIErrorMSToNetwork MessageTypeIndicator = 4
	MTIErrorNetworkToMS MessageTypeIndicator = 5
	MTISMMAMSToNetwork  MessageTypeIndicator = 6
)

var mtiNames = map[MessageTypeIndicator]string{
	MTIDataMSToNetwork:  "RP-DATA (MS to network)",
	MTIDataNetworkToMS:  "RP-DATA (network to MS)",
	MTIAckMSToNetwork:   "RP-ACK (MS to network)",
	MTIAckNetworkToMS:   "RP-ACK (network to MS)",
	MTIErrorMSToNetwork: "RP-ERROR (MS to network)",
	MTIErrorNetworkToMS: "RP-ERROR (network to MS)",
	MTISMMAMSToNetwork:  "RP-SMMA",
}

// Name returns the value's spec description, or the empty string when the value
// is not one TS 24.011 assigns.
func (m MessageTypeIndicator) Name() string { return mtiNames[m] }

func (m MessageTypeIndicator) String() string {
	if name, ok := mtiNames[m]; ok {
		return name
	}

	return fmt.Sprintf("reserved MTI (%d)", uint8(m))
}

// Direction returns the direction in which the indicator is defined.
func (m MessageTypeIndicator) Direction() nas.Direction {
	if m%2 == 0 {
		return nas.DirectionUplink
	}

	return nas.DirectionDownlink
}

func mti(base MessageTypeIndicator, dir nas.Direction) MessageTypeIndicator {
	if dir == nas.DirectionDownlink {
		return base + 1
	}

	return base
}

// RPCause is an RP-Cause value (TS 24.011 table 8.4).
type RPCause uint8

// RP-Cause values (TS 24.011 table 8.4).
const (
	RPCauseUnassignedNumber                RPCause = 1
	RPCauseOperatorDeterminedBarring       RPCause = 8
	RPCauseCallBarred                      RPCause = 10
	RPCauseReserved                        RPCause = 11
	RPCauseShortMessageTransferRejected    RPCause = 21
	RPCauseMemoryCapacityExceeded          RPCause = 22
	RPCauseDestinationOutOfOrder           RPCause = 27
	RPCauseUnidentifiedSubscriber          RPCause = 28
	RPCauseFacilityRejected                RPCause = 29
	RPCauseUnknownSubscriber               RPCause = 30
	RPCauseNetworkOutOfOrder               RPCause = 38
	RPCauseTemporaryFailure                RPCause = 41
	RPCauseCongestion                      RPCause = 42
	RPCauseResourcesUnavailable            RPCause = 47
	RPCauseRequestedFacilityNotSubscribed  RPCause = 50
	RPCauseRequestedFacilityNotImplemented RPCause = 69
	RPCauseInvalidShortMessageReference    RPCause = 81
	RPCauseSemanticallyIncorrectMessage    RPCause = 95
	RPCauseInvalidMandatoryInformation     RPCause = 96
	RPCauseMessageTypeNonExistent          RPCause = 97
	RPCauseMessageNotCompatibleWithState   RPCause = 98
	RPCauseInformationElementNonExistent   RPCause = 99
	RPCauseProtocolErrorUnspecified        RPCause = 111
	RPCauseInterworkingUnspecified         RPCause = 127
)

var rpCauseNames = map[RPCause]string{
	RPCauseUnassignedNumber:                "Unassigned (unallocated) number",
	RPCauseOperatorDeterminedBarring:       "Operator determined barring",
	RPCauseCallBarred:                      "Call barred",
	RPCauseReserved:                        "Reserved",
	RPCauseShortMessageTransferRejected:    "Short message transfer rejected",
	RPCauseMemoryCapacityExceeded:          "Memory capacity exceeded",
	RPCauseDestinationOutOfOrder:           "Destination out of order",
	RPCauseUnidentifiedSubscriber:          "Unidentified subscriber",
	RPCauseFacilityRejected:                "Facility rejected",
	RPCauseUnknownSubscriber:               "Unknown subscriber",
	RPCauseNetworkOutOfOrder:               "Network out of order",
	RPCauseTemporaryFailure:                "Temporary failure",
	RPCauseCongestion:                      "Congestion",
	RPCauseResourcesUnavailable:            "Resources unavailable, unspecified",
	RPCauseRequestedFacilityNotSubscribed:  "Requested facility not subscribed",
	RPCauseRequestedFacilityNotImplemented: "Requested facility not implemented",
	RPCauseInvalidShortMessageReference:    "Invalid short message transfer reference value",
	RPCauseSemanticallyIncorrectMessage:    "Semantically incorrect message",
	RPCauseInvalidMandatoryInformation:     "Invalid mandatory information",
	RPCauseMessageTypeNonExistent:          "Message type non-existent or not implemented",
	RPCauseMessageNotCompatibleWithState:   "Message not compatible with short message protocol state",
	RPCauseInformationElementNonExistent:   "Information element non-existent or not implemented",
	RPCauseProtocolErrorUnspecified:        "Protocol error, unspecified",
	RPCauseInterworkingUnspecified:         "Interworking, unspecified",
}

// Name returns the value's spec description, or the empty string when the value
// is not one TS 24.011 assigns.
func (c RPCause) Name() string { return rpCauseNames[c] }

// Transfer is the kind of transfer an RP-ERROR answers, which decides how an
// unassigned RP-Cause value is treated (TS 24.011 table 8.4).
type Transfer uint8

// Transfers (TS 24.011 table 8.4 parts 1 to 3).
const (
	TransferMobileOriginated Transfer = iota
	TransferMobileTerminated
	TransferMemoryAvailable
)

var rpCausesPerTransfer = map[Transfer]struct {
	assigned []RPCause
	fallback RPCause
}{
	TransferMobileOriginated: {
		[]RPCause{
			RPCauseUnassignedNumber, RPCauseOperatorDeterminedBarring, RPCauseCallBarred, RPCauseReserved,
			RPCauseShortMessageTransferRejected, RPCauseDestinationOutOfOrder, RPCauseUnidentifiedSubscriber,
			RPCauseFacilityRejected, RPCauseUnknownSubscriber, RPCauseNetworkOutOfOrder, RPCauseTemporaryFailure,
			RPCauseCongestion, RPCauseResourcesUnavailable, RPCauseRequestedFacilityNotSubscribed,
			RPCauseRequestedFacilityNotImplemented, RPCauseInvalidShortMessageReference,
			RPCauseSemanticallyIncorrectMessage, RPCauseInvalidMandatoryInformation, RPCauseMessageTypeNonExistent,
			RPCauseMessageNotCompatibleWithState, RPCauseInformationElementNonExistent, RPCauseProtocolErrorUnspecified,
			RPCauseInterworkingUnspecified,
		},
		RPCauseTemporaryFailure,
	},
	TransferMobileTerminated: {
		[]RPCause{
			RPCauseMemoryCapacityExceeded, RPCauseInvalidShortMessageReference, RPCauseSemanticallyIncorrectMessage,
			RPCauseInvalidMandatoryInformation, RPCauseMessageTypeNonExistent, RPCauseMessageNotCompatibleWithState,
			RPCauseInformationElementNonExistent, RPCauseProtocolErrorUnspecified,
		},
		RPCauseProtocolErrorUnspecified,
	},
	TransferMemoryAvailable: {
		[]RPCause{
			RPCauseUnknownSubscriber, RPCauseNetworkOutOfOrder, RPCauseTemporaryFailure, RPCauseCongestion,
			RPCauseResourcesUnavailable, RPCauseRequestedFacilityNotImplemented, RPCauseSemanticallyIncorrectMessage,
			RPCauseInvalidMandatoryInformation, RPCauseMessageTypeNonExistent, RPCauseMessageNotCompatibleWithState,
			RPCauseInformationElementNonExistent, RPCauseProtocolErrorUnspecified, RPCauseInterworkingUnspecified,
		},
		RPCauseTemporaryFailure,
	},
}

// Effective returns the cause a receiver acts on for an RP-ERROR answering
// transfer t: a value table 8.4 does not assign for that transfer is treated as
// the table's fallback, #41 or #111.
func (c RPCause) Effective(t Transfer) RPCause {
	causes, ok := rpCausesPerTransfer[t]
	if !ok {
		return c
	}

	for _, assigned := range causes.assigned {
		if c == assigned {
			return c
		}
	}

	return causes.fallback
}

func (c RPCause) String() string {
	if name, ok := rpCauseNames[c]; ok {
		return name
	}

	return fmt.Sprintf("unknown RP cause (%d)", uint8(c))
}

// RPMessage is an RP-DATA, RP-ACK, RP-ERROR or RP-SMMA message (TS 24.011 §7.3).
type RPMessage interface {
	MTI() MessageTypeIndicator
	MessageReference() uint8
	AppendBinary(b []byte) ([]byte, error)
	MarshalBinary() ([]byte, error)
	isRPMessage()
}

// RPData is the RP-DATA message (TS 24.011 §7.3.1). The address the direction
// uses (Destination from the MS, Originator from the network) is mandatory; the
// other is sent with zero length when nil. UserData carries the TPDU.
type RPData struct {
	Direction   nas.Direction
	Reference   uint8
	Originator  *Address
	Destination *Address
	UserData    []byte

	Unrecognized []nas.RawIE
}

// RPAck is the RP-ACK message (TS 24.011 §7.3.3); a nil UserData is absent.
type RPAck struct {
	Direction nas.Direction
	Reference uint8
	UserData  []byte

	Unrecognized []nas.RawIE
}

// RPError is the RP-ERROR message (TS 24.011 §7.3.4); a nil UserData is absent.
type RPError struct {
	Direction  nas.Direction
	Reference  uint8
	Cause      RPCause
	Diagnostic []byte
	UserData   []byte

	Unrecognized []nas.RawIE
}

// RPSMMA is the RP-SMMA message (TS 24.011 §7.3.2), sent only by the MS.
type RPSMMA struct {
	Reference uint8

	Unrecognized []nas.RawIE
}

func (m *RPData) addresses() [2]rpAddress {
	return [2]rpAddress{
		{&m.Originator, "RP-Originator Address", m.Direction == nas.DirectionDownlink},
		{&m.Destination, "RP-Destination Address", m.Direction == nas.DirectionUplink},
	}
}

type rpAddress struct {
	field     **Address
	name      string
	mandatory bool
}

// AppendBinary encodes the message onto b.
func (m *RPData) AppendBinary(b []byte) ([]byte, error) {
	if err := checkRPUserData(m.UserData); err != nil {
		return b, err
	}

	w := nas.NewWriter(b)
	w.U8(uint8(m.MTI()))
	w.U8(m.Reference)

	for _, a := range m.addresses() {
		if *a.field == nil {
			if a.mandatory {
				return b, fmt.Errorf("sms: RP-DATA %s without %s: %w", m.Direction, a.name, ErrInvalidMandatoryIE)
			}

			w.U8(0)

			continue
		}

		raw, err := (*a.field).MarshalBinary()
		if err != nil {
			return b, fmt.Errorf("sms: %s: %w", a.name, err)
		}

		w.LV(raw)
	}

	w.LV(m.UserData)

	return finishIEs(w, b, m.Unrecognized)
}

// AppendBinary encodes the message onto b.
func (m *RPAck) AppendBinary(b []byte) ([]byte, error) {
	if err := checkRPUserData(m.UserData); err != nil {
		return b, err
	}

	w := nas.NewWriter(b)
	w.U8(uint8(m.MTI()))
	w.U8(m.Reference)

	return finishWithUserData(w, b, m.UserData, m.Unrecognized)
}

// AppendBinary encodes the message onto b.
func (m *RPError) AppendBinary(b []byte) ([]byte, error) {
	if err := checkRPUserData(m.UserData); err != nil {
		return b, err
	}

	if len(m.Diagnostic) > maxDiagnostic {
		return b, fmt.Errorf("sms: RP-Cause diagnostic is %d octets, want at most %d: %w", len(m.Diagnostic), maxDiagnostic, ErrElementTooLong)
	}

	w := nas.NewWriter(b)
	w.U8(uint8(m.MTI()))
	w.U8(m.Reference)
	w.LV(append([]byte{uint8(m.Cause) & 0x7F}, m.Diagnostic...))

	return finishWithUserData(w, b, m.UserData, m.Unrecognized)
}

// AppendBinary encodes the message onto b.
func (m *RPSMMA) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)
	w.U8(uint8(MTISMMAMSToNetwork))
	w.U8(m.Reference)

	return finishIEs(w, b, m.Unrecognized)
}

func checkRPUserData(userData []byte) error {
	if len(userData) > maxRPUserData {
		return fmt.Errorf("sms: RP-User data is %d octets, want at most %d: %w", len(userData), maxRPUserData, ErrElementTooLong)
	}

	return nil
}

func finishWithUserData(w *nas.Writer, b, userData []byte, unrecognized []nas.RawIE) ([]byte, error) {
	var o nas.OptionalWriter

	if userData != nil {
		o.TLV(ieiRPUserData, userData)
	}

	o.Raw(unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

func finishIEs(w *nas.Writer, b []byte, unrecognized []nas.RawIE) ([]byte, error) {
	return finishWithUserData(w, b, nil, unrecognized)
}

var rpOptionalIEs = []nas.OptionalIE{
	{IEI: ieiRPUserData, Format: nas.IETLV, Name: "RP-User Data"},
}

// RPHeader is the message type indicator and message reference every RP message
// starts with.
type RPHeader struct {
	MTI       MessageTypeIndicator
	Reference uint8
}

// ParseRPHeader decodes the header of an RP message travelling in direction dir.
// For a message type indicator reserved in that direction it returns the header
// with ErrUnknownMessageType, so the receiver can answer RP-ERROR #97 with the
// received reference (TS 24.011 §9.3.3).
func ParseRPHeader(b []byte, dir nas.Direction) (RPHeader, error) {
	if len(b) < rpHeaderLength {
		return RPHeader{}, &nas.Error{Op: "RP header", Offset: len(b), Err: nas.ErrTruncated}
	}

	h := RPHeader{MTI: MessageTypeIndicator(b[0] & 0x07), Reference: b[1]}
	if _, ok := mtiNames[h.MTI]; !ok || h.MTI.Direction() != dir {
		return h, fmt.Errorf("sms: MTI %d %s: %w", uint8(h.MTI), dir, ErrUnknownMessageType)
	}

	return h, nil
}

// ParseRP decodes an RP message travelling in direction dir. When it fails, the
// header is still available from ParseRPHeader so the receiver can answer with
// the received reference: ErrUnknownMessageType maps to cause #97, and
// ErrInvalidMandatoryIE or nas.ErrTruncated to cause #96 (TS 24.011 §9.3).
func ParseRP(b []byte, dir nas.Direction) (RPMessage, error) {
	h, err := ParseRPHeader(b, dir)
	if err != nil {
		return nil, err
	}

	r := nas.NewReader(b[rpHeaderLength:])

	var out RPMessage

	switch h.MTI {
	case MTIDataMSToNetwork, MTIDataNetworkToMS:
		out, err = parseRPData(r, dir, h.Reference)
	case MTIAckMSToNetwork, MTIAckNetworkToMS:
		ack := &RPAck{Direction: dir, Reference: h.Reference}
		ack.Unrecognized, err = walkIEs(r, rpOptionalIEs, &ack.UserData)
		out = ack
	case MTIErrorMSToNetwork, MTIErrorNetworkToMS:
		out, err = parseRPError(r, dir, h.Reference)
	default:
		smma := &RPSMMA{Reference: h.Reference}
		smma.Unrecognized, err = walkIEs(r, nil, nil)
		out = smma
	}

	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	return out, err
}

func parseRPData(r *nas.Reader, dir nas.Direction, ref uint8) (RPMessage, error) {
	out := &RPData{Direction: dir, Reference: ref}

	var soft []error

	for _, a := range out.addresses() {
		raw, err := r.LV()
		if err != nil {
			return nil, err
		}

		if len(raw) == 0 {
			if a.mandatory {
				return nil, fmt.Errorf("sms: RP-DATA %s without %s: %w", dir, a.name, ErrInvalidMandatoryIE)
			}

			continue
		}

		parsed, err := ParseAddress(raw)
		if err == nil {
			*a.field = &parsed

			continue
		}

		if a.mandatory {
			return nil, fmt.Errorf("sms: %s: %w: %w", a.name, ErrInvalidMandatoryIE, err)
		}

		*a.field = &Address{Raw: raw}
		soft = append(soft, &nas.IEError{Name: a.name, Format: nas.IETLV, Raw: raw, Err: err})
	}

	userData, err := r.LV()
	if err != nil {
		return nil, err
	}

	out.UserData = userData
	out.Unrecognized, err = walkIEs(r, nil, nil)

	return out, errors.Join(append(soft, err)...)
}

func parseRPError(r *nas.Reader, dir nas.Direction, ref uint8) (RPMessage, error) {
	out := &RPError{Direction: dir, Reference: ref}

	cause, err := r.LV()
	if err != nil || len(cause) == 0 {
		out.Cause = RPCauseProtocolErrorUnspecified

		if err == nil {
			err = fmt.Errorf("sms: empty RP-Cause")
		}

		return out, &nas.IEError{Name: "RP-Cause", Format: nas.IETLV, Raw: cause, Err: err}
	}

	out.Cause = RPCause(cause[0] & 0x7F)

	if len(cause) > 1 {
		out.Diagnostic = cause[1:]
	}

	out.Unrecognized, err = walkIEs(r, rpOptionalIEs, &out.UserData)

	return out, err
}

func walkIEs(r *nas.Reader, table []nas.OptionalIE, userData *[]byte) ([]nas.RawIE, error) {
	return nas.Walker{
		Table:   table,
		Unknown: nas.UnknownIESkipTLV,
		Emit: func(iei uint8, value []byte) (bool, error) {
			if iei != ieiRPUserData || userData == nil {
				return false, nil
			}

			*userData = value

			return true, nil
		},
	}.Walk(r)
}
