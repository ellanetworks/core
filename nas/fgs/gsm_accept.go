// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// PDUSessionEstablishmentAccept is the PDU SESSION ESTABLISHMENT ACCEPT
// (TS 24.501 §8.3.2). Mandatory: selected PDU session type and SSC mode,
// authorized QoS rules, session AMBR. The remaining fields are optional and
// emitted in the message's IE order when set.
type PDUSessionEstablishmentAccept struct {
	PDUSessionID        uint8
	PTI                 uint8
	PDUSessionType      uint8
	SSCMode             uint8
	QoSRules            []byte // authorized QoS rules IE content (see MarshalQoSRules)
	SessionAMBR         SessionAMBR
	Cause               uint8       // optional; 0 omits the IE
	PDUAddress          *PDUAddress // optional
	SNSSAI              *SNSSAI     // optional
	AlwaysOn            *uint8      // optional always-on PDU session indication
	QoSFlowDescriptions []byte      // optional IE content (see MarshalCreateQoSFlow)
	ExtendedPCO         []byte      // optional PCO content (see PCO.Marshal)
	DNN                 string      // optional; "" omits the IE
}

// Marshal encodes the plain PDU SESSION ESTABLISHMENT ACCEPT message.
func (m *PDUSessionEstablishmentAccept) Marshal() ([]byte, error) {
	var w common.Writer

	writeGSMHeader(&w, m.PDUSessionID, m.PTI, MsgPDUSessionEstablishmentAccept)

	// Selected SSC mode (bits 5-7) and selected PDU session type (bits 1-3).
	w.U8((m.SSCMode&0x07)<<4 | (m.PDUSessionType & 0x07))

	// Authorized QoS rules (mandatory, LV-E).
	w.U16(uint16(len(m.QoSRules)))
	w.Raw(m.QoSRules)

	// Session AMBR (mandatory, LV).
	ambr := m.SessionAMBR.marshalValue()
	w.U8(uint8(len(ambr)))
	w.Raw(ambr)

	if m.Cause != 0 {
		writeTV2(&w, iei5GSMCause, m.Cause)
	}

	if m.PDUAddress != nil {
		writeTLV(&w, ieiPDUAddress, m.PDUAddress.marshalValue())
	}

	if m.SNSSAI != nil {
		writeTLV(&w, ieiSNSSAI, m.SNSSAI.marshalValue())
	}

	if m.AlwaysOn != nil {
		w.U8(ieiAlwaysOnIndication | (*m.AlwaysOn & 0x01))
	}

	if m.QoSFlowDescriptions != nil {
		writeTLVE(&w, ieiQoSFlowDescription, m.QoSFlowDescriptions)
	}

	if m.ExtendedPCO != nil {
		writeTLVE(&w, ieiExtendedPCO, m.ExtendedPCO)
	}

	if m.DNN != "" {
		writeTLV(&w, ieiDNN, dnnLabels(m.DNN))
	}

	return w.Bytes(), nil
}

// ParsePDUSessionEstablishmentAccept decodes the message. QoS rules and QoS flow
// descriptions are returned as their raw IE content.
func ParsePDUSessionEstablishmentAccept(b []byte) (*PDUSessionEstablishmentAccept, error) {
	r := common.NewReader(b)

	psi, pti, err := readGSMHeader(r, MsgPDUSessionEstablishmentAccept)
	if err != nil {
		return nil, err
	}

	sscType, err := r.U8()
	if err != nil {
		return nil, err
	}

	rules, err := r.LVE()
	if err != nil {
		return nil, err
	}

	ambr, err := r.LV()
	if err != nil {
		return nil, err
	}

	sessAMBR, err := parseSessionAMBR(ambr)
	if err != nil {
		return nil, err
	}

	out := &PDUSessionEstablishmentAccept{
		PDUSessionID:   psi,
		PTI:            pti,
		SSCMode:        (sscType >> 4) & 0x07,
		PDUSessionType: sscType & 0x07,
		QoSRules:       rules,
		SessionAMBR:    sessAMBR,
	}

	_, err = common.WalkOptionalIEs(r, establishmentAcceptIEs, func(iei uint8, value []byte) error {
		switch iei {
		case iei5GSMCause:
			if len(value) >= 1 {
				out.Cause = value[0]
			}
		case ieiPDUAddress:
			out.PDUAddress = parsePDUAddress(value)
		case ieiSNSSAI:
			out.SNSSAI = parseSNSSAI(value)
		case ieiAlwaysOnIndication:
			v := value[0] & 0x01
			out.AlwaysOn = &v
		case ieiQoSFlowDescription:
			out.QoSFlowDescriptions = value
		case ieiExtendedPCO:
			out.ExtendedPCO = value
		case ieiDNN:
			out.DNN = labelsToDNN(value)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}
