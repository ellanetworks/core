// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// PDUSessionEstablishmentRequest is the PDU SESSION ESTABLISHMENT REQUEST
// (TS 24.501 §8.3.1): the 5GSM header, a mandatory integrity protection maximum
// data rate, and optional information elements. Only the fields the network acts
// on are modelled; unmodelled optional IEs are stepped over during parsing.
type PDUSessionEstablishmentRequest struct {
	PDUSessionID             uint8
	PTI                      uint8
	IntegrityProtMaxDataRate [2]byte
	PDUSessionType           *uint8 // optional (IEI 0x9), value bits 1-3
	AlwaysOnRequested        bool   // optional (IEI 0xB) present
	ExtendedPCO              []byte // optional (IEI 0x7B) content
}

// Marshal encodes the plain PDU SESSION ESTABLISHMENT REQUEST message. Only the
// modelled fields are emitted; it is the inverse of Parse for those.
func (m *PDUSessionEstablishmentRequest) Marshal() ([]byte, error) {
	var w common.Writer

	writeGSMHeader(&w, m.PDUSessionID, m.PTI, MsgPDUSessionEstablishmentRequest)
	w.Raw(m.IntegrityProtMaxDataRate[:])

	if m.PDUSessionType != nil {
		w.U8(ieiPDUSessionType | (*m.PDUSessionType & 0x07))
	}

	if m.AlwaysOnRequested {
		w.U8(ieiAlwaysOnRequested | 0x01)
	}

	if m.ExtendedPCO != nil {
		writeTLVE(&w, ieiExtendedPCO, m.ExtendedPCO)
	}

	return w.Bytes(), nil
}

// ParsePDUSessionEstablishmentRequest decodes the message.
func ParsePDUSessionEstablishmentRequest(b []byte) (*PDUSessionEstablishmentRequest, error) {
	r := common.NewReader(b)

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

	_, err = common.WalkOptionalIEs(r, establishmentRequestIEs, func(iei uint8, value []byte) error {
		switch iei {
		case ieiPDUSessionType:
			if len(value) >= 1 {
				t := value[0] & 0x07
				out.PDUSessionType = &t
			}
		case ieiAlwaysOnRequested:
			out.AlwaysOnRequested = true
		case ieiExtendedPCO:
			out.ExtendedPCO = value
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

// PDUSessionEstablishmentReject is the PDU SESSION ESTABLISHMENT REJECT
// (TS 24.501 §8.3.3): the 5GSM header followed by a mandatory 5GSM cause.
type PDUSessionEstablishmentReject struct {
	PDUSessionID uint8
	PTI          uint8
	Cause        uint8
}

// Marshal encodes the plain PDU SESSION ESTABLISHMENT REJECT message.
func (m *PDUSessionEstablishmentReject) Marshal() ([]byte, error) {
	var w common.Writer

	writeGSMHeader(&w, m.PDUSessionID, m.PTI, MsgPDUSessionEstablishmentReject)
	w.U8(m.Cause)

	return w.Bytes(), nil
}
