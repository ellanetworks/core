// Copyright 2026 Ella Networks
//
// SPDX-License-Identifier: Apache-2.0

package context_test

import (
	"testing"
	"time"

	amfcontext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/models"
)

func TestRadioRanNodeTypeName(t *testing.T) {
	tests := []struct {
		ranPresent int
		expected   string
	}{
		{amfcontext.RanPresentGNbID, "gNB"},
		{amfcontext.RanPresentNgeNbID, "ng-eNB"},
		{amfcontext.RanPresentN3IwfID, "N3IWF"},
		{0, "Unknown"},
		{99, "Unknown"},
	}

	for _, tt := range tests {
		radio := &amfcontext.Radio{RanPresent: tt.ranPresent}

		got := radio.RanNodeTypeName()
		if got != tt.expected {
			t.Errorf("RanPresent=%d: expected %q, got %q", tt.ranPresent, tt.expected, got)
		}
	}
}

func TestRadioTouchLastSeen(t *testing.T) {
	radio := &amfcontext.Radio{
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
	blank := &amfcontext.Radio{}

	if !blank.ConnectedAt.IsZero() {
		t.Fatal("expected ConnectedAt to be zero on a blank Radio")
	}

	if !blank.LastSeenAt.IsZero() {
		t.Fatal("expected LastSeenAt to be zero on a blank Radio")
	}

	now := time.Now()

	radio := &amfcontext.Radio{
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
		radio      *amfcontext.Radio
		expectedID string
	}{
		{
			name:       "nil RanID",
			radio:      &amfcontext.Radio{},
			expectedID: "",
		},
		{
			name: "gNB",
			radio: &amfcontext.Radio{
				RanPresent: amfcontext.RanPresentGNbID,
				RanID: &models.GlobalRanNodeID{
					GNbID: &models.GNbID{GNBValue: "00102"},
				},
			},
			expectedID: "00102",
		},
		{
			name: "ng-eNB",
			radio: &amfcontext.Radio{
				RanPresent: amfcontext.RanPresentNgeNbID,
				RanID:      &models.GlobalRanNodeID{NgeNbID: "MacroNGeNB-abcdef"},
			},
			expectedID: "MacroNGeNB-abcdef",
		},
		{
			name: "N3IWF",
			radio: &amfcontext.Radio{
				RanPresent: amfcontext.RanPresentN3IwfID,
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
