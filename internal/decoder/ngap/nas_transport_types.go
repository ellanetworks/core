// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import fgsdec "github.com/ellanetworks/core/internal/decoder/nas/fgs"

type NASPDU struct {
	Protocol string             `json:"protocol"`
	RawHex   string             `json:"raw_hex"`
	Decoded  *fgsdec.NASMessage `json:"decoded"`
}

type RATRestriction struct {
	PLMNID                    PLMNID `json:"plmn_id"`
	RATRestrictionInformation string `json:"rat_restriction_information"`
}

type ForbiddenAreaInformation struct {
	PLMNID        PLMNID   `json:"plmn_id"`
	ForbiddenTACs []string `json:"forbidden_tacs"`
}

type ServiceAreaInformation struct {
	PLMNID         PLMNID   `json:"plmn_id"`
	AllowedTACs    []string `json:"allowed_tacs,omitempty"`
	NotAllowedTACs []string `json:"not_allowed_tacs,omitempty"`
}

type MobilityRestrictionList struct {
	ServingPLMN              PLMNID                     `json:"serving_plmn"`
	EquivalentPLMNs          []PLMNID                   `json:"equivalent_plmns,omitempty"`
	RATRestrictions          []RATRestriction           `json:"rat_restrictions,omitempty"`
	ForbiddenAreaInformation []ForbiddenAreaInformation `json:"forbidden_area_information,omitempty"`
	ServiceAreaInformation   []ServiceAreaInformation   `json:"service_area_information,omitempty"`
}

type UEAggregateMaximumBitRate struct {
	Downlink int64  `json:"downlink"`
	Uplink   int64  `json:"uplink"`
	Unit     string `json:"unit"`
}
