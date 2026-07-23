// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// GSMStatus is the 5GSM STATUS message (TS 24.501 §8.3.16): the 5GSM header
// followed by a mandatory 5GSM cause, reporting an error condition detected on a
// PDU session. It may be sent by both the UE and the network.
type GSMStatus struct {
	PDUSessionID uint8
	PTI          uint8
	Cause        uint8
}

// Marshal encodes the plain 5GSM STATUS message.
func (m *GSMStatus) Marshal() ([]byte, error) {
	var w common.Writer

	writeGSMHeader(&w, m.PDUSessionID, m.PTI, MsgGSMStatus)
	w.U8(m.Cause)

	return w.Bytes(), nil
}

// ParseGSMStatus decodes the 5GSM STATUS message.
func ParseGSMStatus(b []byte) (*GSMStatus, error) {
	r := common.NewReader(b)

	psi, pti, err := readGSMHeader(r, MsgGSMStatus)
	if err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	return &GSMStatus{PDUSessionID: psi, PTI: pti, Cause: cause}, nil
}
