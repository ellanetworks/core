// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// Service type values (TS 24.501 §9.11.3.50).
const (
	ServiceTypeSignalling                ServiceType = 0x00
	ServiceTypeData                      ServiceType = 0x01
	ServiceTypeMobileTerminatedServices  ServiceType = 0x02
	ServiceTypeEmergencyServices         ServiceType = 0x03
	ServiceTypeEmergencyServicesFallback ServiceType = 0x04
	ServiceTypeHighPriorityAccess        ServiceType = 0x05
	ServiceTypeElevatedSignalling        ServiceType = 0x06
)

// ServiceRequest is the SERVICE REQUEST message (TS 24.501 §8.2.15): the service
// type and ngKSI, the UE's 5G-S-TMSI, and optional status IEs.
type ServiceRequest struct {
	ServiceType             ServiceType          // bits 5-8 of the service-type/ngKSI octet (TS 24.501 §9.11.3.50)
	NgKSI                   nas.KeySetIdentifier // bits 1-4
	MobileIdentity          MobileIdentity       // mandatory 5G-S-TMSI (type 6, LVE)
	UplinkDataStatus        *PSIBitmap           // optional (IEI 0x40)
	PDUSessionStatus        *PSIBitmap           // optional (IEI 0x50)
	AllowedPDUSessionStatus *PSIBitmap           // optional (IEI 0x25)
	NASMessageContainer     []byte               // optional (IEI 0x71)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

var serviceRequestIEs = []nas.OptionalIE{
	{IEI: ieiUplinkDataStatus, Format: nas.IETLV, Name: "Uplink data status"},
	{IEI: ieiPDUSessionStatus, Format: nas.IETLV, Name: "PDU session status"},
	{IEI: ieiAllowedPDUSessionStatus, Format: nas.IETLV, Name: "Allowed PDU session status"},
	{IEI: ieiNASMessageContainer, Format: nas.IETLVE, Critical: true, Name: "NAS message container"},
}

// AppendBinary encodes the plain SERVICE REQUEST message (TS 24.501 §8.2.16).
// The encoding is appended to b.
func (m *ServiceRequest) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgServiceRequest)
	w.U8(uint8(m.ServiceType)<<4 | m.NgKSI.HalfOctet())

	mi, err := m.MobileIdentity.MarshalBinary()
	if err != nil {
		return b, err
	}

	w.LVE(mi)

	if m.UplinkDataStatus != nil {
		raw, err := m.UplinkDataStatus.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiUplinkDataStatus, raw)
	}

	if m.PDUSessionStatus != nil {
		raw, err := m.PDUSessionStatus.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiPDUSessionStatus, raw)
	}

	if m.AllowedPDUSessionStatus != nil {
		raw, err := m.AllowedPDUSessionStatus.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiAllowedPDUSessionStatus, raw)
	}

	if m.NASMessageContainer != nil {
		o.TLVE(ieiNASMessageContainer, m.NASMessageContainer)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *ServiceRequest) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseServiceRequest decodes a plain SERVICE REQUEST message.
func ParseServiceRequest(b []byte) (*ServiceRequest, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgServiceRequest); err != nil {
		return nil, err
	}

	octet, err := r.U8()
	if err != nil {
		return nil, err
	}

	raw, err := r.LVE()
	if err != nil {
		return nil, err
	}

	mi, err := ParseMobileIdentity(raw)
	if err != nil {
		return nil, err
	}

	out := &ServiceRequest{
		ServiceType:    ServiceType(octet >> 4),
		NgKSI:          nas.ParseKeySetIdentifier(octet),
		MobileIdentity: mi,
	}

	_unrec, err := walkOptionalIEs(r, serviceRequestIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiUplinkDataStatus:
			bitmap, err := ParsePSIBitmap(value)
			if err != nil {
				return false, err
			}

			out.UplinkDataStatus = &bitmap
		case ieiPDUSessionStatus:
			bitmap, err := ParsePSIBitmap(value)
			if err != nil {
				return false, err
			}

			out.PDUSessionStatus = &bitmap
		case ieiAllowedPDUSessionStatus:
			bitmap, err := ParsePSIBitmap(value)
			if err != nil {
				return false, err
			}

			out.AllowedPDUSessionStatus = &bitmap
		case ieiNASMessageContainer:
			out.NASMessageContainer = value
		default:
			return false, nil
		}

		return true, nil
	})
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// ServiceAccept is the SERVICE ACCEPT message (TS 24.501 §8.2.17): optional PDU
// session status, reactivation result, reactivation-result error cause and EAP
// message. Ella sets only the first three; the parser accepts the EAP message.
type ServiceAccept struct {
	PDUSessionStatus             *PSIBitmap                   // optional (IEI 0x50)
	PDUSessionReactivationResult *PSIBitmap                   // optional (IEI 0x26)
	ReactivationResultErrorCause ReactivationResultErrorCause // optional (IEI 0x72), TLV-E
	EAP                          []byte                       // optional (IEI 0x78), TLV-E

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

var serviceAcceptIEs = []nas.OptionalIE{
	{IEI: ieiPDUSessionStatus, Format: nas.IETLV, Name: "PDU session status"},
	{IEI: ieiPDUReactResult, Format: nas.IETLV, Name: "PDU session reactivation result"},
	{IEI: ieiPDUReactErrCause, Format: nas.IETLVE, Name: "PDU session reactivation result error cause"},
	{IEI: ieiEAPMessage, Format: nas.IETLVE, Name: "EAP message"},
}

// ParseServiceAccept decodes the message.
func ParseServiceAccept(b []byte) (*ServiceAccept, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgServiceAccept); err != nil {
		return nil, err
	}

	out := &ServiceAccept{}

	_unrec, err := walkOptionalIEs(r, serviceAcceptIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiPDUSessionStatus:
			bitmap, err := ParsePSIBitmap(value)
			if err != nil {
				return false, err
			}

			out.PDUSessionStatus = &bitmap
		case ieiPDUReactResult:
			bitmap, err := ParsePSIBitmap(value)
			if err != nil {
				return false, err
			}

			out.PDUSessionReactivationResult = &bitmap
		case ieiPDUReactErrCause:
			causes, err := ParseReactivationResultErrorCause(value)
			if err != nil {
				return false, err
			}

			out.ReactivationResultErrorCause = causes
		case ieiEAPMessage:
			out.EAP = value
		default:
			return false, nil
		}

		return true, nil
	})
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// AppendBinary encodes the plain SERVICE ACCEPT message.
// The encoding is appended to b.
func (m *ServiceAccept) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgServiceAccept)

	if m.PDUSessionStatus != nil {
		raw, err := m.PDUSessionStatus.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiPDUSessionStatus, raw)
	}

	if m.PDUSessionReactivationResult != nil {
		raw, err := m.PDUSessionReactivationResult.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiPDUReactResult, raw)
	}

	if m.ReactivationResultErrorCause != nil {
		raw, err := m.ReactivationResultErrorCause.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLVE(ieiPDUReactErrCause, raw)
	}

	if m.EAP != nil {
		o.TLVE(ieiEAPMessage, m.EAP)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *ServiceAccept) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// PSIBitmap is a PDU session identity bitmap: PSI(n) in bit n of a two-octet
// field, which later releases may extend (TS 24.501 §9.11.3.44). It is the value
// of the PDU session status, PDU session reactivation result, uplink data status
// and allowed PDU session status elements. Rest carries any octets beyond the
// first two so the element re-encodes at the length it arrived with.
type PSIBitmap struct {
	PSI  [16]bool
	Rest []byte
}

// ParsePSIBitmap decodes a PDU session identity bitmap. A value shorter than the
// mandatory two octets is malformed: decoding it as an empty bitmap would report
// every session inactive.
func ParsePSIBitmap(b []byte) (PSIBitmap, error) {
	if len(b) < 2 || len(b) > maxPSIBitmapLen {
		return PSIBitmap{}, fmt.Errorf(
			"nas/fgs: PDU session identity bitmap is %d octets, want 2 to %d", len(b), maxPSIBitmapLen)
	}

	var out PSIBitmap

	for i := range 16 {
		if b[i/8]&(1<<(i%8)) != 0 {
			out.PSI[i] = true
		}
	}

	if len(b) > 2 {
		out.Rest = bytes.Clone(b[2:])
	}

	return out, nil
}

// maxPSIBitmapLen is the longest value the elements carrying this bitmap take:
// TS 24.501 §9.11.3.44 caps the PDU session status element at 34 octets, two of
// which are its IEI and length.
const maxPSIBitmapLen = 32

// AppendBinary encodes the PDU session identity bitmap onto b.
func (p PSIBitmap) AppendBinary(b []byte) ([]byte, error) {
	if n := 2 + len(p.Rest); n > maxPSIBitmapLen {
		return b, fmt.Errorf(
			"nas/fgs: PDU session identity bitmap is %d octets, want at most %d", n, maxPSIBitmapLen)
	}

	var buf [2]byte

	for i := range 16 {
		if p.PSI[i] {
			buf[i/8] |= 1 << (i % 8)
		}
	}

	b = append(b, buf[:]...)

	return append(b, p.Rest...), nil
}

// MarshalBinary encodes the PDU session identity bitmap.
func (p PSIBitmap) MarshalBinary() ([]byte, error) { return p.AppendBinary(nil) }

// Set marks PDU session identity psi active. It ignores an identity outside the
// two octets this type models.
func (p *PSIBitmap) Set(psi int) {
	if psi >= 0 && psi < len(p.PSI) {
		p.PSI[psi] = true
	}
}
