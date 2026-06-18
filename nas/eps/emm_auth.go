// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas/common"

// ieiAuthFailureParameter is the IEI of the optional Authentication failure
// parameter (AUTS) in AUTHENTICATION FAILURE (TS 24.301 §8.2.5). It is a type-4
// TLV IE (TS 24.301 §9.9.3.1).
const ieiAuthFailureParameter uint8 = 0x30

// authenticationFailureIEs are the optional IEs of an AUTHENTICATION FAILURE
// (TS 24.301 §8.2.5): the Authentication failure parameter (AUTS).
var authenticationFailureIEs = []common.OptionalIE{
	{IEI: ieiAuthFailureParameter, Format: common.IETLV},
}

// AuthenticationRequest is the AUTHENTICATION REQUEST message (TS 24.301 §8.2.7),
// sent by the MME with the EPS-AKA challenge.
type AuthenticationRequest struct {
	NASKeySetIdentifier uint8
	RAND                [16]byte
	AUTN                []byte
}

// Marshal encodes the plain AUTHENTICATION REQUEST message.
func (m *AuthenticationRequest) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgAuthenticationRequest)
	w.U8(m.NASKeySetIdentifier & 0x0F) // spare half octet | NAS KSI
	w.Raw(m.RAND[:])

	if err := w.LV(m.AUTN); err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

// ParseAuthenticationRequest decodes a plain AUTHENTICATION REQUEST message.
func ParseAuthenticationRequest(b []byte) (*AuthenticationRequest, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgAuthenticationRequest); err != nil {
		return nil, err
	}

	ksi, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &AuthenticationRequest{NASKeySetIdentifier: ksi & 0x0F}

	rand, err := r.Bytes(16)
	if err != nil {
		return nil, err
	}

	copy(m.RAND[:], rand)

	if m.AUTN, err = r.LV(); err != nil {
		return nil, err
	}

	return m, nil
}

// AuthenticationResponse is the AUTHENTICATION RESPONSE message (TS 24.301
// §8.2.8), carrying the UE's RES.
type AuthenticationResponse struct {
	RES []byte
}

// Marshal encodes the plain AUTHENTICATION RESPONSE message.
func (m *AuthenticationResponse) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgAuthenticationResponse)

	if err := w.LV(m.RES); err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

// ParseAuthenticationResponse decodes a plain AUTHENTICATION RESPONSE message.
func ParseAuthenticationResponse(b []byte) (*AuthenticationResponse, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgAuthenticationResponse); err != nil {
		return nil, err
	}

	res, err := r.LV()
	if err != nil {
		return nil, err
	}

	return &AuthenticationResponse{RES: res}, nil
}

// AuthenticationReject is the AUTHENTICATION REJECT message (TS 24.301 §8.2.6).
// It has no mandatory information elements.
type AuthenticationReject struct{}

// Marshal encodes the plain AUTHENTICATION REJECT message.
func (m *AuthenticationReject) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgAuthenticationReject)

	return w.Bytes(), nil
}

// ParseAuthenticationReject decodes a plain AUTHENTICATION REJECT message.
func ParseAuthenticationReject(b []byte) (*AuthenticationReject, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgAuthenticationReject); err != nil {
		return nil, err
	}

	return &AuthenticationReject{}, nil
}

// AuthenticationFailure is the AUTHENTICATION FAILURE message (TS 24.301 §8.2.5).
// AUTS is the optional Authentication failure parameter (present for a
// "synch failure"); it is nil when absent.
type AuthenticationFailure struct {
	Cause uint8
	AUTS  []byte
}

// Marshal encodes the plain AUTHENTICATION FAILURE message.
func (m *AuthenticationFailure) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgAuthenticationFailure)
	w.U8(m.Cause)

	if m.AUTS != nil {
		w.U8(ieiAuthFailureParameter)

		if err := w.LV(m.AUTS); err != nil {
			return nil, err
		}
	}

	return w.Bytes(), nil
}

// ParseAuthenticationFailure decodes a plain AUTHENTICATION FAILURE message.
func ParseAuthenticationFailure(b []byte) (*AuthenticationFailure, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgAuthenticationFailure); err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &AuthenticationFailure{Cause: cause}

	if _, err := common.WalkOptionalIEs(r, authenticationFailureIEs, func(iei uint8, value []byte) error {
		if iei == ieiAuthFailureParameter {
			m.AUTS = value
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return m, nil
}
