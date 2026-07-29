// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas"

// ESMInformationRequest is the ESM INFORMATION REQUEST message (TS 24.301).
// It has no information elements beyond the header.
type ESMInformationRequest struct {
	EPSBearerIdentity EPSBearerIdentity
	PTI               nas.ProcedureTransactionIdentity

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged. The spec defines none for this message, but a later release may.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the ESM INFORMATION REQUEST message.
// The encoding is appended to b.
func (m *ESMInformationRequest) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgESMInformationRequest)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *ESMInformationRequest) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseESMInformationRequest decodes the message.
func ParseESMInformationRequest(b []byte) (*ESMInformationRequest, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgESMInformationRequest)
	if err != nil {
		return nil, err
	}

	out := &ESMInformationRequest{EPSBearerIdentity: ebi, PTI: pti}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// ESMInformationResponse is the ESM INFORMATION RESPONSE message (TS 24.301),
// the UE's reply to an ESM INFORMATION REQUEST, carrying the Access
// Point Name and/or Protocol Configuration Options it withheld from the PDN
// CONNECTIVITY REQUEST.
type ESMInformationResponse struct {
	EPSBearerIdentity            EPSBearerIdentity
	PTI                          nas.ProcedureTransactionIdentity
	AccessPointName              *APN                              // APN value part (IEI 0x28), nil if absent
	ProtocolConfigurationOptions *nas.ProtocolConfigurationOptions // optional (IEI 0x27)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// esmInformationResponseIEs are the optional IEs of an ESM INFORMATION RESPONSE
// (TS 24.301): the Access Point Name and the Protocol Configuration
// Options, both type-4 TLVs.
var esmInformationResponseIEs = []nas.OptionalIE{
	{IEI: ieiAccessPointName, Format: nas.IETLV, Name: "Access point name"},
	{IEI: ieiProtocolConfigurationOptions, Format: nas.IETLV, Name: "Protocol configuration options"},
}

// AppendBinary encodes the ESM INFORMATION RESPONSE message.
// The encoding is appended to b.
func (m *ESMInformationResponse) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgESMInformationResponse)

	if m.AccessPointName != nil {
		raw, err := m.AccessPointName.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiAccessPointName, raw)
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

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *ESMInformationResponse) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseESMInformationResponse decodes the message, extracting the Access Point
// Name and Protocol Configuration Options with the shared IE walker.
func ParseESMInformationResponse(b []byte) (*ESMInformationResponse, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgESMInformationResponse)
	if err != nil {
		return nil, err
	}

	m := &ESMInformationResponse{EPSBearerIdentity: ebi, PTI: pti}

	_unrec, err := walkOptionalIEs(r, esmInformationResponseIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiAccessPointName:
			parsed, err := ParseAPN(value)
			if err != nil {
				return false, err
			}

			m.AccessPointName = &parsed
		case ieiProtocolConfigurationOptions:
			parsed, err := nas.ParseProtocolConfigurationOptions(value, nas.PCOMSToNetwork)
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
