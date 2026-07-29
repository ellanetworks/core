// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// PDNConnectivityRequest is the PDN CONNECTIVITY REQUEST message (TS 24.301),
// sent by the UE — inside the Attach Request's ESM container for the
// default bearer, or standalone to open an additional PDN connection. For an
// additional connection the UE names the data network in the Access Point Name
// optional IE; ProtocolConfigurationOptions carries the UE's PCO request (e.g.
// the DNS-server request).
type PDNConnectivityRequest struct {
	EPSBearerIdentity            EPSBearerIdentity
	PTI                          nas.ProcedureTransactionIdentity
	RequestType                  RequestType
	PDNType                      PDNType
	AccessPointName              *APN                              // APN value part (IEI 0x28), nil if absent
	ProtocolConfigurationOptions *nas.ProtocolConfigurationOptions // optional (IEI 0x27)
	// ESMInformationTransferFlag is the EIT bit (IEI 0xD, TS 24.301): the
	// UE will supply the APN and PCO in an ESM INFORMATION RESPONSE, not in this
	// message.
	ESMInformationTransferFlag bool

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// pdnConnectivityRequestIEs are the optional IEs Ella Core consumes from a PDN
// CONNECTIVITY REQUEST (TS 24.301): the Access Point Name and Protocol
// Configuration Options, both type-4 TLVs. The ESM information transfer flag that
// may precede them is a type-1 IE the walker delimits inherently, so the APN is
// reached even when it does not lead the optional part.
var pdnConnectivityRequestIEs = []nas.OptionalIE{
	{IEI: ieiAccessPointName, Format: nas.IETLV, Name: "Access point name"},
	{IEI: ieiProtocolConfigurationOptions, Format: nas.IETLV, Name: "Protocol configuration options"},
}

// AppendBinary encodes the PDN CONNECTIVITY REQUEST message.
// The encoding is appended to b.
func (m *PDNConnectivityRequest) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgPDNConnectivityRequest)
	w.U8((uint8(m.PDNType)&0x07)<<4 | uint8(m.RequestType)&0x07)

	if m.ESMInformationTransferFlag {
		o.TV1(ieiESMInformationTransferFlag, 0x01)
	}

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

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *PDNConnectivityRequest) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParsePDNConnectivityRequest decodes a PDN CONNECTIVITY REQUEST message,
// extracting the Access Point Name and Protocol Configuration Options from the
// optional part with the shared IE walker (TS 24.301).
func ParsePDNConnectivityRequest(b []byte) (*PDNConnectivityRequest, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgPDNConnectivityRequest)
	if err != nil {
		return nil, err
	}

	octet, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &PDNConnectivityRequest{
		EPSBearerIdentity: ebi,
		PTI:               pti,
		RequestType:       RequestType(octet & 0x07),
		PDNType:           PDNType(octet >> 4 & 0x07),
	}

	_unrec, err := walkOptionalIEs(r, pdnConnectivityRequestIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiESMInformationTransferFlag:
			// Only the "transfer required" value is modelled; TS 24.301 §9.9.4.5
			// reserves the other, which makes it a syntactically incorrect
			// optional element: absent, but preserved (§7.7.1).
			if len(value) != 1 || value[0]&0x0F != 0x01 {
				return false, fmt.Errorf("nas/eps: ESM information transfer flag value %#x is reserved", value)
			}

			m.ESMInformationTransferFlag = true
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

// PDNConnectivityReject is the PDN CONNECTIVITY REJECT message (TS 24.301).
type PDNConnectivityReject struct {
	EPSBearerIdentity EPSBearerIdentity
	PTI               nas.ProcedureTransactionIdentity
	Cause             ESMCause

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the PDN CONNECTIVITY REJECT message.
// The encoding is appended to b.
func (m *PDNConnectivityReject) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeESMHeader(w, m.EPSBearerIdentity, m.PTI, MsgPDNConnectivityReject)
	w.U8(uint8(m.Cause))

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *PDNConnectivityReject) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParsePDNConnectivityReject decodes a PDN CONNECTIVITY REJECT message.
func ParsePDNConnectivityReject(b []byte) (*PDNConnectivityReject, error) {
	r := nas.NewReader(b)

	ebi, pti, err := readESMHeader(r, MsgPDNConnectivityReject)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &PDNConnectivityReject{
		EPSBearerIdentity: ebi, PTI: pti, Cause: ESMCause(cause),
	}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}
