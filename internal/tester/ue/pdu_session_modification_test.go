// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func TestBuildPDUSessionModificationRequestCarriesThe5GSMParameters(t *testing.T) {
	pdu, err := BuildPDUSessionModificationRequest(&PDUSessionModificationRequestOpts{
		PDUSessionID:      3,
		PTI:               9,
		ReflectiveQoS:     true,
		MultiHomedIPv6:    true,
		MaxPacketFilters:  64,
		IntegrityMaxRate:  true,
		AlwaysOnRequested: true,
		RequestDNSServer:  true,
	})
	if err != nil {
		t.Fatalf("BuildPDUSessionModificationRequest: %v", err)
	}

	req, err := fgs.ParsePDUSessionModificationRequest(pdu)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if uint8(req.PDUSessionID) != 3 || uint8(req.PTI) != 9 {
		t.Errorf("header = %d/%d, want 3/9", req.PDUSessionID, req.PTI)
	}

	if req.GSMCapability == nil || !req.GSMCapability.RqoS || !req.GSMCapability.MH6PDU {
		t.Errorf("5GSM capability = %+v, want reflective QoS and multi-homed IPv6", req.GSMCapability)
	}

	if req.MaxPacketFilters == nil || *req.MaxPacketFilters != 64 {
		t.Errorf("maximum supported packet filters = %v, want 64", req.MaxPacketFilters)
	}

	if req.IntegrityProtMaxDataRate == nil {
		t.Error("no integrity protection maximum data rate")
	}

	if req.AlwaysOnRequested == nil || !*req.AlwaysOnRequested {
		t.Error("the always-on request was dropped")
	}

	if req.ExtendedPCO == nil {
		t.Fatal("no extended protocol configuration options carrying the DNS server request")
	}

	var sawV4 bool

	for _, id := range req.ExtendedPCO.ContainerIDs() {
		if id == nas.PCOContainerDNSServerIPv4Address {
			sawV4 = true
		}
	}

	if !sawV4 {
		t.Errorf("no DNS server IPv4 address request: %+v", req.ExtendedPCO.Containers)
	}

	if len(req.RequestedQoSFlows) != 0 {
		t.Errorf("requested QoS flows = %+v, want none for a capability indication", req.RequestedQoSFlows)
	}
}

func TestBuildPDUSessionModificationRequestRejectsUnusablePTI(t *testing.T) {
	for _, pti := range []uint8{0, 0xff} {
		if _, err := BuildPDUSessionModificationRequest(&PDUSessionModificationRequestOpts{
			PDUSessionID: 1,
			PTI:          pti,
		}); err == nil {
			t.Errorf("PTI %d was accepted, want an error (TS 24.501 §7.3.1 c, d)", pti)
		}
	}
}

func TestBuildPDUSessionModificationCompleteEchoesThePTI(t *testing.T) {
	pdu, err := BuildPDUSessionModificationComplete(&PDUSessionModificationCompleteOpts{
		PDUSessionID: 4,
		PTI:          11,
	})
	if err != nil {
		t.Fatalf("BuildPDUSessionModificationComplete: %v", err)
	}

	complete, err := fgs.ParsePDUSessionModificationComplete(pdu)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if uint8(complete.PDUSessionID) != 4 || uint8(complete.PTI) != 11 {
		t.Errorf("header = %d/%d, want 4/11", complete.PDUSessionID, complete.PTI)
	}
}
