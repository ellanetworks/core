// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import "slices"

type UdmInfo struct {
	GroupID string `json:"groupId,omitempty"`
}

type AusfInfo struct {
	GroupID string `json:"groupId,omitempty"`
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

func (n NFProfile) GroupID() string {
	switch {
	case n.NfType == NFTypeUDM && n.UdmInfo != nil:
		return n.UdmInfo.GroupID
	case n.NfType == NFTypeAUSF && n.AusfInfo != nil:
		return n.AusfInfo.GroupID
	}

	return ""
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
