// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas/common"

// IdentityRequest is the IDENTITY REQUEST message (TS 24.301 §8.2.18). The
// identity type selects which identity the network wants (e.g. 1 = IMSI).
type IdentityRequest struct {
	IdentityType uint8
}

// Marshal encodes the plain IDENTITY REQUEST message.
func (m *IdentityRequest) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgIdentityRequest)
	w.U8(m.IdentityType & 0x07) // identity type | spare half octet

	return w.Bytes(), nil
}

// ParseIdentityRequest decodes a plain IDENTITY REQUEST message.
func ParseIdentityRequest(b []byte) (*IdentityRequest, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgIdentityRequest); err != nil {
		return nil, err
	}

	octet, err := r.U8()
	if err != nil {
		return nil, err
	}

	return &IdentityRequest{IdentityType: octet & 0x07}, nil
}

// IdentityResponse is the IDENTITY RESPONSE message (TS 24.301 §8.2.19). The
// Mobile identity is kept as its raw value part (TS 24.008 §10.5.1.4 coding).
type IdentityResponse struct {
	MobileIdentity []byte
}

// Marshal encodes the plain IDENTITY RESPONSE message.
func (m *IdentityResponse) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgIdentityResponse)

	if err := w.LV(m.MobileIdentity); err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

// ParseIdentityResponse decodes a plain IDENTITY RESPONSE message.
func ParseIdentityResponse(b []byte) (*IdentityResponse, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgIdentityResponse); err != nil {
		return nil, err
	}

	id, err := r.LV()
	if err != nil {
		return nil, err
	}

	return &IdentityResponse{MobileIdentity: id}, nil
}
