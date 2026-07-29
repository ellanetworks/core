// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// IMEISV request values (TS 24.501 §9.11.3.28).
const (
	IMEISVNotRequested uint8 = 0x00
	IMEISVRequested    uint8 = 0x01
)

// SecurityModeCommand is the SECURITY MODE COMMAND message (TS 24.501 §8.2.25):
// the selected NAS security algorithms, the ngKSI, and the replayed UE security
// capabilities, with optional IMEISV request and additional 5G security
// information.
type SecurityModeCommand struct {
	CipheringAlgorithm           nas.CipheringAlgorithm // selected NEA (bits 5-8)
	IntegrityAlgorithm           nas.IntegrityAlgorithm // selected NIA (bits 1-4)
	NgKSI                        nas.KeySetIdentifier   // bits 1-4
	ReplayedUESecurityCapability UESecurityCapability

	// IMEISVRequested asks the UE for its IMEISV in SECURITY MODE COMPLETE
	// (IEI 0xE). Nil means the element is absent, which the UE reads the same way
	// as a false it carries.
	IMEISVRequested *bool

	AdditionalSecurityInformation *AdditionalSecurityInformation // optional (IEI 0x36)

	// Optional IEs Ella never sends, exposed for decoding.
	SelectedEPSNASSecurityAlgorithms *SelectedEPSNASSecurityAlgorithms // optional (IEI 0x57)
	ABBA                             []byte                            // optional (IEI 0x38)
	EAP                              []byte                            // optional (IEI 0x78)
	// ReplayedS1UESecurityCapability is the S1 UE security capability the AMF
	// replays (TS 24.501 §9.11.3.48A, which defers to TS 24.301 §9.9.3.36). It
	// stays opaque deliberately: the UE compares it byte-for-byte against what it
	// sent (TS 33.501 §6.7.2), so decoding and re-encoding it could only lose that.
	ReplayedS1UESecurityCapability []byte // optional (IEI 0x19)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// IMEISV request values (TS 24.501 §9.11.3.28). They match the EPS ones, which
// TS 24.301 §9.9.3.18 codes identically.
const (
	imeisvNotRequested uint8 = 0x00
	imeisvRequested    uint8 = 0x01
)

// parseIMEISVRequest decodes the IMEISV request half-octet. TS 24.501 §9.11.3.28
// leaves every value but the two below unassigned, which makes them
// syntactically incorrect optional elements: absent, but preserved (§7.7.1).
func parseIMEISVRequest(v []byte) (bool, error) {
	if len(v) != 1 || v[0]&0x07 > imeisvRequested {
		return false, fmt.Errorf("nas/fgs: IMEISV request value %#x is not assigned", v)
	}

	return v[0]&0x07 == imeisvRequested, nil
}

// imeisvRequestOctet is the half-octet a request encodes to.
func imeisvRequestOctet(requested bool) uint8 {
	if requested {
		return imeisvRequested
	}

	return imeisvNotRequested
}

// AdditionalSecurityInformation is the additional 5G security information IE
// (TS 24.501 §9.11.3.12): RINMR (retransmission of the initial NAS message
// request, bit 2) and HDP (horizontal derivation parameter, bit 1).
type AdditionalSecurityInformation struct {
	RINMR bool
	HDP   bool
}

// ParseAdditionalSecurityInformation decodes the information element value.
func ParseAdditionalSecurityInformation(v []byte) (AdditionalSecurityInformation, error) {
	if len(v) != 1 {
		return AdditionalSecurityInformation{}, fmt.Errorf("nas/fgs: additional 5G security information: want 1 octet, got %d", len(v))
	}

	return AdditionalSecurityInformation{RINMR: v[0]>>1&0x01 != 0, HDP: v[0]&0x01 != 0}, nil
}

// AppendBinary encodes the information element value onto b.
func (a AdditionalSecurityInformation) AppendBinary(b []byte) ([]byte, error) {
	return append(b, boolBit(a.HDP, 0)|boolBit(a.RINMR, 1)), nil
}

// MarshalBinary encodes the information element value.
func (a AdditionalSecurityInformation) MarshalBinary() ([]byte, error) { return a.AppendBinary(nil) }

// SelectedEPSNASSecurityAlgorithms is the pair of EPS NAS algorithms carried in
// the Selected EPS NAS security algorithms IE (TS 24.501 §9.11.3.25 → TS 24.301
// §9.9.3.23): EPS ciphering (EEA) and EPS integrity (EIA).
type SelectedEPSNASSecurityAlgorithms struct {
	Ciphering uint8 // EEA, bits 5-7
	Integrity uint8 // EIA, bits 1-3
}

// ParseSelectedEPSNASSecurityAlgorithms decodes the information element value
// into its two algorithm fields.
func ParseSelectedEPSNASSecurityAlgorithms(v []byte) (SelectedEPSNASSecurityAlgorithms, error) {
	if len(v) != 1 {
		return SelectedEPSNASSecurityAlgorithms{}, fmt.Errorf("nas/fgs: selected EPS NAS security algorithms: want 1 octet, got %d", len(v))
	}

	return SelectedEPSNASSecurityAlgorithms{
		Ciphering: v[0] >> 4 & 0x07,
		Integrity: v[0] & 0x07,
	}, nil
}

// AppendBinary encodes the information element value onto b.
func (s SelectedEPSNASSecurityAlgorithms) AppendBinary(b []byte) ([]byte, error) {
	return append(b, (s.Ciphering&0x07)<<4|s.Integrity&0x07), nil
}

// MarshalBinary encodes the information element value.
func (s SelectedEPSNASSecurityAlgorithms) MarshalBinary() ([]byte, error) { return s.AppendBinary(nil) }

// AppendBinary encodes the plain SECURITY MODE COMMAND message.
// The encoding is appended to b.
func (m *SecurityModeCommand) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgSecurityModeCommand)
	w.U8(uint8(m.CipheringAlgorithm)&0x0F<<4 | uint8(m.IntegrityAlgorithm)&0x0F)
	w.U8(m.NgKSI.HalfOctet()) // spare half octet in bits 5-8

	replayed, err := m.ReplayedUESecurityCapability.MarshalBinary()
	if err != nil {
		return b, err
	}

	w.LV(replayed)

	// TS 24.501 table 8.2.25.1 lists the optional elements in this order.
	if m.IMEISVRequested != nil {
		o.TV1(ieiIMEISVRequest, imeisvRequestOctet(*m.IMEISVRequested))
	}

	if m.SelectedEPSNASSecurityAlgorithms != nil {
		alg, err := m.SelectedEPSNASSecurityAlgorithms.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TV3(ieiSelectedEPSNASSecAlg, alg)
	}

	if m.AdditionalSecurityInformation != nil {
		info, err := m.AdditionalSecurityInformation.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiAdditional5GSec, info)
	}

	if m.EAP != nil {
		o.TLVE(ieiEAPMessage, m.EAP)
	}

	if m.ABBA != nil {
		o.TLV(ieiABBA, m.ABBA)
	}

	if m.ReplayedS1UESecurityCapability != nil {
		o.TLV(ieiReplayedS1UESecCap, m.ReplayedS1UESecurityCapability)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *SecurityModeCommand) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

var securityModeCommandIEs = []nas.OptionalIE{
	{IEI: ieiReplayedS1UESecCap, Format: nas.IETLV, Critical: true, Name: "Replayed S1 UE security capability"},
	{IEI: ieiAdditional5GSec, Format: nas.IETLV, Name: "Additional 5G security information"},
	{IEI: ieiABBA, Format: nas.IETLV, Critical: true, Name: "ABBA"},
	{IEI: ieiSelectedEPSNASSecAlg, Format: nas.IETV3, Len: 1, Name: "Selected EPS NAS security algorithms"},
	{IEI: ieiEAPMessage, Format: nas.IETLVE, Name: "EAP message"},
}

// ParseSecurityModeCommand decodes the message.
func ParseSecurityModeCommand(b []byte) (*SecurityModeCommand, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgSecurityModeCommand); err != nil {
		return nil, err
	}

	alg, err := r.U8()
	if err != nil {
		return nil, err
	}

	ngksi, err := r.U8()
	if err != nil {
		return nil, err
	}

	replayedRaw, err := r.LV()
	if err != nil {
		return nil, err
	}

	replayed, err := ParseUESecurityCapability(replayedRaw)
	if err != nil {
		return nil, err
	}

	out := &SecurityModeCommand{
		CipheringAlgorithm:           nas.CipheringAlgorithm(alg >> 4),
		IntegrityAlgorithm:           nas.IntegrityAlgorithm(alg & 0x0F),
		NgKSI:                        nas.ParseKeySetIdentifier(ngksi),
		ReplayedUESecurityCapability: replayed,
	}

	_unrec, err := walkOptionalIEs(r, securityModeCommandIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiIMEISVRequest: // type 1: value is the low nibble
			requested, err := parseIMEISVRequest(value)
			if err != nil {
				return false, err
			}

			out.IMEISVRequested = &requested
		case ieiReplayedS1UESecCap:
			out.ReplayedS1UESecurityCapability = value
		case ieiAdditional5GSec:
			info, err := ParseAdditionalSecurityInformation(value)
			if err != nil {
				return false, err
			}

			out.AdditionalSecurityInformation = &info
		case ieiABBA:
			out.ABBA = value
		case ieiSelectedEPSNASSecAlg:
			alg, err := ParseSelectedEPSNASSecurityAlgorithms(value)
			if err != nil {
				return false, err
			}

			out.SelectedEPSNASSecurityAlgorithms = &alg
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

// SecurityModeComplete is the SECURITY MODE COMPLETE message (TS 24.501 §8.2.26).
// It has no mandatory IEs; the UE includes its IMEISV (a 5GS mobile identity,
// IEI 0x77) when the network requested it, and — when it rejected the replayed
// UE security capabilities — the complete plain REGISTRATION REQUEST it originally
// sent, in the NAS message container (IEI 0x71), so the network can recover the
// genuine triggering message (TS 24.501 §5.4.2.3).
type SecurityModeComplete struct {
	IMEISV              *MobileIdentity // IMEISV 5GS mobile identity (IEI 0x77), when present
	NASMessageContainer []byte          // complete triggering NAS message (IEI 0x71), when present

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

var securityModeCompleteIEs = []nas.OptionalIE{
	{IEI: ieiIMEISV, Format: nas.IETLVE, Name: "IMEISV"},
	{IEI: ieiNASMessageContainer, Format: nas.IETLVE, Critical: true, Name: "NAS message container"},
}

// AppendBinary encodes the plain SECURITY MODE COMPLETE message (TS 24.501 §8.2.26).
// The encoding is appended to b.
func (m *SecurityModeComplete) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgSecurityModeComplete)

	if m.IMEISV != nil {
		raw, err := m.IMEISV.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLVE(ieiIMEISV, raw)
	}

	if m.NASMessageContainer != nil {
		o.TLVE(ieiNASMessageContainer, m.NASMessageContainer)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *SecurityModeComplete) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseSecurityModeComplete decodes a plain SECURITY MODE COMPLETE message.
func ParseSecurityModeComplete(b []byte) (*SecurityModeComplete, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgSecurityModeComplete); err != nil {
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
		case ieiNASMessageContainer:
			m.NASMessageContainer = value
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
