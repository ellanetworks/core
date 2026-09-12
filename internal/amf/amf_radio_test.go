// SPDX-FileCopyrightText: Ella Networks Inc.
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
		name     string
		ranID    *models.GlobalRanNodeID
		expected string
	}{
		{"gNB", &models.GlobalRanNodeID{GNbID: &models.GNbID{GNBValue: "000102", BitLength: 24}}, "gNB"},
		{"ng-eNB", &models.GlobalRanNodeID{NgeNbID: "MacroNGeNB-0abcd"}, "ng-eNB"},
		{"eNB", &models.GlobalRanNodeID{ENbID: "MacroeNB-0abcd"}, "eNB"},
		{"N3IWF", &models.GlobalRanNodeID{N3IwfID: "deadbeef"}, "N3IWF"},
		{"W-AGF", &models.GlobalRanNodeID{WAgfID: "deadbeef"}, "W-AGF"},
		{"TNGF", &models.GlobalRanNodeID{TngfID: "deadbeef"}, "TNGF"},
		{"no alternative", &models.GlobalRanNodeID{}, "Unknown"},
		{"unclaimed", nil, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			radio := &amf.Radio{RanID: tt.ranID}

			if got := radio.RanNodeTypeName(); got != tt.expected {
				t.Errorf("RanNodeTypeName() = %q, want %q", got, tt.expected)
			}
		})
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
	amfInstance := amf.New(nil, nil, nil)

	blank := &amf.Radio{}

	if !amfInstance.RadioConnectedAtForTest(blank).IsZero() {
		t.Fatal("expected ConnectedAt to be zero on a blank Radio")
	}

	if !blank.LastSeenAt().IsZero() {
		t.Fatal("expected LastSeenAt to be zero on a blank Radio")
	}

	now := time.Now()

	radio := &amf.Radio{}

	radio.SetLastSeenAt(now)

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
				RanID: &models.GlobalRanNodeID{
					GNbID: &models.GNbID{GNBValue: "00102"},
				},
			},
			expectedID: "00102",
		},
		{
			name: "ng-eNB",
			radio: &amf.Radio{
				RanID: &models.GlobalRanNodeID{NgeNbID: "MacroNGeNB-abcdef"},
			},
			expectedID: "MacroNGeNB-abcdef",
		},
		{
			name: "N3IWF",
			radio: &amf.Radio{
				RanID: &models.GlobalRanNodeID{N3IwfID: "deadbeef"},
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

func TestRadioConcurrentHandoverTargetsCoexist(t *testing.T) {
	radio := &amf.Radio{Log: logger.AmfLog}
	amfInstance := amf.New(nil, nil, nil)
	radio.BindAMFForTest(amfInstance)

	target1 := amf.NewUeConnForTest(radio, models.RanUeNgapIDUnspecified, 500, logger.AmfLog)
	target2 := amf.NewUeConnForTest(radio, models.RanUeNgapIDUnspecified, 501, logger.AmfLog)

	if got := amfInstance.FindUEByAmfUeNgapID(radio, 500); got != target1 {
		t.Errorf("FindUEByAmfUeNgapID(500) = %v, want first target", got)
	}

	if got := amfInstance.FindUEByAmfUeNgapID(radio, 501); got != target2 {
		t.Errorf("FindUEByAmfUeNgapID(501) = %v, want second target", got)
	}

	amfInstance.UpdateUERanNgapID(target1, 100)
	amfInstance.UpdateUERanNgapID(target2, 101)

	if got := amfInstance.FindUEByRanUeNgapID(radio, 100); got != target1 {
		t.Errorf("FindUEByRanUeNgapID(100) = %v, want first target", got)
	}

	if got := amfInstance.FindUEByRanUeNgapID(radio, 101); got != target2 {
		t.Errorf("FindUEByRanUeNgapID(101) = %v, want second target", got)
	}

	if got := amfInstance.FindUEByAmfUeNgapID(radio, 500); got != target1 {
		t.Errorf("after RAN ID assignment, FindUEByAmfUeNgapID(500) = %v, want first target", got)
	}
}
