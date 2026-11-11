// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/ellanetworks/core/internal/roaming/wire"
)

const (
	pointerSorInfo = "/sorInfo"
	pointerUpuInfo = "/upuInfo"
	resourceAmData = "am-data"
)

type SDMChange struct {
	SoR        *wire.SorInfo
	SoRPartial bool
	UPU        bool
	Refresh    bool
}

func ClassifySDMChange(supi string, n wire.ModificationNotification) SDMChange {
	var out SDMChange

	if len(n.NotifyItems) == 0 {
		out.Refresh = true
		return out
	}

	for _, item := range n.NotifyItems {
		if !amDataResource(supi, item.ResourceID) {
			out.Refresh = true
			continue
		}

		for _, c := range item.Changes {
			out.add(c)
		}
	}

	return out
}

func (s *SDMChange) add(c wire.ChangeItem) {
	path := ""
	if c.Path != nil {
		path = strings.TrimSuffix(*c.Path, "/")
	}

	switch {
	case path == pointerSorInfo:
		if c.Op == wire.ChangeAdd || c.Op == wire.ChangeReplace {
			if info, err := wire.Decode[wire.SorInfo](c.NewValue); err == nil {
				s.SoR = &info
			} else {
				s.SoRPartial = true
			}
		}
	case strings.HasPrefix(path, pointerSorInfo+"/"):
		s.SoRPartial = true
	case path == pointerUpuInfo || strings.HasPrefix(path, pointerUpuInfo+"/"):
		s.UPU = true
	case path == "":
		s.Refresh = true

		if len(c.NewValue) == 0 {
			return
		}

		var whole struct {
			SorInfo json.RawMessage `json:"sorInfo"`
			UpuInfo json.RawMessage `json:"upuInfo"`
		}

		if json.Unmarshal(c.NewValue, &whole) != nil {
			return
		}

		if len(whole.SorInfo) > 0 && string(whole.SorInfo) != "null" {
			if info, err := wire.Decode[wire.SorInfo](whole.SorInfo); err == nil {
				s.SoR = &info
			} else {
				s.SoRPartial = true
			}
		}

		if len(whole.UpuInfo) > 0 && string(whole.UpuInfo) != "null" {
			s.UPU = true
		}
	default:
		s.Refresh = true
	}
}

func amDataResource(supi, resource string) bool {
	path := resource
	if u, err := url.Parse(resource); err == nil {
		path = u.Path
	}

	path = strings.TrimSuffix(path, "/")

	return path == supi+"/"+resourceAmData || strings.HasSuffix(path, "/"+supi+"/"+resourceAmData)
}
