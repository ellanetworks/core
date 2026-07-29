// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas"

// EMMStatus is the EMM STATUS message, reporting an error condition detected on
// received EMM protocol data (TS 24.301 §5.7). It may be sent by both the MME and
// the UE and triggers no state transition on receipt.
type EMMStatus struct {
	Cause EMMCause

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged. The spec defines none for this message, but a later release may.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain EMM STATUS message.
// The encoding is appended to b.
func (m *EMMStatus) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgEMMStatus)
	w.U8(uint8(m.Cause))

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *EMMStatus) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseEMMStatus decodes the EMM STATUS message.
func ParseEMMStatus(b []byte) (*EMMStatus, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgEMMStatus); err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &EMMStatus{Cause: EMMCause(cause)}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}
