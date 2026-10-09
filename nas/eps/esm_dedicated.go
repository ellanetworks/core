// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// ActivateDedicatedEPSBearerContextRequest is the ACTIVATE DEDICATED EPS BEARER
// CONTEXT REQUEST message (TS 24.301 §8.3.3).
type ActivateDedicatedEPSBearerContextRequest struct {
	EPSBearerIdentity                    EPSBearerIdentity
	PTI                                  nas.ProcedureTransactionIdentity
	LinkedEPSBearerIdentity              EPSBearerIdentity
	EPSQoS                               EPSQoS
	TFT                                  TrafficFlowTemplate
	ProtocolConfigurationOptions         *nas.ProtocolConfigurationOptions
	ExtendedProtocolConfigurationOptions *nas.ProtocolConfigurationOptions
	Unrecognized                         []nas.RawIE
}

var activateDedicatedEPSBearerContextRequestIEs = []nas.OptionalIE{
	{IEI: ieiNegotiatedLLCSAPI, Format: nas.IETV3, Len: 1, Name: "Negotiated LLC SAPI"},
	{IEI: ieiProtocolConfigurationOptions, Format: nas.IETLV, Name: "Protocol configuration options"},
	{IEI: ieiExtendedProtocolConfigurationOptions, Format: nas.IETLVE, Name: "Extended protocol configuration options"},
}

// AppendBinary encodes the message, appending it to b.
func (m *ActivateDedicatedEPSBearerContextRequest) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgActivateDedicatedEPSBearerContextRequest)
	w.U8(uint8(m.LinkedEPSBearerIdentity) & 0x0F)

	qos, err := m.EPSQoS.MarshalBinary()
	if err != nil {
		return b, err
	}

	tft, err := m.TFT.MarshalBinary()
	if err != nil {
		return b, err
	}

	w.LV(qos)
	w.LV(tft)

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
			return b, fmt.Errorf("nas/eps: encode extended protocol configuration options: %w", err)
		}

		o.TLVE(ieiExtendedProtocolConfigurationOptions, raw)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *ActivateDedicatedEPSBearerContextRequest) MarshalBinary() ([]byte, error) {
	return marshalMessage(m)
}

// ParseActivateDedicatedEPSBearerContextRequest decodes the message.
func ParseActivateDedicatedEPSBearerContextRequest(b []byte) (*ActivateDedicatedEPSBearerContextRequest, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgActivateDedicatedEPSBearerContextRequest)
	if err != nil {
		return nil, err
	}

	linked, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &ActivateDedicatedEPSBearerContextRequest{EPSBearerIdentity: ebi, PTI: pti, LinkedEPSBearerIdentity: EPSBearerIdentity(linked & 0x0F)}

	qosRaw, err := r.LV()
	if err != nil {
		return nil, err
	}

	if m.EPSQoS, err = ParseEPSQoS(qosRaw); err != nil {
		return nil, err
	}

	tftRaw, err := r.LV()
	if err != nil {
		return nil, err
	}

	if m.TFT, err = ParseTrafficFlowTemplate(tftRaw); err != nil {
		return nil, err
	}

	unrec, err := walkOptionalIEs(r, activateDedicatedEPSBearerContextRequestIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
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

	m.Unrecognized = unrec

	return m, err
}

// ActivateDedicatedEPSBearerContextAccept is the ACTIVATE DEDICATED EPS BEARER
// CONTEXT ACCEPT message (TS 24.301 §8.3.1).
type ActivateDedicatedEPSBearerContextAccept struct {
	EPSBearerIdentity                    EPSBearerIdentity
	PTI                                  nas.ProcedureTransactionIdentity
	ProtocolConfigurationOptions         *nas.ProtocolConfigurationOptions
	ExtendedProtocolConfigurationOptions *nas.ProtocolConfigurationOptions
	Unrecognized                         []nas.RawIE
}

var dedicatedBearerResponseIEs = []nas.OptionalIE{
	{IEI: ieiProtocolConfigurationOptions, Format: nas.IETLV, Name: "Protocol configuration options"},
	{IEI: ieiExtendedProtocolConfigurationOptions, Format: nas.IETLVE, Name: "Extended protocol configuration options"},
}

// AppendBinary encodes the message, appending it to b.
func (m *ActivateDedicatedEPSBearerContextAccept) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgActivateDedicatedEPSBearerContextAccept)

	if err := writeResponsePCO(w, m.ProtocolConfigurationOptions, m.ExtendedProtocolConfigurationOptions, m.Unrecognized); err != nil {
		return b, err
	}

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *ActivateDedicatedEPSBearerContextAccept) MarshalBinary() ([]byte, error) {
	return marshalMessage(m)
}

// ParseActivateDedicatedEPSBearerContextAccept decodes the message.
func ParseActivateDedicatedEPSBearerContextAccept(b []byte) (*ActivateDedicatedEPSBearerContextAccept, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgActivateDedicatedEPSBearerContextAccept)
	if err != nil {
		return nil, err
	}

	m := &ActivateDedicatedEPSBearerContextAccept{EPSBearerIdentity: ebi, PTI: pti}

	m.ProtocolConfigurationOptions, m.ExtendedProtocolConfigurationOptions, m.Unrecognized, err = readResponsePCO(r)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	return m, err
}

// ActivateDedicatedEPSBearerContextReject is the ACTIVATE DEDICATED EPS BEARER
// CONTEXT REJECT message (TS 24.301 §8.3.2).
type ActivateDedicatedEPSBearerContextReject struct {
	EPSBearerIdentity                    EPSBearerIdentity
	PTI                                  nas.ProcedureTransactionIdentity
	Cause                                ESMCause
	ProtocolConfigurationOptions         *nas.ProtocolConfigurationOptions
	ExtendedProtocolConfigurationOptions *nas.ProtocolConfigurationOptions
	Unrecognized                         []nas.RawIE
}

// AppendBinary encodes the message, appending it to b.
func (m *ActivateDedicatedEPSBearerContextReject) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgActivateDedicatedEPSBearerContextReject)
	w.U8(uint8(m.Cause))

	if err := writeResponsePCO(w, m.ProtocolConfigurationOptions, m.ExtendedProtocolConfigurationOptions, m.Unrecognized); err != nil {
		return b, err
	}

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *ActivateDedicatedEPSBearerContextReject) MarshalBinary() ([]byte, error) {
	return marshalMessage(m)
}

// ParseActivateDedicatedEPSBearerContextReject decodes the message.
func ParseActivateDedicatedEPSBearerContextReject(b []byte) (*ActivateDedicatedEPSBearerContextReject, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgActivateDedicatedEPSBearerContextReject)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &ActivateDedicatedEPSBearerContextReject{EPSBearerIdentity: ebi, PTI: pti, Cause: ESMCause(cause)}

	m.ProtocolConfigurationOptions, m.ExtendedProtocolConfigurationOptions, m.Unrecognized, err = readResponsePCO(r)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	return m, err
}

func writeResponsePCO(w *nas.Writer, pco, epco *nas.ProtocolConfigurationOptions, unrecognized []nas.RawIE) error {
	var o nas.OptionalWriter

	if pco != nil {
		raw, err := pco.MarshalBinary()
		if err != nil {
			return err
		}

		o.TLV(ieiProtocolConfigurationOptions, raw)
	}

	if epco != nil {
		raw, err := epco.MarshalBinary()
		if err != nil {
			return err
		}

		o.TLVE(ieiExtendedProtocolConfigurationOptions, raw)
	}

	o.Raw(unrecognized...)
	o.WriteTo(w)

	return nil
}

func readResponsePCO(r *nas.Reader) (pco, epco *nas.ProtocolConfigurationOptions, unrecognized []nas.RawIE, err error) {
	unrecognized, err = walkOptionalIEs(r, dedicatedBearerResponseIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiProtocolConfigurationOptions:
			parsed, perr := nas.ParseProtocolConfigurationOptions(value, nas.PCOMSToNetwork)
			if perr != nil {
				return false, perr
			}

			pco = &parsed
		case ieiExtendedProtocolConfigurationOptions:
			parsed, perr := nas.ParseExtendedProtocolConfigurationOptions(value, nas.PCOMSToNetwork)
			if perr != nil {
				return false, perr
			}

			epco = &parsed
		default:
			return false, nil
		}

		return true, nil
	})

	return pco, epco, unrecognized, err
}
