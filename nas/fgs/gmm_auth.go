// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// AuthenticationRequest is the AUTHENTICATION REQUEST message (TS 24.501
// §8.2.1): the ngKSI, the ABBA, and the 5G-AKA challenge (RAND and AUTN). An
// EAP-based run instead carries the EAP message; Ella never sends it, but the
// parser accepts it.
type AuthenticationRequest struct {
	NgKSI nas.KeySetIdentifier // bits 1-4
	ABBA  []byte
	RAND  *[16]byte
	AUTN  *[16]byte
	EAP   []byte // optional EAP message (IEI 0x78)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain AUTHENTICATION REQUEST message.
// The encoding is appended to b.
func (m *AuthenticationRequest) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgAuthenticationRequest)
	w.U8(m.NgKSI.HalfOctet()) // spare half octet in bits 5-8

	w.LV(m.ABBA)

	if m.RAND != nil {
		o.TV3(ieiRAND, m.RAND[:])
	}

	if m.AUTN != nil {
		o.TLV(ieiAUTN, m.AUTN[:])
	}

	if m.EAP != nil {
		o.TLVE(ieiEAPMessage, m.EAP)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *AuthenticationRequest) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

var authRequestIEs = []nas.OptionalIE{
	{IEI: ieiRAND, Format: nas.IETV3, Len: 16, Name: "RAND"},
	{IEI: ieiAUTN, Format: nas.IETLV, Name: "AUTN"},
	{IEI: ieiEAPMessage, Format: nas.IETLVE, Name: "EAP message"},
}

// ParseAuthenticationRequest decodes the message.
func ParseAuthenticationRequest(b []byte) (*AuthenticationRequest, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgAuthenticationRequest); err != nil {
		return nil, err
	}

	octet, err := r.U8()
	if err != nil {
		return nil, err
	}

	abba, err := r.LV()
	if err != nil {
		return nil, err
	}

	out := &AuthenticationRequest{NgKSI: nas.ParseKeySetIdentifier(octet), ABBA: abba}

	_unrec, err := walkOptionalIEs(r, authRequestIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiRAND:
			if len(value) != len(out.RAND) {
				return false, fmt.Errorf("nas/fgs: RAND is %d octets, want 16", len(value))
			}

			var rand [16]byte

			copy(rand[:], value)

			out.RAND = &rand
		case ieiAUTN:
			if len(value) != 16 {
				return false, fmt.Errorf("nas/fgs: AUTN is %d octets, want 16", len(value))
			}

			var autn [16]byte

			copy(autn[:], value)

			out.AUTN = &autn
		case ieiEAPMessage:
			out.EAP = value
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

// AuthenticationReject is the AUTHENTICATION REJECT message (TS 24.501 §8.2.5).
// Ella sends the 5GMM header alone; an EAP-based run may carry an EAP message,
// which the parser accepts.
type AuthenticationReject struct {
	EAP []byte // optional EAP message (IEI 0x78)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain AUTHENTICATION REJECT message.
// The encoding is appended to b.
func (m *AuthenticationReject) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgAuthenticationReject)

	if m.EAP != nil {
		o.TLVE(ieiEAPMessage, m.EAP)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *AuthenticationReject) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

var authRejectIEs = []nas.OptionalIE{
	{IEI: ieiEAPMessage, Format: nas.IETLVE, Name: "EAP message"},
}

// ParseAuthenticationReject decodes the message.
func ParseAuthenticationReject(b []byte) (*AuthenticationReject, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgAuthenticationReject); err != nil {
		return nil, err
	}

	out := &AuthenticationReject{}

	_unrec, err := walkOptionalIEs(r, authRejectIEs, func(iei uint8, value []byte) (bool, error) {
		if iei != ieiEAPMessage {
			return false, nil
		}

		out.EAP = value

		return true, nil
	})
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// AuthenticationResponse is the AUTHENTICATION RESPONSE message (TS 24.501
// §8.2.2): the optional authentication response parameter (RES*).
type AuthenticationResponse struct {
	RES []byte // authentication response parameter (RES*), IEI 0x2D
	EAP []byte // optional EAP message (IEI 0x78)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain AUTHENTICATION RESPONSE message.
// The encoding is appended to b.
func (m *AuthenticationResponse) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgAuthenticationResponse)

	if m.RES != nil {
		o.TLV(ieiAuthResponseParam, m.RES)
	}

	if m.EAP != nil {
		o.TLVE(ieiEAPMessage, m.EAP)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *AuthenticationResponse) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseAuthenticationResponse decodes the message.
func ParseAuthenticationResponse(b []byte) (*AuthenticationResponse, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgAuthenticationResponse); err != nil {
		return nil, err
	}

	out := &AuthenticationResponse{}

	_unrec, err := walkOptionalIEs(r, authResponseIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiAuthResponseParam:
			out.RES = value
		case ieiEAPMessage:
			out.EAP = value
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

// AuthenticationFailure is the AUTHENTICATION FAILURE message (TS 24.501
// §8.2.4): a mandatory 5GMM cause and, for a synch failure (#21), the AUTS.
type AuthenticationFailure struct {
	Cause GMMCause
	AUTS  []byte // authentication failure parameter (AUTS), IEI 0x30

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// ParseAuthenticationFailure decodes the message.
func ParseAuthenticationFailure(b []byte) (*AuthenticationFailure, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgAuthenticationFailure); err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &AuthenticationFailure{Cause: GMMCause(cause)}

	_unrec, err := walkOptionalIEs(r, authFailureIEs, func(iei uint8, value []byte) (bool, error) {
		if iei != ieiAuthFailureParam {
			return false, nil
		}

		out.AUTS = value

		return true, nil
	})
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// AppendBinary encodes the plain AUTHENTICATION FAILURE message
// (TS 24.501 §8.2.4). The encoding is appended to b.
func (m *AuthenticationFailure) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgAuthenticationFailure)
	w.U8(uint8(m.Cause))

	if m.AUTS != nil {
		o.TLV(ieiAuthFailureParam, m.AUTS)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *AuthenticationFailure) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

var (
	authResponseIEs = []nas.OptionalIE{
		{IEI: ieiAuthResponseParam, Format: nas.IETLV, Name: "Authentication response parameter"},
		{IEI: ieiEAPMessage, Format: nas.IETLVE, Name: "EAP message"},
	}
	authFailureIEs = []nas.OptionalIE{
		{IEI: ieiAuthFailureParam, Format: nas.IETLV, Name: "Authentication failure parameter"},
	}
)
