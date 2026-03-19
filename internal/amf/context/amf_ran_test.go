// Copyright 2026 Ella Networks
//
// SPDX-License-Identifier: Apache-2.0

package context_test

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	amfcontext "github.com/ellanetworks/core/internal/amf/context"
	"go.uber.org/zap"
)

func TestRadioConnectedUECount(t *testing.T) {
	radio := &amfcontext.Radio{
		RanUEs: make(map[int64]*amfcontext.RanUe),
		Log:    zap.NewNop(),
	}

	if radio.ConnectedUECount() != 0 {
		t.Fatalf("expected 0, got %d", radio.ConnectedUECount())
	}

	radio.RanUEs[1] = &amfcontext.RanUe{}
	radio.RanUEs[2] = &amfcontext.RanUe{}

	if radio.ConnectedUECount() != 2 {
		t.Fatalf("expected 2, got %d", radio.ConnectedUECount())
	}
}

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

func TestRadioConnectedSubscribers(t *testing.T) {
	radio := &amfcontext.Radio{
		RanUEs: make(map[int64]*amfcontext.RanUe),
		Log:    zap.NewNop(),
	}

	// Empty case
	supis := radio.ConnectedSubscribers()
	if len(supis) != 0 {
		t.Fatalf("expected 0 subscribers, got %d", len(supis))
	}

	// UE with nil AmfUe should be skipped
	radio.RanUEs[1] = &amfcontext.RanUe{}

	supis = radio.ConnectedSubscribers()
	if len(supis) != 0 {
		t.Fatalf("expected 0 subscribers (nil AmfUe), got %d", len(supis))
	}

	// UE with valid IMSI
	supi, err := etsi.NewSUPIFromIMSI("001019756139935")
	if err != nil {
		t.Fatalf("failed to create SUPI: %v", err)
	}

	ue := amfcontext.NewAmfUe()
	ue.Supi = supi
	ue.Log = zap.NewNop()

	radio.RanUEs[2] = &amfcontext.RanUe{
		AmfUe: ue,
	}

	supis = radio.ConnectedSubscribers()
	if len(supis) != 1 {
		t.Fatalf("expected 1 subscriber, got %d", len(supis))
	}

	if supis[0] != "001019756139935" {
		t.Fatalf("expected IMSI 001019756139935, got %s", supis[0])
	}
}

func TestRadioTimestampsSetOnCreation(t *testing.T) {
	radio := &amfcontext.Radio{
		ConnectedAt: time.Now(),
		LastSeenAt:  time.Now(),
	}

	if radio.ConnectedAt.IsZero() {
		t.Fatal("expected ConnectedAt to be non-zero")
	}

	if radio.LastSeenAt.IsZero() {
		t.Fatal("expected LastSeenAt to be non-zero")
	}
}
