// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// Payload container type values (TS 24.501 §9.11.3.40).
const (
	PayloadContainerTypeN1SMInfo          PayloadContainerType = 0x01
	PayloadContainerTypeSMS               PayloadContainerType = 0x02
	PayloadContainerTypeLPP               PayloadContainerType = 0x03
	PayloadContainerTypeSOR               PayloadContainerType = 0x04
	PayloadContainerTypeUEPolicy          PayloadContainerType = 0x05
	PayloadContainerTypeUEParameterUpdate PayloadContainerType = 0x06
	PayloadContainerTypeLocationServices  PayloadContainerType = 0x07
	PayloadContainerTypeCIoTUserData      PayloadContainerType = 0x08
	PayloadContainerTypeServiceLevelAA    PayloadContainerType = 0x09
	PayloadContainerTypeEventNotification PayloadContainerType = 0x0A
	PayloadContainerTypeUPPCMIP           PayloadContainerType = 0x0B
	PayloadContainerTypeSLPP              PayloadContainerType = 0x0C
	PayloadContainerTypeMultiplePayload   PayloadContainerType = 0x0F
)

// UL NAS TRANSPORT request type values (TS 24.501 §9.11.3.47).
const (
	RequestTypeInitialRequest              RequestType = 1
	RequestTypeExistingPDUSession          RequestType = 2
	RequestTypeInitialEmergencyRequest     RequestType = 3
	RequestTypeExistingEmergencyPDUSession RequestType = 4
	RequestTypeModificationRequest         RequestType = 5
	RequestTypeMAPDURequest                RequestType = 6
	RequestTypeReserved                    RequestType = 7
)

// DLNASTransport is the DL NAS TRANSPORT message (TS 24.501 §8.2.11): it carries
// a payload container (typically an SMF-produced 5GSM message) from the network
// to the UE, with optional routing and diagnostic IEs.
type DLNASTransport struct {
	PayloadContainerType PayloadContainerType // bits 1-4 (TS 24.501 §9.11.3.40)
	PayloadContainer     []byte               // LV-E
	PDUSessionID         *PDUSessionID        // optional (IEI 0x12)
	AdditionalInfo       []byte               // optional (IEI 0x24)
	Cause                *GMMCause            // optional 5GMM cause (IEI 0x58)
	BackoffTimer         *nas.GPRSTimer3      // optional back-off timer value (IEI 0x37)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

var dlNASTransportIEs = []nas.OptionalIE{
	{IEI: ieiPduSessionID2, Format: nas.IETV3, Len: 1, Name: "PDU session identity 2"},
	{IEI: ieiAdditionalInfo, Format: nas.IETLV, Name: "Additional information"},
	{IEI: ieiBackoffTimer, Format: nas.IETLV, Name: "Back-off timer value"},
	{IEI: ieiCause5GMM, Format: nas.IETV3, Len: 1, Name: "5GMM cause"},
}

// ParseDLNASTransport decodes a plain DL NAS TRANSPORT message.
func ParseDLNASTransport(b []byte) (*DLNASTransport, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgDLNASTransport); err != nil {
		return nil, err
	}

	octet1, err := r.U8()
	if err != nil {
		return nil, err
	}

	pc, err := r.LVE()
	if err != nil {
		return nil, err
	}

	out := &DLNASTransport{
		PayloadContainerType: PayloadContainerType(octet1 & 0x0F),
		PayloadContainer:     pc,
	}

	_unrec, err := walkOptionalIEs(r, dlNASTransportIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiPduSessionID2:
			if len(value) == 0 {
				return false, nil
			}

			psi := PDUSessionID(value[0])
			out.PDUSessionID = &psi
		case ieiAdditionalInfo:
			out.AdditionalInfo = value
		case ieiBackoffTimer:
			timer, err := nas.ParseGPRSTimer3(value)
			if err != nil {
				return false, err
			}

			out.BackoffTimer = &timer
		case ieiCause5GMM:
			if len(value) == 0 {
				return false, nil
			}

			cause := GMMCause(value[0])
			out.Cause = &cause
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

// AppendBinary encodes the plain DL NAS TRANSPORT message.
// The encoding is appended to b.
func (m *DLNASTransport) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgDLNASTransport)
	w.U8(uint8(m.PayloadContainerType & 0x0F)) // spare half octet in bits 5-8

	w.LVE(m.PayloadContainer)

	if m.PDUSessionID != nil {
		o.TV3(ieiPduSessionID2, []byte{uint8(*m.PDUSessionID)})
	}

	if m.AdditionalInfo != nil {
		o.TLV(ieiAdditionalInfo, m.AdditionalInfo)
	}

	if m.Cause != nil {
		o.TV3(ieiCause5GMM, []byte{uint8(*m.Cause)})
	}

	if m.BackoffTimer != nil {
		raw, err := m.BackoffTimer.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiBackoffTimer, raw)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *DLNASTransport) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ULNASTransport is the UL NAS TRANSPORT message (TS 24.501 §8.2.10): the UE
// tunnels a payload container (typically a 5GSM message) to the network, with
// optional routing IEs identifying the PDU session, slice and DNN.
type ULNASTransport struct {
	PayloadContainerType  PayloadContainerType // bits 1-4 (TS 24.501 §9.11.3.40)
	PayloadContainer      []byte               // LV-E
	PDUSessionID          *PDUSessionID        // optional (IEI 0x12)
	OldPDUSessionID       *PDUSessionID        // optional (IEI 0x59)
	RequestType           *RequestType         // optional (IEI 0x8-, type 1): bits 1-3 (TS 24.501 §9.11.3.47)
	SNSSAI                *SNSSAI              // optional (IEI 0x22)
	DNN                   *DNN                 // optional (IEI 0x25)
	AdditionalInformation []byte               // optional (IEI 0x24)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

var ulNASTransportIEs = []nas.OptionalIE{
	{IEI: ieiPduSessionID2, Format: nas.IETV3, Len: 1, Name: "PDU session identity 2"},
	{IEI: ieiOldPDUSessionID, Format: nas.IETV3, Len: 1, Name: "Old PDU session ID"},
	{IEI: ieiSNSSAI, Format: nas.IETLV, Name: "S-NSSAI"},
	{IEI: ieiDNN, Format: nas.IETLV, Name: "DNN"},
	{IEI: ieiAdditionalInfo, Format: nas.IETLV, Name: "Additional information"},
}

// AppendBinary encodes the plain UL NAS TRANSPORT message (TS 24.501 §8.2.10).
// The encoding is appended to b.
func (m *ULNASTransport) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgULNASTransport)
	w.U8(uint8(m.PayloadContainerType & 0x0F)) // spare half octet in bits 5-8

	w.LVE(m.PayloadContainer)

	if m.PDUSessionID != nil {
		o.TV3(ieiPduSessionID2, []byte{uint8(*m.PDUSessionID)})
	}

	if m.OldPDUSessionID != nil {
		o.TV3(ieiOldPDUSessionID, []byte{uint8(*m.OldPDUSessionID)})
	}

	if m.RequestType != nil {
		o.TV1(ieiRequestType, uint8(*m.RequestType)) // type 1
	}

	if m.SNSSAI != nil {
		raw, err := m.SNSSAI.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiSNSSAI, raw)
	}

	if m.DNN != nil {
		raw, err := m.DNN.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiDNN, raw)
	}

	if m.AdditionalInformation != nil {
		o.TLV(ieiAdditionalInfo, m.AdditionalInformation)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return messageResult(w, b)
}

// MarshalBinary encodes the message.
func (m *ULNASTransport) MarshalBinary() ([]byte, error) { return marshalMessage(m) }

// ParseULNASTransport decodes a plain UL NAS TRANSPORT message.
func ParseULNASTransport(b []byte) (*ULNASTransport, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgULNASTransport); err != nil {
		return nil, err
	}

	octet1, err := r.U8()
	if err != nil {
		return nil, err
	}

	pc, err := r.LVE()
	if err != nil {
		return nil, err
	}

	out := &ULNASTransport{
		PayloadContainerType: PayloadContainerType(octet1 & 0x0F),
		PayloadContainer:     pc,
	}

	_unrec, err := walkOptionalIEs(r, ulNASTransportIEs, func(iei uint8, value []byte) (bool, error) {
		switch iei {
		case ieiPduSessionID2:
			if len(value) == 0 {
				return false, nil
			}

			psi := PDUSessionID(value[0])
			out.PDUSessionID = &psi
		case ieiOldPDUSessionID:
			if len(value) == 0 {
				return false, nil
			}

			old := PDUSessionID(value[0])
			out.OldPDUSessionID = &old
		case ieiRequestType: // type 1: value is the low nibble
			if len(value) == 0 {
				return false, nil
			}

			v := value[0] & 0x07
			rt := RequestType(v)
			out.RequestType = &rt
		case ieiSNSSAI:
			parsed, err := ParseSNSSAI(value)
			if err != nil {
				return false, err
			}

			out.SNSSAI = &parsed
		case ieiDNN:
			parsed, err := ParseDNN(value)
			if err != nil {
				return false, err
			}

			out.DNN = &parsed
		case ieiAdditionalInfo:
			out.AdditionalInformation = value
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

// UPUHeaderAck is the UPU-Header value acknowledging a UE Parameters Update
// (TS 24.501 §D.2).
const UPUHeaderAck uint8 = 0x01

// UPUAcknowledgement is a UE parameters update acknowledgement payload container
// (TS 24.501 §D.2): a UPU-Header octet followed by the 16-octet UPU-MAC.
type UPUAcknowledgement struct {
	MAC [16]byte
}

// ParseUPUAcknowledgement decodes a UE parameters update acknowledgement payload
// container (TS 24.501 §D.2): a UPU-Header octet of 0x01 (acknowledgement)
// followed by the 16-octet UPU-MAC. It returns the UPU-MAC.
func ParseUPUAcknowledgement(b []byte) (UPUAcknowledgement, error) {
	if len(b) != 17 {
		return UPUAcknowledgement{}, fmt.Errorf("nas/fgs: UPU acknowledgement: want 17 octets, got %d", len(b))
	}

	if b[0] != UPUHeaderAck {
		return UPUAcknowledgement{}, fmt.Errorf("nas/fgs: UPU acknowledgement: unexpected UPU-Header %#x", b[0])
	}

	var ack UPUAcknowledgement

	copy(ack.MAC[:], b[1:])

	return ack, nil
}

// AppendBinary encodes the UE parameters update acknowledgement onto b.
func (a UPUAcknowledgement) AppendBinary(b []byte) ([]byte, error) {
	b = append(b, UPUHeaderAck)

	return append(b, a.MAC[:]...), nil
}

// MarshalBinary encodes the UE parameters update acknowledgement.
func (a UPUAcknowledgement) MarshalBinary() ([]byte, error) { return a.AppendBinary(nil) }
