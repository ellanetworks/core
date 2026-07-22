// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// Status5GSM is the 5GSM STATUS message (TS 24.501 §8.3.16): the 5GSM header
// followed by a mandatory 5GSM cause, reporting an error condition detected on a
// PDU session. It may be sent by both the UE and the network.
type Status5GSM struct {
	PDUSessionID uint8
	PTI          uint8
	Cause        uint8
}

// Marshal encodes the plain 5GSM STATUS message.
func (m *Status5GSM) Marshal() ([]byte, error) {
	var w common.Writer

	writeSMHeader(&w, m.PDUSessionID, m.PTI, Msg5GSMStatus)
	w.U8(m.Cause)

	return w.Bytes(), nil
}

// ParseStatus5GSM decodes the 5GSM STATUS message.
func ParseStatus5GSM(b []byte) (*Status5GSM, error) {
	r := common.NewReader(b)

	psi, pti, err := readSMHeader(r, Msg5GSMStatus)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	return &Status5GSM{PDUSessionID: psi, PTI: pti, Cause: cause}, nil
}
