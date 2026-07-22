// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// Session-AMBR unit values (TS 24.501 §9.11.4.14, table 9.11.4.14.1).
const (
	SessionAMBRUnitNotUsed uint8 = 0x00
	SessionAMBRUnit1Kbps   uint8 = 0x01
	SessionAMBRUnit1Mbps   uint8 = 0x06
	SessionAMBRUnit1Gbps   uint8 = 0x0B
	SessionAMBRUnit1Tbps   uint8 = 0x10
	SessionAMBRUnit1Pbps   uint8 = 0x15
)

// SessionAMBR is the session aggregate maximum bit rate (TS 24.501 §9.11.4.14):
// a per-direction unit and a 16-bit value.
type SessionAMBR struct {
	DownlinkUnit uint8
	Downlink     uint16
	UplinkUnit   uint8
	Uplink       uint16
}

// marshalValue encodes the 6-octet Session-AMBR IE value: downlink unit,
// downlink value, uplink unit, uplink value.
func (a SessionAMBR) marshalValue() []byte {
	var w common.Writer

	w.U8(a.DownlinkUnit)
	w.U16(a.Downlink)
	w.U8(a.UplinkUnit)
	w.U16(a.Uplink)

	return w.Bytes()
}

// parseSessionAMBR decodes the 6-octet Session-AMBR IE value.
func parseSessionAMBR(v []byte) (SessionAMBR, error) {
	r := common.NewReader(v)

	var a SessionAMBR

	dlUnit, err := r.U8()
	if err != nil {
		return a, err
	}

	dl, err := r.U16()
	if err != nil {
		return a, err
	}

	ulUnit, err := r.U8()
	if err != nil {
		return a, err
	}

	ul, err := r.U16()
	if err != nil {
		return a, err
	}

	return SessionAMBR{DownlinkUnit: dlUnit, Downlink: dl, UplinkUnit: ulUnit, Uplink: ul}, nil
}
