// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas/common"

// BearerResourceAllocationRequest is the BEARER RESOURCE ALLOCATION REQUEST
// (TS 24.301 §8.3.8). Only the ESM header is modeled: the request is rejected
// unconditionally, so its traffic-flow and QoS body is not decoded.
type BearerResourceAllocationRequest struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
}

func (m *BearerResourceAllocationRequest) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgBearerResourceAllocationRequest)

	return w.Bytes(), nil
}

func ParseBearerResourceAllocationRequest(b []byte) (*BearerResourceAllocationRequest, error) {
	r := common.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgBearerResourceAllocationRequest)
	if err != nil {
		return nil, err
	}

	return &BearerResourceAllocationRequest{EPSBearerIdentity: ebi, ProcedureTransactionIdentity: pti}, nil
}

// BearerResourceAllocationReject is the BEARER RESOURCE ALLOCATION REJECT
// (TS 24.301 §8.3.7).
type BearerResourceAllocationReject struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
	ESMCause                     uint8
}

func (m *BearerResourceAllocationReject) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgBearerResourceAllocationReject)
	w.U8(m.ESMCause)

	return w.Bytes(), nil
}

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
// (TS 24.301 §8.3.10). Only the ESM header is modeled: the request is rejected
// unconditionally, so its traffic-flow and QoS body is not decoded.
type BearerResourceModificationRequest struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
}

func (m *BearerResourceModificationRequest) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgBearerResourceModificationRequest)

	return w.Bytes(), nil
}

func ParseBearerResourceModificationRequest(b []byte) (*BearerResourceModificationRequest, error) {
	r := common.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgBearerResourceModificationRequest)
	if err != nil {
		return nil, err
	}

	return &BearerResourceModificationRequest{EPSBearerIdentity: ebi, ProcedureTransactionIdentity: pti}, nil
}

// BearerResourceModificationReject is the BEARER RESOURCE MODIFICATION REJECT
// (TS 24.301 §8.3.9).
type BearerResourceModificationReject struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
	ESMCause                     uint8
}

func (m *BearerResourceModificationReject) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgBearerResourceModificationReject)
	w.U8(m.ESMCause)

	return w.Bytes(), nil
}

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
