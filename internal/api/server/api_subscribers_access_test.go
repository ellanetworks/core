// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"slices"
	"testing"
	"time"
)

// A subscriber can hold a registration on both accesses at once, so the status API has
// to choose which radio it reports. 4G and 5G are peers: the choice follows which access
// last heard from the UE, and an idle access — which knows no radio at all — never
// blanks the radio the other one is serving the UE on.
func TestMergeAccesses(t *testing.T) {
	var (
		older = time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
		newer = time.Date(2026, 8, 17, 10, 5, 0, 0, time.UTC)
	)

	for _, tc := range []struct {
		name       string
		view4G     accessView
		view5G     accessView
		wantRATs   []string
		wantRadio  string
		wantSeenAt time.Time
	}{
		{
			name: "neither access holds a registration",
		},
		{
			name:       "4G only",
			view4G:     accessView{rat: "4G", present: true, radioName: "enb-1", lastSeenAt: older},
			wantRATs:   []string{"4G"},
			wantRadio:  "enb-1",
			wantSeenAt: older,
		},
		{
			name:       "5G only",
			view5G:     accessView{rat: "5G", present: true, radioName: "gnb-1", lastSeenAt: older},
			wantRATs:   []string{"5G"},
			wantRadio:  "gnb-1",
			wantSeenAt: older,
		},
		{
			name:       "both connected, 4G heard from last",
			view4G:     accessView{rat: "4G", present: true, radioName: "enb-1", lastSeenAt: newer},
			view5G:     accessView{rat: "5G", present: true, radioName: "gnb-1", lastSeenAt: older},
			wantRATs:   []string{"4G", "5G"},
			wantRadio:  "enb-1",
			wantSeenAt: newer,
		},
		{
			name:       "both connected, 5G heard from last",
			view4G:     accessView{rat: "4G", present: true, radioName: "enb-1", lastSeenAt: older},
			view5G:     accessView{rat: "5G", present: true, radioName: "gnb-1", lastSeenAt: newer},
			wantRATs:   []string{"4G", "5G"},
			wantRadio:  "gnb-1",
			wantSeenAt: newer,
		},
		{
			// The UE hands over or re-attaches onto the eNB while the 5GS registration
			// is still standing but idle. The eNB is the answer; reporting no radio
			// because the idle access has none is the bug this guards.
			name:       "4G serving, 5G registered but idle",
			view4G:     accessView{rat: "4G", present: true, radioName: "enb-1", lastSeenAt: newer},
			view5G:     accessView{rat: "5G", present: true, lastSeenAt: older},
			wantRATs:   []string{"4G", "5G"},
			wantRadio:  "enb-1",
			wantSeenAt: newer,
		},
		{
			name:       "5G serving, 4G registered but idle",
			view4G:     accessView{rat: "4G", present: true, lastSeenAt: older},
			view5G:     accessView{rat: "5G", present: true, radioName: "gnb-1", lastSeenAt: newer},
			wantRATs:   []string{"4G", "5G"},
			wantRadio:  "gnb-1",
			wantSeenAt: newer,
		},
		{
			// A registration whose access has not heard from the UE since the core
			// started still names its radio if it is the only one that knows it.
			name:       "idle access is the more recent one",
			view4G:     accessView{rat: "4G", present: true, radioName: "enb-1", lastSeenAt: older},
			view5G:     accessView{rat: "5G", present: true, lastSeenAt: newer},
			wantRATs:   []string{"4G", "5G"},
			wantRadio:  "enb-1",
			wantSeenAt: newer,
		},
		{
			name:      "both idle",
			view4G:    accessView{rat: "4G", present: true},
			view5G:    accessView{rat: "5G", present: true},
			wantRATs:  []string{"4G", "5G"},
			wantRadio: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeAccesses(tc.view4G, tc.view5G)

			if !slices.Equal(got.RATs, tc.wantRATs) {
				t.Errorf("RATs = %v, want %v", got.RATs, tc.wantRATs)
			}

			if got.RadioName != tc.wantRadio {
				t.Errorf("RadioName = %q, want %q", got.RadioName, tc.wantRadio)
			}

			if !got.LastSeenAt.Equal(tc.wantSeenAt) {
				t.Errorf("LastSeenAt = %v, want %v", got.LastSeenAt, tc.wantSeenAt)
			}
		})
	}
}

// The identity and NAS security algorithms are per access, so the caller applies the
// access that heard from the UE longer ago first. An access that knows no value must
// not blank what the other one reported.
func TestApplyIdentityKeepsWhatTheOtherAccessKnows(t *testing.T) {
	status := SubscriberDetailStatus{}

	status.applyIdentity("490154203237518", "EEA2", "EIA2")
	status.applyIdentity("", "", "")

	if status.Imei != "490154203237518" || status.CipheringAlgorithm != "EEA2" || status.IntegrityAlgorithm != "EIA2" {
		t.Fatalf("an access knowing nothing blanked the other one's values: %+v", status)
	}

	status.applyIdentity("", "NEA2", "NIA2")

	if status.Imei != "490154203237518" {
		t.Errorf("Imei = %q, want the one the other access reported", status.Imei)
	}

	if status.CipheringAlgorithm != "NEA2" || status.IntegrityAlgorithm != "NIA2" {
		t.Errorf("algorithms = %q/%q, want the more recent access's NEA2/NIA2",
			status.CipheringAlgorithm, status.IntegrityAlgorithm)
	}
}
