// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas_test

import (
	"net"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func buildAccept(t *testing.T, snssai *models.Snssai, pco *smfNas.ProtocolConfigurationOptions, dns net.IP, cause uint8, addrs *smfNas.PDUSessionAddresses, alwaysOn *uint8) *fgs.PDUSessionEstablishmentAccept {
	t.Helper()

	ambr := &models.Ambr{Uplink: "1 Gbps", Downlink: "1 Gbps"}
	qos := &models.QosData{QFI: 1, Var5qi: 9}

	msg, err := smfNas.BuildGSMPDUSessionEstablishmentAccept(ambr, qos, 5, 1, snssai, "internet", pco, dns, 0, cause, addrs, alwaysOn)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	acc, err := fgs.ParsePDUSessionEstablishmentAccept(msg)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	return acc
}

func TestBuildGSMPDUSessionEstablishmentAccept_SNSSAI(t *testing.T) {
	withSD := buildAccept(t, &models.Snssai{Sst: 1, Sd: "010203"}, &smfNas.ProtocolConfigurationOptions{}, nil, 0, nil, nil)
	if withSD.SNSSAI == nil || withSD.SNSSAI.SST != 1 || withSD.SNSSAI.SD == nil || *withSD.SNSSAI.SD != [3]byte{1, 2, 3} {
		t.Errorf("with SD: got %+v", withSD.SNSSAI)
	}

	noSD := buildAccept(t, &models.Snssai{Sst: 1}, &smfNas.ProtocolConfigurationOptions{}, nil, 0, nil, nil)
	if noSD.SNSSAI == nil || noSD.SNSSAI.SST != 1 || noSD.SNSSAI.SD != nil {
		t.Errorf("without SD: got %+v", noSD.SNSSAI)
	}
}

func TestBuildGSMPDUSessionEstablishmentAccept_PDUAddress(t *testing.T) {
	iid := [8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}

	tests := []struct {
		name  string
		addrs *smfNas.PDUSessionAddresses
		check func(*testing.T, *fgs.PDUAddress)
	}{
		{
			"IPv4",
			&smfNas.PDUSessionAddresses{PDUSessionType: fgs.PDUSessionTypeIPv4, IPv4Address: net.IP{10, 45, 0, 1}},
			func(t *testing.T, a *fgs.PDUAddress) {
				if a.SessionType != fgs.PDUSessionTypeIPv4 || a.IPv4 != [4]byte{10, 45, 0, 1} {
					t.Errorf("IPv4 = %+v", a)
				}
			},
		},
		{
			"IPv6",
			&smfNas.PDUSessionAddresses{PDUSessionType: fgs.PDUSessionTypeIPv6, IPv6IID: iid},
			func(t *testing.T, a *fgs.PDUAddress) {
				if a.SessionType != fgs.PDUSessionTypeIPv6 || a.IPv6IID != iid {
					t.Errorf("IPv6 = %+v", a)
				}
			},
		},
		{
			"IPv4v6",
			&smfNas.PDUSessionAddresses{PDUSessionType: fgs.PDUSessionTypeIPv4IPv6, IPv4Address: net.IP{192, 168, 1, 10}, IPv6IID: iid},
			func(t *testing.T, a *fgs.PDUAddress) {
				if a.SessionType != fgs.PDUSessionTypeIPv4IPv6 || a.IPv6IID != iid || a.IPv4 != [4]byte{192, 168, 1, 10} {
					t.Errorf("IPv4v6 = %+v", a)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			acc := buildAccept(t, &models.Snssai{Sst: 1}, &smfNas.ProtocolConfigurationOptions{}, nil, 0, tc.addrs, nil)
			if acc.PDUSessionType != tc.addrs.PDUSessionType {
				t.Errorf("PDU session type = %d, want %d", acc.PDUSessionType, tc.addrs.PDUSessionType)
			}

			if acc.PDUAddress == nil {
				t.Fatal("PDU address IE missing")
			}

			tc.check(t, acc.PDUAddress)
		})
	}
}

func TestBuildGSMPDUSessionEstablishmentAccept_DNS(t *testing.T) {
	v4 := buildAccept(t, &models.Snssai{Sst: 1}, &smfNas.ProtocolConfigurationOptions{DNSIPv4Request: true}, net.ParseIP("8.8.8.8"), 0, nil, nil)
	if v4.ExtendedPCO == nil {
		t.Error("expected EPCO for IPv4 DNS")
	}

	v6 := buildAccept(t, &models.Snssai{Sst: 1}, &smfNas.ProtocolConfigurationOptions{DNSIPv6Request: true}, net.ParseIP("2001:4860:4860::8888"), 0, nil, nil)
	if v6.ExtendedPCO == nil {
		t.Error("expected EPCO for IPv6 DNS")
	}
}

// TS 24.501 §6.4.1: the Always-on indication is omitted when nil, "not allowed"
// (APSI 0) or "required" (APSI 1) otherwise.
func TestBuildGSMPDUSessionEstablishmentAccept_AlwaysOn(t *testing.T) {
	notAllowed := uint8(0)
	required := uint8(1)

	tests := []struct {
		name     string
		alwaysOn *uint8
		wantIE   bool
		wantAPSI uint8
	}{
		{"omitted", nil, false, 0},
		{"not allowed", &notAllowed, true, 0},
		{"required", &required, true, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc := buildAccept(t, &models.Snssai{Sst: 1}, &smfNas.ProtocolConfigurationOptions{}, nil, 0, nil, tt.alwaysOn)
			if tt.wantIE {
				if acc.AlwaysOn == nil || *acc.AlwaysOn != tt.wantAPSI {
					t.Errorf("APSI = %v, want %d", acc.AlwaysOn, tt.wantAPSI)
				}
			} else if acc.AlwaysOn != nil {
				t.Errorf("expected no always-on IE, got %d", *acc.AlwaysOn)
			}
		})
	}
}

func TestBuildGSMPDUSessionEstablishmentAccept_Cause(t *testing.T) {
	addrs := &smfNas.PDUSessionAddresses{PDUSessionType: fgs.PDUSessionTypeIPv4, IPv4Address: net.IP{10, 0, 0, 1}}

	none := buildAccept(t, &models.Snssai{Sst: 1}, &smfNas.ProtocolConfigurationOptions{}, nil, 0, addrs, nil)
	if none.Cause != 0 {
		t.Errorf("expected no cause, got %d", none.Cause)
	}

	v4 := buildAccept(t, &models.Snssai{Sst: 1}, &smfNas.ProtocolConfigurationOptions{}, nil, fgs.GSMCausePDUSessionTypeIPv4OnlyAllowed, addrs, nil)
	if v4.Cause != fgs.GSMCausePDUSessionTypeIPv4OnlyAllowed {
		t.Errorf("cause = %d, want %d", v4.Cause, fgs.GSMCausePDUSessionTypeIPv4OnlyAllowed)
	}
}
