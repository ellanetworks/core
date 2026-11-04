// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"regexp"
	"slices"
	"strings"
)

const maxSupiRangePattern = 256

type SupiRange struct {
	Start   string `json:"start,omitempty"`
	End     string `json:"end,omitempty"`
	Pattern string `json:"pattern,omitempty"`
}

func (r SupiRange) Contains(supi string) bool {
	imsi, isIMSI := IMSIFromSupi(supi)

	if r.Pattern != "" {
		if len(r.Pattern) > maxSupiRangePattern {
			return false
		}

		re, err := regexp.Compile("^(?:" + r.Pattern + ")$")
		if err != nil {
			return false
		}

		return re.MatchString(supi) || (isIMSI && re.MatchString(imsi))
	}

	if !isIMSI || r.Start == "" || r.End == "" {
		return false
	}

	return compareDigits(r.Start, imsi) <= 0 && compareDigits(imsi, r.End) <= 0
}

func compareDigits(a, b string) int {
	a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")

	if len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}

		return 1
	}

	return strings.Compare(a, b)
}

type UdmInfo struct {
	GroupID           string      `json:"groupId,omitempty"`
	SupiRanges        []SupiRange `json:"supiRanges,omitempty"`
	RoutingIndicators []string    `json:"routingIndicators,omitempty"`
}

type AusfInfo struct {
	GroupID           string      `json:"groupId,omitempty"`
	SupiRanges        []SupiRange `json:"supiRanges,omitempty"`
	RoutingIndicators []string    `json:"routingIndicators,omitempty"`
}

type PlmnOauth2 struct {
	Oauth2RequiredPlmnIDList    []PlmnID `json:"oauth2RequiredPlmnIdList,omitempty"`
	Oauth2NotRequiredPlmnIDList []PlmnID `json:"oauth2NotRequiredPlmnIdList,omitempty"`
}

func (p *PlmnOauth2) required(requester PlmnID) (bool, bool) {
	if p == nil {
		return false, false
	}

	switch {
	case slices.Contains(p.Oauth2RequiredPlmnIDList, requester):
		return true, true
	case slices.Contains(p.Oauth2NotRequiredPlmnIDList, requester):
		return false, true
	default:
		return false, false
	}
}

type Scope struct {
	GroupID           string
	SupiRanges        []SupiRange
	RoutingIndicators []string
}

func (s Scope) Open() bool {
	return s.GroupID == "" && len(s.SupiRanges) == 0 && len(s.RoutingIndicators) == 0
}

func (s Scope) SelfDescribing() bool {
	return s.Open() || len(s.SupiRanges) > 0 || len(s.RoutingIndicators) > 0
}

type Subject struct {
	Supi             string
	RoutingIndicator string
	GroupID          string
}

func (s Scope) Serves(sub Subject) bool {
	if sub.GroupID != "" && s.GroupID != "" && s.GroupID != sub.GroupID {
		return false
	}

	if sub.Supi != "" && len(s.SupiRanges) > 0 && !slices.ContainsFunc(s.SupiRanges, func(r SupiRange) bool { return r.Contains(sub.Supi) }) {
		return false
	}

	if ri := sub.RoutingIndicator; ri != "" && !DefaultRoutingIndicator(ri) && len(s.RoutingIndicators) > 0 && !slices.Contains(s.RoutingIndicators, ri) {
		return false
	}

	return true
}

func (s Scope) Proves(sub Subject) bool {
	if !s.Serves(sub) {
		return false
	}

	switch {
	case s.Open():
		return true
	case sub.GroupID != "" && s.GroupID == sub.GroupID:
		return true
	case sub.Supi != "" && len(s.SupiRanges) > 0:
		return true
	case sub.RoutingIndicator != "" && len(s.RoutingIndicators) > 0:
		return true
	default:
		return sub.RoutingIndicator != "" && DefaultRoutingIndicator(sub.RoutingIndicator)
	}
}

func DefaultRoutingIndicator(ri string) bool {
	return strings.TrimLeft(ri, "0") == ""
}

func (n NFProfile) Scope() Scope {
	switch n.NfType {
	case NFTypeUDM:
		if n.UdmInfo != nil {
			return Scope{GroupID: n.UdmInfo.GroupID, SupiRanges: n.UdmInfo.SupiRanges, RoutingIndicators: n.UdmInfo.RoutingIndicators}
		}
	case NFTypeAUSF:
		if n.AusfInfo != nil {
			return Scope{GroupID: n.AusfInfo.GroupID, SupiRanges: n.AusfInfo.SupiRanges, RoutingIndicators: n.AusfInfo.RoutingIndicators}
		}
	}

	return Scope{}
}

func (n NFProfile) oauth2Required(s NFService, requester PlmnID) bool {
	if required, ok := s.PerPlmnOauth2ReqList.required(requester); ok {
		return required
	}

	if s.Oauth2Required != nil {
		return *s.Oauth2Required
	}

	return n.Oauth2Required != nil && *n.Oauth2Required
}
