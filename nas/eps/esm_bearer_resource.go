// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas"

// BearerResourceAllocationRequest is the BEARER RESOURCE ALLOCATION REQUEST
// (TS 24.301 §8.3.8).
type BearerResourceAllocationRequest struct {
	EPSBearerIdentity       EPSBearerIdentity
	PTI                     nas.ProcedureTransactionIdentity
	LinkedEPSBearerIdentity EPSBearerIdentity

	// TrafficFlowAggregate is the traffic flow aggregate description value
	// (TS 24.301 §9.9.4.15), which TS 24.008 §10.5.6.12 codes as a traffic flow
	// template. This codec carries it verbatim.
	TrafficFlowAggregate []byte

	RequiredTrafficFlowQoS EPSQoS

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
	w.U8(uint8(m.LinkedEPSBearerIdentity) & 0x0F) // linked EPS bearer identity | spare half octet
	w.LV(m.TrafficFlowAggregate)

	qos, err := m.RequiredTrafficFlowQoS.MarshalBinary()
	if err != nil {
		return b, err
	}

	w.LV(qos)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
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

	linked, err := r.U8()
	if err != nil {
		return nil, err
	}

	aggregate, err := r.LV()
	if err != nil {
		return nil, err
	}

	qosValue, err := r.LV()
	if err != nil {
		return nil, err
	}

	qos, err := ParseEPSQoS(qosValue)
	if err != nil {
		return nil, err
	}

	out := &BearerResourceAllocationRequest{
		EPSBearerIdentity:       ebi,
		PTI:                     pti,
		LinkedEPSBearerIdentity: EPSBearerIdentity(linked & 0x0F),
		TrafficFlowAggregate:    aggregate,
		RequiredTrafficFlowQoS:  qos,
	}

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

	return messageResult(w, b)
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
// (TS 24.301 §8.3.10).
type BearerResourceModificationRequest struct {
	EPSBearerIdentity EPSBearerIdentity
	PTI               nas.ProcedureTransactionIdentity

	// EPSBearerIdentityForPacketFilter identifies the bearer the packet filters
	// of TrafficFlowAggregate belong to (TS 24.301 table 8.3.10.1).
	EPSBearerIdentityForPacketFilter EPSBearerIdentity

	// TrafficFlowAggregate is the traffic flow aggregate description value
	// (TS 24.301 §9.9.4.15), which TS 24.008 §10.5.6.12 codes as a traffic flow
	// template. This codec carries it verbatim.
	TrafficFlowAggregate []byte

	RequiredTrafficFlowQoS *EPSQoS   // optional (IEI 0x5B)
	Cause                  *ESMCause // optional (IEI 0x58)

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
	w.U8(uint8(m.EPSBearerIdentityForPacketFilter) & 0x0F) // EPS bearer identity for packet filter | spare half octet
	w.LV(m.TrafficFlowAggregate)

	if m.RequiredTrafficFlowQoS != nil {
		qos, err := m.RequiredTrafficFlowQoS.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiRequiredTrafficFlowQoS, qos)
	}

	if m.Cause != nil {
		o.TV3(ieiESMCause, []byte{uint8(*m.Cause)})
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *BearerResourceModificationRequest) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// bearerResourceModificationRequestIEs are the optional elements of the BEARER
// RESOURCE MODIFICATION REQUEST (TS 24.301 table 8.3.10.1), in message order. The
// ESM cause is a full-octet TV, which the walk cannot delimit on its own.
var bearerResourceModificationRequestIEs = []nas.OptionalIE{
	{IEI: ieiRequiredTrafficFlowQoS, Format: nas.IETLV, Name: "Required traffic flow QoS"},
	{IEI: ieiESMCause, Format: nas.IETV3, Len: 1, Name: "ESM cause"},
}

// ParseBearerResourceModificationRequest decodes the message.
func ParseBearerResourceModificationRequest(b []byte) (*BearerResourceModificationRequest, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgBearerResourceModificationRequest)
	if err != nil {
		return nil, err
	}

	linked, err := r.U8()
	if err != nil {
		return nil, err
	}

	aggregate, err := r.LV()
	if err != nil {
		return nil, err
	}

	out := &BearerResourceModificationRequest{
		EPSBearerIdentity:                ebi,
		PTI:                              pti,
		EPSBearerIdentityForPacketFilter: EPSBearerIdentity(linked & 0x0F),
		TrafficFlowAggregate:             aggregate,
	}

	_unrec, err := walkOptionalIEs(r, bearerResourceModificationRequestIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiRequiredTrafficFlowQoS:
			parsed, err := ParseEPSQoS(value)
			if err != nil {
				return false, err
			}

			out.RequiredTrafficFlowQoS = &parsed
		case ieiESMCause:
			if len(value) == 0 {
				return false, nil
			}

			cause := ESMCause(value[0])
			out.Cause = &cause
		default:
			return false, nil
		}

		return true, nil
	})
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

	return messageResult(w, b)
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
