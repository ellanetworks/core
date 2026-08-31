// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"slices"
	"testing"
	"time"
)

func TestMergeAccesses(t *testing.T) {
	var (
		older = time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
		newer = time.Date(2026, 8, 17, 10, 5, 0, 0, time.UTC)

		on4G = accessView{
			rat: "4G", present: true, connected: true, imei: "490154203237518",
			ciphering: "EEA2", integrity: "EIA2", lastSeenRadio: "enb-1",
		}
		on5G = accessView{
			rat: "5G", present: true, connected: true, imei: "490154203237518",
			ciphering: "NEA2", integrity: "NIA2", lastSeenRadio: "gnb-1",
		}
	)

	at := func(v accessView, seen time.Time) accessView {
		v.lastSeenAt = seen

		return v
	}

	idle := func(v accessView) accessView {
		v.connected = false

		return v
	}

	deregistered := func(v accessView) accessView {
		v.present, v.connected = false, false
		v.imei, v.ciphering, v.integrity = "", "", ""

		return v
	}

	for _, tc := range []struct {
		name     string
		view4G   accessView
		view5G   accessView
		wantRATs []string
		want     mergedAccess
	}{
		{
			name: "neither access holds a registration",
		},
		{
			name:     "4G only",
			view4G:   at(on4G, older),
			wantRATs: []string{"4G"},
			want: mergedAccess{
				Connected: true, LastSeenRadio: "enb-1", Imei: "490154203237518",
				Ciphering: "EEA2", Integrity: "EIA2", LastSeenAt: older,
			},
		},
		{
			name:     "5G only",
			view5G:   at(on5G, older),
			wantRATs: []string{"5G"},
			want: mergedAccess{
				Connected: true, LastSeenRadio: "gnb-1", Imei: "490154203237518",
				Ciphering: "NEA2", Integrity: "NIA2", LastSeenAt: older,
			},
		},
		{
			name:     "both connected, 4G heard from last",
			view4G:   at(on4G, newer),
			view5G:   at(on5G, older),
			wantRATs: []string{"4G", "5G"},
			want: mergedAccess{
				Connected: true, LastSeenRadio: "enb-1", Imei: "490154203237518",
				Ciphering: "EEA2", Integrity: "EIA2", LastSeenAt: newer,
			},
		},
		{
			name:     "both connected, 5G heard from last",
			view4G:   at(on4G, older),
			view5G:   at(on5G, newer),
			wantRATs: []string{"4G", "5G"},
			want: mergedAccess{
				Connected: true, LastSeenRadio: "gnb-1", Imei: "490154203237518",
				Ciphering: "NEA2", Integrity: "NIA2", LastSeenAt: newer,
			},
		},
		{
			name:     "4G connected, 5G registered but idle",
			view4G:   at(on4G, newer),
			view5G:   at(idle(on5G), older),
			wantRATs: []string{"4G", "5G"},
			want: mergedAccess{
				Connected: true, LastSeenRadio: "enb-1", Imei: "490154203237518",
				Ciphering: "EEA2", Integrity: "EIA2", LastSeenAt: newer,
			},
		},
		{
			name:     "the more recent access is idle, and still names the radio that served it",
			view4G:   at(on4G, older),
			view5G:   at(idle(on5G), newer),
			wantRATs: []string{"4G", "5G"},
			want: mergedAccess{
				Connected: true, LastSeenRadio: "gnb-1", Imei: "490154203237518",
				Ciphering: "NEA2", Integrity: "NIA2", LastSeenAt: newer,
			},
		},
		{
			name:     "both idle",
			view4G:   at(idle(on4G), older),
			view5G:   at(idle(on5G), newer),
			wantRATs: []string{"4G", "5G"},
			want: mergedAccess{
				LastSeenRadio: "gnb-1", Imei: "490154203237518",
				Ciphering: "NEA2", Integrity: "NIA2", LastSeenAt: newer,
			},
		},
		{
			name:   "deregistered on both accesses",
			view4G: at(deregistered(on4G), older),
			view5G: at(deregistered(on5G), newer),
			want: mergedAccess{
				LastSeenRadio: "gnb-1", LastSeenAt: newer,
			},
		},
		{
			// The IMEI describes the device, not the access, so the access that knows it
			// answers even when it is not the serving one.
			name:   "only the older access reported an IMEI",
			view4G: at(on4G, older),
			view5G: at(accessView{
				rat: "5G", present: true, connected: true, lastSeenRadio: "gnb-1",
				ciphering: "NEA2", integrity: "NIA2",
			}, newer),
			wantRATs: []string{"4G", "5G"},
			want: mergedAccess{
				Connected: true, LastSeenRadio: "gnb-1", Imei: "490154203237518",
				Ciphering: "NEA2", Integrity: "NIA2", LastSeenAt: newer,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeAccesses(tc.view4G, tc.view5G)

			if !slices.Equal(got.RATs, tc.wantRATs) {
				t.Errorf("RATs = %v, want %v", got.RATs, tc.wantRATs)
			}

			if got.Connected != tc.want.Connected {
				t.Errorf("Connected = %v, want %v", got.Connected, tc.want.Connected)
			}

			if got.LastSeenRadio != tc.want.LastSeenRadio {
				t.Errorf("LastSeenRadio = %q, want %q", got.LastSeenRadio, tc.want.LastSeenRadio)
			}

			if got.Imei != tc.want.Imei {
				t.Errorf("Imei = %q, want %q", got.Imei, tc.want.Imei)
			}

			if got.Ciphering != tc.want.Ciphering || got.Integrity != tc.want.Integrity {
				t.Errorf("algorithms = %q/%q, want %q/%q",
					got.Ciphering, got.Integrity, tc.want.Ciphering, tc.want.Integrity)
			}

			if !got.LastSeenAt.Equal(tc.want.LastSeenAt) {
				t.Errorf("LastSeenAt = %v, want %v", got.LastSeenAt, tc.want.LastSeenAt)
			}
		})
	}
}

// The last-seen radio and the algorithms are read together, so they have to come from the
// same access however the two accesses are staggered.
func TestMergeAccessesNeverPairsARadioWithAnotherAccessesAlgorithms(t *testing.T) {
	base := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)

	for i := range 4 {
		for j := range 4 {
			view4G := accessView{
				rat: "4G", present: true, lastSeenRadio: "enb-1",
				ciphering: "EEA2", integrity: "EIA2", lastSeenAt: base.Add(time.Duration(i) * time.Minute),
			}
			view5G := accessView{
				rat: "5G", present: true, lastSeenRadio: "gnb-1",
				ciphering: "NEA2", integrity: "NIA2", lastSeenAt: base.Add(time.Duration(j) * time.Minute),
			}

			got := mergeAccesses(view4G, view5G)

			switch got.LastSeenRadio {
			case "enb-1":
				if got.Ciphering != "EEA2" || got.Integrity != "EIA2" {
					t.Errorf("4G=%d 5G=%d: radio enb-1 reported with %q/%q", i, j, got.Ciphering, got.Integrity)
				}
			case "gnb-1":
				if got.Ciphering != "NEA2" || got.Integrity != "NIA2" {
					t.Errorf("4G=%d 5G=%d: radio gnb-1 reported with %q/%q", i, j, got.Ciphering, got.Integrity)
				}
			default:
				t.Errorf("4G=%d 5G=%d: no radio reported for two registered accesses", i, j)
			}
		}
	}
}

func TestConnectionState(t *testing.T) {
	for _, tc := range []struct {
		name       string
		registered bool
		connected  bool
		want       string
	}{
		{name: "registered and connected", registered: true, connected: true, want: "connected"},
		{name: "registered and idle", registered: true, want: "idle"},
		{name: "not registered occupies neither state", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := connectionState(tc.registered, tc.connected); got != tc.want {
				t.Errorf("connectionState(%v, %v) = %q, want %q", tc.registered, tc.connected, got, tc.want)
			}
		})
	}
}

func TestLastSeenAtPrefersTheLiveContext(t *testing.T) {
	var (
		retained = time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
		live     = time.Date(2026, 8, 17, 10, 5, 0, 0, time.UTC)
	)

	if got := lastSeenAt(true, live, retained); !got.Equal(live) {
		t.Errorf("registered access = %v, want the live timestamp %v", got, live)
	}

	if got := lastSeenAt(false, live, retained); !got.Equal(retained) {
		t.Errorf("deregistered access = %v, want the retained timestamp %v", got, retained)
	}

	if got := lastSeenAt(true, time.Time{}, retained); !got.Equal(retained) {
		t.Errorf("registered access with no live timestamp = %v, want %v", got, retained)
	}
}
