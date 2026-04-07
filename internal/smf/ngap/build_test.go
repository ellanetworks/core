// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package ngap_test

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func decodeSetupRequestGTPTunnel(t *testing.T, buf []byte) (uint32, [4]byte) {
	t.Helper()

	var transfer ngapType.PDUSessionResourceSetupRequestTransfer
	if err := aper.UnmarshalWithParams(buf, &transfer, "valueExt"); err != nil {
		t.Fatalf("unmarshal PDUSessionResourceSetupRequestTransfer: %v", err)
	}

	for _, ie := range transfer.ProtocolIEs.List {
		if ie.Id.Value != ngapType.ProtocolIEIDULNGUUPTNLInformation {
			continue
		}

		tunnel := ie.Value.ULNGUUPTNLInformation.GTPTunnel
		teid := binary.BigEndian.Uint32(tunnel.GTPTEID.Value)

		bs := tunnel.TransportLayerAddress.Value
		if bs.BitLength != 32 {
			t.Fatalf("expected BitLength=32, got %d", bs.BitLength)
		}

		var ip [4]byte
		copy(ip[:], bs.Bytes)

		return teid, ip
	}

	t.Fatal("ULNGUUPTNLInformation IE not found")

	return 0, [4]byte{}
}

func TestBuildPDUSessionResourceSetupRequestTransfer(t *testing.T) {
	ambr := &models.Ambr{Uplink: "100 Mbps", Downlink: "200 Mbps"}
	qos := &models.QosData{Var5qi: 9, Arp: &models.Arp{PriorityLevel: 1}, QFI: 1}
	addr := netip.MustParseAddr("10.3.0.2")

	buf, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(ambr, qos, 42, addr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	teid, ip := decodeSetupRequestGTPTunnel(t, buf)

	if teid != 42 {
		t.Errorf("TEID: got %d, want 42", teid)
	}

	if ip != [4]byte{10, 3, 0, 2} {
		t.Errorf("IP: got %v, want [10 3 0 2]", ip)
	}
}

func TestBuildPDUSessionResourceSetupRequestTransfer_NilAmbr(t *testing.T) {
	_, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(nil, nil, 1, netip.MustParseAddr("1.2.3.4"))
	if err == nil {
		t.Fatal("expected error for nil ambr")
	}
}

func TestBuildHandoverCommandTransfer(t *testing.T) {
	addr := netip.MustParseAddr("192.168.1.100")

	buf, err := ngap.BuildHandoverCommandTransfer(99, addr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var transfer ngapType.HandoverCommandTransfer
	if err := aper.UnmarshalWithParams(buf, &transfer, "valueExt"); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	tunnel := transfer.DLForwardingUPTNLInformation.GTPTunnel
	teid := binary.BigEndian.Uint32(tunnel.GTPTEID.Value)

	if teid != 99 {
		t.Errorf("TEID: got %d, want 99", teid)
	}

	bs := tunnel.TransportLayerAddress.Value
	if bs.BitLength != 32 {
		t.Fatalf("BitLength: got %d, want 32", bs.BitLength)
	}

	var ip [4]byte
	copy(ip[:], bs.Bytes)

	if ip != [4]byte{192, 168, 1, 100} {
		t.Errorf("IP: got %v, want [192 168 1 100]", ip)
	}
}

func TestBuildPathSwitchRequestAcknowledgeTransfer(t *testing.T) {
	addr := netip.MustParseAddr("172.16.0.1")

	buf, err := ngap.BuildPathSwitchRequestAcknowledgeTransfer(7, addr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var transfer ngapType.PathSwitchRequestAcknowledgeTransfer
	if err := aper.UnmarshalWithParams(buf, &transfer, "valueExt"); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	tunnel := transfer.ULNGUUPTNLInformation.GTPTunnel
	teid := binary.BigEndian.Uint32(tunnel.GTPTEID.Value)

	if teid != 7 {
		t.Errorf("TEID: got %d, want 7", teid)
	}

	bs := tunnel.TransportLayerAddress.Value
	if bs.BitLength != 32 {
		t.Fatalf("BitLength: got %d, want 32", bs.BitLength)
	}

	var ip [4]byte
	copy(ip[:], bs.Bytes)

	if ip != [4]byte{172, 16, 0, 1} {
		t.Errorf("IP: got %v, want [172 16 0 1]", ip)
	}
}
