// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas"

// BearerResourceAllocationRequest is the BEARER RESOURCE ALLOCATION REQUEST
// (TS 24.301 §8.3.8). Only the ESM header is modeled: the request is rejected
// unconditionally, so its traffic-flow and QoS body is not decoded.
type BearerResourceAllocationRequest struct {
	EPSBearerIdentity EPSBearerIdentity
	PTI               nas.ProcedureTransactionIdentity

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain BEARER RESOURCE ALLOCATION REQUEST message.
// The encoding is appended to b.
func (m *BearerResourceAllocationRequest) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgBearerResourceAllocationRequest)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *BearerResourceAllocationRequest) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseBearerResourceAllocationRequest decodes the message.
func ParseBearerResourceAllocationRequest(b []byte) (*BearerResourceAllocationRequest, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgBearerResourceAllocationRequest)
	if err != nil {
		return nil, err
	}

	out := &BearerResourceAllocationRequest{EPSBearerIdentity: ebi, PTI: pti}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// BearerResourceAllocationReject is the BEARER RESOURCE ALLOCATION REJECT
// (TS 24.301 §8.3.7).
type BearerResourceAllocationReject struct {
	EPSBearerIdentity EPSBearerIdentity
	PTI               nas.ProcedureTransactionIdentity
	Cause             ESMCause

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain BEARER RESOURCE ALLOCATION REJECT message.
// The encoding is appended to b.
func (m *BearerResourceAllocationReject) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgBearerResourceAllocationReject)
	w.U8(uint8(m.Cause))

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *BearerResourceAllocationReject) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseBearerResourceAllocationReject decodes the message.
func ParseBearerResourceAllocationReject(b []byte) (*BearerResourceAllocationReject, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgBearerResourceAllocationReject)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &BearerResourceAllocationReject{
		EPSBearerIdentity: ebi, PTI: pti, Cause: ESMCause(cause),
	}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// BearerResourceModificationRequest is the BEARER RESOURCE MODIFICATION REQUEST
// (TS 24.301 §8.3.10). Only the ESM header is modeled: the request is rejected
// unconditionally, so its traffic-flow and QoS body is not decoded.
type BearerResourceModificationRequest struct {
	EPSBearerIdentity EPSBearerIdentity
	PTI               nas.ProcedureTransactionIdentity

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain BEARER RESOURCE MODIFICATION REQUEST message.
// The encoding is appended to b.
func (m *BearerResourceModificationRequest) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgBearerResourceModificationRequest)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *BearerResourceModificationRequest) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// bearerResourceModificationRequestIEs declares the framing of the one optional
// element of the BEARER RESOURCE MODIFICATION REQUEST (TS 24.301 table 8.3.10.1)
// that the walk cannot delimit on its own: the ESM cause is a full-octet TV.
var bearerResourceModificationRequestIEs = []nas.OptionalIE{
	{IEI: ieiESMCause, Format: nas.IETV3, Len: 1, Name: "ESM cause"},
}

// ParseBearerResourceModificationRequest decodes the message.
func ParseBearerResourceModificationRequest(b []byte) (*BearerResourceModificationRequest, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgBearerResourceModificationRequest)
	if err != nil {
		return nil, err
	}

	out := &BearerResourceModificationRequest{EPSBearerIdentity: ebi, PTI: pti}

	_unrec, err := walkOptionalIEs(r, bearerResourceModificationRequestIEs, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// BearerResourceModificationReject is the BEARER RESOURCE MODIFICATION REJECT
// (TS 24.301 §8.3.9).
type BearerResourceModificationReject struct {
	EPSBearerIdentity EPSBearerIdentity
	PTI               nas.ProcedureTransactionIdentity
	Cause             ESMCause

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain BEARER RESOURCE MODIFICATION REJECT message.
// The encoding is appended to b.
func (m *BearerResourceModificationReject) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgBearerResourceModificationReject)
	w.U8(uint8(m.Cause))

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *BearerResourceModificationReject) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseBearerResourceModificationReject decodes the message.
func ParseBearerResourceModificationReject(b []byte) (*BearerResourceModificationReject, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgBearerResourceModificationReject)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &BearerResourceModificationReject{
		EPSBearerIdentity: ebi, PTI: pti, Cause: ESMCause(cause),
	}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}
