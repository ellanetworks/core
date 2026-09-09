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
			rat: "4G", present: true, registered: true, connected: true,
			lastSeenRadio: "enb-1",
		}
		on5G = accessView{
			rat: "5G", present: true, registered: true, connected: true,
			lastSeenRadio: "gnb-1",
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
		v.present, v.registered, v.connected = false, false, false

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
				Registered: true, Connected: true, LastSeenRadio: "enb-1", LastSeenAt: older,
			},
		},
		{
			name:     "5G only",
			view5G:   at(on5G, older),
			wantRATs: []string{"5G"},
			want: mergedAccess{
				Registered: true, Connected: true, LastSeenRadio: "gnb-1", LastSeenAt: older,
			},
		},
		{
			name:     "both connected, 4G heard from last",
			view4G:   at(on4G, newer),
			view5G:   at(on5G, older),
			wantRATs: []string{"4G", "5G"},
			want: mergedAccess{
				Registered: true, Connected: true, LastSeenRadio: "enb-1", LastSeenAt: newer,
			},
		},
		{
			name:     "both connected, 5G heard from last",
			view4G:   at(on4G, older),
			view5G:   at(on5G, newer),
			wantRATs: []string{"4G", "5G"},
			want: mergedAccess{
				Registered: true, Connected: true, LastSeenRadio: "gnb-1", LastSeenAt: newer,
			},
		},
		{
			name:     "4G connected, 5G registered but idle",
			view4G:   at(on4G, newer),
			view5G:   at(idle(on5G), older),
			wantRATs: []string{"4G", "5G"},
			want: mergedAccess{
				Registered: true, Connected: true, LastSeenRadio: "enb-1", LastSeenAt: newer,
			},
		},
		{
			name:     "the more recent access is idle, and still names the radio that served it",
			view4G:   at(on4G, older),
			view5G:   at(idle(on5G), newer),
			wantRATs: []string{"4G", "5G"},
			want: mergedAccess{
				Registered: true, Connected: true, LastSeenRadio: "gnb-1", LastSeenAt: newer,
			},
		},
		{
			name:     "the deregistered access is more recent than the registered one",
			view4G:   at(deregistered(on4G), newer),
			view5G:   at(on5G, older),
			wantRATs: []string{"5G"},
			want: mergedAccess{
				Registered: true, Connected: true, LastSeenRadio: "gnb-1", LastSeenAt: older,
			},
		},
		{
			name:     "both idle",
			view4G:   at(idle(on4G), older),
			view5G:   at(idle(on5G), newer),
			wantRATs: []string{"4G", "5G"},
			want: mergedAccess{
				Registered: true, LastSeenRadio: "gnb-1", LastSeenAt: newer,
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeAccesses(tc.view4G, tc.view5G)

			if !slices.Equal(got.RATs, tc.wantRATs) {
				t.Errorf("RATs = %v, want %v", got.RATs, tc.wantRATs)
			}

			if got.Registered != tc.want.Registered {
				t.Errorf("Registered = %v, want %v", got.Registered, tc.want.Registered)
			}

			if got.Connected != tc.want.Connected {
				t.Errorf("Connected = %v, want %v", got.Connected, tc.want.Connected)
			}

			if got.LastSeenRadio != tc.want.LastSeenRadio {
				t.Errorf("LastSeenRadio = %q, want %q", got.LastSeenRadio, tc.want.LastSeenRadio)
			}

			if !got.LastSeenAt.Equal(tc.want.LastSeenAt) {
				t.Errorf("LastSeenAt = %v, want %v", got.LastSeenAt, tc.want.LastSeenAt)
			}
		})
	}
}

func TestConnectionState(t *testing.T) {
	for _, tc := range []struct {
		name      string
		present   bool
		connected bool
		want      string
	}{
		{name: "a context holding a signalling connection", present: true, connected: true, want: "connected"},
		{name: "a context with no signalling connection", present: true, want: "idle"},
		{name: "no context on either access", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := connectionState(tc.present, tc.connected); got != tc.want {
				t.Errorf("connectionState(%v, %v) = %q, want %q", tc.present, tc.connected, got, tc.want)
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
