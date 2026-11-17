// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

func TestMergeSystems(t *testing.T) {
	var (
		older = time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
		newer = time.Date(2026, 8, 17, 10, 5, 0, 0, time.UTC)

		on4G = systemView{
			system: System4G, present: true, registered: true, connected: true,
			lastSeenRadio: "enb-1",
		}
		on5G = systemView{
			system: System5G, present: true, registered: true, connected: true,
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
			wantSystems: []string{System4G},
			want: mergedStatus{
				Registered: true, Connected: true, LastSeenRadio: "enb-1", LastSeenAt: older,
			},
		},
		{
			name:        "5G only",
			view5G:      at(on5G, older),
			wantSystems: []string{System5G},
			want: mergedStatus{
				Registered: true, Connected: true, LastSeenRadio: "gnb-1", LastSeenAt: older,
			},
		},
		{
			name:        "both connected, 4G heard from last",
			view4G:      at(on4G, newer),
			view5G:      at(on5G, older),
			wantSystems: []string{System5G, System4G},
			want: mergedStatus{
				Registered: true, Connected: true, LastSeenRadio: "enb-1", LastSeenAt: newer,
			},
		},
		{
			name:        "both connected, 5G heard from last",
			view4G:      at(on4G, older),
			view5G:      at(on5G, newer),
			wantSystems: []string{System5G, System4G},
			want: mergedStatus{
				Registered: true, Connected: true, LastSeenRadio: "gnb-1", LastSeenAt: newer,
			},
		},
		{
			name:        "4G connected, 5G registered but idle",
			view4G:      at(on4G, newer),
			view5G:      at(idle(on5G), older),
			wantSystems: []string{System5G, System4G},
			want: mergedStatus{
				Registered: true, Connected: true, LastSeenRadio: "enb-1", LastSeenAt: newer,
			},
		},
		{
			name:        "the more recent system is idle, and still names the radio that served it",
			view4G:      at(on4G, older),
			view5G:      at(idle(on5G), newer),
			wantSystems: []string{System5G, System4G},
			want: mergedStatus{
				Registered: true, Connected: true, LastSeenRadio: "gnb-1", LastSeenAt: newer,
			},
		},
		{
			name:        "the deregistered system is more recent than the registered one",
			view4G:      at(deregistered(on4G), newer),
			view5G:      at(on5G, older),
			wantSystems: []string{System5G},
			want: mergedStatus{
				Registered: true, Connected: true, LastSeenRadio: "gnb-1", LastSeenAt: older,
			},
		},
		{
			name:        "both idle",
			view4G:      at(idle(on4G), older),
			view5G:      at(idle(on5G), newer),
			wantSystems: []string{System5G, System4G},
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

func TestRegisteredElsewhere(t *testing.T) {
	regs := []db.UERegistration{
		{Imsi: "001010000000001", Type: db.UERegistrationTypeMME, NodeID: "3"},
		{Imsi: "001010000000001", Type: db.UERegistrationTypeAMF3GPPAccess, NodeID: "2"},
		{Imsi: "001010000000002", Type: db.UERegistrationTypeMME, NodeID: "1"},
		{Imsi: "001010000000003", Type: db.UERegistrationTypeMME, NodeID: "3", Purged: true},
		{Imsi: "001010000000004", Type: "unknown", NodeID: "3"},
	}

	got := registeredElsewhere(regs, "1")

	want := map[string]map[string]bool{
		"001010000000001": {System4G: true, System5G: true},
	}

	if !maps.EqualFunc(got, want, maps.Equal) {
		t.Fatalf("registeredElsewhere = %v, want %v", got, want)
	}
}

func TestWithElsewhere(t *testing.T) {
	for _, tc := range []struct {
		name      string
		local     []string
		elsewhere map[string]bool
		want      []string
	}{
		{name: "nothing elsewhere keeps the local systems", local: []string{System4G}, want: []string{System4G}},
		{name: "a system held only elsewhere", elsewhere: map[string]bool{System4G: true}, want: []string{System4G}},
		{name: "local and elsewhere keep 5G first", local: []string{System4G}, elsewhere: map[string]bool{System5G: true}, want: []string{System5G, System4G}},
		{name: "the same system locally and elsewhere is listed once", local: []string{System5G}, elsewhere: map[string]bool{System5G: true}, want: []string{System5G}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := withElsewhere(tc.local, tc.elsewhere); !slices.Equal(got, tc.want) {
				t.Errorf("withElsewhere = %v, want %v", got, tc.want)
			}
		})
	}
}
