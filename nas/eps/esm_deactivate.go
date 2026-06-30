// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas/common"

// DeactivateEPSBearerContextRequest is the DEACTIVATE EPS BEARER CONTEXT REQUEST
// (TS 24.301): the ESM header followed by a mandatory ESM cause. Cause
// #39 "reactivation requested" tells the UE to deactivate the bearer and
// re-establish the PDN connection with the network's updated configuration
// (TS 24.301).
type DeactivateEPSBearerContextRequest struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
	ESMCause                     uint8
}

// Marshal encodes the DEACTIVATE EPS BEARER CONTEXT REQUEST.
func (m *DeactivateEPSBearerContextRequest) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgDeactivateEPSBearerContextRequest)
	w.U8(m.ESMCause)

	return w.Bytes(), nil
}

// ParseDeactivateEPSBearerContextRequest decodes the message.
func ParseDeactivateEPSBearerContextRequest(b []byte) (*DeactivateEPSBearerContextRequest, error) {
	r := common.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgDeactivateEPSBearerContextRequest)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	return &DeactivateEPSBearerContextRequest{
		EPSBearerIdentity:            ebi,
		ProcedureTransactionIdentity: pti,
		ESMCause:                     cause,
	}, nil
}

// DeactivateEPSBearerContextAccept is the DEACTIVATE EPS BEARER CONTEXT ACCEPT
// (TS 24.301): the UE's acknowledgement of the deactivation, carrying no
// mandatory information beyond the ESM header.
type DeactivateEPSBearerContextAccept struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
}

// Marshal encodes the DEACTIVATE EPS BEARER CONTEXT ACCEPT.
func (m *DeactivateEPSBearerContextAccept) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgDeactivateEPSBearerContextAccept)

	return w.Bytes(), nil
}

// ParseDeactivateEPSBearerContextAccept decodes the message.
func ParseDeactivateEPSBearerContextAccept(b []byte) (*DeactivateEPSBearerContextAccept, error) {
	r := common.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgDeactivateEPSBearerContextAccept)
	if err != nil {
		return nil, err
	}

	return &DeactivateEPSBearerContextAccept{
		EPSBearerIdentity:            ebi,
		ProcedureTransactionIdentity: pti,
	}, nil
}
