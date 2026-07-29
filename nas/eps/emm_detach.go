// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas"

// Type of detach values for a UE-originating DETACH REQUEST
// (TS 24.301 §9.9.3.7, table 9.9.3.7.1).
const (
	DetachTypeEPS      DetachType = 1
	DetachTypeIMSI     DetachType = 2
	DetachTypeCombined DetachType = 3
)

// Type of detach values for a network-originating DETACH REQUEST. The same three
// bit patterns name different procedures in this direction
// (TS 24.301 §9.9.3.7, table 9.9.3.7.1).
const (
	DetachTypeReattachRequired    DetachTypeNetwork = 1
	DetachTypeReattachNotRequired DetachTypeNetwork = 2
	DetachTypeNetworkIMSI         DetachTypeNetwork = 3
)

// detachRequestNetworkIEs are the optional IEs of a network-originating DETACH
// REQUEST (TS 24.301): the EMM cause.
var detachRequestNetworkIEs = []nas.OptionalIE{
	{IEI: ieiEMMCause, Format: nas.IETV3, Len: 1, Name: "EMM cause"},
}

// DetachRequestUE is the UE-originating DETACH REQUEST message (TS 24.301).
// SwitchOff indicates the UE is powering off, in which case the
// network does not send a Detach Accept.
type DetachRequestUE struct {
	SwitchOff           bool
	TypeOfDetach        DetachType
	NASKeySetIdentifier nas.KeySetIdentifier
	EPSMobileIdentity   EPSMobileIdentity

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged. The spec
	// defines none for this message, but a later release may.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain UE-originating DETACH REQUEST message.
// The encoding is appended to b.
func (m *DetachRequestUE) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgDetachRequest)

	octet := m.NASKeySetIdentifier.HalfOctet()<<4 | uint8(m.TypeOfDetach)&0x07
	if m.SwitchOff {
		octet |= 0x08
	}

	w.U8(octet)

	mi, err := m.EPSMobileIdentity.MarshalBinary()
	if err != nil {
		return b, err
	}

	w.LV(mi)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *DetachRequestUE) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseDetachRequestUE decodes a plain UE-originating DETACH REQUEST message.
func ParseDetachRequestUE(b []byte) (*DetachRequestUE, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgDetachRequest); err != nil {
		return nil, err
	}

	octet, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &DetachRequestUE{
		SwitchOff:           octet&0x08 != 0,
		TypeOfDetach:        DetachType(octet & 0x07),
		NASKeySetIdentifier: nas.ParseKeySetIdentifier(octet >> 4),
	}

	raw, err := r.LV()
	if err != nil {
		return nil, err
	}

	if m.EPSMobileIdentity, err = ParseEPSMobileIdentity(raw); err != nil {
		return nil, err
	}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	m.Unrecognized = _unrec

	return m, err
}

// DetachRequestNetwork is the network-originating DETACH REQUEST message
// (TS 24.301). EMMCause is nil when the optional cause is absent.
type DetachRequestNetwork struct {
	TypeOfDetach DetachTypeNetwork
	Cause        *EMMCause

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain network-originating DETACH REQUEST message.
// The encoding is appended to b.
func (m *DetachRequestNetwork) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgDetachRequest)
	w.U8(uint8(m.TypeOfDetach & 0x07)) // detach type | spare half octet

	if m.Cause != nil {
		o.TV3(ieiEMMCause, []byte{uint8(*m.Cause)})
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *DetachRequestNetwork) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseDetachRequestNetwork decodes a plain network-originating DETACH REQUEST
// message.
func ParseDetachRequestNetwork(b []byte) (*DetachRequestNetwork, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgDetachRequest); err != nil {
		return nil, err
	}

	octet, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &DetachRequestNetwork{TypeOfDetach: DetachTypeNetwork(octet & 0x07)}

	_unrec, err := walkOptionalIEs(r, detachRequestNetworkIEs, func(iei uint8, value []byte) (bool, error) {
		if iei != ieiEMMCause || len(value) != 1 {
			return false, nil
		}

		cause := EMMCause(value[0])
		m.Cause = &cause

		return true, nil
	})
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	m.Unrecognized = _unrec

	return m, err
}

// DetachAccept is the DETACH ACCEPT message (TS 24.301), used in both
// directions; it has no information elements beyond the header.
type DetachAccept struct {
	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged. The spec
	// defines none for this message, but a later release may.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain DETACH ACCEPT message.
// The encoding is appended to b.
func (m *DetachAccept) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgDetachAccept)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *DetachAccept) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseDetachAccept decodes a plain DETACH ACCEPT message.
func ParseDetachAccept(b []byte) (*DetachAccept, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgDetachAccept); err != nil {
		return nil, err
	}

	out := &DetachAccept{}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}
