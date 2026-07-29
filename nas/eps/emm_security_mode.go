// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// IMEISVRequest is the IMEISV request value (TS 24.008 §10.5.5.10, which
// TS 24.301 §9.9.3.18 adopts): 1 asks the UE for its IMEISV and every other
// value is read as not asking for it. The value is carried as it arrived rather
// than collapsed to a bool, since the reading is many-to-one and the element has
// to re-encode to the octet it came from.
type IMEISVRequest uint8

const (
	// IMEISVNotRequested is the assigned "not requested" value; values 2 to 7 are
	// read the same way.
	IMEISVNotRequested IMEISVRequest = 0
	// IMEISVRequested asks the UE to include its IMEISV in the SECURITY MODE
	// COMPLETE.
	IMEISVRequested IMEISVRequest = 1
)

// Requested reports whether the value asks for the IMEISV.
func (r IMEISVRequest) Requested() bool { return r == IMEISVRequested }

func (r IMEISVRequest) String() string {
	if r.Requested() {
		return "IMEISV requested"
	}

	return "IMEISV not requested"
}

// parseIMEISVRequest decodes the IMEISV request half-octet.
func parseIMEISVRequest(v []byte) (IMEISVRequest, error) {
	if len(v) != 1 {
		return 0, fmt.Errorf("nas/eps: IMEISV request is %d octets, want 1", len(v))
	}

	return IMEISVRequest(v[0] & 0x0F), nil
}

// SecurityModeCommand is the SECURITY MODE COMMAND message (TS 24.301),
// sent by the MME to select the NAS security algorithms. The optional part is
// preserved verbatim.
type SecurityModeCommand struct {
	CipheringAlgorithm           nas.CipheringAlgorithm
	IntegrityAlgorithm           nas.IntegrityAlgorithm
	NASKeySetIdentifier          nas.KeySetIdentifier
	ReplayedUESecurityCapability UESecurityCapability

	// IMEISVRequested asks the UE for its IMEISV in SECURITY MODE COMPLETE
	// (IEI 0xC). Nil means the element is absent, which the UE reads the same way
	// as a false it carries.
	IMEISVRequested *IMEISVRequest

	HASHMME []byte // 8-octet hash of the triggering plain Attach/TAU (TS 24.301), nil when absent

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain SECURITY MODE COMMAND message.
// The encoding is appended to b.
func (m *SecurityModeCommand) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgSecurityModeCommand)
	w.U8(uint8(m.CipheringAlgorithm)&0x07<<4 | uint8(m.IntegrityAlgorithm)&0x07) // selected NAS security algorithms
	w.U8(m.NASKeySetIdentifier.HalfOctet())                                      // NAS KSI | spare half octet

	replayed, err := m.ReplayedUESecurityCapability.MarshalBinary()
	if err != nil {
		return b, err
	}

	w.LV(replayed)

	if m.IMEISVRequested != nil {
		o.TV1(ieiIMEISVRequest, uint8(*m.IMEISVRequested))
	}

	if m.HASHMME != nil {
		o.TLV(ieiHashMME, m.HASHMME)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *SecurityModeCommand) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseSecurityModeCommand decodes a plain SECURITY MODE COMMAND message.
func ParseSecurityModeCommand(b []byte) (*SecurityModeCommand, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgSecurityModeCommand); err != nil {
		return nil, err
	}

	alg, err := r.U8()
	if err != nil {
		return nil, err
	}

	ksi, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &SecurityModeCommand{
		CipheringAlgorithm:  nas.CipheringAlgorithm(alg >> 4 & 0x07),
		IntegrityAlgorithm:  nas.IntegrityAlgorithm(alg & 0x07),
		NASKeySetIdentifier: nas.ParseKeySetIdentifier(ksi),
	}

	replayedRaw, err := r.LV()
	if err != nil {
		return nil, err
	}

	if m.ReplayedUESecurityCapability, err = ParseUESecurityCapability(replayedRaw); err != nil {
		return nil, err
	}

	// The IMEISV request is a type-1 IE the walker delimits inherently (IEI >= 0x80
	// after the half-octet shift); HashMME is a type-4 TLV and needs a table entry.
	_unrec, err := walkOptionalIEs(r, securityModeCommandIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiIMEISVRequest:
			// Only the "IMEISV requested" value is modelled; TS 24.301 §9.9.3.12
			// leaves the others unassigned, which makes them syntactically
			// incorrect optional elements: absent, but preserved (§7.7.1).
			requested, err := parseIMEISVRequest(value)
			if err != nil {
				return false, err
			}

			m.IMEISVRequested = &requested
		case ieiHashMME:
			m.HASHMME = value
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

// securityModeCommandIEs are the optional type-4 IEs Ella Core round-trips in a
// SECURITY MODE COMMAND (TS 24.301): the HashMME.
var securityModeCommandIEs = []nas.OptionalIE{
	{IEI: ieiReplayedNonceUE, Format: nas.IETV3, Len: 4, Name: "Replayed nonce UE"},
	{IEI: ieiNonceMME, Format: nas.IETV3, Len: 4, Name: "Nonce MME"},
	{IEI: ieiHashMME, Format: nas.IETLV, Name: "HashMME"},
}

// SecurityModeComplete is the SECURITY MODE COMPLETE message (TS 24.301).
// It has no mandatory information elements; the UE includes its IMEISV
// (a mobile identity, IEI 0x23) when the MME requested it, and — when its HASHMME
// check fails — the complete plain ATTACH/TAU REQUEST it originally sent, in the
// Replayed NAS message container (IEI 0x79), so the network can recover the
// genuine triggering message (TS 24.301 §5.4.3.4).
type SecurityModeComplete struct {
	IMEISV                      *MobileIdentity // IMEISV mobile identity (IEI 0x23), when present
	ReplayedNASMessageContainer []byte          // complete triggering NAS message (IEI 0x79), when present

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// securityModeCompleteIEs are the optional IEs Ella Core consumes from a
// SECURITY MODE COMPLETE (TS 24.301): the UE's IMEISV mobile identity and the
// Replayed NAS message container.
var securityModeCompleteIEs = []nas.OptionalIE{
	{IEI: ieiIMEISV, Format: nas.IETLV, Name: "IMEISV"},
	{IEI: ieiReplayedNASMessage, Format: nas.IETLVE, Critical: true, Name: "Replayed NAS message"},
}

// AppendBinary encodes the plain SECURITY MODE COMPLETE message.
// The encoding is appended to b.
func (m *SecurityModeComplete) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgSecurityModeComplete)

	if m.IMEISV != nil {
		raw, err := m.IMEISV.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiIMEISV, raw)
	}

	if m.ReplayedNASMessageContainer != nil {
		o.TLVE(ieiReplayedNASMessage, m.ReplayedNASMessageContainer)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *SecurityModeComplete) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseSecurityModeComplete decodes a plain SECURITY MODE COMPLETE message.
func ParseSecurityModeComplete(b []byte) (*SecurityModeComplete, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgSecurityModeComplete); err != nil {
		return nil, err
	}

	m := &SecurityModeComplete{}

	_unrec, err := walkOptionalIEs(r, securityModeCompleteIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiIMEISV:
			parsed, err := ParseMobileIdentity(value)
			if err != nil {
				return false, err
			}

			m.IMEISV = &parsed
		case ieiReplayedNASMessage:
			m.ReplayedNASMessageContainer = value
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

// SecurityModeReject is the SECURITY MODE REJECT message (TS 24.301).
type SecurityModeReject struct {
	Cause EMMCause

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged. The spec defines none for this message, but a later release may.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain SECURITY MODE REJECT message.
// The encoding is appended to b.
func (m *SecurityModeReject) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeEMMHeader(w, MsgSecurityModeReject)
	w.U8(uint8(m.Cause))

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *SecurityModeReject) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseSecurityModeReject decodes a plain SECURITY MODE REJECT message.
func ParseSecurityModeReject(b []byte) (*SecurityModeReject, error) {
	r := nas.NewReader(b)

	if err := readEMMHeader(r, MsgSecurityModeReject); err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &SecurityModeReject{Cause: EMMCause(cause)}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}
