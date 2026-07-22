// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// PDUSessionModificationReject is the PDU SESSION MODIFICATION REJECT (TS 24.501
// §8.3.8): the 5GSM header followed by a mandatory 5GSM cause.
type PDUSessionModificationReject struct {
	PDUSessionID uint8
	PTI          uint8
	Cause        uint8
}

// Marshal encodes the plain PDU SESSION MODIFICATION REJECT message.
func (m *PDUSessionModificationReject) Marshal() ([]byte, error) {
	var w common.Writer

	writeSMHeader(&w, m.PDUSessionID, m.PTI, MsgPDUSessionModificationReject)
	w.U8(m.Cause)

	return w.Bytes(), nil
}

// PDUSessionModificationCommand is the PDU SESSION MODIFICATION COMMAND
// (TS 24.501 §8.3.9). All information elements are optional; they are emitted in
// the message's IE order when set.
type PDUSessionModificationCommand struct {
	PDUSessionID        uint8
	PTI                 uint8
	SessionAMBR         *SessionAMBR // optional (IEI 0x2A)
	QoSFlowDescriptions []byte       // optional IE content (IEI 0x79)
	ExtendedPCO         []byte       // optional PCO content (IEI 0x7B)
}

// Marshal encodes the plain PDU SESSION MODIFICATION COMMAND message.
func (m *PDUSessionModificationCommand) Marshal() ([]byte, error) {
	var w common.Writer

	writeSMHeader(&w, m.PDUSessionID, m.PTI, MsgPDUSessionModificationCommand)

	if m.SessionAMBR != nil {
		writeTLV(&w, ieiSessionAMBR, m.SessionAMBR.marshalValue())
	}

	if m.QoSFlowDescriptions != nil {
		writeTLVE(&w, ieiQoSFlowDescription, m.QoSFlowDescriptions)
	}

	if m.ExtendedPCO != nil {
		writeTLVE(&w, ieiExtendedPCO, m.ExtendedPCO)
	}

	return w.Bytes(), nil
}

// ParsePDUSessionModificationCommand decodes the message.
func ParsePDUSessionModificationCommand(b []byte) (*PDUSessionModificationCommand, error) {
	r := common.NewReader(b)

	psi, pti, err := readSMHeader(r, MsgPDUSessionModificationCommand)
	if err != nil {
		return nil, err
	}

	out := &PDUSessionModificationCommand{PDUSessionID: psi, PTI: pti}

	_, err = common.WalkOptionalIEs(r, modificationCommandIEs, func(iei uint8, value []byte) error {
		switch iei {
		case ieiSessionAMBR:
			ambr, err := parseSessionAMBR(value)
			if err != nil {
				return err
			}

			out.SessionAMBR = &ambr
		case ieiQoSFlowDescription:
			out.QoSFlowDescriptions = value
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

// PDUSessionModificationComplete is the PDU SESSION MODIFICATION COMPLETE
// (TS 24.501 §8.3.10): the 5GSM header and optional information elements; it
// carries no field the network acts on.
type PDUSessionModificationComplete struct {
	PDUSessionID uint8
	PTI          uint8
}

// ParsePDUSessionModificationComplete decodes the message.
func ParsePDUSessionModificationComplete(b []byte) (*PDUSessionModificationComplete, error) {
	r := common.NewReader(b)

	psi, pti, err := readSMHeader(r, MsgPDUSessionModificationComplete)
	if err != nil {
		return nil, err
	}

	return &PDUSessionModificationComplete{PDUSessionID: psi, PTI: pti}, nil
}

// PDUSessionModificationCommandReject is the PDU SESSION MODIFICATION COMMAND
// REJECT (TS 24.501 §8.3.11): the 5GSM header followed by a mandatory 5GSM
// cause.
type PDUSessionModificationCommandReject struct {
	PDUSessionID uint8
	PTI          uint8
	Cause        uint8
}

// ParsePDUSessionModificationCommandReject decodes the message.
func ParsePDUSessionModificationCommandReject(b []byte) (*PDUSessionModificationCommandReject, error) {
	r := common.NewReader(b)

	psi, pti, err := readSMHeader(r, MsgPDUSessionModificationCmdReject)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	return &PDUSessionModificationCommandReject{PDUSessionID: psi, PTI: pti, Cause: cause}, nil
}
