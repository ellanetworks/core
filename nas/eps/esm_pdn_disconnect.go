// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas/common"

// PDNDisconnectRequest is the PDN DISCONNECT REQUEST message (TS 24.301),
// sent by the UE to release one of its PDN connections. The header EPS
// bearer identity is "no bearer assigned" (0); the PDN to disconnect is named by
// the Linked EPS Bearer Identity (the default bearer of that PDN connection).
type PDNDisconnectRequest struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
	LinkedEPSBearerIdentity      uint8
}

// Marshal encodes the PDN DISCONNECT REQUEST message.
func (m *PDNDisconnectRequest) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgPDNDisconnectRequest)
	w.U8(m.LinkedEPSBearerIdentity & 0x0F)

	return w.Bytes(), nil
}

// ParsePDNDisconnectRequest decodes the message. The Linked EPS Bearer Identity
// is the low half-octet of the octet after the header (the high half-octet is
// spare, TS 24.301).
func ParsePDNDisconnectRequest(b []byte) (*PDNDisconnectRequest, error) {
	r := common.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgPDNDisconnectRequest)
	if err != nil {
		return nil, err
	}

	linked, err := r.U8()
	if err != nil {
		return nil, err
	}

	return &PDNDisconnectRequest{
		EPSBearerIdentity:            ebi,
		ProcedureTransactionIdentity: pti,
		LinkedEPSBearerIdentity:      linked & 0x0F,
	}, nil
}

// PDNDisconnectReject is the PDN DISCONNECT REJECT message (TS 24.301),
// sent by the network when it cannot honour a PDN disconnect request (e.g. the
// linked bearer is the last PDN connection, ESM cause #49).
type PDNDisconnectReject struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
	ESMCause                     uint8
}

// Marshal encodes the PDN DISCONNECT REJECT message.
func (m *PDNDisconnectReject) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgPDNDisconnectReject)
	w.U8(m.ESMCause)

	return w.Bytes(), nil
}

// ParsePDNDisconnectReject decodes the message.
func ParsePDNDisconnectReject(b []byte) (*PDNDisconnectReject, error) {
	r := common.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgPDNDisconnectReject)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	return &PDNDisconnectReject{
		EPSBearerIdentity:            ebi,
		ProcedureTransactionIdentity: pti,
		ESMCause:                     cause,
	}, nil
}
