// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// PDUSessionReleaseCommand is the PDU SESSION RELEASE COMMAND (TS 24.501
// §8.3.14): the 5GSM header followed by a mandatory 5GSM cause. PTI is the
// UE-allocated value for a UE-requested release or 0 for a network-requested
// release.
type PDUSessionReleaseCommand struct {
	PDUSessionID uint8
	PTI          uint8
	Cause        uint8
}

// Marshal encodes the plain PDU SESSION RELEASE COMMAND message.
func (m *PDUSessionReleaseCommand) Marshal() ([]byte, error) {
	var w common.Writer

	writeSMHeader(&w, m.PDUSessionID, m.PTI, MsgPDUSessionReleaseCommand)
	w.U8(m.Cause)

	return w.Bytes(), nil
}

// PDUSessionReleaseRequest is the PDU SESSION RELEASE REQUEST (TS 24.501
// §8.3.12): the 5GSM header and an optional 5GSM cause.
type PDUSessionReleaseRequest struct {
	PDUSessionID uint8
	PTI          uint8
	Cause        *uint8 // optional (IEI 0x59)
}

// ParsePDUSessionReleaseRequest decodes the message.
func ParsePDUSessionReleaseRequest(b []byte) (*PDUSessionReleaseRequest, error) {
	r := common.NewReader(b)

	psi, pti, err := readSMHeader(r, MsgPDUSessionReleaseRequest)
	if err != nil {
		return nil, err
	}

	out := &PDUSessionReleaseRequest{PDUSessionID: psi, PTI: pti}

	if _, err := common.WalkOptionalIEs(r, causeAndPCOIEs, causeCollector(&out.Cause)); err != nil {
		return nil, err
	}

	return out, nil
}

// PDUSessionReleaseComplete is the PDU SESSION RELEASE COMPLETE (TS 24.501
// §8.3.15): the 5GSM header and an optional 5GSM cause.
type PDUSessionReleaseComplete struct {
	PDUSessionID uint8
	PTI          uint8
	Cause        *uint8 // optional (IEI 0x59)
}

// ParsePDUSessionReleaseComplete decodes the message.
func ParsePDUSessionReleaseComplete(b []byte) (*PDUSessionReleaseComplete, error) {
	r := common.NewReader(b)

	psi, pti, err := readSMHeader(r, MsgPDUSessionReleaseComplete)
	if err != nil {
		return nil, err
	}

	out := &PDUSessionReleaseComplete{PDUSessionID: psi, PTI: pti}

	if _, err := common.WalkOptionalIEs(r, causeAndPCOIEs, causeCollector(&out.Cause)); err != nil {
		return nil, err
	}

	return out, nil
}

// causeCollector returns an emit callback that stores the optional 5GSM cause
// value (IEI 0x59) into dst, ignoring all other optional IEs.
func causeCollector(dst **uint8) func(iei uint8, value []byte) error {
	return func(iei uint8, value []byte) error {
		if iei == iei5GSMCause && len(value) >= 1 {
			c := value[0]
			*dst = &c
		}

		return nil
	}
}
