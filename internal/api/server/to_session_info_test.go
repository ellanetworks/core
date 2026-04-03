package server

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/models"
)

func TestToSessionInfo_ActiveWithAllFields(t *testing.T) {
	pdu := amf.PDUSessionExport{
		PDUSessionID: 1,
		Inactive:     false,
		PDUAddress:   "10.45.0.2",
		DNN:          "internet",
		Snssai:       &models.Snssai{Sst: 1, Sd: "000001"},
		PolicyData: &amf.PolicyDataExport{
			Ambr: &models.Ambr{Uplink: "100 Mbps", Downlink: "200 Mbps"},
		},
	}

	s := toSessionInfo(pdu)

	if s.PDUSessionID != 1 {
		t.Errorf("PDUSessionID = %d, want 1", s.PDUSessionID)
	}

	if s.Status != "active" {
		t.Errorf("Status = %q, want %q", s.Status, "active")
	}

	if s.IPAddress != "10.45.0.2" {
		t.Errorf("IPAddress = %q, want %q", s.IPAddress, "10.45.0.2")
	}

	if s.DNN != "internet" {
		t.Errorf("DNN = %q, want %q", s.DNN, "internet")
	}

	if s.SST != 1 {
		t.Errorf("SST = %d, want 1", s.SST)
	}

	if s.SD != "000001" {
		t.Errorf("SD = %q, want %q", s.SD, "000001")
	}

	if s.SessionAmbrUp != "100 Mbps" {
		t.Errorf("SessionAmbrUp = %q, want %q", s.SessionAmbrUp, "100 Mbps")
	}

	if s.SessionAmbrDown != "200 Mbps" {
		t.Errorf("SessionAmbrDown = %q, want %q", s.SessionAmbrDown, "200 Mbps")
	}
}

func TestToSessionInfo_Inactive(t *testing.T) {
	pdu := amf.PDUSessionExport{
		PDUSessionID: 5,
		Inactive:     true,
		PDUAddress:   "10.45.0.10",
		DNN:          "iot",
	}

	s := toSessionInfo(pdu)

	if s.Status != "inactive" {
		t.Errorf("Status = %q, want %q", s.Status, "inactive")
	}

	if s.PDUSessionID != 5 {
		t.Errorf("PDUSessionID = %d, want 5", s.PDUSessionID)
	}
}

func TestToSessionInfo_NilSnssai(t *testing.T) {
	pdu := amf.PDUSessionExport{
		PDUSessionID: 2,
		DNN:          "internet",
	}

	s := toSessionInfo(pdu)

	if s.SST != 0 {
		t.Errorf("SST = %d, want 0 when Snssai is nil", s.SST)
	}

	if s.SD != "" {
		t.Errorf("SD = %q, want empty when Snssai is nil", s.SD)
	}
}

func TestToSessionInfo_NilPolicyData(t *testing.T) {
	pdu := amf.PDUSessionExport{
		PDUSessionID: 3,
		DNN:          "internet",
		Snssai:       &models.Snssai{Sst: 1},
	}

	s := toSessionInfo(pdu)

	if s.SessionAmbrUp != "" {
		t.Errorf("SessionAmbrUp = %q, want empty when PolicyData is nil", s.SessionAmbrUp)
	}

	if s.SessionAmbrDown != "" {
		t.Errorf("SessionAmbrDown = %q, want empty when PolicyData is nil", s.SessionAmbrDown)
	}
}

func TestToSessionInfo_NilAmbr(t *testing.T) {
	pdu := amf.PDUSessionExport{
		PDUSessionID: 4,
		DNN:          "internet",
		PolicyData:   &amf.PolicyDataExport{},
	}

	s := toSessionInfo(pdu)

	if s.SessionAmbrUp != "" {
		t.Errorf("SessionAmbrUp = %q, want empty when Ambr is nil", s.SessionAmbrUp)
	}

	if s.SessionAmbrDown != "" {
		t.Errorf("SessionAmbrDown = %q, want empty when Ambr is nil", s.SessionAmbrDown)
	}
}

func TestToSessionInfo_EmptySD(t *testing.T) {
	pdu := amf.PDUSessionExport{
		PDUSessionID: 6,
		Snssai:       &models.Snssai{Sst: 1, Sd: ""},
	}

	s := toSessionInfo(pdu)

	if s.SST != 1 {
		t.Errorf("SST = %d, want 1", s.SST)
	}

	if s.SD != "" {
		t.Errorf("SD = %q, want empty", s.SD)
	}
}
