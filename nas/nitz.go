// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "time"

// NetworkTime is the time-carrying part of the NITZ information.
type NetworkTime struct {
	LocalTimeZone      TimeZone
	UniversalTime      TimeZoneAndTime
	DaylightSavingTime DaylightSavingTime
}

// NewNetworkTime describes an instant as the elements the network sends.
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
