// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// ModifyEPSBearerContextRequest is the MODIFY EPS BEARER CONTEXT REQUEST
// (TS 24.301): the ESM header followed by entirely optional IEs. The
// network uses it to update an active bearer's parameters in place — the per-APN
// Session-AMBR (APN-AMBR) and/or the Protocol Configuration Options
// carrying a changed DNS server (TS 24.008) — without deactivating the
// bearer.
type ModifyEPSBearerContextRequest struct {
	EPSBearerIdentity                    EPSBearerIdentity
	PTI                                  nas.ProcedureTransactionIdentity
	NewEPSQoS                            *EPSQoS  // optional (IEI 0x5B)
	APNAMBR                              *APNAMBR // optional (IEI 0x5E)
	ProtocolConfigurationOptions         *nas.ProtocolConfigurationOptions
	ExtendedProtocolConfigurationOptions *nas.ProtocolConfigurationOptions
	Unrecognized                         []nas.RawIE
}

// modifyEPSBearerContextRequestIEs are the optional IEs Ella Core sends in a
// MODIFY EPS BEARER CONTEXT REQUEST (TS 24.301), in message order.
var modifyEPSBearerContextRequestIEs = []nas.OptionalIE{
	{IEI: ieiNewEPSQoS, Format: nas.IETLV, Name: "New EPS QoS"},
	{IEI: ieiNegotiatedLLCSAPI, Format: nas.IETV3, Len: 1, Name: "Negotiated LLC SAPI"},
	{IEI: ieiAPNAMBR, Format: nas.IETLV, Name: "APN-AMBR"},
	{IEI: ieiProtocolConfigurationOptions, Format: nas.IETLV, Name: "Protocol configuration options"},
	{IEI: ieiExtendedProtocolConfigurationOptions, Format: nas.IETLVE, Name: "Extended protocol configuration options"},
}

// AppendBinary encodes the MODIFY EPS BEARER CONTEXT REQUEST.
// The encoding is appended to b.
func (m *ModifyEPSBearerContextRequest) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgModifyEPSBearerContextRequest)

	if m.NewEPSQoS != nil {
		raw, err := m.NewEPSQoS.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiNewEPSQoS, raw)
	}

	if m.APNAMBR != nil {
		raw, err := m.APNAMBR.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiAPNAMBR, raw)
	}

	if m.ProtocolConfigurationOptions != nil {
		raw, err := m.ProtocolConfigurationOptions.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiProtocolConfigurationOptions, raw)
	}

	if m.ExtendedProtocolConfigurationOptions != nil {
		raw, err := m.ExtendedProtocolConfigurationOptions.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("nas/eps: encode extended protocol configuration options: %w", err)
		}

		o.TLVE(ieiExtendedProtocolConfigurationOptions, raw)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *ModifyEPSBearerContextRequest) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseModifyEPSBearerContextRequest decodes the message, extracting the PCO from
// the optional part with the shared IE walker.
func ParseModifyEPSBearerContextRequest(b []byte) (*ModifyEPSBearerContextRequest, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgModifyEPSBearerContextRequest)
	if err != nil {
		return nil, err
	}

	m := &ModifyEPSBearerContextRequest{EPSBearerIdentity: ebi, PTI: pti}

	_unrec, err := walkOptionalIEs(r, modifyEPSBearerContextRequestIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiNewEPSQoS:
			parsed, err := ParseEPSQoS(value)
			if err != nil {
				return false, err
			}

			m.NewEPSQoS = &parsed
		case ieiAPNAMBR:
			parsed, err := ParseAPNAMBR(value)
			if err != nil {
				return false, err
			}

			m.APNAMBR = &parsed
		case ieiProtocolConfigurationOptions:
			parsed, err := nas.ParseProtocolConfigurationOptions(value, nas.PCONetworkToMS)
			if err != nil {
				return false, err
			}

			m.ProtocolConfigurationOptions = &parsed
		case ieiExtendedProtocolConfigurationOptions:
			parsed, err := nas.ParseExtendedProtocolConfigurationOptions(value, nas.PCONetworkToMS)
			if err != nil {
				return false, err
			}

			m.ExtendedProtocolConfigurationOptions = &parsed
		default:
			return false, nil
		}

		return true, nil
	})
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	m.Unrecognized = _unrec

	return m, err
}

// ModifyEPSBearerContextAccept is the MODIFY EPS BEARER CONTEXT ACCEPT
// (TS 24.301): the UE's acknowledgement, carrying no mandatory
// information beyond the ESM header. Its optional IEs are not used.
type ModifyEPSBearerContextAccept struct {
	EPSBearerIdentity EPSBearerIdentity
	PTI               nas.ProcedureTransactionIdentity

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the MODIFY EPS BEARER CONTEXT ACCEPT.
// The encoding is appended to b.
func (m *ModifyEPSBearerContextAccept) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgModifyEPSBearerContextAccept)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *ModifyEPSBearerContextAccept) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseModifyEPSBearerContextAccept decodes the message.
func ParseModifyEPSBearerContextAccept(b []byte) (*ModifyEPSBearerContextAccept, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgModifyEPSBearerContextAccept)
	if err != nil {
		return nil, err
	}

	out := &ModifyEPSBearerContextAccept{EPSBearerIdentity: ebi, PTI: pti}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// ModifyEPSBearerContextReject is the MODIFY EPS BEARER CONTEXT REJECT
// (TS 24.301): the UE's refusal of a network-requested modification,
// carrying a mandatory ESM cause.
type ModifyEPSBearerContextReject struct {
	EPSBearerIdentity EPSBearerIdentity
	PTI               nas.ProcedureTransactionIdentity
	Cause             ESMCause

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the MODIFY EPS BEARER CONTEXT REJECT.
// The encoding is appended to b.
func (m *ModifyEPSBearerContextReject) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgModifyEPSBearerContextReject)
	w.U8(uint8(m.Cause))

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *ModifyEPSBearerContextReject) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseModifyEPSBearerContextReject decodes the message.
func ParseModifyEPSBearerContextReject(b []byte) (*ModifyEPSBearerContextReject, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgModifyEPSBearerContextReject)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &ModifyEPSBearerContextReject{
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
