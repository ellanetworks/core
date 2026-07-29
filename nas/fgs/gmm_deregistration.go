// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas"

// Access type values (TS 24.501 §9.11.3.20): the 3GPP/non-3GPP access indicated
// in the de-registration type and, numerically, the 3GPP-access 5GS registration
// result (§9.11.3.6).
const (
	AccessType3GPP    AccessType = 0x01
	AccessTypeNon3GPP AccessType = 0x02
	AccessTypeBoth    AccessType = 0x03
)

// DeregistrationRequestUETerminated is the network-initiated DEREGISTRATION
// REQUEST (TS 24.501 §8.2.14): a mandatory de-registration type. The optional
// IEs Ella does not set are omitted.
type DeregistrationRequestUETerminated struct {
	AccessType             AccessType // bits 1-2
	ReRegistrationRequired bool       // bit 3
	SwitchOff              bool       // bit 4

	Cause *GMMCause       // optional (IEI 0x58)
	T3346 *nas.GPRSTimer2 // optional (IEI 0x5F)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// deregistrationRequestUETerminatedIEs is the optional-IE table of the
// UE-terminated DEREGISTRATION REQUEST (TS 24.501 §8.2.14, table 8.2.14.1.1).
var deregistrationRequestUETerminatedIEs = []nas.OptionalIE{
	{IEI: ieiCause5GMM, Format: nas.IETV3, Len: 1, Name: "5GMM cause"},
	{IEI: ieiT3346Value, Format: nas.IETLV, Name: "T3346 value"},
}

// AppendBinary encodes the plain DEREGISTRATION REQUEST (UE terminated) message.
// The encoding is appended to b.
func (m *DeregistrationRequestUETerminated) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgDeregistrationRequestUETerm)

	octet := uint8(m.AccessType) & 0x03
	if m.ReRegistrationRequired {
		octet |= 1 << 2
	}

	if m.SwitchOff {
		octet |= 1 << 3
	}

	w.U8(octet) // spare half octet in bits 5-8

	if m.Cause != nil {
		o.TV3(ieiCause5GMM, []byte{uint8(*m.Cause)})
	}

	if m.T3346 != nil {
		raw, err := m.T3346.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiT3346Value, raw)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *DeregistrationRequestUETerminated) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseDeregistrationRequestUETerminated decodes a network-originated
// DEREGISTRATION REQUEST (TS 24.501 §8.2.14).
func ParseDeregistrationRequestUETerminated(b []byte) (*DeregistrationRequestUETerminated, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgDeregistrationRequestUETerm); err != nil {
		return nil, err
	}

	octet, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &DeregistrationRequestUETerminated{
		AccessType:             AccessType(octet & 0x03),
		ReRegistrationRequired: octet&(1<<2) != 0,
		SwitchOff:              octet&(1<<3) != 0,
	}

	_unrec, err := walkOptionalIEs(r, deregistrationRequestUETerminatedIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiCause5GMM:
			if len(value) == 0 {
				return false, nil
			}

			cause := GMMCause(value[0])
			out.Cause = &cause
		case ieiT3346Value:
			timer, err := nas.ParseGPRSTimer2(value)
			if err != nil {
				return false, err
			}

			out.T3346 = &timer
		default:
			return false, nil
		}

		return true, nil
	})
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// DeregistrationRequestUEOriginating is the UE-originating DEREGISTRATION REQUEST
// (TS 24.501 §8.2.12): a de-registration type (with ngKSI) and the UE's 5GS mobile
// identity. The ngKSI (bits 5-8 of the type octet) is not needed.
type DeregistrationRequestUEOriginating struct {
	AccessType             AccessType           // bits 1-2
	ReRegistrationRequired bool                 // bit 3
	SwitchOff              bool                 // bit 4
	NgKSI                  nas.KeySetIdentifier // bits 5-8
	MobileIdentity         MobileIdentity       // mandatory 5GS mobile identity (type 6, LVE)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain DEREGISTRATION REQUEST (UE-originating) message
// (TS 24.501 §8.2.12).
// The encoding is appended to b.
func (m *DeregistrationRequestUEOriginating) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgDeregistrationRequestUEOrig)

	octet := uint8(m.AccessType) & 0x03
	if m.ReRegistrationRequired {
		octet |= 1 << 2
	}

	if m.SwitchOff {
		octet |= 1 << 3
	}

	octet |= m.NgKSI.HalfOctet() << 4
	w.U8(octet)

	mi, err := m.MobileIdentity.MarshalBinary()
	if err != nil {
		return b, err
	}

	w.LVE(mi)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *DeregistrationRequestUEOriginating) MarshalBinary() ([]byte, error) {
	return marshalMessage(m)
}

// ParseDeregistrationRequestUEOriginating decodes a UE-originating DEREGISTRATION
// REQUEST (TS 24.501 §8.2.12, §9.11.3.20).
func ParseDeregistrationRequestUEOriginating(b []byte) (*DeregistrationRequestUEOriginating, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgDeregistrationRequestUEOrig); err != nil {
		return nil, err
	}

	octet, err := r.U8()
	if err != nil {
		return nil, err
	}

	raw, err := r.LVE()
	if err != nil {
		return nil, err
	}

	mi, err := ParseMobileIdentity(raw)
	if err != nil {
		return nil, err
	}

	out := &DeregistrationRequestUEOriginating{
		AccessType:             AccessType(octet & 0x03),
		ReRegistrationRequired: octet&(1<<2) != 0,
		SwitchOff:              octet&(1<<3) != 0,
		NgKSI:                  nas.ParseKeySetIdentifier(octet >> 4),
		MobileIdentity:         mi,
	}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// DeregistrationAcceptUEOriginating is the network's DEREGISTRATION ACCEPT for a
// UE-originating de-registration (TS 24.501 §8.2.13): the 5GMM header alone.
type DeregistrationAcceptUEOriginating struct {
	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged. TS 24.501
	// §8.2.13 defines none, but a later release may.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain DEREGISTRATION ACCEPT (UE originating) message.
// The encoding is appended to b.
func (m *DeregistrationAcceptUEOriginating) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgDeregistrationAcceptUEOrig)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *DeregistrationAcceptUEOriginating) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseDeregistrationAcceptUEOriginating decodes the message answering a
// UE-originating de-registration (TS 24.501 §8.2.13).
func ParseDeregistrationAcceptUEOriginating(b []byte) (*DeregistrationAcceptUEOriginating, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgDeregistrationAcceptUEOrig); err != nil {
		return nil, err
	}

	out := &DeregistrationAcceptUEOriginating{}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// DeregistrationAcceptUETerminated is the UE's DEREGISTRATION ACCEPT for a
// network-initiated de-registration (TS 24.501 §8.2.15): the 5GMM header alone.
type DeregistrationAcceptUETerminated struct {
	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged. TS 24.501
	// §8.2.15 defines none, but a later release may.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain DEREGISTRATION ACCEPT (UE terminated) message.
// The encoding is appended to b.
func (m *DeregistrationAcceptUETerminated) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgDeregistrationAcceptUETerm)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *DeregistrationAcceptUETerminated) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseDeregistrationAcceptUETerminated decodes the message answering a
// network-initiated de-registration (TS 24.501 §8.2.15).
func ParseDeregistrationAcceptUETerminated(b []byte) (*DeregistrationAcceptUETerminated, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgDeregistrationAcceptUETerm); err != nil {
		return nil, err
	}

	out := &DeregistrationAcceptUETerminated{}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}
