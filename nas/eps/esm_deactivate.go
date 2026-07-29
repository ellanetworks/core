// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas"

// DeactivateEPSBearerContextRequest is the DEACTIVATE EPS BEARER CONTEXT REQUEST
// (TS 24.301): the ESM header followed by a mandatory ESM cause. Cause
// #39 "reactivation requested" tells the UE to deactivate the bearer and
// re-establish the PDN connection with the network's updated configuration
// (TS 24.301).
type DeactivateEPSBearerContextRequest struct {
	EPSBearerIdentity EPSBearerIdentity
	PTI               nas.ProcedureTransactionIdentity
	Cause             ESMCause

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the DEACTIVATE EPS BEARER CONTEXT REQUEST.
// The encoding is appended to b.
func (m *DeactivateEPSBearerContextRequest) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgDeactivateEPSBearerContextRequest)
	w.U8(uint8(m.Cause))

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *DeactivateEPSBearerContextRequest) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseDeactivateEPSBearerContextRequest decodes the message.
func ParseDeactivateEPSBearerContextRequest(b []byte) (*DeactivateEPSBearerContextRequest, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgDeactivateEPSBearerContextRequest)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &DeactivateEPSBearerContextRequest{
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

// DeactivateEPSBearerContextAccept is the DEACTIVATE EPS BEARER CONTEXT ACCEPT
// (TS 24.301): the UE's acknowledgement of the deactivation, carrying no
// mandatory information beyond the ESM header.
type DeactivateEPSBearerContextAccept struct {
	EPSBearerIdentity EPSBearerIdentity
	PTI               nas.ProcedureTransactionIdentity

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the DEACTIVATE EPS BEARER CONTEXT ACCEPT.
// The encoding is appended to b.
func (m *DeactivateEPSBearerContextAccept) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgDeactivateEPSBearerContextAccept)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *DeactivateEPSBearerContextAccept) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseDeactivateEPSBearerContextAccept decodes the message.
func ParseDeactivateEPSBearerContextAccept(b []byte) (*DeactivateEPSBearerContextAccept, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgDeactivateEPSBearerContextAccept)
	if err != nil {
		return nil, err
	}

	out := &DeactivateEPSBearerContextAccept{
		EPSBearerIdentity: ebi,
		PTI:               pti,
	}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}
