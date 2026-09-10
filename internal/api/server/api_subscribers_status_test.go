// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"slices"
	"testing"
	"time"
)

func TestMergeSystems(t *testing.T) {
	var (
		older = time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
		newer = time.Date(2026, 8, 17, 10, 5, 0, 0, time.UTC)

		on4G = systemView{
			system: SystemEPS, present: true, registered: true, connected: true,
			lastSeenRadio: "enb-1",
		}
		on5G = systemView{
			system: System5GS, present: true, registered: true, connected: true,
			lastSeenRadio: "gnb-1",
		}
	)

	at := func(v systemView, seen time.Time) systemView {
		v.lastSeenAt = seen

		return v
	}

	idle := func(v systemView) systemView {
		v.connected = false

		return v
	}

	deregistered := func(v systemView) systemView {
		v.present, v.registered, v.connected = false, false, false

		return v
	}

	for _, tc := range []struct {
		name        string
		view4G      systemView
		view5G      systemView
		wantSystems []string
		want        mergedStatus
	}{
		{
			name: "neither system holds a registration",
		},
		{
			name:        "4G only",
			view4G:      at(on4G, older),
			wantSystems: []string{SystemEPS},
			want: mergedStatus{
				Registered: true, Connected: true, LastSeenRadio: "enb-1", LastSeenAt: older,
			},
		},
		{
			name:        "5G only",
			view5G:      at(on5G, older),
			wantSystems: []string{System5GS},
			want: mergedStatus{
				Registered: true, Connected: true, LastSeenRadio: "gnb-1", LastSeenAt: older,
			},
		},
		{
			name:        "both connected, 4G heard from last",
			view4G:      at(on4G, newer),
			view5G:      at(on5G, older),
			wantSystems: []string{System5GS, SystemEPS},
			want: mergedStatus{
				Registered: true, Connected: true, LastSeenRadio: "enb-1", LastSeenAt: newer,
			},
		},
		{
			name:        "both connected, 5G heard from last",
			view4G:      at(on4G, older),
			view5G:      at(on5G, newer),
			wantSystems: []string{System5GS, SystemEPS},
			want: mergedStatus{
				Registered: true, Connected: true, LastSeenRadio: "gnb-1", LastSeenAt: newer,
			},
		},
		{
			name:        "4G connected, 5G registered but idle",
			view4G:      at(on4G, newer),
			view5G:      at(idle(on5G), older),
			wantSystems: []string{System5GS, SystemEPS},
			want: mergedStatus{
				Registered: true, Connected: true, LastSeenRadio: "enb-1", LastSeenAt: newer,
			},
		},
		{
			name:        "the more recent system is idle, and still names the radio that served it",
			view4G:      at(on4G, older),
			view5G:      at(idle(on5G), newer),
			wantSystems: []string{System5GS, SystemEPS},
			want: mergedStatus{
				Registered: true, Connected: true, LastSeenRadio: "gnb-1", LastSeenAt: newer,
			},
		},
		{
			name:        "the deregistered system is more recent than the registered one",
			view4G:      at(deregistered(on4G), newer),
			view5G:      at(on5G, older),
			wantSystems: []string{System5GS},
			want: mergedStatus{
				Registered: true, Connected: true, LastSeenRadio: "gnb-1", LastSeenAt: older,
			},
		},
		{
			name:        "both idle",
			view4G:      at(idle(on4G), older),
			view5G:      at(idle(on5G), newer),
			wantSystems: []string{System5GS, SystemEPS},
			want: mergedStatus{
				Registered: true, LastSeenRadio: "gnb-1", LastSeenAt: newer,
			},
		},
		{
			name:   "deregistered on both systems",
			view4G: at(deregistered(on4G), older),
			view5G: at(deregistered(on5G), newer),
			want: mergedStatus{
				LastSeenRadio: "gnb-1", LastSeenAt: newer,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeSystems(tc.view5G, tc.view4G)

			if !slices.Equal(got.Systems, tc.wantSystems) {
				t.Errorf("Systems = %v, want %v", got.Systems, tc.wantSystems)
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
		{name: "no context on either system", want: ""},
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
		t.Errorf("registered system = %v, want the live timestamp %v", got, live)
	}

	if got := lastSeenAt(false, live, retained); !got.Equal(retained) {
		t.Errorf("deregistered system = %v, want the retained timestamp %v", got, retained)
	}

	if got := lastSeenAt(true, time.Time{}, retained); !got.Equal(retained) {
		t.Errorf("registered system with no live timestamp = %v, want %v", got, retained)
	}
}
