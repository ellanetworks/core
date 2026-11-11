// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"maps"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/mme"
)

func TestLiveStatusOnRadio(t *testing.T) {
	s := liveStatus{
		amf5G:         map[string]amf.ConnectedSubscriber{"208930000000001": {}, "208930000000002": {}},
		amf5GLastSeen: map[string]amf.LastSeen{"208930000000001": {RadioName: "gnb-1"}, "208930000000002": {RadioName: "gnb-2"}, "208930000000004": {RadioName: "gnb-1"}},
		mme4G:         map[string]mme.ConnectedSubscriber{"208930000000003": {}},
		mmeLastSeen:   map[string]mme.LastSeen{"208930000000003": {RadioName: "gnb-1"}},
	}

	got := slices.Sorted(maps.Keys(s.onRadio("gnb-1")))
	if !slices.Equal(got, []string{"208930000000001", "208930000000003"}) {
		t.Fatalf("roamers on gnb-1 = %v, want the connected ones last seen there", got)
	}
}
