// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas"

// NotificationResponse is the NOTIFICATION RESPONSE message (TS 24.501 §8.2.24):
// an optional PDU session status listing the UE's active PDU sessions.
type NotificationResponse struct {
	PDUSessionStatus *PSIBitmap // optional (IEI 0x50)

	// Unrecognized carries the optional information elements this message does
	// not model, so they survive decoding and re-encode unchanged.
	Unrecognized []nas.RawIE
}

var notificationResponseIEs = []nas.OptionalIE{
	{IEI: ieiPDUSessionStatus, Format: nas.IETLV, Name: "PDU session status"},
}

// ParseNotificationResponse decodes a NOTIFICATION RESPONSE message.
func ParseNotificationResponse(b []byte) (*NotificationResponse, error) {
	r := nas.NewReader(b)

	if err := readGMMHeader(r, MsgNotificationResponse); err != nil {
		return nil, err
	}

	out := &NotificationResponse{}

	_unrec, err := walkOptionalIEs(r, notificationResponseIEs, func(iei uint8, value []byte) (bool, error) {
		if iei != ieiPDUSessionStatus {
			return false, nil
		}

		bitmap, err := ParsePSIBitmap(value)
		if err != nil {
			return false, err
		}

		out.PDUSessionStatus = &bitmap

		return true, nil
	})
	if err != nil && !nas.SoftOnly(err) {
		return nil, err
	}

	out.Unrecognized = _unrec

	return out, err
}

// AppendBinary encodes the plain NOTIFICATION RESPONSE message
// (TS 24.501 §8.2.23). The encoding is appended to b.
func (m *NotificationResponse) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	var o nas.OptionalWriter

	writeGMMHeader(w, MsgNotificationResponse)

	if m.PDUSessionStatus != nil {
		raw, err := m.PDUSessionStatus.MarshalBinary()
		if err != nil {
			return b, err
		}

		o.TLV(ieiPDUSessionStatus, raw)
	}

	o.Raw(m.Unrecognized...)
	o.WriteTo(w)

	return w.Result(b)
}

// MarshalBinary encodes the message.
func (m *NotificationResponse) MarshalBinary() ([]byte, error) { return marshalMessage(m) }
