// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"crypto/subtle"
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// ServiceRequest is the SERVICE REQUEST message (TS 24.301): a 4-octet
// frame with no message identity — the security header type octet, then the KSI
// and a 5-bit truncated NAS sequence number, then a 2-octet short MAC. The UE
// sends it from EMM-IDLE to re-establish the NAS connection.
type ServiceRequest struct {
	// KSI is the key set identifier value alone: the KSI and sequence number IE
	// carries no type-of-security-context flag (TS 24.301 §9.9.3.19), so this
	// message is the one place the identifier is not a nas.KeySetIdentifier.
	KSI      uint8
	SeqShort uint8 // 5-bit truncated uplink NAS sequence number
	ShortMAC [2]byte
}

// ParseServiceRequest decodes a SERVICE REQUEST. The caller has already
// identified it by the security header type (SHTServiceRequest).
func ParseServiceRequest(b []byte) (*ServiceRequest, error) {
	if len(b) != 4 {
		return nil, fmt.Errorf("nas/eps: SERVICE REQUEST is %d octets, want 4", len(b))
	}

	return &ServiceRequest{
		KSI:      (b[1] >> 5) & 0x07,
		SeqShort: b[1] & 0x1f,
		ShortMAC: [2]byte{b[2], b[3]},
	}, nil
}

// AppendBinary encodes the SERVICE REQUEST message (TS 24.301 §8.2.25). Unlike
// every other EPS message it is not a plain NAS message: its four octets are the
// security-header type and protocol discriminator, the KSI and truncated
// sequence number, and the two-octet short MAC.
// The encoding is appended to b.
func (m *ServiceRequest) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	w.U8(uint8(SHTServiceRequest)<<4 | uint8(PDEMM))
	w.U8(m.KSI&0x07<<5 | m.SeqShort&0x1F)
	w.Raw(m.ShortMAC[:])

	return messageResult(w, b)
}

// MarshalBinary encodes the SERVICE REQUEST message.
func (m *ServiceRequest) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// NewServiceRequest builds a SERVICE REQUEST for the given NAS COUNT, computing
// the short MAC the network will verify (TS 24.301 §8.2.25). The message carries
// the five least significant bits of the count's sequence number, which is what
// the network reconstructs the full count from.
func NewServiceRequest(ksi uint8, count nas.Count, sc *nas.SecurityContext) (*ServiceRequest, error) {
	if sc == nil {
		return nil, nas.ErrNoSecurityContext
	}

	m := &ServiceRequest{KSI: ksi & 0x07, SeqShort: count.SQN() & 0x1F}

	mac, err := m.shortMAC(count, sc)
	if err != nil {
		return nil, err
	}

	m.ShortMAC = mac

	return m, nil
}

// VerifyServiceRequestShortMAC checks the short MAC a SERVICE REQUEST carries,
// in constant time, and returns [ErrMACMismatch] when it does not verify.
//
// The message is uplink by definition: a UE sends it to resume a connection
// (TS 24.301 §5.6.1). Estimate count from the message's truncated sequence
// number with a [nas.UplinkCounter] and commit it only once this has returned
// without error — the short MAC is 16 bits, so replay protection rests on the
// counter, not on the MAC.
func VerifyServiceRequestShortMAC(m *ServiceRequest, count nas.Count, sc *nas.SecurityContext) error {
	if sc == nil {
		return nas.ErrNoSecurityContext
	}

	want, err := m.shortMAC(count, sc)
	if err != nil {
		return err
	}

	if subtle.ConstantTimeCompare(want[:], m.ShortMAC[:]) != 1 {
		return ErrMACMismatch
	}

	return nil
}

// shortMAC is the NAS-MAC over the message's first two octets — the span
// TS 24.301 §8.2.25 protects — truncated to its two least significant octets.
func (m *ServiceRequest) shortMAC(count nas.Count, sc *nas.SecurityContext) ([2]byte, error) {
	header := []byte{
		uint8(SHTServiceRequest)<<4 | uint8(PDEMM),
		m.KSI&0x07<<5 | m.SeqShort&0x1F,
	}

	mac, err := sc.MAC(header, count, nasBearer, nas.DirectionUplink)
	if err != nil {
		return [2]byte{}, err
	}

	return [2]byte{mac[2], mac[3]}, nil
}

// ServiceReject is the SERVICE REJECT message (TS 24.301), sent by the
// network to refuse a service request with an EMM cause.
type ServiceReject struct {
	Cause EMMCause

	// T3442 is the element TS 24.301 table 8.2.24.1 calls a GPRS timer rather
	// than a GPRS timer 2; TS 24.008 §10.5.7.4 defines timer 2's value as octet 2
	// of it, so the two differ only in framing and share this value type.
	T3442 *nas.GPRSTimer2 // conditional (IEI 0x5B): sent for EMM cause #39
	T3346 *nas.GPRSTimer2 // optional (IEI 0x5F)
	T3448 *nas.GPRSTimer2 // optional (IEI 0x6B)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// serviceRejectIEs is the optional-IE table of the SERVICE REJECT message
// (TS 24.301 §8.2.24, table 8.2.24.1).
var serviceRejectIEs = []nas.OptionalIE{
	{IEI: ieiT3442Value, Format: nas.IETV3, Len: 1, Name: "T3442 value"},
	{IEI: ieiT3346Value, Format: nas.IETLV, Name: "T3346 value"},
	{IEI: ieiT3448Value, Format: nas.IETLV, Name: "T3448 value"},
	{IEI: ieiLowerBoundTimer, Format: nas.IETLV, Name: "Lower bound timer value"},
	{IEI: ieiForbiddenTAIRoaming, Format: nas.IETLV, Name: "Forbidden TAIs for roaming"},
	{IEI: ieiForbiddenTAIRegional, Format: nas.IETLV, Name: "Forbidden TAIs for regional provision of service"},
}

// AppendBinary encodes the plain SERVICE REJECT message.
// The encoding is appended to b.
func (m *ServiceReject) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgServiceReject)
	w.U8(uint8(m.Cause))

	if m.T3442 != nil {
		raw, err := m.T3442.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TV3(ieiT3442Value, raw)
	}

	if m.T3346 != nil {
		raw, err := m.T3346.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiT3346Value, raw)
	}

	if m.T3448 != nil {
		raw, err := m.T3448.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiT3448Value, raw)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *ServiceReject) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseServiceReject decodes the message.
func ParseServiceReject(b []byte) (*ServiceReject, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgServiceReject); err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &ServiceReject{Cause: EMMCause(cause)}

	_unrec, err := walkOptionalIEs(r, serviceRejectIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiT3442Value:
			timer, err := nas.ParseGPRSTimer2(value)
			if err != nil {
				return false, err
			}

			out.T3442 = &timer
		case ieiT3346Value:
			timer, err := nas.ParseGPRSTimer2(value)
			if err != nil {
				return false, err
			}

			out.T3346 = &timer
		case ieiT3448Value:
			timer, err := nas.ParseGPRSTimer2(value)
			if err != nil {
				return false, err
			}

			out.T3448 = &timer
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

// ServiceAccept is the SERVICE ACCEPT message (TS 24.301 §8.2.34), which the
// network sends to complete a service request. It is the EPS counterpart of
// fgs.ServiceAccept.
type ServiceAccept struct {
	EPSBearerContextStatus *nas.EPSBearerContextStatus // optional (IEI 0x57)
	T3448                  *nas.GPRSTimer2             // optional (IEI 0x6B)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain SERVICE ACCEPT message.
// The encoding is appended to b.
func (m *ServiceAccept) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgServiceAccept)

	if m.EPSBearerContextStatus != nil {
		raw, err := m.EPSBearerContextStatus.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiEPSBearerContextStatus, raw)
	}

	if m.T3448 != nil {
		raw, err := m.T3448.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiT3448Value, raw)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *ServiceAccept) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// serviceAcceptIEs is the optional-IE table of the SERVICE ACCEPT (TS 24.301
// §8.2.34, table 8.2.34.1). Every element the message models needs an entry:
// the walker only offers an element to a message that framed it, and preserves
// the rest untouched.
var serviceAcceptIEs = []nas.OptionalIE{
	{IEI: ieiEPSBearerContextStatus, Format: nas.IETLV, Name: "EPS bearer context status"},
	{IEI: ieiT3448Value, Format: nas.IETLV, Name: "T3448 value"},
}

// ParseServiceAccept decodes the message.
func ParseServiceAccept(b []byte) (*ServiceAccept, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgServiceAccept); err != nil {
		return nil, err
	}

	out := &ServiceAccept{}

	_unrec, err := walkOptionalIEs(r, serviceAcceptIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiEPSBearerContextStatus:
			status, err := nas.ParseEPSBearerContextStatus(value)
			if err != nil {
				return false, err
			}

			out.EPSBearerContextStatus = &status
		case ieiT3448Value:
			timer, err := nas.ParseGPRSTimer2(value)
			if err != nil {
				return false, err
			}

			out.T3448 = &timer
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
