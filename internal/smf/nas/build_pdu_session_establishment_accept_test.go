// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas_test

import (
	"bytes"
	"net"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func buildAccept(t *testing.T, snssai *models.Snssai, pco *smfNas.ProtocolConfigurationOptions, dns net.IP, cause *fgs.GSMCause, addrs *smfNas.PDUSessionAddresses, alwaysOn *bool) *fgs.PDUSessionEstablishmentAccept {
	t.Helper()

	ambr := &models.Ambr{Uplink: models.MustParseBitRate("1 Gbps"), Downlink: models.MustParseBitRate("1 Gbps")}
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
	withSD := buildAccept(t, &models.Snssai{Sst: 1, Sd: "010203"}, &smfNas.ProtocolConfigurationOptions{}, nil, nil, nil, nil)

	sd := *withSD.SNSSAI

	if sd.SST != 1 || sd.SD == nil || *sd.SD != [3]byte{1, 2, 3} {
		t.Errorf("with SD: got %+v", sd)
	}

	noSD := buildAccept(t, &models.Snssai{Sst: 1}, &smfNas.ProtocolConfigurationOptions{}, nil, nil, nil, nil)

	ns := *noSD.SNSSAI

	if ns.SST != 1 || ns.SD != nil {
		t.Errorf("without SD: got %+v", ns)
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
			&smfNas.PDUSessionAddresses{PDUSessionType: fgs.PDUSessionTypeIPv4v6, IPv4Address: net.IP{192, 168, 1, 10}, IPv6IID: iid},
			func(t *testing.T, a *fgs.PDUAddress) {
				if a.SessionType != fgs.PDUSessionTypeIPv4v6 || a.IPv6IID != iid || a.IPv4 != [4]byte{192, 168, 1, 10} {
					t.Errorf("IPv4v6 = %+v", a)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			acc := buildAccept(t, &models.Snssai{Sst: 1}, &smfNas.ProtocolConfigurationOptions{}, nil, nil, tc.addrs, nil)
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
	v4 := buildAccept(t, &models.Snssai{Sst: 1}, &smfNas.ProtocolConfigurationOptions{DNSIPv4Request: true}, net.ParseIP("8.8.8.8"), nil, nil, nil)
	if v4.ExtendedPCO == nil {
		t.Error("expected EPCO for IPv4 DNS")
	}

	v6 := buildAccept(t, &models.Snssai{Sst: 1}, &smfNas.ProtocolConfigurationOptions{DNSIPv6Request: true}, net.ParseIP("2001:4860:4860::8888"), nil, nil, nil)
	if v6.ExtendedPCO == nil {
		t.Error("expected EPCO for IPv6 DNS")
	}
}

func pcoContainer(t *testing.T, acc *fgs.PDUSessionEstablishmentAccept, id uint16) ([]byte, bool) {
	t.Helper()

	if acc.ExtendedPCO == nil {
		return nil, false
	}

	for _, c := range acc.ExtendedPCO.Containers {
		if c.ID == id {
			return c.Content, true
		}
	}

	return nil, false
}

// Rendering an IPv4 resolver into the IPv6 container yields ::ffff:a.b.c.d,
// which is not reachable; an IPv6 one into the IPv4 container is dropped
// entirely. A UE commonly requests both.
func TestBuildGSMPDUSessionEstablishmentAccept_DNSFamilyFollowsResolver(t *testing.T) {
	for _, tc := range []struct {
		name        string
		pco         *smfNas.ProtocolConfigurationOptions
		dns         net.IP
		wantID      uint16
		wantContent []byte
		absentID    uint16
	}{
		{
			"IPv4 resolver, both requested",
			&smfNas.ProtocolConfigurationOptions{DNSIPv4Request: true, DNSIPv6Request: true},
			net.ParseIP("8.8.8.8"),
			nas.PCOContainerDNSServerIPv4Address,
			[]byte{8, 8, 8, 8},
			nas.PCOContainerDNSServerIPv6Address,
		},
		{
			"IPv4 resolver, only IPv6 requested",
			&smfNas.ProtocolConfigurationOptions{DNSIPv6Request: true},
			net.ParseIP("8.8.8.8"),
			nas.PCOContainerDNSServerIPv4Address,
			[]byte{8, 8, 8, 8},
			nas.PCOContainerDNSServerIPv6Address,
		},
		{
			"IPv6 resolver, only IPv4 requested",
			&smfNas.ProtocolConfigurationOptions{DNSIPv4Request: true},
			net.ParseIP("2001:4860:4860::8888"),
			nas.PCOContainerDNSServerIPv6Address, net.ParseIP("2001:4860:4860::8888").To16(),
			nas.PCOContainerDNSServerIPv4Address,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			acc := buildAccept(t, &models.Snssai{Sst: 1}, tc.pco, tc.dns, nil, nil, nil)

			got, ok := pcoContainer(t, acc, tc.wantID)
			if !ok {
				t.Fatalf("container %#04x missing", tc.wantID)
			}

			if !bytes.Equal(got, tc.wantContent) {
				t.Errorf("container %#04x = % x, want % x", tc.wantID, got, tc.wantContent)
			}

			if _, ok := pcoContainer(t, acc, tc.absentID); ok {
				t.Errorf("container %#04x present, want the resolver's family only", tc.absentID)
			}
		})
	}
}

// Meaningless on a session with no IPv4, so withheld even when requested — the
// guard both EPS builders apply. The IPv6 link MTU rides the RA instead.
func TestBuildGSMPDUSessionEstablishmentAccept_LinkMTUGatedOnPDUSessionType(t *testing.T) {
	const mtu = uint16(1400)

	for _, tc := range []struct {
		name        string
		sessionType fgs.PDUSessionType
		want        bool
	}{
		{"IPv4", fgs.PDUSessionTypeIPv4, true},
		{"IPv4v6", fgs.PDUSessionTypeIPv4v6, true},
		{"IPv6", fgs.PDUSessionTypeIPv6, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ambr := &models.Ambr{Uplink: models.MustParseBitRate("1 Gbps"), Downlink: models.MustParseBitRate("1 Gbps")}
			qos := &models.QosData{QFI: 1, Var5qi: 9}
			pco := &smfNas.ProtocolConfigurationOptions{IPv4LinkMTURequest: true}
			addrs := &smfNas.PDUSessionAddresses{PDUSessionType: tc.sessionType}

			raw, err := smfNas.BuildGSMPDUSessionEstablishmentAccept(
				ambr, qos, 5, 1, &models.Snssai{Sst: 1}, "internet", pco, nil, mtu, nil, addrs, nil)
			if err != nil {
				t.Fatalf("build failed: %v", err)
			}

			acc, err := fgs.ParsePDUSessionEstablishmentAccept(raw)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			wantContent := []byte{byte(mtu >> 8), byte(mtu & 0xff)}

			content, ok := pcoContainer(t, acc, nas.PCOContainerIPv4LinkMTU)
			if ok != tc.want {
				t.Fatalf("IPv4 Link MTU container present = %v, want %v", ok, tc.want)
			}

			if ok && !bytes.Equal(content, wantContent) {
				t.Errorf("MTU content = % x, want % x", content, wantContent)
			}
		})
	}
}

// TS 24.501 §6.4.1: the Always-on indication is omitted when nil, "not allowed"
// (APSI 0) or "required" (APSI 1) otherwise.
func TestBuildGSMPDUSessionEstablishmentAccept_AlwaysOn(t *testing.T) {
	notAllowed := false
	required := true

	tests := []struct {
		name     string
		alwaysOn *bool
		wantIE   bool
		wantAPSI bool
	}{
		{"omitted", nil, false, false},
		{"not allowed", &notAllowed, true, false},
		{"required", &required, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc := buildAccept(t, &models.Snssai{Sst: 1}, &smfNas.ProtocolConfigurationOptions{}, nil, nil, nil, tt.alwaysOn)
			if tt.wantIE {
				if acc.AlwaysOn == nil || *acc.AlwaysOn != tt.wantAPSI {
					t.Errorf("APSI = %v, want %t", acc.AlwaysOn, tt.wantAPSI)
				}
			} else if acc.AlwaysOn != nil {
				t.Errorf("expected no always-on IE, got %t", *acc.AlwaysOn)
			}
		})
	}
}

func TestBuildGSMPDUSessionEstablishmentAccept_Cause(t *testing.T) {
	addrs := &smfNas.PDUSessionAddresses{PDUSessionType: fgs.PDUSessionTypeIPv4, IPv4Address: net.IP{10, 0, 0, 1}}

	none := buildAccept(t, &models.Snssai{Sst: 1}, &smfNas.ProtocolConfigurationOptions{}, nil, nil, addrs, nil)
	if none.Cause != nil {
		t.Errorf("expected no cause, got %v", none.Cause)
	}

	v4 := buildAccept(t, &models.Snssai{Sst: 1}, &smfNas.ProtocolConfigurationOptions{}, nil, new(fgs.GSMCausePDUSessionTypeIPv4OnlyAllowed), addrs, nil)
	if v4.Cause == nil || *v4.Cause != fgs.GSMCausePDUSessionTypeIPv4OnlyAllowed {
		t.Errorf("cause = %v, want %d", v4.Cause, fgs.GSMCausePDUSessionTypeIPv4OnlyAllowed)
	}
}
