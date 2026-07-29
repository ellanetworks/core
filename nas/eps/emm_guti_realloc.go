// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas"

// GUTIReallocationCommand is the GUTI REALLOCATION COMMAND message (TS 24.301 §8.2.16).
// The assigned GUTI is the only mandatory IE (an LV); the optional TAI list is omitted,
// as a standalone reallocation leaves the registration area unchanged.
type GUTIReallocationCommand struct {
	GUTI EPSMobileIdentity

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain GUTI REALLOCATION COMMAND message.
// The encoding is appended to b.
func (m *GUTIReallocationCommand) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgGUTIReallocationCommand)

	guti, err := m.GUTI.MarshalBinary()
	if err != nil {
		return b, err
	}

	w.LV(guti)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *GUTIReallocationCommand) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseGUTIReallocationCommand decodes a plain GUTI REALLOCATION COMMAND message.
func ParseGUTIReallocationCommand(b []byte) (*GUTIReallocationCommand, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgGUTIReallocationCommand); err != nil {
		return nil, err
	}

	raw, err := r.LV()
	if err != nil {
		return nil, err
	}

	guti, err := ParseEPSMobileIdentity(raw)
	if err != nil {
		return nil, err
	}

	out := &GUTIReallocationCommand{GUTI: guti}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// GUTIReallocationComplete is the GUTI REALLOCATION COMPLETE message
// (TS 24.301 §8.2.17), which carries no information elements.
type GUTIReallocationComplete struct {
	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged. The spec
	// defines none for this message, but a later release may.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain GUTI REALLOCATION COMPLETE message.
// The encoding is appended to b.
func (m *GUTIReallocationComplete) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgGUTIReallocationComplete)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *GUTIReallocationComplete) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseGUTIReallocationComplete decodes a plain GUTI REALLOCATION COMPLETE message.
func ParseGUTIReallocationComplete(b []byte) (*GUTIReallocationComplete, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgGUTIReallocationComplete); err != nil {
		return nil, err
	}

	out := &GUTIReallocationComplete{}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}
