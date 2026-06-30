// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas/common"

// ModifyEPSBearerContextRequest is the MODIFY EPS BEARER CONTEXT REQUEST
// (TS 24.301): the ESM header followed by entirely optional IEs. The
// network uses it to update an active bearer's parameters in place — the per-APN
// Session-AMBR (APN-AMBR) and/or the Protocol Configuration Options
// carrying a changed DNS server (TS 24.008) — without deactivating the
// bearer.
type ModifyEPSBearerContextRequest struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
	NewEPSQoS                    []byte // EPS QoS value part (QCI), empty = omit
	APNAMBR                      []byte // APN-AMBR value part, empty = omit
	ProtocolConfigurationOptions []byte
}

// modifyEPSBearerContextRequestIEs are the optional IEs Ella Core sends in a
// MODIFY EPS BEARER CONTEXT REQUEST (TS 24.301), in message order.
var modifyEPSBearerContextRequestIEs = []common.OptionalIE{
	{IEI: newEPSQoSIEI, Format: common.IETLV},
	{IEI: apnAMBRIEI, Format: common.IETLV},
	{IEI: protocolConfigurationOptionsIEI, Format: common.IETLV},
}

// Marshal encodes the MODIFY EPS BEARER CONTEXT REQUEST.
func (m *ModifyEPSBearerContextRequest) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgModifyEPSBearerContextRequest)

	if len(m.NewEPSQoS) > 0 {
		w.U8(newEPSQoSIEI)

		if err := w.LV(m.NewEPSQoS); err != nil {
			return nil, err
		}
	}

	if len(m.APNAMBR) > 0 {
		w.U8(apnAMBRIEI)

		if err := w.LV(m.APNAMBR); err != nil {
			return nil, err
		}
	}

	if len(m.ProtocolConfigurationOptions) > 0 {
		w.U8(protocolConfigurationOptionsIEI)

		if err := w.LV(m.ProtocolConfigurationOptions); err != nil {
			return nil, err
		}
	}

	return w.Bytes(), nil
}

// ParseModifyEPSBearerContextRequest decodes the message, extracting the PCO from
// the optional part with the shared IE walker.
func ParseModifyEPSBearerContextRequest(b []byte) (*ModifyEPSBearerContextRequest, error) {
	r := common.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgModifyEPSBearerContextRequest)
	if err != nil {
		return nil, err
	}

	m := &ModifyEPSBearerContextRequest{EPSBearerIdentity: ebi, ProcedureTransactionIdentity: pti}

	if _, err := common.WalkOptionalIEs(r, modifyEPSBearerContextRequestIEs, func(iei uint8, value []byte) error {
		switch iei {
		case newEPSQoSIEI:
			m.NewEPSQoS = value
		case apnAMBRIEI:
			m.APNAMBR = value
		case protocolConfigurationOptionsIEI:
			m.ProtocolConfigurationOptions = value
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return m, nil
}

// ModifyEPSBearerContextAccept is the MODIFY EPS BEARER CONTEXT ACCEPT
// (TS 24.301): the UE's acknowledgement, carrying no mandatory
// information beyond the ESM header. Its optional IEs are not used.
type ModifyEPSBearerContextAccept struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
}

// Marshal encodes the MODIFY EPS BEARER CONTEXT ACCEPT.
func (m *ModifyEPSBearerContextAccept) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgModifyEPSBearerContextAccept)

	return w.Bytes(), nil
}

// ParseModifyEPSBearerContextAccept decodes the message.
func ParseModifyEPSBearerContextAccept(b []byte) (*ModifyEPSBearerContextAccept, error) {
	r := common.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgModifyEPSBearerContextAccept)
	if err != nil {
		return nil, err
	}

	return &ModifyEPSBearerContextAccept{EPSBearerIdentity: ebi, ProcedureTransactionIdentity: pti}, nil
}

// ModifyEPSBearerContextReject is the MODIFY EPS BEARER CONTEXT REJECT
// (TS 24.301): the UE's refusal of a network-requested modification,
// carrying a mandatory ESM cause.
type ModifyEPSBearerContextReject struct {
	EPSBearerIdentity            uint8
	ProcedureTransactionIdentity uint8
	ESMCause                     uint8
}

// Marshal encodes the MODIFY EPS BEARER CONTEXT REJECT.
func (m *ModifyEPSBearerContextReject) Marshal() ([]byte, error) {
	var w common.Writer

	writeESMHeader(&w, m.EPSBearerIdentity, m.ProcedureTransactionIdentity, MsgModifyEPSBearerContextReject)
	w.U8(m.ESMCause)

	return w.Bytes(), nil
}

// ParseModifyEPSBearerContextReject decodes the message.
func ParseModifyEPSBearerContextReject(b []byte) (*ModifyEPSBearerContextReject, error) {
	r := common.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgModifyEPSBearerContextReject)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	return &ModifyEPSBearerContextReject{
		EPSBearerIdentity:            ebi,
		ProcedureTransactionIdentity: pti,
		ESMCause:                     cause,
	}, nil
}
