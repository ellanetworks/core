// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas"

// IdentityRequest is the IDENTITY REQUEST message (TS 24.301 §8.2.18): the
// identity the network asks the UE to provide.
type IdentityRequest struct {
	// IdentityType is the identity type 2 IE (TS 24.301 §9.9.3.17), which codes
	// the same values as a mobile identity's type of identity.
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

	writeEMMHeader(w, MsgIdentityRequest)
	w.U8(uint8(m.IdentityType) & 0x07) // identity type | spare half octet

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *IdentityRequest) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseIdentityRequest decodes a plain IDENTITY REQUEST message.
func ParseIdentityRequest(b []byte) (*IdentityRequest, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgIdentityRequest); err != nil {
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

// IdentityResponse is the IDENTITY RESPONSE message (TS 24.301 §8.2.19): the
// mobile identity of TS 24.008 §10.5.1.4, carried as an LV.
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

	writeEMMHeader(w, MsgIdentityResponse)

	mi, err := m.MobileIdentity.MarshalBinary()
	if err != nil {
		return b, err
	}

	w.LV(mi)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *IdentityResponse) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseIdentityResponse decodes a plain IDENTITY RESPONSE message.
func ParseIdentityResponse(b []byte) (*IdentityResponse, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgIdentityResponse); err != nil {
		return nil, err
	}

	raw, err := r.LV()
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
