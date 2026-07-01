// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
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
	radio := &amf.Radio{}
	radio.SetLastSeenAt(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))

	before := time.Now()

	radio.TouchLastSeen()

	after := time.Now()

	lastSeen := radio.LastSeenAt()
	if lastSeen.Before(before) || lastSeen.After(after) {
		t.Fatalf("expected LastSeenAt between %v and %v, got %v", before, after, lastSeen)
	}
}

func TestRadioTimestampsSetOnCreation(t *testing.T) {
	blank := &amf.Radio{}

	if !blank.ConnectedAt.IsZero() {
		t.Fatal("expected ConnectedAt to be zero on a blank Radio")
	}

	if !blank.LastSeenAt().IsZero() {
		t.Fatal("expected LastSeenAt to be zero on a blank Radio")
	}

	now := time.Now()

	radio := &amf.Radio{
		ConnectedAt: now,
	}

	radio.SetLastSeenAt(now)

	if radio.ConnectedAt.IsZero() || radio.ConnectedAt != now {
		t.Fatalf("expected ConnectedAt to be %v, got %v", now, radio.ConnectedAt)
	}

	lastSeen := radio.LastSeenAt()
	if lastSeen.IsZero() || !lastSeen.Equal(now) {
		t.Fatalf("expected LastSeenAt to be %v, got %v", now, lastSeen)
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

// TestRadioConcurrentHandoverTargetsCoexist guards against the regression where
// concurrent N2 handover targets to one gNB evicted each other: a target has no
// RAN UE NGAP ID until its Handover Request Acknowledge, so targets are keyed by
// their distinct AMF UE NGAP IDs and must coexist, then each becomes reachable
// by its assigned RAN UE NGAP ID.
func TestRadioConcurrentHandoverTargetsCoexist(t *testing.T) {
	radio := &amf.Radio{Log: logger.AmfLog}
	radio.BindAMFForTest(amf.New(nil, nil, nil))

	target1 := amf.NewRanUeForTest(radio, models.RanUeNgapIDUnspecified, 500, logger.AmfLog)
	target2 := amf.NewRanUeForTest(radio, models.RanUeNgapIDUnspecified, 501, logger.AmfLog)

	if got := radio.FindUEByAmfUeNgapID(500); got != target1 {
		t.Errorf("FindUEByAmfUeNgapID(500) = %v, want first target", got)
	}

	if got := radio.FindUEByAmfUeNgapID(501); got != target2 {
		t.Errorf("FindUEByAmfUeNgapID(501) = %v, want second target", got)
	}

	radio.UpdateUERanNgapID(target1, 100)
	radio.UpdateUERanNgapID(target2, 101)

	if got := radio.FindUEByRanUeNgapID(100); got != target1 {
		t.Errorf("FindUEByRanUeNgapID(100) = %v, want first target", got)
	}

	if got := radio.FindUEByRanUeNgapID(101); got != target2 {
		t.Errorf("FindUEByRanUeNgapID(101) = %v, want second target", got)
	}

	if got := radio.FindUEByAmfUeNgapID(500); got != target1 {
		t.Errorf("after RAN ID assignment, FindUEByAmfUeNgapID(500) = %v, want first target", got)
	}
}
