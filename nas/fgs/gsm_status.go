// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas"

// GSMStatus is the 5GSM STATUS message (TS 24.501 §8.3.16): the 5GSM header
// followed by a mandatory 5GSM cause, reporting an error condition detected on a
// PDU session. It may be sent by both the UE and the network.
type GSMStatus struct {
	PDUSessionID PDUSessionID
	PTI          nas.ProcedureTransactionIdentity
	Cause        GSMCause

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged. The spec defines none for this message, but a later release may.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain 5GSM STATUS message.
// The encoding is appended to b.
func (m *GSMStatus) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGSMHeader(w, m.PDUSessionID, m.PTI, MsgGSMStatus)
	w.U8(uint8(m.Cause))

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *GSMStatus) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseGSMStatus decodes the 5GSM STATUS message.
func ParseGSMStatus(b []byte) (*GSMStatus, error) {
	r := nas.NewReader(b)

	psi, pti, err := readGSMHeader(r, MsgGSMStatus)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &GSMStatus{PDUSessionID: psi, PTI: pti, Cause: GSMCause(cause)}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}
