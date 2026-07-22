// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// IdentityRequest is the IDENTITY REQUEST message (TS 24.501 §8.2.21): the 5GS
// identity type the network asks the UE to provide.
type IdentityRequest struct {
	IdentityType uint8 // 5GS identity type, bits 1-3
}

// Marshal encodes the plain IDENTITY REQUEST message.
func (m *IdentityRequest) Marshal() ([]byte, error) {
	var w common.Writer

	writeGMMHeader(&w, MsgIdentityRequest)
	w.U8(m.IdentityType & 0x07) // spare half octet in bits 5-8, bit 4 spare

	return w.Bytes(), nil
}

// IdentityResponse is the IDENTITY RESPONSE message (TS 24.501 §8.2.22): the 5GS
// mobile identity carried as a type-6 LV-E.
type IdentityResponse struct {
	MobileIdentity []byte
}

// ParseIdentityResponse decodes the message.
func ParseIdentityResponse(b []byte) (*IdentityResponse, error) {
	r := common.NewReader(b)

	if err := readGMMHeader(r, MsgIdentityResponse); err != nil {
		return nil, err
	}

	mi, err := r.LVE()
	if err != nil {
		return nil, err
	}

	return &IdentityResponse{MobileIdentity: mi}, nil
}
