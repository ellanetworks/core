// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"fmt"
	"time"
)

// TimeZone is the time zone information element value (TS 24.008 §10.5.3.8),
// which TS 24.301 §9.9.3.29 and TS 24.501 §9.11.3.52 both carry verbatim: the
// offset from UTC the network tells the UE applies where it is.
//
// The octet is the offset in quarter-hours as two swapped BCD digits — the tens
// digit in the low nibble, the units digit in the high nibble — with bit 4 of
// the low nibble carrying the sign (TS 23.040 §9.2.3.11). That clause places the
// sign at "bit 3", counting from zero; the tens digit reaches 5 for a 14-hour
// offset and so needs the low nibble's three remaining bits, which leaves bit 4
// as the only position the sign can occupy.
//
// It is kept as the octet rather than a duration because the nibbles are BCD and
// need not be decimal: keeping what arrived is what lets the element re-encode
// unchanged, and Offset reports whether it was readable.
type TimeZone byte

// timeZoneSign is the bit of the low nibble that makes the offset negative.
const timeZoneSign byte = 0x08

// Offset returns the offset from UTC and whether the octet held a decimal value.
func (t TimeZone) Offset() (time.Duration, bool) {
	tens, units := byte(t)&0x07, byte(t)>>4
	if units > 9 {
		return 0, false
	}

	quarters := time.Duration(tens*10+units) * 15 * time.Minute
	if byte(t)&timeZoneSign != 0 {
		quarters = -quarters
	}

	return quarters, true
}

// NewTimeZone encodes an offset from UTC, which must be a whole number of
// quarter-hours within ±19 hours 45 minutes.
func NewTimeZone(offset time.Duration) (TimeZone, error) {
	if offset%(15*time.Minute) != 0 {
		return 0, fmt.Errorf("nas: time zone offset %s is not a whole number of quarter-hours", offset)
	}

	quarters := int(offset / (15 * time.Minute))

	sign := byte(0)
	if quarters < 0 {
		sign, quarters = timeZoneSign, -quarters
	}

	if quarters > 79 {
		return 0, fmt.Errorf("nas: time zone offset %s exceeds the two digits the element holds", offset)
	}

	return TimeZone(byte(quarters%10)<<4 | sign | byte(quarters/10)), nil
}

func (t TimeZone) String() string {
	offset, ok := t.Offset()
	if !ok {
		return fmt.Sprintf("invalid time zone (%#02x)", byte(t))
	}

	return fmt.Sprintf("UTC%+.2g", offset.Hours())
}

// ParseTimeZone decodes a time zone IE value.
func ParseTimeZone(b []byte) (TimeZone, error) {
	if len(b) != 1 {
		return 0, fmt.Errorf("nas: time zone is %d octets, want 1", len(b))
	}

	return TimeZone(b[0]), nil
}

// AppendBinary encodes the time zone IE value onto b.
func (t TimeZone) AppendBinary(b []byte) ([]byte, error) { return append(b, byte(t)), nil }

// MarshalBinary encodes the time zone IE value.
func (t TimeZone) MarshalBinary() ([]byte, error) { return t.AppendBinary(nil) }

// TimeZoneAndTime is the time zone and time information element value
// (TS 24.008 §10.5.3.9), which TS 24.301 §9.9.3.30 and TS 24.501 §9.11.3.53 both
// carry verbatim: the universal time at which the network sent the element,
// followed by the local offset.
//
// The six timestamp octets are swapped BCD digit pairs (TS 23.040 §9.2.3.11) and
// are kept as they arrived for the same reason TimeZone is; Time reports whether
// they were readable.
type TimeZoneAndTime struct {
	Year, Month, Day     byte
	Hour, Minute, Second byte
	Zone                 TimeZone
}

// Time returns the instant the element names and whether every field was
// decimal and in range. The year is two digits, which TS 23.040 leaves to the
// receiver to place; this reads it as 2000-2099.
func (t TimeZoneAndTime) Time() (time.Time, bool) {
	fields := [6]int{}

	for i, o := range [6]byte{t.Year, t.Month, t.Day, t.Hour, t.Minute, t.Second} {
		v, ok := swappedBCDPair(o)
		if !ok {
			return time.Time{}, false
		}

		fields[i] = v
	}

	offset, ok := t.Zone.Offset()
	if !ok {
		return time.Time{}, false
	}

	when := time.Date(2000+fields[0], time.Month(fields[1]), fields[2],
		fields[3], fields[4], fields[5], 0, time.UTC)

	// A field out of range rolls over rather than failing, so the round trip is
	// what proves every field named a real date.
	if when.Year()-2000 != fields[0] || int(when.Month()) != fields[1] || when.Day() != fields[2] ||
		when.Hour() != fields[3] || when.Minute() != fields[4] || when.Second() != fields[5] {
		return time.Time{}, false
	}

	return when.In(time.FixedZone("", int(offset/time.Second))), true
}

// NewTimeZoneAndTime encodes an instant and the offset its location is at.
func NewTimeZoneAndTime(when time.Time) (TimeZoneAndTime, error) {
	_, offsetSeconds := when.Zone()

	zone, err := NewTimeZone(time.Duration(offsetSeconds) * time.Second)
	if err != nil {
		return TimeZoneAndTime{}, err
	}

	utc := when.UTC()

	year := utc.Year()
	if year < 2000 || year > 2099 {
		return TimeZoneAndTime{}, fmt.Errorf("nas: year %d is outside the two digits the element holds", year)
	}

	return TimeZoneAndTime{
		Year:   swapBCDPair(year - 2000),
		Month:  swapBCDPair(int(utc.Month())),
		Day:    swapBCDPair(utc.Day()),
		Hour:   swapBCDPair(utc.Hour()),
		Minute: swapBCDPair(utc.Minute()),
		Second: swapBCDPair(utc.Second()),
		Zone:   zone,
	}, nil
}

func (t TimeZoneAndTime) String() string {
	when, ok := t.Time()
	if !ok {
		return "invalid time zone and time"
	}

	return when.Format(time.RFC3339)
}

// ParseTimeZoneAndTime decodes a time zone and time IE value.
func ParseTimeZoneAndTime(b []byte) (TimeZoneAndTime, error) {
	if len(b) != 7 {
		return TimeZoneAndTime{}, fmt.Errorf("nas: time zone and time is %d octets, want 7", len(b))
	}

	return TimeZoneAndTime{
		Year: b[0], Month: b[1], Day: b[2],
		Hour: b[3], Minute: b[4], Second: b[5],
		Zone: TimeZone(b[6]),
	}, nil
}

// AppendBinary encodes the time zone and time IE value onto b.
func (t TimeZoneAndTime) AppendBinary(b []byte) ([]byte, error) {
	return append(b, t.Year, t.Month, t.Day, t.Hour, t.Minute, t.Second, byte(t.Zone)), nil
}

// MarshalBinary encodes the time zone and time IE value.
func (t TimeZoneAndTime) MarshalBinary() ([]byte, error) { return t.AppendBinary(nil) }

// DaylightSavingTime is the daylight saving time information element value
// (TS 24.008 §10.5.3.12), which TS 24.301 §9.9.3.6 and TS 24.501 §9.11.3.19 both
// carry verbatim: how much of the local offset is a summer-time adjustment.
type DaylightSavingTime byte

// Daylight saving adjustments (TS 24.008 table 10.5.97a).
const (
	DaylightSavingNone    DaylightSavingTime = 0
	DaylightSavingOneHour DaylightSavingTime = 1
	DaylightSavingTwoHour DaylightSavingTime = 2
)

// NewDaylightSavingTime reports the summer-time part of when's offset.
func NewDaylightSavingTime(when time.Time) DaylightSavingTime {
	_, offset := when.Zone()

	switch adjustment := time.Duration(offset-standardOffset(when)) * time.Second; adjustment {
	case time.Hour:
		return DaylightSavingOneHour
	case 2 * time.Hour:
		return DaylightSavingTwoHour
	default:
		return DaylightSavingNone
	}
}

func standardOffset(when time.Time) int {
	loc, year := when.Location(), when.Year()

	_, january := time.Date(year, time.January, 15, 12, 0, 0, 0, loc).Zone()
	_, july := time.Date(year, time.July, 15, 12, 0, 0, 0, loc).Zone()

	return min(january, july)
}

// Adjustment returns the summer-time adjustment and whether the value is one
// TS 24.008 assigns; the fourth is reserved.
func (d DaylightSavingTime) Adjustment() (time.Duration, bool) {
	if d&0x03 > 2 {
		return 0, false
	}

	return time.Duration(d&0x03) * time.Hour, true
}

func (d DaylightSavingTime) String() string {
	adjustment, ok := d.Adjustment()
	if !ok {
		return "reserved daylight saving time"
	}

	return fmt.Sprintf("+%s", adjustment)
}

// ParseDaylightSavingTime decodes a daylight saving time IE value.
func ParseDaylightSavingTime(b []byte) (DaylightSavingTime, error) {
	if len(b) != 1 {
		return 0, fmt.Errorf("nas: daylight saving time is %d octets, want 1", len(b))
	}

	return DaylightSavingTime(b[0]), nil
}

// AppendBinary encodes the daylight saving time IE value onto b.
func (d DaylightSavingTime) AppendBinary(b []byte) ([]byte, error) { return append(b, byte(d)), nil }

// MarshalBinary encodes the daylight saving time IE value.
func (d DaylightSavingTime) MarshalBinary() ([]byte, error) { return d.AppendBinary(nil) }

// swappedBCDPair reads a two-digit decimal held as a swapped BCD pair: the tens
// digit in the low nibble, the units digit in the high nibble.
func swappedBCDPair(o byte) (int, bool) {
	tens, units := o&0x0F, o>>4
	if tens > 9 || units > 9 {
		return 0, false
	}

	return int(tens)*10 + int(units), true
}

// swapBCDPair is the inverse of swappedBCDPair for a value of 0-99.
func swapBCDPair(v int) byte { return byte(v%10)<<4 | byte(v/10) }
