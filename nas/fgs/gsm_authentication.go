// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas"

// PDUSessionAuthenticationComplete is the PDU SESSION AUTHENTICATION COMPLETE
// (TS 24.501 §8.3.5): the 5GSM header, a mandatory EAP message, and optionally
// the extended protocol configuration options.
//
// A network that runs no secondary authentication still has to decode it, since
// TS 24.501 §7.3.1 b) has the receiver answer a PTI that is not the unassigned
// value with a 5GSM STATUS.
type PDUSessionAuthenticationComplete struct {
	PDUSessionID PDUSessionID
	PTI          nas.ProcedureTransactionIdentity
	// EAP is the EAP packet the UE answers the authentication command with; this
	// codec carries it without interpreting it (TS 24.501 §9.11.2.2).
	EAP         []byte
	ExtendedPCO *nas.ProtocolConfigurationOptions // optional (IEI 0x7B)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain PDU SESSION AUTHENTICATION COMPLETE message.
// The encoding is appended to b.
func (m *PDUSessionAuthenticationComplete) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGSMHeader(w, m.PDUSessionID, m.PTI, MsgPDUSessionAuthenticationComplete)
	w.LVE(m.EAP)

	if m.ExtendedPCO != nil {
		raw, err := m.ExtendedPCO.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLVE(ieiExtendedPCO, raw)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *PDUSessionAuthenticationComplete) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// authenticationCompleteIEs is the full-octet optional-IE table of the PDU
// SESSION AUTHENTICATION COMPLETE (TS 24.501 §8.3.5, table 8.3.5.1.1).
var authenticationCompleteIEs = []nas.OptionalIE{
	{IEI: ieiExtendedPCO, Format: nas.IETLVE, Name: "Extended PCO"},
}

// ParsePDUSessionAuthenticationComplete decodes the message.
func ParsePDUSessionAuthenticationComplete(b []byte) (*PDUSessionAuthenticationComplete, error) {
	r := nas.NewReader(b)

	psi, pti, err := readGSMHeader(r, MsgPDUSessionAuthenticationComplete)
	if err != nil {
		return nil, err
	}

	eap, err := r.LVE()
	if err != nil {
		return nil, err
	}

	out := &PDUSessionAuthenticationComplete{PDUSessionID: psi, PTI: pti, EAP: eap}

	_unrec, err := walkOptionalIEs(r, authenticationCompleteIEs, func(iei uint8, value []byte) (bool, error) {
		if iei != ieiExtendedPCO {
			return false, nil
		}

		parsed, err := nas.ParseProtocolConfigurationOptions(value, nas.PCOMSToNetwork)
		if err != nil {
			return false, err
		}

		out.ExtendedPCO = &parsed

		return true, nil
	})
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}
