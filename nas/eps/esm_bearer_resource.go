// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas/common"

// BearerResourceAllocationRequest is the BEARER RESOURCE ALLOCATION REQUEST
// (TS 24.301 §8.3.8), the UE's request to allocate a dedicated bearer resource.
// Ella Core rejects the procedure unconditionally — the bearer QoS is
// network-determined and not modifiable on UE request — so only the ESM header,
// which the reject echoes and validates, is modeled; the traffic-flow-aggregate
// and QoS body is left undecoded.
type BearerResourceAllocationRequest struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
}

// Marshal encodes the ESM header of a BEARER RESOURCE ALLOCATION REQUEST.
func (m *BearerResourceAllocationRequest) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgBearerResourceAllocationRequest)

	return w.Bytes(), nil
}

// ParseBearerResourceAllocationRequest decodes the ESM header of the message.
func ParseBearerResourceAllocationRequest(b []byte) (*BearerResourceAllocationRequest, error) {
	r := common.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgBearerResourceAllocationRequest)
	if err != nil {
		return nil, err
	}

	return &BearerResourceAllocationRequest{EPSBearerIdentity: ebi, ProcedureTransactionIdentity: pti}, nil
}

// BearerResourceAllocationReject is the BEARER RESOURCE ALLOCATION REJECT
// (TS 24.301 §8.3.7), the network's refusal of a UE-requested dedicated-bearer
// allocation, carrying a mandatory ESM cause.
type BearerResourceAllocationReject struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
	ESMCause                     uint8
}

// Marshal encodes the BEARER RESOURCE ALLOCATION REJECT message.
func (m *BearerResourceAllocationReject) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgBearerResourceAllocationReject)
	w.U8(m.ESMCause)

	return w.Bytes(), nil
}

// ParseBearerResourceAllocationReject decodes the message.
func ParseBearerResourceAllocationReject(b []byte) (*BearerResourceAllocationReject, error) {
	r := common.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgBearerResourceAllocationReject)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	return &BearerResourceAllocationReject{
		EPSBearerIdentity: ebi, ProcedureTransactionIdentity: pti, ESMCause: cause,
	}, nil
}

// BearerResourceModificationRequest is the BEARER RESOURCE MODIFICATION REQUEST
// (TS 24.301 §8.3.10), the UE's request to modify a dedicated bearer resource.
// Ella Core rejects the procedure unconditionally, so only the ESM header is
// modeled; the traffic-flow-aggregate and QoS body is left undecoded.
type BearerResourceModificationRequest struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
}

// Marshal encodes the ESM header of a BEARER RESOURCE MODIFICATION REQUEST.
func (m *BearerResourceModificationRequest) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgBearerResourceModificationRequest)

	return w.Bytes(), nil
}

// ParseBearerResourceModificationRequest decodes the ESM header of the message.
func ParseBearerResourceModificationRequest(b []byte) (*BearerResourceModificationRequest, error) {
	r := common.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgBearerResourceModificationRequest)
	if err != nil {
		return nil, err
	}

	return &BearerResourceModificationRequest{EPSBearerIdentity: ebi, ProcedureTransactionIdentity: pti}, nil
}

// BearerResourceModificationReject is the BEARER RESOURCE MODIFICATION REJECT
// (TS 24.301 §8.3.9), the network's refusal of a UE-requested dedicated-bearer
// modification, carrying a mandatory ESM cause.
type BearerResourceModificationReject struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
	ESMCause                     uint8
}

// Marshal encodes the BEARER RESOURCE MODIFICATION REJECT message.
func (m *BearerResourceModificationReject) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgBearerResourceModificationReject)
	w.U8(m.ESMCause)

	return w.Bytes(), nil
}

// ParseBearerResourceModificationReject decodes the message.
func ParseBearerResourceModificationReject(b []byte) (*BearerResourceModificationReject, error) {
	r := common.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgBearerResourceModificationReject)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	return &BearerResourceModificationReject{
		EPSBearerIdentity: ebi, ProcedureTransactionIdentity: pti, ESMCause: cause,
	}, nil
}
