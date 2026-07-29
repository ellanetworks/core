// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// Session-AMBR unit values (TS 24.501 §9.11.4.14, table 9.11.4.14.1).
const (
	SessionAMBRUnitNotUsed SessionAMBRUnit = 0x00
	SessionAMBRUnit1Kbps   SessionAMBRUnit = 0x01
	SessionAMBRUnit1Mbps   SessionAMBRUnit = 0x06
	SessionAMBRUnit1Gbps   SessionAMBRUnit = 0x0B
	SessionAMBRUnit1Tbps   SessionAMBRUnit = 0x10
	SessionAMBRUnit1Pbps   SessionAMBRUnit = 0x15
)

// SessionAMBR is the session aggregate maximum bit rate (TS 24.501 §9.11.4.14):
// a per-direction unit and a 16-bit value.
type SessionAMBR struct {
	DownlinkUnit SessionAMBRUnit
	Downlink     uint16
	UplinkUnit   SessionAMBRUnit
	Uplink       uint16
}

// AppendBinary encodes the 6-octet Session-AMBR IE value: downlink unit,
// downlink value, uplink unit, uplink value.
// The encoding is appended to b.
func (a SessionAMBR) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	w.U8(uint8(a.DownlinkUnit))
	w.U16(a.Downlink)
	w.U8(uint8(a.UplinkUnit))
	w.U16(a.Uplink)

	return w.Result(b)
}

// MarshalBinary encodes the SessionAMBR information element value.
func (a SessionAMBR) MarshalBinary() ([]byte, error) { return a.AppendBinary(nil) }

// ParseSessionAMBR decodes the 6-octet Session-AMBR IE value.
func ParseSessionAMBR(v []byte) (SessionAMBR, error) {
	if len(v) != 6 {
		return SessionAMBR{}, fmt.Errorf("nas/fgs: session-AMBR: want 6 octets, got %d", len(v))
	}

	r := nas.NewReader(v)

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

	return SessionAMBR{DownlinkUnit: SessionAMBRUnit(dlUnit), Downlink: dl, UplinkUnit: SessionAMBRUnit(ulUnit), Uplink: ul}, nil
}

// sessionAMBRUnitKbps returns the kbps one unit of a Session-AMBR value stands
// for. The units climb 1, 4, 16, 64, 256 within each decimal decade and then move
// to the next (TS 24.501 §9.11.4.14, table 9.11.4.14.1), the same ladder the QoS
// flow bit rates use. Unit 0 is "value is not used" and carries no rate; codes
// above the table's last read as 256 Pbps.
func sessionAMBRUnitKbps(unit SessionAMBRUnit) (uint64, bool) {
	if unit == SessionAMBRUnitNotUsed {
		return 0, false
	}

	return qosRateUnitKbps(uint8(unit)), true
}

// Kbps returns the downlink and uplink rates in kbit/s, and whether both units
// denote one — a unit of "not used" does not.
func (a SessionAMBR) Kbps() (downlink, uplink uint64, ok bool) {
	dl, dlOK := sessionAMBRUnitKbps(a.DownlinkUnit)
	ul, ulOK := sessionAMBRUnitKbps(a.UplinkUnit)

	if !dlOK || !ulOK {
		return 0, 0, false
	}

	return uint64(a.Downlink) * dl, uint64(a.Uplink) * ul, true
}

// SessionAMBRFromKbps builds a Session-AMBR carrying the given rates, choosing
// for each direction the finest unit whose 16-bit value can hold it. Like the EPS
// APN-AMBR, a rate between two steps rounds down: the value is a maximum, and the
// nearest expressible maximum below it is still a valid one.
func SessionAMBRFromKbps(downlinkKbps, uplinkKbps uint64) (SessionAMBR, error) {
	dlUnit, dl, err := sessionAMBRFields(downlinkKbps)
	if err != nil {
		return SessionAMBR{}, fmt.Errorf("nas/fgs: Session-AMBR downlink: %w", err)
	}

	ulUnit, ul, err := sessionAMBRFields(uplinkKbps)
	if err != nil {
		return SessionAMBR{}, fmt.Errorf("nas/fgs: Session-AMBR uplink: %w", err)
	}

	return SessionAMBR{DownlinkUnit: dlUnit, Downlink: dl, UplinkUnit: ulUnit, Uplink: ul}, nil
}

func sessionAMBRFields(kbps uint64) (SessionAMBRUnit, uint16, error) {
	for unit := uint8(1); unit <= qosRateUnitMax; unit++ {
		if v := kbps / qosRateUnitKbps(unit); v <= 0xFFFF {
			return SessionAMBRUnit(unit), uint16(v), nil
		}
	}

	return 0, 0, fmt.Errorf("%d kbit/s exceeds the range the element carries", kbps)
}
