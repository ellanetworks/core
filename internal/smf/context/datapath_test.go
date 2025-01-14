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
		PDUAddress: &context.UeIPAddr{
			IP: net.IPv4(192, 168, 1, 1),
		},
		Dnn: "internet",
	}

	defQER := &context.QER{}

	dpNode := &context.DataPathNode{
		UPF: &context.UPF{},
		UpLinkTunnel: &context.GTPTunnel{
			PDR: map[string]*context.PDR{
				"default": {
					Precedence: 0,
					FAR:        &context.FAR{},
				},
			},
		},
	}

	err := dpNode.ActivateUpLinkPdr(smContext, defQER, 10)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	pdr := dpNode.UpLinkTunnel.PDR["default"]
	if pdr == nil {
		t.Fatalf("expected pdr to be not nil")
	}

	if pdr.PDI.SourceInterface.InterfaceValue != context.SourceInterfaceAccess {
		t.Errorf("expected SourceInterface to be %v, got %v", context.SourceInterfaceAccess, pdr.PDI.SourceInterface.InterfaceValue)
	}
	if pdr.PDI.LocalFTeid == nil {
		t.Errorf("expected pdr.PDI.LocalFTeid to be not nil")
	}
	if !pdr.PDI.LocalFTeid.Ch {
		t.Errorf("expected pdr.PDI.LocalFTeid.Ch to be true")
	}
	if pdr.PDI.UEIPAddress == nil {
		t.Errorf("expected pdr.PDI.UEIPAddress to be not nil")
	}
	if !pdr.PDI.UEIPAddress.V4 {
		t.Errorf("expected pdr.PDI.UEIPAddress.V4 to be true")
	}
	if !pdr.PDI.UEIPAddress.Ipv4Address.Equal(net.IP{192, 168, 1, 1}) {
		t.Errorf("expected pdr.PDI.UEIPAddress.Ipv4Address to be %v, got %v", net.IP{192, 168, 1, 1}, pdr.PDI.UEIPAddress.Ipv4Address)
	}
	if string(pdr.PDI.NetworkInstance) != "internet" {
		t.Errorf("expected pdr.PDI.NetworkInstance to be 'internet', got %v", string(pdr.PDI.NetworkInstance))
	}
}

func TestActivateDlLinkPdr(t *testing.T) {
	smContext := &context.SMContext{
		PDUAddress: &context.UeIPAddr{
			IP: net.IP{192, 168, 1, 1},
		},
		Dnn: "internet",
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

	dpNode := &context.DataPathNode{
		UPF: &context.UPF{},
		DownLinkTunnel: &context.GTPTunnel{
			PDR: map[string]*context.PDR{
				"default": {
					Precedence: 0,
					FAR:        &context.FAR{},
				},
			},
		},
	}

	dataPath := &context.DataPath{
		FirstDPNode: dpNode,
	}

	err := dpNode.ActivateDlLinkPdr(smContext, defQER, 10, dataPath)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	pdr := dpNode.DownLinkTunnel.PDR["default"]
	if pdr == nil {
		t.Fatalf("expected pdr to be not nil")
	}

	if pdr.PDI.SourceInterface.InterfaceValue != context.SourceInterfaceCore {
		t.Errorf("expected SourceInterface to be %v, got %v", context.SourceInterfaceCore, pdr.PDI.SourceInterface.InterfaceValue)
	}
	if pdr.PDI.UEIPAddress == nil {
		t.Errorf("expected pdr.PDI.UEIPAddress to be not nil")
	}
	if !pdr.PDI.UEIPAddress.V4 {
		t.Errorf("expected pdr.PDI.UEIPAddress.V4 to be true")
	}
	if !pdr.PDI.UEIPAddress.Ipv4Address.Equal(net.IP{192, 168, 1, 1}) {
		t.Errorf("expected pdr.PDI.UEIPAddress.Ipv4Address to be %v, got %v", net.IP{192, 168, 1, 1}, pdr.PDI.UEIPAddress.Ipv4Address)
	}
}
