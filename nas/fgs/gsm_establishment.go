// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// PDUSessionEstablishmentRequest is the PDU SESSION ESTABLISHMENT REQUEST
// (TS 24.501 §8.3.1): the 5GSM header, a mandatory integrity protection maximum
// data rate, and optional information elements. Only the fields the network acts
// on are modelled; unmodelled optional IEs are stepped over during parsing.
type PDUSessionEstablishmentRequest struct {
	PDUSessionID             PDUSessionID
	PTI                      nas.ProcedureTransactionIdentity
	IntegrityProtMaxDataRate [2]byte
	PDUSessionType           *PDUSessionType                   // optional (IEI 0x9), value bits 1-3
	SSCMode                  *SSCMode                          // optional (IEI 0xA), value bits 1-3
	GSMCapability            *GSMCapability                    // optional (IEI 0x28)
	AlwaysOnRequested        *bool                             // optional (IEI 0xB), value bit 1
	ExtendedPCO              *nas.ProtocolConfigurationOptions // optional (IEI 0x7B)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain PDU SESSION ESTABLISHMENT REQUEST message. Only the
// modelled fields are emitted; it is the inverse of Parse for those.
// The encoding is appended to b.
func (m *PDUSessionEstablishmentRequest) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGSMHeader(w, m.PDUSessionID, m.PTI, MsgPDUSessionEstablishmentRequest)
	w.Raw(m.IntegrityProtMaxDataRate[:])

	if m.PDUSessionType != nil {
		o.TV1(ieiPDUSessionType, uint8(*m.PDUSessionType))
	}

	if m.SSCMode != nil {
		o.TV1(ieiSSCMode, uint8(*m.SSCMode))
	}

	if m.GSMCapability != nil {
		raw, err := m.GSMCapability.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(iei5GSMCapability, raw)
	}

	if m.AlwaysOnRequested != nil {
		o.TV1(ieiAlwaysOnRequested, boolBit(*m.AlwaysOnRequested, 0))
	}

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
func (m *PDUSessionEstablishmentRequest) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParsePDUSessionEstablishmentRequest decodes the message.
func ParsePDUSessionEstablishmentRequest(b []byte) (*PDUSessionEstablishmentRequest, error) {
	r := nas.NewReader(b)

	psi, pti, err := readGSMHeader(r, MsgPDUSessionEstablishmentRequest)
	if err != nil {
		return nil, err
	}

	rate, err := r.Bytes(2)
	if err != nil {
		return nil, err
	}

	out := &PDUSessionEstablishmentRequest{PDUSessionID: psi, PTI: pti}
	copy(out.IntegrityProtMaxDataRate[:], rate)

	_unrec, err := walkOptionalIEs(r, establishmentRequestIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiPDUSessionType:
			if len(value) == 0 {
				return false, nil
			}

			typ := PDUSessionType(value[0] & 0x07)
			out.PDUSessionType = &typ
		case ieiSSCMode:
			if len(value) == 0 {
				return false, nil
			}

			mode := SSCMode(value[0] & 0x07)
			out.SSCMode = &mode
		case iei5GSMCapability:
			parsed, err := ParseGSMCapability(value)
			if err != nil {
				return false, err
			}

			out.GSMCapability = &parsed
		case ieiAlwaysOnRequested:
			// TS 24.501 table 9.11.4.4.1 assigns both values — 0 "not requested",
			// 1 "requested" — so the element carries its own meaning and the field
			// records whether it arrived, not just that a bit was set.
			if len(value) != 1 {
				return false, fmt.Errorf("nas/fgs: always-on PDU session requested is %d octets, want 1", len(value))
			}

			requested := value[0]&0x01 != 0
			out.AlwaysOnRequested = &requested
		case ieiExtendedPCO:
			parsed, err := nas.ParseExtendedProtocolConfigurationOptions(value, nas.PCOMSToNetwork)
			if err != nil {
				return false, err
			}

			out.ExtendedPCO = &parsed
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

// GSMCapability is the octet-3 content of the 5GSM capability IE
// (TS 24.501 §9.11.4.1, table 9.11.4.1.1).
type GSMCapability struct {
	RqoS    bool  // bit 1: reflective QoS supported
	MH6PDU  bool  // bit 2: multi-homed IPv6 PDU session supported
	EPTS1   bool  // bit 3: Ethernet PDN type in S1 mode supported
	ATSSSST uint8 // bits 4-7: supported ATSSS steering functionalities and modes

	// Rest holds octets 4 onwards verbatim: the per-release feature bits, which
	// this codec does not interpret but must not lose.
	Rest []byte
}

// maxGSMCapabilityLen is the element's longest value: TS 24.501 §9.11.4.1 caps
// the whole element at 15 octets, two of which are its IEI and length.
const maxGSMCapabilityLen = 13

// ParseGSMCapability decodes the 5GSM capability IE content.
func ParseGSMCapability(v []byte) (GSMCapability, error) {
	if len(v) < 1 {
		return GSMCapability{}, fmt.Errorf("nas/fgs: 5GSM capability: want at least 1 octet, got %d", len(v))
	}

	if len(v) > maxGSMCapabilityLen {
		return GSMCapability{}, fmt.Errorf(
			"nas/fgs: 5GSM capability is %d octets, want at most %d", len(v), maxGSMCapabilityLen)
	}

	c := GSMCapability{
		RqoS:    v[0]&0x01 != 0,
		MH6PDU:  v[0]&0x02 != 0,
		EPTS1:   v[0]&0x04 != 0,
		ATSSSST: v[0] >> 3 & 0x0F,
	}

	if len(v) > 1 {
		c.Rest = bytes.Clone(v[1:])
	}

	return c, nil
}

// AppendBinary encodes the 5GSM capability information element value onto b.
func (c GSMCapability) AppendBinary(b []byte) ([]byte, error) {
	if n := 1 + len(c.Rest); n > maxGSMCapabilityLen {
		return b, fmt.Errorf("nas/fgs: 5GSM capability is %d octets, want at most %d", n, maxGSMCapabilityLen)
	}

	var o uint8
	if c.RqoS {
		o |= 0x01
	}

	if c.MH6PDU {
		o |= 0x02
	}

	if c.EPTS1 {
		o |= 0x04
	}

	return append(append(b, o|(c.ATSSSST&0x0F)<<3), c.Rest...), nil
}

// MarshalBinary encodes the 5GSM capability information element value.
func (c GSMCapability) MarshalBinary() ([]byte, error) { return c.AppendBinary(nil) }

// PDUSessionEstablishmentReject is the PDU SESSION ESTABLISHMENT REJECT
// (TS 24.501 §8.3.3): the 5GSM header followed by a mandatory 5GSM cause.
type PDUSessionEstablishmentReject struct {
	PDUSessionID PDUSessionID
	PTI          nas.ProcedureTransactionIdentity
	Cause        GSMCause

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain PDU SESSION ESTABLISHMENT REJECT message.
// The encoding is appended to b.
func (m *PDUSessionEstablishmentReject) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGSMHeader(w, m.PDUSessionID, m.PTI, MsgPDUSessionEstablishmentReject)
	w.U8(uint8(m.Cause))

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *PDUSessionEstablishmentReject) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParsePDUSessionEstablishmentReject decodes a PDU SESSION ESTABLISHMENT REJECT.
func ParsePDUSessionEstablishmentReject(b []byte) (*PDUSessionEstablishmentReject, error) {
	r := nas.NewReader(b)

	pduSessionID, pti, err := readGSMHeader(r, MsgPDUSessionEstablishmentReject)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &PDUSessionEstablishmentReject{
		PDUSessionID: pduSessionID,
		PTI:          pti,
		Cause:        GSMCause(cause),
	}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// PDUSessionEstablishmentAccept is the PDU SESSION ESTABLISHMENT ACCEPT
// (TS 24.501 §8.3.2). Mandatory: selected PDU session type and SSC mode,
// authorized QoS rules, session AMBR. The remaining fields are optional and
// emitted in the message's IE order when set.
type PDUSessionEstablishmentAccept struct {
	PDUSessionID   PDUSessionID
	PTI            nas.ProcedureTransactionIdentity
	PDUSessionType PDUSessionType
	SSCMode        SSCMode
	QoSRules       QoSRules    // mandatory authorized QoS rules
	SessionAMBR    SessionAMBR // mandatory
	Cause          *GSMCause   // optional (IEI 0x59)
	PDUAddress     *PDUAddress // optional
	SNSSAI         *SNSSAI     // optional
	// AlwaysOn is the always-on PDU session indication (TS 24.501 §9.11.4.3):
	// true when the network requires the session to be always-on. Nil means the
	// element is absent.
	AlwaysOn            *bool
	QoSFlowDescriptions QoSFlowDescriptions               // optional
	ExtendedPCO         *nas.ProtocolConfigurationOptions // optional
	DNN                 *DNN                              // optional

	// Optional IEs Ella never sends, exposed for decoding (nil = absent).
	RQTimer *nas.GPRSTimer2 // optional (IEI 0x56), a GPRS timer
	EAP     []byte          // optional (IEI 0x78)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain PDU SESSION ESTABLISHMENT ACCEPT message.
// The encoding is appended to b.
func (m *PDUSessionEstablishmentAccept) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGSMHeader(w, m.PDUSessionID, m.PTI, MsgPDUSessionEstablishmentAccept)

	// Selected SSC mode (bits 5-7) and selected PDU session type (bits 1-3).
	w.U8((uint8(m.SSCMode)&0x07)<<4 | (uint8(m.PDUSessionType) & 0x07))

	// Authorized QoS rules (mandatory, LV-E).
	rules, err := m.QoSRules.MarshalBinary()
	if err != nil {
		return b, err
	}

	w.LVE(rules)

	// Session AMBR (mandatory, LV).
	ambr, err := m.SessionAMBR.MarshalBinary()
	if err != nil {
		return b, err
	}

	w.LV(ambr)

	if m.Cause != nil {
		o.TV3(iei5GSMCause, []byte{uint8(*m.Cause)})
	}

	if m.PDUAddress != nil {
		raw, err := m.PDUAddress.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiPDUAddress, raw)
	}

	if m.RQTimer != nil {
		raw, err := m.RQTimer.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TV3(ieiRQTimerValue, raw)
	}

	if m.SNSSAI != nil {
		raw, err := m.SNSSAI.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiSNSSAI, raw)
	}

	if m.AlwaysOn != nil {
		o.TV1(ieiAlwaysOnIndication, boolBit(*m.AlwaysOn, 0))
	}

	if m.EAP != nil {
		o.TLVE(ieiEAPMessageSession, m.EAP)
	}

	if m.QoSFlowDescriptions != nil {
		raw, err := m.QoSFlowDescriptions.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLVE(ieiQoSFlowDescription, raw)
	}

	if m.ExtendedPCO != nil {
		raw, err := m.ExtendedPCO.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLVE(ieiExtendedPCO, raw)
	}

	if m.DNN != nil {
		raw, err := m.DNN.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiDNN, raw)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *PDUSessionEstablishmentAccept) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParsePDUSessionEstablishmentAccept decodes the message.
func ParsePDUSessionEstablishmentAccept(b []byte) (*PDUSessionEstablishmentAccept, error) {
	r := nas.NewReader(b)

	psi, pti, err := readGSMHeader(r, MsgPDUSessionEstablishmentAccept)
	if err != nil {
		return nil, err
	}

	sscType, err := r.U8()
	if err != nil {
		return nil, err
	}

	rulesRaw, err := r.LVE()
	if err != nil {
		return nil, err
	}

	rules, err := ParseQoSRules(rulesRaw)
	if err != nil {
		return nil, err
	}

	ambrRaw, err := r.LV()
	if err != nil {
		return nil, err
	}

	ambr, err := ParseSessionAMBR(ambrRaw)
	if err != nil {
		return nil, err
	}

	out := &PDUSessionEstablishmentAccept{
		PDUSessionID:   psi,
		PTI:            pti,
		SSCMode:        SSCMode((sscType >> 4) & 0x07),
		PDUSessionType: PDUSessionType(sscType & 0x07),
		QoSRules:       rules,
		SessionAMBR:    ambr,
	}

	_unrec, err := walkOptionalIEs(r, establishmentAcceptIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case iei5GSMCause:
			if len(value) == 0 {
				return false, nil
			}

			cause := GSMCause(value[0])
			out.Cause = &cause
		case ieiPDUAddress:
			parsed, err := ParsePDUAddress(value)
			if err != nil {
				return false, err
			}

			out.PDUAddress = &parsed
		case ieiSNSSAI:
			parsed, err := ParseSNSSAI(value)
			if err != nil {
				return false, err
			}

			out.SNSSAI = &parsed
		case ieiAlwaysOnIndication:
			required := value[0]&0x01 != 0
			out.AlwaysOn = &required
		case ieiQoSFlowDescription:
			parsed, err := ParseQoSFlowDescriptions(value)
			if err != nil {
				return false, err
			}

			out.QoSFlowDescriptions = parsed
		case ieiExtendedPCO:
			parsed, err := nas.ParseExtendedProtocolConfigurationOptions(value, nas.PCONetworkToMS)
			if err != nil {
				return false, err
			}

			out.ExtendedPCO = &parsed
		case ieiDNN:
			parsed, err := ParseDNN(value)
			if err != nil {
				return false, err
			}

			out.DNN = &parsed
		case ieiRQTimerValue:
			timer, err := nas.ParseGPRSTimer2(value)
			if err != nil {
				return false, err
			}

			out.RQTimer = &timer
		case ieiEAPMessageSession:
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
