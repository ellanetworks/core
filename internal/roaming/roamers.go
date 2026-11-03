// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"encoding/json"
	"time"
)

const (
	RAT4G = "4G"
	RAT5G = "5G"

	ClusterRoamersPath = "/cluster/internal/roaming/roamers"
)

type LiveRoamer struct {
	IMSI        string              `json:"imsi"`
	PartnerID   string              `json:"partnerId"`
	PartnerName string              `json:"partnerName"`
	HomeMcc     string              `json:"homeMcc"`
	HomeMnc     string              `json:"homeMnc"`
	RAT         string              `json:"rat"`
	Registered  bool                `json:"registered"`
	Connected   bool                `json:"connected"`
	Since       time.Time           `json:"since"`
	LastSeenAt  time.Time           `json:"lastSeenAt"`
	Radio       string              `json:"radio,omitempty"`
	Sessions    []LiveRoamerSession `json:"sessions"`
}

type PartnerLoad struct {
	Roamers  int
	Sessions int
}

type LiveRoamerSession struct {
	ID          uint8  `json:"id"`
	DataNetwork string `json:"dataNetwork,omitempty"`
	IPv4Address string `json:"ipv4Address,omitempty"`
	IPv6Prefix  string `json:"ipv6Prefix,omitempty"`
}

func DecodeLiveRoamers(body []byte) ([]LiveRoamer, error) {
	var out []LiveRoamer
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}

	return out, nil
}
