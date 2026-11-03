// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"encoding/json"
	"time"
)

func SDMLcsPrivacyDataPath(supi string) string {
	return ServiceNudmSDM.path(supi, "lcs-privacy-data")
}

type LcsClientClass string

const (
	LcsClientBroadcastService  LcsClientClass = "BROADCAST_SERVICE"
	LcsClientOMInHPLMN         LcsClientClass = "OM_IN_HPLMN"
	LcsClientOMInVPLMN         LcsClientClass = "OM_IN_VPLMN"
	LcsClientAnonymousLocation LcsClientClass = "ANONYMOUS_LOCATION_SERVICE"
	LcsClientSpecificService   LcsClientClass = "SPECIFIC_SERVICE"
	LcsClientNWDAFInHPLMN      LcsClientClass = "NWDAF_IN_HPLMN"
	LcsClientNWDAFInVPLMN      LcsClientClass = "NWDAF_IN_VPLMN"
)

func (c LcsClientClass) Known() bool {
	return oneOf(c, LcsClientBroadcastService, LcsClientOMInHPLMN, LcsClientOMInVPLMN, LcsClientAnonymousLocation,
		LcsClientSpecificService, LcsClientNWDAFInHPLMN, LcsClientNWDAFInVPLMN)
}

type LocationPrivacyInd string

const (
	LocationDisallowed LocationPrivacyInd = "LOCATION_DISALLOWED"
	LocationAllowed    LocationPrivacyInd = "LOCATION_ALLOWED"
)

func (l LocationPrivacyInd) Known() bool {
	return oneOf(l, LocationDisallowed, LocationAllowed)
}

type ValidTimePeriod struct {
	StartTime DateTime `json:"startTime,omitempty"`
	EndTime   DateTime `json:"endTime,omitempty"`
}

func (v ValidTimePeriod) Check(p Policy) error {
	if p != Strict {
		return nil
	}

	return firstError(
		checkDateTime("/startTime", v.StartTime, true),
		checkDateTime("/endTime", v.EndTime, true),
	)
}

func (v ValidTimePeriod) Covers(now time.Time) bool {
	if v.StartTime != "" {
		if start, err := v.StartTime.Time(); err == nil && now.Before(start) {
			return false
		}
	}

	if v.EndTime != "" {
		if end, err := v.EndTime.Time(); err == nil && !now.Before(end) {
			return false
		}
	}

	return true
}

type Lpi struct {
	LocationPrivacyInd LocationPrivacyInd `json:"locationPrivacyInd"`
	ValidTimePeriod    *ValidTimePeriod   `json:"validTimePeriod,omitempty"`
}

func (l Lpi) Check(p Policy) error {
	return firstError(
		CheckEnum("/locationPrivacyInd", l.LocationPrivacyInd, p, false),
		CheckOptional("/validTimePeriod", l.ValidTimePeriod, p),
	)
}

func (l Lpi) Disallows(now time.Time) bool {
	if l.LocationPrivacyInd == LocationAllowed {
		return false
	}

	return l.ValidTimePeriod == nil || l.ValidTimePeriod.Covers(now)
}

type PlmnOperatorClass struct {
	LcsClientClass LcsClientClass `json:"lcsClientClass"`
	LcsClientIds   []string       `json:"lcsClientIds"`
}

func (c PlmnOperatorClass) Check(p Policy) error {
	if err := CheckEnum("/lcsClientClass", c.LcsClientClass, p, false); err != nil || p != Strict {
		return err
	}

	return present("/lcsClientIds", len(c.LcsClientIds) > 0)
}

type LcsPrivacyData struct {
	Lpi                 *Lpi                `json:"lpi,omitempty"`
	PlmnOperatorClasses []PlmnOperatorClass `json:"plmnOperatorClasses,omitempty"`
}

func (d *LcsPrivacyData) UnmarshalJSON(data []byte) error {
	var aux struct {
		Lpi                 *Lpi              `json:"lpi"`
		PlmnOperatorClasses []json.RawMessage `json:"plmnOperatorClasses"`
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*d = LcsPrivacyData{Lpi: aux.Lpi, PlmnOperatorClasses: decodeValid[PlmnOperatorClass](aux.PlmnOperatorClasses)}

	return nil
}

func (d LcsPrivacyData) Check(p Policy) error {
	if err := firstError(
		CheckOptional("/lpi", d.Lpi, p),
		CheckAll("/plmnOperatorClasses", d.PlmnOperatorClasses, p),
	); err != nil || p != Strict {
		return err
	}

	if d.PlmnOperatorClasses != nil && len(d.PlmnOperatorClasses) == 0 {
		return Incorrect("/plmnOperatorClasses", true, "empty list")
	}

	return nil
}

func (d LcsPrivacyData) AllowsPlmnOperatorClass(c LcsClientClass) bool {
	for _, oc := range d.PlmnOperatorClasses {
		if oc.LcsClientClass == c {
			return true
		}
	}

	return false
}
