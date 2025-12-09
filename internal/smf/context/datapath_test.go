// Copyright 2024 Ella Networks
// Copyright 2024 Canonical Ltd.
// SPDX-License-Identifier: Apache-2.0

package context_test

import (
	"net"
	"testing"

	"github.com/ellanetworks/core/internal/smf/context"
)

func TestActivateUpLinkPdr(t *testing.T) {
	smContext := &context.SMContext{
		PDUAddress: net.IPv4(192, 168, 1, 1),
		Dnn:        "internet",
	}

	defQER := &context.QER{}
	defURR := &context.URR{}

	node := &context.DataPathNode{
		UPF: &context.UPF{},
		UpLinkTunnel: &context.GTPTunnel{
			PDR: &context.PDR{
				Precedence: 0,
				FAR:        &context.FAR{},
			},
		},
	}

	err := node.ActivateUpLinkPdr(smContext, defQER, defURR, 10)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if node.UpLinkTunnel.PDR == nil {
		t.Fatalf("expected pdr to be not nil")
	}

	if node.UpLinkTunnel.PDR.PDI.SourceInterface.InterfaceValue != context.SourceInterfaceAccess {
		t.Errorf("expected SourceInterface to be %v, got %v", context.SourceInterfaceAccess, node.UpLinkTunnel.PDR.PDI.SourceInterface.InterfaceValue)
	}
	if node.UpLinkTunnel.PDR.PDI.LocalFTeID == nil {
		t.Errorf("expected pdr.PDI.LocalFTeID to be not nil")
	}
	if !node.UpLinkTunnel.PDR.PDI.LocalFTeID.Ch {
		t.Errorf("expected pdr.PDI.LocalFTeID.Ch to be true")
	}
	if node.UpLinkTunnel.PDR.PDI.UEIPAddress == nil {
		t.Errorf("expected pdr.PDI.UEIPAddress to be not nil")
	}
	if !node.UpLinkTunnel.PDR.PDI.UEIPAddress.V4 {
		t.Errorf("expected pdr.PDI.UEIPAddress.V4 to be true")
	}
	if !node.UpLinkTunnel.PDR.PDI.UEIPAddress.IPv4Address.Equal(net.IP{192, 168, 1, 1}) {
		t.Errorf("expected pdr.PDI.UEIPAddress.IPv4Address to be %v, got %v", net.IP{192, 168, 1, 1}, node.UpLinkTunnel.PDR.PDI.UEIPAddress.IPv4Address)
	}
	if node.UpLinkTunnel.PDR.PDI.NetworkInstance != "internet" {
		t.Errorf("expected pdr.PDI.NetworkInstance to be 'internet', got %v", node.UpLinkTunnel.PDR.PDI.NetworkInstance)
	}
}

func TestActivateDlLinkPdr(t *testing.T) {
	smContext := &context.SMContext{
		PDUAddress: net.IP{192, 168, 1, 1},
		Dnn:        "internet",
		Tunnel: &context.UPTunnel{
			ANInformation: struct {
				IPAddress net.IP
				TEID      uint32
			}{
				IPAddress: net.IP{10, 0, 0, 1},
				TEID:      12345,
			},
		},
	}

	defQER := &context.QER{}
	defURR := &context.URR{}

	node := &context.DataPathNode{
		UPF: &context.UPF{},
		DownLinkTunnel: &context.GTPTunnel{
			PDR: &context.PDR{
				Precedence: 0,
				FAR:        &context.FAR{},
				URR:        &context.URR{},
			},
		},
	}

	dataPath := &context.DataPath{
		DPNode: node,
	}

	err := node.ActivateDlLinkPdr(smContext, defQER, defURR, 10, dataPath)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if node.DownLinkTunnel.PDR == nil {
		t.Fatalf("expected pdr to be not nil")
	}

	if node.DownLinkTunnel.PDR.PDI.SourceInterface.InterfaceValue != context.SourceInterfaceCore {
		t.Errorf("expected SourceInterface to be %v, got %v", context.SourceInterfaceCore, node.DownLinkTunnel.PDR.PDI.SourceInterface.InterfaceValue)
	}
	if node.DownLinkTunnel.PDR.PDI.UEIPAddress == nil {
		t.Errorf("expected pdr.PDI.UEIPAddress to be not nil")
	}
	if !node.DownLinkTunnel.PDR.PDI.UEIPAddress.V4 {
		t.Errorf("expected pdr.PDI.UEIPAddress.V4 to be true")
	}
	if !node.DownLinkTunnel.PDR.PDI.UEIPAddress.IPv4Address.Equal(net.IP{192, 168, 1, 1}) {
		t.Errorf("expected pdr.PDI.UEIPAddress.IPv4Address to be %v, got %v", net.IP{192, 168, 1, 1}, node.DownLinkTunnel.PDR.PDI.UEIPAddress.IPv4Address)
	}
}
