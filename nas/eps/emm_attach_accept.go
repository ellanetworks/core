// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// EPS attach result values (TS 24.301).
const (
	AttachResultEPS      AttachResult = 1
	AttachResultCombined AttachResult = 2
)

// AttachAccept is the ATTACH ACCEPT message (TS 24.301).
type AttachAccept struct {
	EPSAttachResult     AttachResult
	T3412               nas.GPRSTimer2
	TAIList             TAIList
	ESMMessageContainer []byte
	GUTI                *EPSMobileIdentity // assigned GUTI (IEI 0x50), when present
	Cause               *EMMCause          // EMM cause (IEI 0x53), when present
	// EPS network feature support (IEI 0x64), when present (TS 24.301).
	NetworkFeatureSupport *NetworkFeatureSupport

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// attachAcceptIEs are the optional IEs Ella Core emits in an ATTACH ACCEPT
// (TS 24.301): the assigned GUTI, the EMM cause, and the EPS network
// feature support. EMM cause is a type-3 IE with a one-octet value; the others
// are type-4 TLVs.
var attachAcceptIEs = []nas.OptionalIE{
	{IEI: ieiGUTI, Format: nas.IETLV, Name: "GUTI"},
	{IEI: ieiLocationAreaID, Format: nas.IETV3, Len: 5, Name: "Location area identification"},
	{IEI: ieiEMMCause, Format: nas.IETV3, Len: 1, Name: "EMM cause"},
	{IEI: ieiT3402ValueAccept, Format: nas.IETV3, Len: 1, Name: "T3402 value"},
	{IEI: ieiT3423Value, Format: nas.IETV3, Len: 1, Name: "T3423 value"},
	{IEI: ieiNetworkFeatureSupport, Format: nas.IETLV, Name: "Network feature support"},
}

// NetworkFeatureSupport is the EPS network feature support IE
// (TS 24.301 §9.9.3.12A): what the network supports in S1 mode, in a type 4 IE
// whose content is 1 to 3 octets. IMSVoPS is the one the UE feeds to voice
// access-domain selection (TS 23.221).
//
// It is the EPS counterpart of fgs.NetworkFeatureSupport, and like it keeps the
// octets it does not interpret so the element re-encodes at the length it
// arrived with.
type NetworkFeatureSupport struct {
	IMSVoPS bool  // IMS voice over PS session in S1 mode (octet 3, bit 1)
	EMCBS   bool  // emergency bearer services in S1 mode (octet 3, bit 2)
	EPCLCS  bool  // location services via EPC (octet 3, bit 3)
	CSLCS   uint8 // location services via the CS domain (octet 3, bits 4-5)
	ESRPS   bool  // EXTENDED SERVICE REQUEST for packet services (octet 3, bit 6)
	ERwoPDN bool  // attach without PDN connectivity (octet 3, bit 7)
	CPCIoT  bool  // control plane CIoT EPS optimisation (octet 3, bit 8)

	// HasOctet4 records whether the sender included octet 4, so the element
	// re-encodes at the length it arrived with; Rest carries octet 5 onwards.
	HasOctet4 bool
	Octet4    uint8
	Rest      []byte
}

// ParseNetworkFeatureSupport decodes an EPS network feature support IE value.
func ParseNetworkFeatureSupport(b []byte) (NetworkFeatureSupport, error) {
	if len(b) < 1 {
		return NetworkFeatureSupport{}, fmt.Errorf("nas/eps: empty EPS network feature support")
	}

	out := NetworkFeatureSupport{
		IMSVoPS: b[0]&0x01 != 0,
		EMCBS:   b[0]>>1&0x01 != 0,
		EPCLCS:  b[0]>>2&0x01 != 0,
		CSLCS:   b[0] >> 3 & 0x03,
		ESRPS:   b[0]>>5&0x01 != 0,
		ERwoPDN: b[0]>>6&0x01 != 0,
		CPCIoT:  b[0]>>7&0x01 != 0,
	}

	if len(b) > 1 {
		out.HasOctet4 = true
		out.Octet4 = b[1]
	}

	if len(b) > 2 {
		out.Rest = bytes.Clone(b[2:])
	}

	return out, nil
}

// AppendBinary encodes the EPS network feature support IE value onto b.
func (n NetworkFeatureSupport) AppendBinary(b []byte) ([]byte, error) {
	if (len(n.Rest) > 0 || n.Octet4 != 0) && !n.HasOctet4 {
		return b, fmt.Errorf("nas/eps: EPS network feature support: octet 5 onwards requires octet 4")
	}

	octet := boolBit(n.IMSVoPS, 0) | boolBit(n.EMCBS, 1) | boolBit(n.EPCLCS, 2) |
		n.CSLCS&0x03<<3 | boolBit(n.ESRPS, 5) | boolBit(n.ERwoPDN, 6) | boolBit(n.CPCIoT, 7)

	b = append(b, octet)

	if !n.HasOctet4 {
		return b, nil
	}

	return append(append(b, n.Octet4), n.Rest...), nil
}

// MarshalBinary encodes the NetworkFeatureSupport information element value.
func (n NetworkFeatureSupport) MarshalBinary() ([]byte, error) { return n.AppendBinary(nil) }

// AppendBinary encodes the plain ATTACH ACCEPT message.
// The encoding is appended to b.
func (m *AttachAccept) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgAttachAccept)
	w.U8(uint8(m.EPSAttachResult & 0x07)) // EPS attach result | spare half octet

	timer, err := m.T3412.MarshalBinary()
	if err != nil {
		return b, err
	}

	w.Raw(timer)

	taiList, err := m.TAIList.MarshalBinary()
	if err != nil {
		return b, err
	}

	w.LV(taiList)

	w.LVE(m.ESMMessageContainer)

	if m.GUTI != nil {
		raw, err := m.GUTI.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiGUTI, raw)
	}

	if m.Cause != nil {
		o.TV3(ieiEMMCause, []byte{uint8(*m.Cause)})
	}

	if m.NetworkFeatureSupport != nil {
		raw, err := m.NetworkFeatureSupport.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiNetworkFeatureSupport, raw)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *AttachAccept) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseAttachAccept decodes a plain ATTACH ACCEPT message.
func ParseAttachAccept(b []byte) (*AttachAccept, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgAttachAccept); err != nil {
		return nil, err
	}

	result, err := r.U8()
	if err != nil {
		return nil, err
	}

	t3412Octet, err := r.U8()
	if err != nil {
		return nil, err
	}

	t3412, err := nas.ParseGPRSTimer2([]byte{t3412Octet})
	if err != nil {
		return nil, err
	}

	m := &AttachAccept{EPSAttachResult: AttachResult(result & 0x07), T3412: t3412}

	taiRaw, err := r.LV()
	if err != nil {
		return nil, err
	}

	if m.TAIList, err = ParseTAIList(taiRaw); err != nil {
		return nil, err
	}

	if m.ESMMessageContainer, err = r.LVE(); err != nil {
		return nil, err
	}

	_unrec, err := walkOptionalIEs(r, attachAcceptIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiGUTI:
			parsed, err := ParseEPSMobileIdentity(value)
			if err != nil {
				return false, err
			}

			m.GUTI = &parsed
		case ieiEMMCause:
			if len(value) == 0 {
				return false, nil
			}

			cause := EMMCause(value[0])
			m.Cause = &cause
		case ieiNetworkFeatureSupport:
			parsed, err := ParseNetworkFeatureSupport(value)
			if err != nil {
				return false, err
			}

			m.NetworkFeatureSupport = &parsed
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

// AttachComplete is the ATTACH COMPLETE message (TS 24.301).
type AttachComplete struct {
	ESMMessageContainer []byte

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged. The spec defines none for this message, but a later release may.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain ATTACH COMPLETE message.
// The encoding is appended to b.
func (m *AttachComplete) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgAttachComplete)

	w.LVE(m.ESMMessageContainer)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *AttachComplete) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseAttachComplete decodes a plain ATTACH COMPLETE message.
func ParseAttachComplete(b []byte) (*AttachComplete, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgAttachComplete); err != nil {
		return nil, err
	}

	esm, err := r.LVE()
	if err != nil {
		return nil, err
	}

	out := &AttachComplete{ESMMessageContainer: esm}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// AttachReject is the ATTACH REJECT message (TS 24.301). Ella Core sends the
// mandatory EMM cause and, when non-zero, the optional T3402 value (§8.2.3.4) —
// the back-off before the UE retries, mirroring the 5G T3502 in REGISTRATION
// REJECT.
type AttachReject struct {
	Cause EMMCause
	// T3402 is the encoded one-octet GPRS timer value (§9.9.3.16A); 0 omits the IE.
	T3402 *nas.GPRSTimer2

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// attachRejectIEs are the optional IEs Ella Core emits in an ATTACH REJECT: the
// T3402 value, a type-4 (TLV) "GPRS timer 2" IE with a one-octet value (§8.2.3.4).
var attachRejectIEs = []nas.OptionalIE{
	{IEI: ieiT3402Value, Format: nas.IETLV, Name: "T3402 value"},
}

// AppendBinary encodes the plain ATTACH REJECT message.
// The encoding is appended to b.
func (m *AttachReject) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgAttachReject)
	w.U8(uint8(m.Cause))

	if m.T3402 != nil {
		raw, err := m.T3402.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiT3402Value, raw)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *AttachReject) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseAttachReject decodes a plain ATTACH REJECT message.
func ParseAttachReject(b []byte) (*AttachReject, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgAttachReject); err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &AttachReject{Cause: EMMCause(cause)}

	_unrec, err := walkOptionalIEs(r, attachRejectIEs, func(iei uint8, value []byte) (bool, error) {
		if iei != ieiT3402Value {
			return false, nil
		}

		timer, err := nas.ParseGPRSTimer2(value)
		if err != nil {
			return false, err
		}

		m.T3402 = &timer

		return true, nil
	})
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	m.Unrecognized = _unrec

	return m, err
}
