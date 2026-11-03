// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"net/url"
	"strings"
)

type partnerNamer interface {
	PartnerName(partnerID string) string
}

func (h *HomeSBI) record(partnerID, target string, status int, err error) {
	name := ""
	if n, ok := h.n32.(partnerNamer); ok {
		name = n.PartnerName(partnerID)
	}

	recordHomeSBI(name, serviceOf(target), status, err)
}

func serviceOf(target string) string {
	u, err := url.Parse(target)
	if err != nil {
		return "unknown"
	}

	for seg := range strings.SplitSeq(strings.Trim(u.Path, "/"), "/") {
		if strings.HasPrefix(seg, "n") && strings.Contains(seg, "-") {
			return seg
		}
	}

	return "unknown"
}
