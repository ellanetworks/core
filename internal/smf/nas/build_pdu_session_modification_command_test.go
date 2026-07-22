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

func decodeModCmd(t *testing.T, b []byte) *fgs.PDUSessionModificationCommand {
	t.Helper()

	m, err := fgs.ParsePDUSessionModificationCommand(b)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	return m
}

func TestBuildPDUSessionModificationCommand_AmbrAndQoS(t *testing.T) {
	ambr := &models.Ambr{Uplink: "200 Mbps", Downlink: "200 Mbps"}
	qos := &models.QosData{QFI: 1, Var5qi: 8, Arp: &models.Arp{PriorityLevel: 14}}

	encoded, err := smfNas.BuildPDUSessionModificationCommand(1, ambr, qos, nil)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	m := decodeModCmd(t, encoded)
	if m.SessionAMBR == nil {
		t.Fatal("SessionAMBR missing")
	}

	// 200 Mbps encodes as unit 1 Mbps (0x06), value 200.
	if m.SessionAMBR.UplinkUnit != fgs.SessionAMBRUnit1Mbps || m.SessionAMBR.Uplink != 200 {
		t.Errorf("uplink AMBR = %d unit %d", m.SessionAMBR.Uplink, m.SessionAMBR.UplinkUnit)
	}

	if m.SessionAMBR.DownlinkUnit != fgs.SessionAMBRUnit1Mbps || m.SessionAMBR.Downlink != 200 {
		t.Errorf("downlink AMBR = %d unit %d", m.SessionAMBR.Downlink, m.SessionAMBR.DownlinkUnit)
	}

	if m.QoSFlowDescriptions == nil {
		t.Fatal("QoS flow descriptions missing")
	}
}

func TestBuildPDUSessionModificationCommand_AmbrOnly(t *testing.T) {
	ambr := &models.Ambr{Uplink: "300 Mbps", Downlink: "400 Mbps"}

	m := decodeModCmd(t, mustBuildModCmd(t, 5, ambr, nil, nil))
	if m.SessionAMBR == nil {
		t.Fatal("SessionAMBR missing")
	}

	if m.QoSFlowDescriptions != nil {
		t.Fatal("QoS flow descriptions should be absent for AMBR-only")
	}
}

func TestBuildPDUSessionModificationCommand_QoSOnly(t *testing.T) {
	qos := &models.QosData{QFI: 1, Var5qi: 7, Arp: &models.Arp{PriorityLevel: 10}}

	m := decodeModCmd(t, mustBuildModCmd(t, 3, nil, qos, nil))
	if m.SessionAMBR != nil {
		t.Fatal("SessionAMBR should be absent for QoS-only")
	}

	if m.QoSFlowDescriptions == nil {
		t.Fatal("QoS flow descriptions missing")
	}
}

func TestBuildPDUSessionModificationCommand_WithDNS(t *testing.T) {
	for _, dns := range []net.IP{net.ParseIP("8.8.8.8"), net.ParseIP("2001:4860:4860::8888")} {
		m := decodeModCmd(t, mustBuildModCmd(t, 2, nil, nil, dns))
		if m.ExtendedPCO == nil {
			t.Errorf("EPCO missing for DNS %s", dns)
		}
	}
}

func TestBuildPDUSessionModificationCommand_AllNil(t *testing.T) {
	if _, err := smfNas.BuildPDUSessionModificationCommand(1, nil, nil, nil); err == nil {
		t.Fatal("expected error when all inputs are nil")
	}
}

func mustBuildModCmd(t *testing.T, psi uint8, ambr *models.Ambr, qos *models.QosData, dns net.IP) []byte {
	t.Helper()

	b, err := smfNas.BuildPDUSessionModificationCommand(psi, ambr, qos, dns)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	return b
}
