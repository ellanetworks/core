// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// DLNASTransport is the DL NAS TRANSPORT message (TS 24.501 §8.2.11): it carries
// a payload container (typically an SMF-produced 5GSM message) from the network
// to the UE, with optional routing and diagnostic IEs.
type DLNASTransport struct {
	PayloadContainerType uint8  // bits 1-4 (TS 24.501 §9.11.3.40)
	PayloadContainer     []byte // LV-E
	PDUSessionID         uint8  // optional (IEI 0x12); 0 omits the IE
	AdditionalInfo       []byte // optional (IEI 0x24)
	Cause                *uint8 // optional 5GMM cause (IEI 0x58)
}

// Marshal encodes the plain DL NAS TRANSPORT message.
func (m *DLNASTransport) Marshal() ([]byte, error) {
	var w common.Writer

	writeMMHeader(&w, MsgDLNASTransport)
	w.U8(m.PayloadContainerType & 0x0F) // spare half octet in bits 5-8

	if err := w.LVE(m.PayloadContainer); err != nil {
		return nil, err
	}

	if m.PDUSessionID != 0 {
		writeTV2(&w, ieiPduSessionID2, m.PDUSessionID)
	}

	if len(m.AdditionalInfo) > 0 {
		writeTLV(&w, ieiAdditionalInfo, m.AdditionalInfo)
	}

	if m.Cause != nil {
		writeTV2(&w, ieiCause5GMM, *m.Cause)
	}

	return w.Bytes(), nil
}
