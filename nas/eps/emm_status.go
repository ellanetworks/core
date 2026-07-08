// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas/common"

// EMMStatus is the EMM STATUS message, reporting an error condition detected on
// received EMM protocol data (TS 24.301 §5.7). It may be sent by both the MME and
// the UE and triggers no state transition on receipt.
type EMMStatus struct {
	EMMCause uint8
}

// Marshal encodes the plain EMM STATUS message.
func (m *EMMStatus) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgEMMStatus)
	w.U8(m.EMMCause)

	return w.Bytes(), nil
}

// ParseEMMStatus decodes the EMM STATUS message.
func ParseEMMStatus(b []byte) (*EMMStatus, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgEMMStatus); err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	return &EMMStatus{EMMCause: cause}, nil
}
