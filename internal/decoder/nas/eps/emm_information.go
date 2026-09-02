// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

type NetworkName struct {
	Name   string `json:"name"`
	Coding string `json:"coding"`
	AddCI  bool   `json:"add_country_initials,omitempty"`
}

type EMMInformation struct {
	FullNameForNetwork  *NetworkName `json:"full_name_for_network,omitempty"`
	ShortNameForNetwork *NetworkName `json:"short_name_for_network,omitempty"`
	LocalTimeZone       *string      `json:"local_time_zone,omitempty"`
	UniversalTime       *string      `json:"universal_time,omitempty"`
	DaylightSavingTime  *uint8       `json:"daylight_saving_time,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildEMMInformation(msg *eps.EMMInformation) *EMMInformation {
	out := &EMMInformation{
		FullNameForNetwork:  networkName(msg.FullNameForNetwork),
		ShortNameForNetwork: networkName(msg.ShortNameForNetwork),
		LocalTimeZone:       timeZone(msg.LocalTimeZone),
	}

	if msg.UniversalTime != nil {
		if t, ok := msg.UniversalTime.Time(); ok {
			s := t.Format("2006-01-02T15:04:05Z07:00")
			out.UniversalTime = &s
		}
	}

	if msg.DaylightSavingTime != nil {
		v := uint8(*msg.DaylightSavingTime)
		out.DaylightSavingTime = &v
	}

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

func networkName(n *nas.NetworkName) *NetworkName {
	if n == nil {
		return nil
	}

	return &NetworkName{Name: n.Name, Coding: n.Coding.String(), AddCI: n.AddCI}
}

func timeZone(z *nas.TimeZone) *string {
	if z == nil {
		return nil
	}

	s := z.String()

	return &s
}
