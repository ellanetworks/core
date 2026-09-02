// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package nas renders the information elements both NAS generations carry. The
// codec draws the same line: what 5GS and EPS share lives in its base package,
// what belongs to one generation lives in fgs or eps.
package nas

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/decoder/utils"
	naslib "github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// UENetworkCapability is the security and feature support a UE reports for S1
// mode (TS 24.301 §9.9.3.34).
type UENetworkCapability struct {
	Hex string   `json:"hex"`
	EEA []string `json:"eea,omitempty"`
	EIA []string `json:"eia,omitempty"`
	UEA []string `json:"uea,omitempty"`
	UIA []string `json:"uia,omitempty"`
	// UCS2NoPreference is octet 6 bit 8: set means the UE has no preference
	// between the default alphabet and UCS2, clear means it prefers the default
	// alphabet. It is not a statement of UCS2 support.
	UCS2NoPreference bool   `json:"ucs2_no_preference,omitempty"`
	Error            string `json:"error,omitempty"`
}

// UENetworkCapabilityFromBytes renders the element from the octets a 5GS message
// replays (TS 24.501 §9.11.3.48), keeping them so the UE's bidding-down
// comparison stays visible.
func UENetworkCapabilityFromBytes(b []byte) *UENetworkCapability {
	if len(b) == 0 {
		return nil
	}

	c, err := eps.ParseUENetworkCapability(b)
	if err != nil {
		return &UENetworkCapability{Hex: hex.EncodeToString(b), Error: err.Error()}
	}

	return UENetworkCapabilityFrom(c)
}

// UENetworkCapabilityFrom renders the element the codec already parsed.
func UENetworkCapabilityFrom(c eps.UENetworkCapability) *UENetworkCapability {
	raw, err := c.MarshalBinary()
	if err != nil {
		raw = nil
	}

	out := &UENetworkCapability{
		Hex: hex.EncodeToString(raw),
		EEA: utils.AlgorithmNames("EEA", c.EEA),
		EIA: utils.AlgorithmNames("EIA", c.EIA),
	}

	if c.HasUMTS {
		out.UEA = utils.AlgorithmNames("UEA", c.UEA)
		out.UIA = utils.AlgorithmNames("UIA", c.UIA)
		out.UCS2NoPreference = c.UCS2
	}

	return out
}

// UESecurityCapability is the capability the network replays to the UE so it can
// detect a bidding-down attack (TS 24.301 §9.9.3.36).
type UESecurityCapability struct {
	Hex   string   `json:"hex"`
	EEA   []string `json:"eea,omitempty"`
	EIA   []string `json:"eia,omitempty"`
	UEA   []string `json:"uea,omitempty"`
	UIA   []string `json:"uia,omitempty"`
	GEA   []string `json:"gea,omitempty"`
	Error string   `json:"error,omitempty"`
}

// UESecurityCapabilityFromBytes renders the element from the octets a 5GS
// SECURITY MODE COMMAND replays (TS 24.501 §9.11.3.48A).
func UESecurityCapabilityFromBytes(b []byte) *UESecurityCapability {
	if len(b) == 0 {
		return nil
	}

	c, err := eps.ParseUESecurityCapability(b)
	if err != nil {
		return &UESecurityCapability{Hex: hex.EncodeToString(b), Error: err.Error()}
	}

	return UESecurityCapabilityFrom(c)
}

// UESecurityCapabilityFrom renders the element the codec already parsed.
func UESecurityCapabilityFrom(c eps.UESecurityCapability) *UESecurityCapability {
	raw, err := c.MarshalBinary()
	if err != nil {
		raw = nil
	}

	out := &UESecurityCapability{
		Hex: hex.EncodeToString(raw),
		EEA: utils.AlgorithmNames("EEA", c.EEA),
		EIA: utils.AlgorithmNames("EIA", c.EIA),
	}

	if c.HasUMTS {
		out.UEA = utils.AlgorithmNames("UEA", c.UEA)
		out.UIA = utils.AlgorithmNames("UIA", c.UIA)
	}

	if c.HasGERAN {
		out.GEA = utils.AlgorithmNames("GEA", c.GEA)
	}

	return out
}

// EPSBearerContextStatus renders the per-bearer state (TS 24.301 §9.9.2.1).
func EPSBearerContextStatus(s *naslib.EPSBearerContextStatus) []EPSBearerContextStatusItem {
	if s == nil {
		return nil
	}

	// EBI(0) is spare (TS 24.301 §9.9.2.1), so there is no bearer 0 to report.
	out := make([]EPSBearerContextStatusItem, 0, len(s.Active)-1)
	for i := 1; i < len(s.Active); i++ {
		out = append(out, EPSBearerContextStatusItem{EPSBearerIdentity: i, Active: s.Active[i]})
	}

	return out
}

type EPSBearerContextStatusItem struct {
	EPSBearerIdentity int  `json:"eps_bearer_identity"`
	Active            bool `json:"active"`
}
