// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"fmt"
	"time"
)

// GPRSTimerUnit is the unit code in the high three bits of a GPRS timer octet.
// The unit table differs between GPRS timer 2 (TS 24.008 §10.5.7.4) and GPRS
// timer 3 (§10.5.7.4a); only the deactivated code is common to both.
type GPRSTimerUnit uint8

// GPRSTimerUnitDeactivated marks the timer deactivated in either timer.
const GPRSTimerUnitDeactivated GPRSTimerUnit = 0b111

// GPRS timer 2 unit codes (TS 24.008 §10.5.7.4). Codes 0b011 to 0b110 are
// unassigned and mean 1 minute in this version of the protocol.
const (
	GPRSTimer2Unit2Seconds  GPRSTimerUnit = 0b000
	GPRSTimer2Unit1Minute   GPRSTimerUnit = 0b001
	GPRSTimer2UnitDecihours GPRSTimerUnit = 0b010
)

// GPRS timer 3 unit codes (TS 24.008 §10.5.7.4a).
const (
	GPRSTimer3Unit10Minutes GPRSTimerUnit = 0b000
	GPRSTimer3Unit1Hour     GPRSTimerUnit = 0b001
	GPRSTimer3Unit10Hours   GPRSTimerUnit = 0b010
	GPRSTimer3Unit2Seconds  GPRSTimerUnit = 0b011
	GPRSTimer3Unit30Seconds GPRSTimerUnit = 0b100
	GPRSTimer3Unit1Minute   GPRSTimerUnit = 0b101
	GPRSTimer3Unit320Hours  GPRSTimerUnit = 0b110
)

// gprsTimerValueMax is the largest value the 5-bit value field can carry.
const gprsTimerValueMax = 31

// GPRSTimer2 is a GPRS timer 2 information element value: a unit and a 5-bit
// count (TS 24.008 §10.5.7.4). It holds the octet as sent rather than a
// duration, because several encodings denote the same duration and one unit
// marks the timer deactivated.
//
// TS 24.008 §10.5.7.4 defines its value part as octet 2 of the GPRS timer IE
// (§10.5.7.3), so this type also decodes every element the specs call a plain
// "GPRS timer" — TS 24.501 §9.11.2.3 and TS 24.301 §9.9.3.16 — which differ from
// GPRS timer 2 only in framing. GPRSTimer3 has its own unit table and does not.
type GPRSTimer2 struct {
	Unit  GPRSTimerUnit
	Value uint8
}

// GPRSTimer3 is a GPRS timer 3 information element value: a unit and a 5-bit
// count (TS 24.008 §10.5.7.4a). It spans a wider range than GPRS timer 2 and
// uses a different unit table.
type GPRSTimer3 struct {
	Unit  GPRSTimerUnit
	Value uint8
}

// ParseGPRSTimer2 decodes a one-octet GPRS timer 2 information element value.
func ParseGPRSTimer2(b []byte) (GPRSTimer2, error) {
	unit, value, err := parseGPRSTimerOctet(b, "GPRS timer 2")
	if err != nil {
		return GPRSTimer2{}, err
	}

	return GPRSTimer2{Unit: unit, Value: value}, nil
}

// ParseGPRSTimer3 decodes a one-octet GPRS timer 3 information element value.
func ParseGPRSTimer3(b []byte) (GPRSTimer3, error) {
	unit, value, err := parseGPRSTimerOctet(b, "GPRS timer 3")
	if err != nil {
		return GPRSTimer3{}, err
	}

	return GPRSTimer3{Unit: unit, Value: value}, nil
}

func parseGPRSTimerOctet(b []byte, name string) (GPRSTimerUnit, uint8, error) {
	if len(b) != 1 {
		return 0, 0, fmt.Errorf("nas: %s: want 1 octet, got %d", name, len(b))
	}

	return GPRSTimerUnit(b[0] >> 5), b[0] & 0x1F, nil
}

func appendGPRSTimerOctet(b []byte, unit GPRSTimerUnit, value uint8, name string) ([]byte, error) {
	if value > gprsTimerValueMax {
		return nil, fmt.Errorf("nas: %s: value %d exceeds the 5-bit field", name, value)
	}

	if unit > 0b111 {
		return nil, fmt.Errorf("nas: %s: unit %#b exceeds the 3-bit field", name, unit)
	}

	return append(b, uint8(unit)<<5|value), nil
}

// AppendBinary encodes the GPRS timer 2 information element value onto b.
func (t GPRSTimer2) AppendBinary(b []byte) ([]byte, error) {
	return appendGPRSTimerOctet(b, t.Unit, t.Value, "GPRS timer 2")
}

// MarshalBinary encodes the GPRS timer 2 information element value.
func (t GPRSTimer2) MarshalBinary() ([]byte, error) { return t.AppendBinary(nil) }

// AppendBinary encodes the GPRS timer 3 information element value onto b.
func (t GPRSTimer3) AppendBinary(b []byte) ([]byte, error) {
	return appendGPRSTimerOctet(b, t.Unit, t.Value, "GPRS timer 3")
}

// MarshalBinary encodes the GPRS timer 3 information element value.
func (t GPRSTimer3) MarshalBinary() ([]byte, error) { return t.AppendBinary(nil) }

// Deactivated reports whether the timer is deactivated, which is distinct from
// a timer whose duration is zero.
func (t GPRSTimer2) Deactivated() bool { return t.Unit == GPRSTimerUnitDeactivated }

// Deactivated reports whether the timer is deactivated, which is distinct from
// a timer whose duration is zero.
func (t GPRSTimer3) Deactivated() bool { return t.Unit == GPRSTimerUnitDeactivated }

// Duration returns the timer's duration and whether the unit denotes one. It
// reports false only for a deactivated timer, which has none; every other unit
// code denotes one, the unassigned ones meaning a minute (TS 24.008 §10.5.7.3).
func (t GPRSTimer2) Duration() (time.Duration, bool) {
	step, ok := gprsTimer2Step(t.Unit)
	if !ok {
		return 0, false
	}

	return time.Duration(t.Value) * step, true
}

// Duration returns the timer's duration and whether the unit denotes one. It
// reports false for a deactivated timer, which has no duration.
func (t GPRSTimer3) Duration() (time.Duration, bool) {
	step, ok := gprsTimer3Step(t.Unit)
	if !ok {
		return 0, false
	}

	return time.Duration(t.Value) * step, true
}

func gprsTimer2Step(u GPRSTimerUnit) (time.Duration, bool) {
	switch u {
	case GPRSTimer2Unit2Seconds:
		return 2 * time.Second, true
	case GPRSTimer2Unit1Minute:
		return time.Minute, true
	case GPRSTimer2UnitDecihours:
		return 6 * time.Minute, true
	case GPRSTimerUnitDeactivated:
		return 0, false
	default: // TS 24.008 §10.5.7.3: "Other values shall be interpreted as multiples of 1 minute"
		return time.Minute, true
	}
}

func gprsTimer3Step(u GPRSTimerUnit) (time.Duration, bool) {
	switch u {
	case GPRSTimer3Unit10Minutes:
		return 10 * time.Minute, true
	case GPRSTimer3Unit1Hour:
		return time.Hour, true
	case GPRSTimer3Unit10Hours:
		return 10 * time.Hour, true
	case GPRSTimer3Unit2Seconds:
		return 2 * time.Second, true
	case GPRSTimer3Unit30Seconds:
		return 30 * time.Second, true
	case GPRSTimer3Unit1Minute:
		return time.Minute, true
	case GPRSTimer3Unit320Hours:
		return 320 * time.Hour, true
	default: // deactivated
		return 0, false
	}
}

// String renders the timer for logs: its duration, or "deactivated", or the raw
// unit and value when the unit is reserved.
func (t GPRSTimer2) String() string {
	return gprsTimerString(t.Deactivated(), t.Unit, t.Value, t.Duration)
}

// UnitName returns the name TS 24.008 §10.5.7.3 gives the timer's unit. The
// unassigned codes mean a minute, so every code names a unit.
func (t GPRSTimer2) UnitName() string {
	switch t.Unit {
	case GPRSTimer2Unit2Seconds:
		return "2 seconds"
	case GPRSTimer2Unit1Minute:
		return "1 minute"
	case GPRSTimer2UnitDecihours:
		return "6 minutes"
	case GPRSTimerUnitDeactivated:
		return "deactivated"
	default:
		return "1 minute"
	}
}

// UnitName returns the name TS 24.008 §10.5.7.4a gives the timer's unit, or the
// empty string for a unit the table does not name.
func (t GPRSTimer3) UnitName() string {
	switch t.Unit {
	case GPRSTimer3Unit10Minutes:
		return "10 minutes"
	case GPRSTimer3Unit1Hour:
		return "1 hour"
	case GPRSTimer3Unit10Hours:
		return "10 hours"
	case GPRSTimer3Unit2Seconds:
		return "2 seconds"
	case GPRSTimer3Unit30Seconds:
		return "30 seconds"
	case GPRSTimer3Unit1Minute:
		return "1 minute"
	case GPRSTimer3Unit320Hours:
		return "320 hours"
	case GPRSTimerUnitDeactivated:
		return "deactivated"
	default:
		return ""
	}
}

// String renders the timer for logs: its duration, or "deactivated".
func (t GPRSTimer3) String() string {
	return gprsTimerString(t.Deactivated(), t.Unit, t.Value, t.Duration)
}

func gprsTimerString(deactivated bool, unit GPRSTimerUnit, value uint8, duration func() (time.Duration, bool)) string {
	if deactivated {
		return "deactivated"
	}

	if d, ok := duration(); ok {
		return d.String()
	}

	return fmt.Sprintf("unit %#03b value %d", unit, value)
}

// GPRSTimer2FromDuration encodes d as a GPRS timer 2, choosing the smallest unit
// that represents d exactly. It errors when no unit does, so a configured timer
// the element cannot carry fails loudly rather than being silently rounded.
func GPRSTimer2FromDuration(d time.Duration) (GPRSTimer2, error) {
	for _, u := range []GPRSTimerUnit{GPRSTimer2Unit2Seconds, GPRSTimer2Unit1Minute, GPRSTimer2UnitDecihours} {
		step, _ := gprsTimer2Step(u)
		if v, ok := exactGPRSTimerValue(d, step); ok {
			return GPRSTimer2{Unit: u, Value: v}, nil
		}
	}

	return GPRSTimer2{}, fmt.Errorf("nas: cannot encode %v as a GPRS timer 2 (TS 24.008 §10.5.7.4)", d)
}

// GPRSTimer3FromDuration encodes d as a GPRS timer 3, choosing the smallest unit
// that represents d exactly. It errors when no unit does.
func GPRSTimer3FromDuration(d time.Duration) (GPRSTimer3, error) {
	// Finest first, so the smallest exact unit wins.
	for _, u := range []GPRSTimerUnit{
		GPRSTimer3Unit2Seconds,
		GPRSTimer3Unit30Seconds,
		GPRSTimer3Unit1Minute,
		GPRSTimer3Unit10Minutes,
		GPRSTimer3Unit1Hour,
		GPRSTimer3Unit10Hours,
		GPRSTimer3Unit320Hours,
	} {
		step, _ := gprsTimer3Step(u)
		if v, ok := exactGPRSTimerValue(d, step); ok {
			return GPRSTimer3{Unit: u, Value: v}, nil
		}
	}

	return GPRSTimer3{}, fmt.Errorf("nas: cannot encode %v as a GPRS timer 3 (TS 24.008 §10.5.7.4a)", d)
}

func exactGPRSTimerValue(d, step time.Duration) (uint8, bool) {
	if d < 0 || d%step != 0 {
		return 0, false
	}

	n := d / step
	if n > gprsTimerValueMax {
		return 0, false
	}

	return uint8(n), true
}
