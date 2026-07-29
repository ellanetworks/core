// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// ieiAuthFailureParameter is the IEI of the optional Authentication failure
// parameter (AUTS) in AUTHENTICATION FAILURE (TS 24.301). It is a type-4
// TLV IE (TS 24.301).
const ieiAuthFailureParameter uint8 = 0x30

// authenticationFailureIEs are the optional IEs of an AUTHENTICATION FAILURE
// (TS 24.301): the Authentication failure parameter (AUTS).
var authenticationFailureIEs = []nas.OptionalIE{
	{IEI: ieiAuthFailureParameter, Format: nas.IETLV, Name: "Authentication failure parameter"},
}

// AuthenticationRequest is the AUTHENTICATION REQUEST message (TS 24.301),
// sent by the MME with the EPS-AKA challenge.
type AuthenticationRequest struct {
	NASKeySetIdentifier nas.KeySetIdentifier
	RAND                [16]byte
	AUTN                [16]byte

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged. The spec
	// defines none for this message, but a later release may.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain AUTHENTICATION REQUEST message.
// The encoding is appended to b.
func (m *AuthenticationRequest) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgAuthenticationRequest)
	w.U8(m.NASKeySetIdentifier.HalfOctet()) // spare half octet | NAS KSI
	w.Raw(m.RAND[:])

	w.LV(m.AUTN[:])

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *AuthenticationRequest) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseAuthenticationRequest decodes a plain AUTHENTICATION REQUEST message.
func ParseAuthenticationRequest(b []byte) (*AuthenticationRequest, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgAuthenticationRequest); err != nil {
		return nil, err
	}

	ksi, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &AuthenticationRequest{NASKeySetIdentifier: nas.ParseKeySetIdentifier(ksi)}

	rand, err := r.Bytes(16)
	if err != nil {
		return nil, err
	}

	copy(m.RAND[:], rand)

	autn, err := r.LV()
	if err != nil {
		return nil, err
	}

	if len(autn) != len(m.AUTN) {
		return nil, fmt.Errorf("nas/eps: AUTN is %d octets, want %d", len(autn), len(m.AUTN))
	}

	copy(m.AUTN[:], autn)

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	m.Unrecognized = _unrec

	return m, err
}

// AuthenticationResponse is the AUTHENTICATION RESPONSE message (TS 24.301),
// carrying the UE's RES.
type AuthenticationResponse struct {
	RES []byte

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged. The spec defines none for this message, but a later release may.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain AUTHENTICATION RESPONSE message.
// The encoding is appended to b.
func (m *AuthenticationResponse) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgAuthenticationResponse)

	w.LV(m.RES)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *AuthenticationResponse) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseAuthenticationResponse decodes a plain AUTHENTICATION RESPONSE message.
func ParseAuthenticationResponse(b []byte) (*AuthenticationResponse, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgAuthenticationResponse); err != nil {
		return nil, err
	}

	res, err := r.LV()
	if err != nil {
		return nil, err
	}

	out := &AuthenticationResponse{RES: res}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// AuthenticationReject is the AUTHENTICATION REJECT message (TS 24.301).
// It has no mandatory information elements.
type AuthenticationReject struct {
	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged. The spec
	// defines none for this message, but a later release may.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain AUTHENTICATION REJECT message.
// The encoding is appended to b.
func (m *AuthenticationReject) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgAuthenticationReject)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *AuthenticationReject) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseAuthenticationReject decodes a plain AUTHENTICATION REJECT message.
func ParseAuthenticationReject(b []byte) (*AuthenticationReject, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgAuthenticationReject); err != nil {
		return nil, err
	}

	out := &AuthenticationReject{}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// AuthenticationFailure is the AUTHENTICATION FAILURE message (TS 24.301).
// AUTS is the optional Authentication failure parameter (present for a
// "synch failure"); it is nil when absent.
type AuthenticationFailure struct {
	Cause EMMCause
	AUTS  []byte

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain AUTHENTICATION FAILURE message.
// The encoding is appended to b.
func (m *AuthenticationFailure) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgAuthenticationFailure)
	w.U8(uint8(m.Cause))

	if m.AUTS != nil {
		o.TLV(ieiAuthFailureParameter, m.AUTS)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *AuthenticationFailure) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseAuthenticationFailure decodes a plain AUTHENTICATION FAILURE message.
func ParseAuthenticationFailure(b []byte) (*AuthenticationFailure, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgAuthenticationFailure); err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &AuthenticationFailure{Cause: EMMCause(cause)}

	_unrec, err := walkOptionalIEs(r, authenticationFailureIEs, func(iei uint8, value []byte) (bool, error) {
		if iei != ieiAuthFailureParameter {
			return false, nil
		}

		m.AUTS = value

		return true, nil
	})
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	m.Unrecognized = _unrec

	return m, err
}
