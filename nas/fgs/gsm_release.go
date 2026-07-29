// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas"

// PDUSessionReleaseCommand is the PDU SESSION RELEASE COMMAND (TS 24.501
// §8.3.14): the 5GSM header followed by a mandatory 5GSM cause. PTI is the
// UE-allocated value for a UE-requested release or 0 for a network-requested
// release.
type PDUSessionReleaseCommand struct {
	PDUSessionID PDUSessionID
	PTI          nas.ProcedureTransactionIdentity
	Cause        GSMCause

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain PDU SESSION RELEASE COMMAND message.
// The encoding is appended to b.
func (m *PDUSessionReleaseCommand) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGSMHeader(w, m.PDUSessionID, m.PTI, MsgPDUSessionReleaseCommand)
	w.U8(uint8(m.Cause))

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *PDUSessionReleaseCommand) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParsePDUSessionReleaseCommand decodes a PDU SESSION RELEASE COMMAND.
func ParsePDUSessionReleaseCommand(b []byte) (*PDUSessionReleaseCommand, error) {
	r := nas.NewReader(b)

	pduSessionID, pti, err := readGSMHeader(r, MsgPDUSessionReleaseCommand)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &PDUSessionReleaseCommand{
		PDUSessionID: pduSessionID,
		PTI:          pti,
		Cause:        GSMCause(cause),
	}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// PDUSessionReleaseRequest is the PDU SESSION RELEASE REQUEST (TS 24.501
// §8.3.12): the 5GSM header and an optional 5GSM cause.
type PDUSessionReleaseRequest struct {
	PDUSessionID PDUSessionID
	PTI          nas.ProcedureTransactionIdentity
	Cause        *GSMCause // optional (IEI 0x59)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// ParsePDUSessionReleaseRequest decodes the message.
func ParsePDUSessionReleaseRequest(b []byte) (*PDUSessionReleaseRequest, error) {
	r := nas.NewReader(b)

	psi, pti, err := readGSMHeader(r, MsgPDUSessionReleaseRequest)
	if err != nil {
		return nil, err
	}

	out := &PDUSessionReleaseRequest{PDUSessionID: psi, PTI: pti}

	unrec, err := walkOptionalIEs(r, causeAndPCOIEs, causeCollector(&out.Cause))
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = unrec

	return out, err
}

// AppendBinary encodes the plain PDU SESSION RELEASE REQUEST message
// (TS 24.501 §8.3.13). The encoding is appended to b.
func (m *PDUSessionReleaseRequest) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGSMHeader(w, m.PDUSessionID, m.PTI, MsgPDUSessionReleaseRequest)

	if m.Cause != nil {
		o.TV3(iei5GSMCause, []byte{uint8(*m.Cause)})
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *PDUSessionReleaseRequest) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// PDUSessionReleaseComplete is the PDU SESSION RELEASE COMPLETE (TS 24.501
// §8.3.15): the 5GSM header and an optional 5GSM cause.
type PDUSessionReleaseComplete struct {
	PDUSessionID PDUSessionID
	PTI          nas.ProcedureTransactionIdentity
	Cause        *GSMCause // optional (IEI 0x59)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// ParsePDUSessionReleaseComplete decodes the message.
func ParsePDUSessionReleaseComplete(b []byte) (*PDUSessionReleaseComplete, error) {
	r := nas.NewReader(b)

	psi, pti, err := readGSMHeader(r, MsgPDUSessionReleaseComplete)
	if err != nil {
		return nil, err
	}

	out := &PDUSessionReleaseComplete{PDUSessionID: psi, PTI: pti}

	unrec, err := walkOptionalIEs(r, causeAndPCOIEs, causeCollector(&out.Cause))
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = unrec

	return out, err
}

// AppendBinary encodes the plain PDU SESSION RELEASE COMPLETE message
// (TS 24.501 §8.3.15). The encoding is appended to b.
func (m *PDUSessionReleaseComplete) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGSMHeader(w, m.PDUSessionID, m.PTI, MsgPDUSessionReleaseComplete)

	if m.Cause != nil {
		o.TV3(iei5GSMCause, []byte{uint8(*m.Cause)})
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *PDUSessionReleaseComplete) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// causeCollector returns an emit callback that stores the optional 5GSM cause
// value (IEI 0x59) into dst, ignoring all other optional IEs.
func causeCollector(dst **GSMCause) func(iei uint8, value []byte) (bool, error) {
	return func(iei uint8, value []byte) (bool, error) {
		if iei != iei5GSMCause {
			return false, nil
		}

		if len(value) == 0 {
			return false, nil
		}

		cause := GSMCause(value[0])
		*dst = &cause

		return true, nil
	}
}
