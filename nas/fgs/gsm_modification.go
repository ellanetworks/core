// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// PDUSessionModificationRequest is the PDU SESSION MODIFICATION REQUEST
// (TS 24.501 §8.3.7): the 5GSM header and optional elements only. The UE sends
// it to ask the network to modify a PDU session.
type PDUSessionModificationRequest struct {
	PDUSessionID PDUSessionID
	PTI          nas.ProcedureTransactionIdentity

	GSMCapability     *GSMCapability                    // optional (IEI 0x28)
	Cause             *GSMCause                         // optional (IEI 0x59)
	AlwaysOnRequested *bool                             // optional (IEI 0xB), value bit 1
	RequestedQoSRules QoSRules                          // optional (IEI 0x7A)
	RequestedQoSFlows QoSFlowDescriptions               // optional (IEI 0x79)
	ExtendedPCO       *nas.ProtocolConfigurationOptions // optional (IEI 0x7B)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// modificationRequestIEs is the full-octet optional-IE table of the PDU SESSION
// MODIFICATION REQUEST (TS 24.501 §8.3.7, table 8.3.7.1.1). The always-on
// requested element is type 1, which the walker delimits generically.
var modificationRequestIEs = []nas.OptionalIE{
	{IEI: iei5GSMCapability, Format: nas.IETLV, Name: "5GSM capability"},
	{IEI: iei5GSMCause, Format: nas.IETV3, Len: 1, Name: "5GSM cause"},
	{IEI: ieiMaxPacketFilters, Format: nas.IETV3, Len: 2, Name: "Maximum number of supported packet filters"},
	{IEI: ieiIntegrityProtMaxRate, Format: nas.IETV3, Len: 2, Name: "Integrity protection maximum data rate"},
	{IEI: ieiAuthorizedQoSRules, Format: nas.IETLVE, Name: "Requested QoS rules"},
	{IEI: ieiQoSFlowDescription, Format: nas.IETLVE, Name: "Requested QoS flow descriptions"},
	{IEI: ieiMappedEPSBearerContext, Format: nas.IETLVE, Name: "Mapped EPS bearer contexts"},
	{IEI: ieiExtendedPCO, Format: nas.IETLVE, Name: "Extended PCO"},
	{IEI: ieiPortMgmtContainer, Format: nas.IETLVE, Name: "Port management information container"},
	{IEI: ieiIPHeaderCompression, Format: nas.IETLV, Name: "IP header compression configuration"},
	{IEI: ieiEthHeaderCompress, Format: nas.IETLV, Name: "Ethernet header compression configuration"},
	{IEI: ieiRequestedMBS, Format: nas.IETLVE, Name: "Requested MBS container"},
	{IEI: ieiServiceLevelAA, Format: nas.IETLVE, Name: "Service-level-AA container"},
	{IEI: ieiNon3GPPDelayBudget, Format: nas.IETLVE, Name: "Non-3GPP delay budget"},
	{IEI: ieiURSPEnforcement, Format: nas.IETLV, Name: "URSP rule enforcement reports"},
}

// AppendBinary encodes the plain PDU SESSION MODIFICATION REQUEST message.
// The encoding is appended to b.
func (m *PDUSessionModificationRequest) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGSMHeader(w, m.PDUSessionID, m.PTI, MsgPDUSessionModificationRequest)

	if m.GSMCapability != nil {
		raw, err := m.GSMCapability.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(iei5GSMCapability, raw)
	}

	if m.Cause != nil {
		o.TV3(iei5GSMCause, []byte{uint8(*m.Cause)})
	}

	if m.AlwaysOnRequested != nil {
		o.TV1(ieiAlwaysOnRequested, boolBit(*m.AlwaysOnRequested, 0))
	}

	if m.RequestedQoSRules != nil {
		raw, err := m.RequestedQoSRules.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLVE(ieiAuthorizedQoSRules, raw)
	}

	if m.RequestedQoSFlows != nil {
		raw, err := m.RequestedQoSFlows.MarshalBinary()
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

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *PDUSessionModificationRequest) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParsePDUSessionModificationRequest decodes the message.
func ParsePDUSessionModificationRequest(b []byte) (*PDUSessionModificationRequest, error) {
	r := nas.NewReader(b)

	psi, pti, err := readGSMHeader(r, MsgPDUSessionModificationRequest)
	if err != nil {
		return nil, err
	}

	out := &PDUSessionModificationRequest{PDUSessionID: psi, PTI: pti}

	_unrec, err := walkOptionalIEs(r, modificationRequestIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case iei5GSMCapability:
			parsed, err := ParseGSMCapability(value)
			if err != nil {
				return false, err
			}

			out.GSMCapability = &parsed
		case iei5GSMCause:
			if len(value) != 1 {
				return false, fmt.Errorf("nas/fgs: 5GSM cause is %d octets, want 1", len(value))
			}

			cause := GSMCause(value[0])
			out.Cause = &cause
		case ieiAlwaysOnRequested:
			// TS 24.501 table 9.11.4.4.1 assigns both values — 0 "not requested",
			// 1 "requested" — so the element carries its own meaning and the field
			// records whether it arrived, not just that a bit was set.
			out.AlwaysOnRequested = tv1Flag(value)
		case ieiAuthorizedQoSRules:
			parsed, err := ParseQoSRules(value)
			if err != nil {
				return false, err
			}

			out.RequestedQoSRules = parsed
		case ieiQoSFlowDescription:
			parsed, err := ParseQoSFlowDescriptions(value)
			if err != nil {
				return false, err
			}

			out.RequestedQoSFlows = parsed
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

// PDUSessionModificationReject is the PDU SESSION MODIFICATION REJECT (TS 24.501
// §8.3.8): the 5GSM header, a mandatory 5GSM cause, and optional elements
// telling the UE when it may retry.
type PDUSessionModificationReject struct {
	PDUSessionID PDUSessionID
	PTI          nas.ProcedureTransactionIdentity
	Cause        GSMCause

	BackoffTimer *nas.GPRSTimer3                   // optional (IEI 0x37)
	ExtendedPCO  *nas.ProtocolConfigurationOptions // optional (IEI 0x7B)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// modificationRejectIEs is the optional-IE table of the PDU SESSION MODIFICATION
// REJECT (TS 24.501 §8.3.8, table 8.3.8.1.1).
var modificationRejectIEs = []nas.OptionalIE{
	{IEI: ieiBackoffTimer, Format: nas.IETLV, Name: "Back-off timer value"},
	{IEI: ieiCongestionReattempt, Format: nas.IETLV, Name: "5GSM congestion re-attempt indicator"},
	{IEI: ieiExtendedPCO, Format: nas.IETLVE, Name: "Extended PCO"},
	{IEI: ieiReattemptIndicator, Format: nas.IETLV, Name: "Re-attempt indicator"},
}

// AppendBinary encodes the plain PDU SESSION MODIFICATION REJECT message.
// The encoding is appended to b.
func (m *PDUSessionModificationReject) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGSMHeader(w, m.PDUSessionID, m.PTI, MsgPDUSessionModificationReject)
	w.U8(uint8(m.Cause))

	if m.BackoffTimer != nil {
		raw, err := m.BackoffTimer.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiBackoffTimer, raw)
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
func (m *PDUSessionModificationReject) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParsePDUSessionModificationReject decodes the message.
func ParsePDUSessionModificationReject(b []byte) (*PDUSessionModificationReject, error) {
	r := nas.NewReader(b)

	psi, pti, err := readGSMHeader(r, MsgPDUSessionModificationReject)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &PDUSessionModificationReject{PDUSessionID: psi, PTI: pti, Cause: GSMCause(cause)}

	_unrec, err := walkOptionalIEs(r, modificationRejectIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiBackoffTimer:
			timer, err := nas.ParseGPRSTimer3(value)
			if err != nil {
				return false, err
			}

			out.BackoffTimer = &timer
		case ieiExtendedPCO:
			parsed, err := nas.ParseExtendedProtocolConfigurationOptions(value, nas.PCONetworkToMS)
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

// PDUSessionModificationCommand is the PDU SESSION MODIFICATION COMMAND
// (TS 24.501 §8.3.9). All information elements are optional; they are emitted in
// the message's IE order when set.
type PDUSessionModificationCommand struct {
	PDUSessionID        PDUSessionID
	PTI                 nas.ProcedureTransactionIdentity
	SessionAMBR         *SessionAMBR                      // optional (IEI 0x2A)
	QoSFlowDescriptions QoSFlowDescriptions               // optional (IEI 0x79)
	ExtendedPCO         *nas.ProtocolConfigurationOptions // optional (IEI 0x7B)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// AppendBinary encodes the plain PDU SESSION MODIFICATION COMMAND message.
// The encoding is appended to b.
func (m *PDUSessionModificationCommand) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGSMHeader(w, m.PDUSessionID, m.PTI, MsgPDUSessionModificationCommand)

	if m.SessionAMBR != nil {
		raw, err := m.SessionAMBR.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiSessionAMBR, raw)
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

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *PDUSessionModificationCommand) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParsePDUSessionModificationCommand decodes the message.
func ParsePDUSessionModificationCommand(b []byte) (*PDUSessionModificationCommand, error) {
	r := nas.NewReader(b)

	psi, pti, err := readGSMHeader(r, MsgPDUSessionModificationCommand)
	if err != nil {
		return nil, err
	}

	out := &PDUSessionModificationCommand{PDUSessionID: psi, PTI: pti}

	_unrec, err := walkOptionalIEs(r, modificationCommandIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiSessionAMBR:
			parsed, err := ParseSessionAMBR(value)
			if err != nil {
				return false, err
			}

			out.SessionAMBR = &parsed
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

// PDUSessionModificationComplete is the PDU SESSION MODIFICATION COMPLETE
// (TS 24.501 §8.3.10): the 5GSM header and optional information elements; it
// carries no field the network acts on.
type PDUSessionModificationComplete struct {
	PDUSessionID PDUSessionID
	PTI          nas.ProcedureTransactionIdentity

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// ParsePDUSessionModificationComplete decodes the message.
func ParsePDUSessionModificationComplete(b []byte) (*PDUSessionModificationComplete, error) {
	r := nas.NewReader(b)

	psi, pti, err := readGSMHeader(r, MsgPDUSessionModificationComplete)
	if err != nil {
		return nil, err
	}

	out := &PDUSessionModificationComplete{PDUSessionID: psi, PTI: pti}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// AppendBinary encodes the plain PDU SESSION MODIFICATION COMPLETE message
// (TS 24.501 §8.3.11). The encoding is appended to b.
func (m *PDUSessionModificationComplete) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGSMHeader(w, m.PDUSessionID, m.PTI, MsgPDUSessionModificationComplete)

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *PDUSessionModificationComplete) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// PDUSessionModificationCommandReject is the PDU SESSION MODIFICATION COMMAND
// REJECT (TS 24.501 §8.3.11): the 5GSM header followed by a mandatory 5GSM
// cause.
type PDUSessionModificationCommandReject struct {
	PDUSessionID PDUSessionID
	PTI          nas.ProcedureTransactionIdentity
	Cause        GSMCause

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

// ParsePDUSessionModificationCommandReject decodes the message.
func ParsePDUSessionModificationCommandReject(b []byte) (*PDUSessionModificationCommandReject, error) {
	r := nas.NewReader(b)

	psi, pti, err := readGSMHeader(r, MsgPDUSessionModificationCmdReject)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	out := &PDUSessionModificationCommandReject{PDUSessionID: psi, PTI: pti, Cause: GSMCause(cause)}

	_unrec, err := walkOptionalIEs(r, nil, declineAll)
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// AppendBinary encodes the plain PDU SESSION MODIFICATION COMMAND REJECT
// message (TS 24.501 §8.3.12). The encoding is appended to b.
func (m *PDUSessionModificationCommandReject) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGSMHeader(w, m.PDUSessionID, m.PTI, MsgPDUSessionModificationCmdReject)
	w.U8(uint8(m.Cause))

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *PDUSessionModificationCommandReject) MarshalBinary() ([]byte, error) {
	return marshalMessage(m)
}
