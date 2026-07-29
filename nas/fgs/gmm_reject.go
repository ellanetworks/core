// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas"

// GMMStatus is the 5GMM STATUS message (TS 24.501 §8.2.29): a mandatory 5GMM
// cause reporting an error detected on received 5GMM signalling.
type GMMStatus struct {
	Cause GMMCause

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged. The spec
	// defines none for this message, but a later release may.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain 5GMM STATUS message.
// The encoding is appended to b.
func (m *GMMStatus) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgGMMStatus)
	w.U8(uint8(m.Cause))

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *GMMStatus) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseGMMStatus decodes the message.
func ParseGMMStatus(b []byte) (*GMMStatus, error) {
	return parseGMMCause(b, MsgGMMStatus)
}

// SecurityModeReject is the SECURITY MODE REJECT message (TS 24.501 §8.2.27): a
// mandatory 5GMM cause.
type SecurityModeReject struct {
	Cause GMMCause

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged. The spec
	// defines none for this message, but a later release may.
	Unrecognized []nas.RawIE
}

// ParseSecurityModeReject decodes the message.
func ParseSecurityModeReject(b []byte) (*SecurityModeReject, error) {
	s, err := parseGMMCause(b, MsgSecurityModeReject)
	if s == nil {
		return nil, err
	}

	return &SecurityModeReject{Cause: s.Cause, Unrecognized: s.Unrecognized}, err
}

// AppendBinary encodes the plain SECURITY MODE REJECT message
// (TS 24.501 §8.2.26). The encoding is appended to b.
func (m *SecurityModeReject) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgSecurityModeReject)
	w.U8(uint8(m.Cause))

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *SecurityModeReject) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ServiceReject is the SERVICE REJECT message (TS 24.501 §8.2.18): a mandatory
// 5GMM cause and optional PDU session status, T3346 timer and EAP message. Ella
// sets only the cause; the parser accepts the rest.
type ServiceReject struct {
	Cause            GMMCause
	PDUSessionStatus *PSIBitmap      // optional (IEI 0x50)
	T3346            *nas.GPRSTimer2 // optional (IEI 0x5F)
	EAP              []byte          // optional (IEI 0x78)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain SERVICE REJECT message.
// The encoding is appended to b.
func (m *ServiceReject) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgServiceReject)
	w.U8(uint8(m.Cause))

	if m.PDUSessionStatus != nil {
		raw, err := m.PDUSessionStatus.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiPDUSessionStatus, raw)
	}

	if m.T3346 != nil {
		raw, err := m.T3346.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiT3346Value, raw)
	}

	if m.EAP != nil {
		o.TLVE(ieiEAPMessage, m.EAP)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *ServiceReject) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

var serviceRejectIEs = []nas.OptionalIE{
	{IEI: ieiPDUSessionStatus, Format: nas.IETLV, Name: "PDU session status"},
	{IEI: ieiT3346Value, Format: nas.IETLV, Name: "T3346 value"},
	{IEI: ieiEAPMessage, Format: nas.IETLVE, Name: "EAP message"},
}

// ParseServiceReject decodes the message.
func ParseServiceReject(b []byte) (*ServiceReject, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgServiceReject); err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &ServiceReject{Cause: GMMCause(cause)}

	_unrec, err := walkOptionalIEs(r, serviceRejectIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiPDUSessionStatus:
			bitmap, err := ParsePSIBitmap(value)
			if err != nil {
				return false, err
			}

			out.PDUSessionStatus = &bitmap
		case ieiT3346Value:
			if len(value) == 0 {
				return false, nil
			}

			timer, err := nas.ParseGPRSTimer2(value)
			if err != nil {
				return false, err
			}

			out.T3346 = &timer
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

// RegistrationReject is the REGISTRATION REJECT message (TS 24.501 §8.2.9): a
// mandatory 5GMM cause and optional T3346/T3502 timers and an EAP message. Ella
// sets only T3502; the parser accepts the rest.
type RegistrationReject struct {
	Cause GMMCause
	T3502 *nas.GPRSTimer2 // optional (IEI 0x16)
	T3346 *nas.GPRSTimer2 // optional (IEI 0x5F)
	EAP   []byte          // optional (IEI 0x78)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain REGISTRATION REJECT message.
// The encoding is appended to b.
func (m *RegistrationReject) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgRegistrationReject)
	w.U8(uint8(m.Cause))

	if m.T3346 != nil {
		raw, err := m.T3346.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiT3346Value, raw)
	}

	if m.T3502 != nil {
		raw, err := m.T3502.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiT3502Value, raw)
	}

	if m.EAP != nil {
		o.TLVE(ieiEAPMessage, m.EAP)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *RegistrationReject) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

var registrationRejectIEs = []nas.OptionalIE{
	{IEI: ieiT3346Value, Format: nas.IETLV, Name: "T3346 value"},
	{IEI: ieiT3502Value, Format: nas.IETLV, Name: "T3502 value"},
	{IEI: ieiEAPMessage, Format: nas.IETLVE, Name: "EAP message"},
}

// ParseRegistrationReject decodes the message.
func ParseRegistrationReject(b []byte) (*RegistrationReject, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgRegistrationReject); err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &RegistrationReject{Cause: GMMCause(cause)}

	_unrec, err := walkOptionalIEs(r, registrationRejectIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiT3346Value:
			if len(value) == 0 {
				return false, nil
			}

			timer, err := nas.ParseGPRSTimer2(value)
			if err != nil {
				return false, err
			}

			out.T3346 = &timer
		case ieiT3502Value:
			if len(value) == 0 {
				return false, nil
			}

			timer, err := nas.ParseGPRSTimer2(value)
			if err != nil {
				return false, err
			}

			out.T3502 = &timer
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

// parseGMMCause decodes a 5GMM message whose body is a single mandatory 5GMM
// cause octet.
func parseGMMCause(b []byte, want MessageType) (*GMMStatus, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, want); err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &GMMStatus{Cause: GMMCause(cause)}

	unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = unrec

	return out, err
}
