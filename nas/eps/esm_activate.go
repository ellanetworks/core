// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas"

// ActivateDefaultEPSBearerContextRequest is the ACTIVATE DEFAULT EPS BEARER
// CONTEXT REQUEST message (TS 24.301), sent by the MME to set up the
// default bearer. PDNAddress carries the assigned UE IP.
type ActivateDefaultEPSBearerContextRequest struct {
	EPSBearerIdentity EPSBearerIdentity
	PTI               nas.ProcedureTransactionIdentity
	EPSQoS            EPSQoS
	AccessPointName   APN
	PDNAddress        PDNAddress
	// APNAMBR, when set, is the APN aggregate maximum bit rate IE value (TS
	// 24.301) — the EPS per-APN session-AMBR signaled to the UE for
	// uplink enforcement. Encoded as the APN-AMBR TLV optional IE (IEI 0x5E).
	APNAMBR *APNAMBR
	// ESMCause, when set, carries the reason the network assigned a narrower PDN
	// type than the UE requested, e.g. #50/#51 on an IPv4v6 downgrade (TS 24.301).
	// Encoded as the ESM cause TV optional IE (IEI 0x58).
	Cause *ESMCause
	// ProtocolConfigurationOptions carries the network-to-UE PCO value (e.g. DNS
	// server addresses), encoded as the PCO TLV optional IE (IEI 0x27).
	ProtocolConfigurationOptions *nas.ProtocolConfigurationOptions

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// activateDefaultEPSBearerContextRequestIEs are the optional IEs Ella Core emits
// in an ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST (TS 24.301): the
// APN-AMBR (a type-4 TLV), the ESM cause (a type-3 IE with a one-octet value),
// and the Protocol Configuration Options (a type-4 TLV).
var activateDefaultEPSBearerContextRequestIEs = []nas.OptionalIE{
	{IEI: ieiNegotiatedLLCSAPI, Format: nas.IETV3, Len: 1, Name: "Negotiated LLC SAPI"},
	{IEI: ieiAPNAMBR, Format: nas.IETLV, Name: "APN-AMBR"},
	{IEI: ieiESMCause, Format: nas.IETV3, Len: 1, Name: "ESM cause"},
	{IEI: ieiProtocolConfigurationOptions, Format: nas.IETLV, Name: "Protocol configuration options"},
}

// AppendBinary encodes the ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST message.
// The encoding is appended to b.
func (m *ActivateDefaultEPSBearerContextRequest) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgActivateDefaultEPSBearerContextRequest)

	qos, err := m.EPSQoS.MarshalBinary()
	if err != nil {
		return b, err
	}

	apn, err := m.AccessPointName.MarshalBinary()
	if err != nil {
		return b, err
	}

	pdnAddr, err := m.PDNAddress.MarshalBinary()
	if err != nil {
		return b, err
	}

	for _, lv := range [][]byte{qos, apn, pdnAddr} {
		w.LV(lv)
	}

	if m.APNAMBR != nil {
		raw, err := m.APNAMBR.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiAPNAMBR, raw)
	}

	if m.Cause != nil {
		o.TV3(ieiESMCause, []byte{uint8(*m.Cause)})
	}

	if m.ProtocolConfigurationOptions != nil {
		raw, err := m.ProtocolConfigurationOptions.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiProtocolConfigurationOptions, raw)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *ActivateDefaultEPSBearerContextRequest) MarshalBinary() ([]byte, error) {
	return marshalMessage(m)
}

// ParseActivateDefaultEPSBearerContextRequest decodes the message, extracting the
// ESM cause and Protocol Configuration Options from the optional part with the
// shared IE walker (TS 24.301).
func ParseActivateDefaultEPSBearerContextRequest(b []byte) (*ActivateDefaultEPSBearerContextRequest, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgActivateDefaultEPSBearerContextRequest)
	if err != nil {
		return nil, err
	}

	m := &ActivateDefaultEPSBearerContextRequest{EPSBearerIdentity: ebi, PTI: pti}

	qosRaw, err := r.LV()
	if err != nil {
		return nil, err
	}

	if m.EPSQoS, err = ParseEPSQoS(qosRaw); err != nil {
		return nil, err
	}

	apnRaw, err := r.LV()
	if err != nil {
		return nil, err
	}

	if m.AccessPointName, err = ParseAPN(apnRaw); err != nil {
		return nil, err
	}

	pdnRaw, err := r.LV()
	if err != nil {
		return nil, err
	}

	if m.PDNAddress, err = ParsePDNAddress(pdnRaw); err != nil {
		return nil, err
	}

	_unrec, err := walkOptionalIEs(r, activateDefaultEPSBearerContextRequestIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiAPNAMBR:
			parsed, err := ParseAPNAMBR(value)
			if err != nil {
				return false, err
			}

			m.APNAMBR = &parsed
		case ieiESMCause:
			if len(value) == 0 {
				return false, nil
			}

			cause := ESMCause(value[0])
			m.Cause = &cause
		case ieiProtocolConfigurationOptions:
			parsed, err := nas.ParseProtocolConfigurationOptions(value, nas.PCONetworkToMS)
			if err != nil {
				return false, err
			}

			m.ProtocolConfigurationOptions = &parsed
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

// ActivateDefaultEPSBearerContextAccept is the ACTIVATE DEFAULT EPS BEARER
// CONTEXT ACCEPT message (TS 24.301).
type ActivateDefaultEPSBearerContextAccept struct {
	EPSBearerIdentity EPSBearerIdentity
	PTI               nas.ProcedureTransactionIdentity

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the ACTIVATE DEFAULT EPS BEARER CONTEXT ACCEPT message.
// The encoding is appended to b.
func (m *ActivateDefaultEPSBearerContextAccept) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgActivateDefaultEPSBearerContextAccept)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *ActivateDefaultEPSBearerContextAccept) MarshalBinary() ([]byte, error) {
	return marshalMessage(m)
}

// ParseActivateDefaultEPSBearerContextAccept decodes the message.
func ParseActivateDefaultEPSBearerContextAccept(b []byte) (*ActivateDefaultEPSBearerContextAccept, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgActivateDefaultEPSBearerContextAccept)
	if err != nil {
		return nil, err
	}

	out := &ActivateDefaultEPSBearerContextAccept{EPSBearerIdentity: ebi, PTI: pti}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// ActivateDefaultEPSBearerContextReject is the ACTIVATE DEFAULT EPS BEARER
// CONTEXT REJECT message (TS 24.301).
type ActivateDefaultEPSBearerContextReject struct {
	EPSBearerIdentity EPSBearerIdentity
	PTI               nas.ProcedureTransactionIdentity
	Cause             ESMCause

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the ACTIVATE DEFAULT EPS BEARER CONTEXT REJECT message.
// The encoding is appended to b.
func (m *ActivateDefaultEPSBearerContextReject) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgActivateDefaultEPSBearerContextReject)
	w.U8(uint8(m.Cause))

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *ActivateDefaultEPSBearerContextReject) MarshalBinary() ([]byte, error) {
	return marshalMessage(m)
}

// ParseActivateDefaultEPSBearerContextReject decodes the message.
func ParseActivateDefaultEPSBearerContextReject(b []byte) (*ActivateDefaultEPSBearerContextReject, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgActivateDefaultEPSBearerContextReject)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &ActivateDefaultEPSBearerContextReject{
		EPSBearerIdentity: ebi, PTI: pti, Cause: ESMCause(cause),
	}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}
