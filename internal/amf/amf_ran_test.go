// Copyright 2026 Ella Networks
//
// SPDX-License-Identifier: Apache-2.0

package amf_test

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/models"
)

func TestRadioRanNodeTypeName(t *testing.T) {
	tests := []struct {
		ranPresent int
		expected   string
	}{
		{amf.RanPresentGNbID, "gNB"},
		{amf.RanPresentNgeNbID, "ng-eNB"},
		{amf.RanPresentN3IwfID, "N3IWF"},
		{0, "Unknown"},
		{99, "Unknown"},
	}

	for _, tt := range tests {
		radio := &amf.Radio{RanPresent: tt.ranPresent}

		got := radio.RanNodeTypeName()
		if got != tt.expected {
			t.Errorf("RanPresent=%d: expected %q, got %q", tt.ranPresent, tt.expected, got)
		}
	}
}

func TestRadioTouchLastSeen(t *testing.T) {
	radio := &amf.Radio{
		LastSeenAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	before := time.Now()

	radio.TouchLastSeen()

	after := time.Now()

	if radio.LastSeenAt.Before(before) || radio.LastSeenAt.After(after) {
		t.Fatalf("expected LastSeenAt between %v and %v, got %v", before, after, radio.LastSeenAt)
	}
}

func TestRadioTimestampsSetOnCreation(t *testing.T) {
	blank := &amf.Radio{}

	if !blank.ConnectedAt.IsZero() {
		t.Fatal("expected ConnectedAt to be zero on a blank Radio")
	}

	if !blank.LastSeenAt.IsZero() {
		t.Fatal("expected LastSeenAt to be zero on a blank Radio")
	}

	now := time.Now()

	radio := &amf.Radio{
		ConnectedAt: now,
		LastSeenAt:  now,
	}

	if radio.ConnectedAt.IsZero() || radio.ConnectedAt != now {
		t.Fatalf("expected ConnectedAt to be %v, got %v", now, radio.ConnectedAt)
	}

	if radio.LastSeenAt.IsZero() || radio.LastSeenAt != now {
		t.Fatalf("expected LastSeenAt to be %v, got %v", now, radio.LastSeenAt)
	}
}

func TestRadioNodeID(t *testing.T) {
	tests := []struct {
		name       string
		radio      *amf.Radio
		expectedID string
	}{
		{
			name:       "nil RanID",
			radio:      &amf.Radio{},
			expectedID: "",
		},
		{
			name: "gNB",
			radio: &amf.Radio{
				RanPresent: amf.RanPresentGNbID,
				RanID: &models.GlobalRanNodeID{
					GNbID: &models.GNbID{GNBValue: "00102"},
				},
			},
			expectedID: "00102",
		},
		{
			name: "ng-eNB",
			radio: &amf.Radio{
				RanPresent: amf.RanPresentNgeNbID,
				RanID:      &models.GlobalRanNodeID{NgeNbID: "MacroNGeNB-abcdef"},
			},
			expectedID: "MacroNGeNB-abcdef",
		},
		{
			name: "N3IWF",
			radio: &amf.Radio{
				RanPresent: amf.RanPresentN3IwfID,
				RanID:      &models.GlobalRanNodeID{N3IwfID: "deadbeef"},
			},
			expectedID: "deadbeef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.radio.NodeID()
			if got != tt.expectedID {
				t.Errorf("expected %q, got %q", tt.expectedID, got)
			}
		})
	}
}
