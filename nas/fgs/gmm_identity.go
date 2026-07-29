// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas"

// IdentityRequest is the IDENTITY REQUEST message (TS 24.501 §8.2.21): the 5GS
// identity type the network asks the UE to provide.
type IdentityRequest struct {
	// IdentityType is the 5GS identity type IE (TS 24.501 §9.11.3.3), which codes
	// the same values as a 5GS mobile identity's type of identity.
	IdentityType MobileIdentityType

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged. The spec defines none for this message, but a later release may.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain IDENTITY REQUEST message.
// The encoding is appended to b.
func (m *IdentityRequest) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgIdentityRequest)
	w.U8(uint8(m.IdentityType) & 0x07) // spare half octet in bits 5-8, bit 4 spare

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *IdentityRequest) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseIdentityRequest decodes the message.
func ParseIdentityRequest(b []byte) (*IdentityRequest, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgIdentityRequest); err != nil {
		return nil, err
	}

	octet, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &IdentityRequest{IdentityType: TypeOfIdentity(octet)}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// IdentityResponse is the IDENTITY RESPONSE message (TS 24.501 §8.2.22): the 5GS
// mobile identity carried as a type-6 LV-E.
type IdentityResponse struct {
	MobileIdentity MobileIdentity

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged. The spec defines none for this message, but a later release may.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain IDENTITY RESPONSE message.
// The encoding is appended to b.
func (m *IdentityResponse) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgIdentityResponse)

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
func (m *IdentityResponse) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseIdentityResponse decodes the message.
func ParseIdentityResponse(b []byte) (*IdentityResponse, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgIdentityResponse); err != nil {
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

	out := &IdentityResponse{MobileIdentity: mi}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}
