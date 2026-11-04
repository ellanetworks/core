// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/udm"
)

const (
	sorMac     = "00112233445566778899aabbccddeeff"
	sorCounter = "0102"
)

func sorPrefix(header byte) []byte {
	return append([]byte{header, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}, 0x01, 0x02)
}

func TestSoRContainerFromTheHomeSorInfo(t *testing.T) {
	yes, no := true, false

	cases := []struct {
		name string
		info wire.SorInfo
		want []byte
		ack  bool
	}{
		{
			name: "no change, acknowledgement requested",
			info: wire.SorInfo{AckInd: &yes, SorMacIausf: sorMac, Countersor: sorCounter},
			want: sorPrefix(0x0c),
			ack:  true,
		},
		{
			name: "PLMN and access technology list",
			info: wire.SorInfo{AckInd: &no, SorMacIausf: sorMac, Countersor: sorCounter, SteeringContainer: &wire.SteeringContainer{SteeringInfo: []wire.SteeringInfo{
				{PlmnID: &wire.PlmnID{Mcc: "001", Mnc: "01"}, AccessTechList: []wire.AccessTech{wire.AccessTechNR, wire.AccessTechEUTRANWBS1AndNBS1}},
				{PlmnID: &wire.PlmnID{Mcc: "310", Mnc: "410"}, AccessTechList: []wire.AccessTech{wire.AccessTechGSMAndECGSMIoT}},
			}}},
			want: append(sorPrefix(0x06), 0x00, 0xf1, 0x10, 0x48, 0x00, 0x13, 0x00, 0x14, 0x00, 0x80),
		},
		{
			name: "secured packet",
			info: wire.SorInfo{AckInd: &no, SorMacIausf: sorMac, Countersor: sorCounter, SteeringContainer: &wire.SteeringContainer{SecuredPacket: []byte{0xde, 0xad}}},
			want: append(sorPrefix(0x02), 0xde, 0xad),
		},
		{
			name: "transparent container from the home network",
			info: wire.SorInfo{AckInd: &no, SorTransparentContainer: []byte{0x73, 0x00, 0x03, 0x08, 0x01, 0x02}},
			want: []byte{0x08, 0x01, 0x02},
			ack:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sor, err := SoRFromHPLMN(&tc.info)
			if err != nil {
				t.Fatalf("SoRFromHPLMN: %v", err)
			}

			if !bytes.Equal(sor.Container, tc.want) || sor.AckRequested != tc.ack {
				t.Fatalf("container %x ack %v, want %x ack %v", sor.Container, sor.AckRequested, tc.want, tc.ack)
			}
		})
	}
}

func TestSoRContainerRejectsMalformedHomeInformation(t *testing.T) {
	no := false

	for _, info := range []wire.SorInfo{
		{AckInd: &no, SorMacIausf: "00", Countersor: sorCounter},
		{AckInd: &no, SorMacIausf: sorMac, Countersor: "01"},
		{AckInd: &no, SorTransparentContainer: []byte{0x72, 0x00, 0x01, 0x00}},
		{AckInd: &no, SorTransparentContainer: []byte{0x73, 0x00, 0x05, 0x00}},
		{AckInd: &no, SorTransparentContainer: []byte{0x73, 0x00, 0x01, 0x01}},
	} {
		if _, err := SoRFromHPLMN(&info); !errors.Is(err, ErrSoRContainer) {
			t.Errorf("%+v: error %v, want ErrSoRContainer", info, err)
		}
	}
}

func TestSoRAcknowledgement(t *testing.T) {
	container := append([]byte{0x01}, bytes.Repeat([]byte{0xab}, 16)...)

	ack, err := SoRAcknowledgement(container, "2026-11-03T10:00:00Z")
	if err != nil || ack.SorMacIue != "abababababababababababababababab" || ack.ProvisioningTime != "2026-11-03T10:00:00Z" {
		t.Fatalf("acknowledgement %+v, %v", ack, err)
	}

	if _, err := SoRAcknowledgement(container[:5], ""); !errors.Is(err, ErrSoRContainer) {
		t.Fatalf("short container: %v", err)
	}
}

func TestMergeAccessAndMobility5G(t *testing.T) {
	local := &udm.AccessAndMobilitySubscription{
		SubscribedSNSSAIs: []models.Snssai{{Sst: 1}, {Sst: 2, Sd: "000001"}},
		UEAMBR:            models.Ambr{Uplink: models.BitRateFromBps(50_000_000), Downlink: models.BitRateFromBps(500_000_000)},
		Visitor:           true,
	}

	home := &wire.AccessAndMobilitySubscriptionData{
		SubscribedUeAmbr: &wire.Ambr{Uplink: "100 Mbps", Downlink: "200 Mbps"},
		Nssai:            &wire.Nssai{DefaultSingleNssais: []wire.Snssai{wire.NewSnssai(1, "")}, SingleNssais: []wire.Snssai{wire.NewSnssai(3, "")}},
	}

	merged, err := MergeAccessAndMobility5G(local, home, func(s models.Snssai) models.Snssai { return s })
	if err != nil {
		t.Fatal(err)
	}

	want := models.Ambr{Uplink: models.BitRateFromBps(50_000_000), Downlink: models.BitRateFromBps(200_000_000)}
	if merged.UEAMBR != want || len(merged.SubscribedSNSSAIs) != 1 || merged.SubscribedSNSSAIs[0].Sst != 1 {
		t.Fatalf("merged %+v", merged)
	}

	if len(local.SubscribedSNSSAIs) != 2 {
		t.Fatal("the local subscription was modified")
	}
}

func TestBreakoutDNN5G(t *testing.T) {
	home := &wire.SmfSelectionSubscriptionData{SubscribedSnssaiInfos: map[string]wire.SnssaiInfo{
		"1": {DnnInfos: []wire.DnnInfo{
			{Dnn: "internet", DefaultDnnIndicator: true, LboRoamingAllowed: true},
			{Dnn: "corp"},
			{Dnn: "ims", LboRoamingAllowed: true},
			{Dnn: "barred", LboRoamingAllowed: true, DnnBarred: true},
		}},
	}}

	local := &udm.SessionManagementSubscription{Visitor: true, DNNs: []udm.DNNConfiguration{
		{Snssai: models.Snssai{Sst: 1}, DNN: "internet", Default: true},
		{Snssai: models.Snssai{Sst: 1}, DNN: "corp"},
		{Snssai: models.Snssai{Sst: 1}, DNN: "ims", IMS: true},
		{Snssai: models.Snssai{Sst: 1}, DNN: "barred"},
	}}

	snssai := models.Snssai{Sst: 1}

	if dnn, ok := DefaultDNN5G(home, snssai); !ok || dnn != "internet" {
		t.Fatalf("default DNN %q", dnn)
	}

	cases := []struct {
		dnn  string
		want error
	}{
		{"internet", nil},
		{"corp", ErrHomeRouted5G},
		{"ims", ErrDNNNotLocal},
		{"barred", ErrDNNNotSubscribed5G},
		{"other", ErrDNNNotSubscribed5G},
	}

	for _, tc := range cases {
		if err := BreakoutDNN5G(home, local, snssai, snssai, tc.dnn); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v, want %v", tc.dnn, err, tc.want)
		}
	}

	if err := BreakoutDNN5G(home, local, models.Snssai{Sst: 2}, models.Snssai{Sst: 2}, "internet"); !errors.Is(err, ErrDNNNotSubscribed5G) {
		t.Errorf("unsubscribed slice: %v", err)
	}
}
