// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "time"

// NetworkTime is the time-carrying part of the NITZ information, which
// TS 24.501 §3.1 defines as the full name for network, the short name for
// network, the local time zone, the universal time and local time zone, and the
// network daylight saving time. The two names come from the operator's identity
// rather than from a clock, so they are supplied separately; these three are
// what an instant determines.
//
// The same three elements reach a 4G UE in the EMM INFORMATION message
// (TS 24.301 §8.2.13) and a 5G UE in the CONFIGURATION UPDATE COMMAND
// (TS 24.501 §8.2.19), with identical encodings, so both cores build this the
// same way.
type NetworkTime struct {
	LocalTimeZone      TimeZone
	UniversalTime      TimeZoneAndTime
	DaylightSavingTime DaylightSavingTime
}

// NewNetworkTime describes an instant as the three elements the network sends.
//
// All three are always populated, which is what keeps the set consistent: both
// TS 24.301 §8.2.13.4 and §8.2.13.5 require that "if the local time zone has
// been adjusted for daylight saving time, the network shall indicate this by
// including the Network daylight saving time IE", and a UE cannot tell an
// unadjusted zone from an omitted adjustment. Sending the explicit "no
// adjustment" value costs three octets and leaves nothing to infer.
//
// when carries its own location, and that location is the network's: the offset
// and the summer-time adjustment are read from it, while the timestamp itself is
// universal time (TS 24.008 §10.5.3.9).
func NewNetworkTime(when time.Time) (NetworkTime, error) {
	universal, err := NewTimeZoneAndTime(when)
	if err != nil {
		return NetworkTime{}, err
	}

	return NetworkTime{
		LocalTimeZone:      universal.Zone,
		UniversalTime:      universal,
		DaylightSavingTime: NewDaylightSavingTime(when),
	}, nil
}
