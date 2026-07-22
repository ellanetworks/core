// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// AuthenticationRequest is the AUTHENTICATION REQUEST message (TS 24.501
// §8.2.1): the ngKSI, the ABBA, and the 5G-AKA challenge (RAND and AUTN).
type AuthenticationRequest struct {
	NgKSI uint8 // ngKSI half octet (bits 1-4): TSC and NAS key set identifier
	ABBA  []byte
	RAND  *[16]byte
	AUTN  *[16]byte
}

// Marshal encodes the plain AUTHENTICATION REQUEST message.
func (m *AuthenticationRequest) Marshal() ([]byte, error) {
	var w common.Writer

	writeGMMHeader(&w, MsgAuthenticationRequest)
	w.U8(m.NgKSI & 0x0F) // spare half octet in bits 5-8

	if err := w.LV(m.ABBA); err != nil {
		return nil, err
	}

	if m.RAND != nil {
		w.U8(ieiRAND)
		w.Raw(m.RAND[:])
	}

	if m.AUTN != nil {
		writeTLV(&w, ieiAUTN, m.AUTN[:])
	}

	return w.Bytes(), nil
}

// AuthenticationReject is the AUTHENTICATION REJECT message (TS 24.501 §8.2.5):
// the 5GMM header alone (Ella carries no EAP message).
type AuthenticationReject struct{}

// Marshal encodes the plain AUTHENTICATION REJECT message.
func (m *AuthenticationReject) Marshal() ([]byte, error) {
	var w common.Writer

	writeGMMHeader(&w, MsgAuthenticationReject)

	return w.Bytes(), nil
}

// AuthenticationResponse is the AUTHENTICATION RESPONSE message (TS 24.501
// §8.2.2): the optional authentication response parameter (RES*).
type AuthenticationResponse struct {
	RES []byte // authentication response parameter (RES*), IEI 0x2D
}

// ParseAuthenticationResponse decodes the message.
func ParseAuthenticationResponse(b []byte) (*AuthenticationResponse, error) {
	r := common.NewReader(b)

	if err := readGMMHeader(r, MsgAuthenticationResponse); err != nil {
		return nil, err
	}

	out := &AuthenticationResponse{}

	_, err := common.WalkOptionalIEs(r, authResponseIEs, func(iei uint8, value []byte) error {
		if iei == ieiAuthResponseParam {
			out.RES = value
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

// AuthenticationFailure is the AUTHENTICATION FAILURE message (TS 24.501
// §8.2.4): a mandatory 5GMM cause and, for a synch failure (#21), the AUTS.
type AuthenticationFailure struct {
	Cause uint8
	AUTS  []byte // authentication failure parameter (AUTS), IEI 0x30
}

// ParseAuthenticationFailure decodes the message.
func ParseAuthenticationFailure(b []byte) (*AuthenticationFailure, error) {
	r := common.NewReader(b)

	if err := readGMMHeader(r, MsgAuthenticationFailure); err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &AuthenticationFailure{Cause: cause}

	_, err = common.WalkOptionalIEs(r, authFailureIEs, func(iei uint8, value []byte) error {
		if iei == ieiAuthFailureParam {
			out.AUTS = value
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

var (
	authResponseIEs = []common.OptionalIE{
		{IEI: ieiAuthResponseParam, Format: common.IETLV},
		{IEI: ieiEAPMessage, Format: common.IETLVE},
	}
	authFailureIEs = []common.OptionalIE{
		{IEI: ieiAuthFailureParam, Format: common.IETLV},
	}
)
