// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas"

// ESMStatus is the ESM STATUS message (TS 24.301).
type ESMStatus struct {
	EPSBearerIdentity EPSBearerIdentity
	PTI               nas.ProcedureTransactionIdentity
	Cause             ESMCause

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged. The spec defines none for this message, but a later release may.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the ESM STATUS message.
// The encoding is appended to b.
func (m *ESMStatus) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgESMStatus)
	w.U8(uint8(m.Cause))

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *ESMStatus) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseESMStatus decodes the ESM STATUS message.
func ParseESMStatus(b []byte) (*ESMStatus, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgESMStatus)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &ESMStatus{EPSBearerIdentity: ebi, PTI: pti, Cause: ESMCause(cause)}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}
