// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
)

func TestSessionFrom5G_AllFields(t *testing.T) {
	s := sessionFrom5G(amf.PDUSessionExport{
		PDUSessionID:   1,
		PDUSessionType: 3, // IPv4v6
		PDUIPV4Address: "10.45.0.2",
		PDUIPV6Prefix:  "2001:db8:ad50:8500::",
		DNN:            "internet",
		Snssai:         &models.Snssai{Sst: 1, Sd: "000001"},
		PolicyData:     &amf.PolicyDataExport{Ambr: &models.Ambr{Uplink: "100 Mbps", Downlink: "200 Mbps"}},
	})

	if s.RadioAccessType != "5G" || s.ID != 1 || s.Status != "active" {
		t.Fatalf("session = %+v", s)
	}

	if s.IPType != "IPv4v6" || s.IPv4Address != "10.45.0.2" || s.IPv6Prefix != "2001:db8:ad50:8500::" {
		t.Fatalf("addressing = %+v", s)
	}

	if s.DataNetwork != "internet" {
		t.Fatalf("DataNetwork = %q", s.DataNetwork)
	}

	if s.Slice == nil || s.Slice.SST != 1 || s.Slice.SD != "000001" {
		t.Fatalf("Slice = %+v", s.Slice)
	}

	if s.AMBRUplink != "100 Mbps" || s.AMBRDownlink != "200 Mbps" {
		t.Fatalf("AMBR = %q/%q", s.AMBRUplink, s.AMBRDownlink)
	}
}

func TestSessionFrom5G_Inactive(t *testing.T) {
	s := sessionFrom5G(amf.PDUSessionExport{PDUSessionID: 5, Inactive: true, PDUSessionType: 1, DNN: "iot"})

	if s.Status != "inactive" || s.ID != 5 || s.IPType != "IPv4" {
		t.Fatalf("session = %+v", s)
	}
}

func TestSessionFrom5G_NilSnssaiAndPolicy(t *testing.T) {
	s := sessionFrom5G(amf.PDUSessionExport{PDUSessionID: 2, DNN: "internet"})

	if s.Slice != nil {
		t.Fatalf("Slice should be nil, got %+v", s.Slice)
	}

	if s.AMBRUplink != "" || s.AMBRDownlink != "" {
		t.Fatalf("AMBR should be empty, got %q/%q", s.AMBRUplink, s.AMBRDownlink)
	}
}

// A 4G PDN connection: radio_access_type 4G, the data network is the APN, and no
// network slice.
func TestSessionFrom4G(t *testing.T) {
	s := sessionFrom4G(&mme.SubscriberSession{
		BearerID:     5,
		APN:          "internet",
		PDNType:      2, // IPv6
		IPv6Prefix:   "2001:db8::",
		AMBRUplink:   "1 Gbps",
		AMBRDownlink: "1 Gbps",
	})

	if s.RadioAccessType != "4G" || s.ID != 5 || s.Status != "active" {
		t.Fatalf("session = %+v", s)
	}

	if s.IPType != "IPv6" || s.IPv6Prefix != "2001:db8::" || s.DataNetwork != "internet" {
		t.Fatalf("session = %+v", s)
	}

	if s.Slice != nil {
		t.Fatalf("4G session must not carry a slice, got %+v", s.Slice)
	}
}
