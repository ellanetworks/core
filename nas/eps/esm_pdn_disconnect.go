// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas"

// PDNDisconnectRequest is the PDN DISCONNECT REQUEST message (TS 24.301),
// sent by the UE to release one of its PDN connections. The header EPS
// bearer identity is "no bearer assigned" (0); the PDN to disconnect is named by
// the Linked EPS Bearer Identity (the default bearer of that PDN connection).
type PDNDisconnectRequest struct {
	EPSBearerIdentity       EPSBearerIdentity
	PTI                     nas.ProcedureTransactionIdentity
	LinkedEPSBearerIdentity EPSBearerIdentity

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the PDN DISCONNECT REQUEST message.
// The encoding is appended to b.
func (m *PDNDisconnectRequest) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgPDNDisconnectRequest)
	w.U8(uint8(m.LinkedEPSBearerIdentity) & 0x0F)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *PDNDisconnectRequest) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParsePDNDisconnectRequest decodes the message. The Linked EPS Bearer Identity
// is the low half-octet of the octet after the header (the high half-octet is
// spare, TS 24.301).
func ParsePDNDisconnectRequest(b []byte) (*PDNDisconnectRequest, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgPDNDisconnectRequest)
	if err != nil {
		return nil, err
	}

	linked, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &PDNDisconnectRequest{
		EPSBearerIdentity:       ebi,
		PTI:                     pti,
		LinkedEPSBearerIdentity: EPSBearerIdentity(linked & 0x0F),
	}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// PDNDisconnectReject is the PDN DISCONNECT REJECT message (TS 24.301),
// sent by the network when it cannot honour a PDN disconnect request (e.g. the
// linked bearer is the last PDN connection, ESM cause #49).
type PDNDisconnectReject struct {
	EPSBearerIdentity EPSBearerIdentity
	PTI               nas.ProcedureTransactionIdentity
	Cause             ESMCause

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the PDN DISCONNECT REJECT message.
// The encoding is appended to b.
func (m *PDNDisconnectReject) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgPDNDisconnectReject)
	w.U8(uint8(m.Cause))

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *PDNDisconnectReject) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParsePDNDisconnectReject decodes the message.
func ParsePDNDisconnectReject(b []byte) (*PDNDisconnectReject, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgPDNDisconnectReject)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &PDNDisconnectReject{
		EPSBearerIdentity: ebi,
		PTI:               pti,
		Cause:             ESMCause(cause),
	}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}
