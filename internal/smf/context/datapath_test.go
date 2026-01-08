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
	defQER := &context.QER{}
	defURR := &context.URR{}

	dp := &context.DataPath{
		UpLinkTunnel: &context.GTPTunnel{
			PDR: &context.PDR{
				FAR: &context.FAR{},
			},
		},
	}

	ip := net.IPv4(192, 168, 1, 1)

	dp.ActivateUpLinkPdr("internet", ip, defQER, defURR)

	if dp.UpLinkTunnel.PDR == nil {
		t.Fatalf("expected pdr to be not nil")
	}

	if dp.UpLinkTunnel.PDR.PDI.SourceInterface.InterfaceValue != context.SourceInterfaceAccess {
		t.Errorf("expected SourceInterface to be %v, got %v", context.SourceInterfaceAccess, dp.UpLinkTunnel.PDR.PDI.SourceInterface.InterfaceValue)
	}

	if dp.UpLinkTunnel.PDR.PDI.LocalFTeID == nil {
		t.Errorf("expected pdr.PDI.LocalFTeID to be not nil")
	}

	if !dp.UpLinkTunnel.PDR.PDI.LocalFTeID.Ch {
		t.Errorf("expected pdr.PDI.LocalFTeID.Ch to be true")
	}

	if dp.UpLinkTunnel.PDR.PDI.UEIPAddress == nil {
		t.Errorf("expected pdr.PDI.UEIPAddress to be not nil")
	}

	if !dp.UpLinkTunnel.PDR.PDI.UEIPAddress.V4 {
		t.Errorf("expected pdr.PDI.UEIPAddress.V4 to be true")
	}

	if !dp.UpLinkTunnel.PDR.PDI.UEIPAddress.IPv4Address.Equal(net.IP{192, 168, 1, 1}) {
		t.Errorf("expected pdr.PDI.UEIPAddress.IPv4Address to be %v, got %v", net.IP{192, 168, 1, 1}, dp.UpLinkTunnel.PDR.PDI.UEIPAddress.IPv4Address)
	}

	if dp.UpLinkTunnel.PDR.PDI.NetworkInstance != "internet" {
		t.Errorf("expected pdr.PDI.NetworkInstance to be 'internet', got %v", dp.UpLinkTunnel.PDR.PDI.NetworkInstance)
	}
}

func TestActivateDlLinkPdr(t *testing.T) {
	defQER := &context.QER{}
	defURR := &context.URR{}

	dp := &context.DataPath{
		DownLinkTunnel: &context.GTPTunnel{
			PDR: &context.PDR{
				FAR: &context.FAR{},
				URR: &context.URR{},
			},
		},
	}

	ip := net.IPv4(192, 168, 1, 1)

	dp.ActivateDlLinkPdr("internet", net.IP{10, 0, 0, 1}, 12345, ip, defQER, defURR)

	if dp.DownLinkTunnel.PDR == nil {
		t.Fatalf("expected pdr to be not nil")
	}

	if dp.DownLinkTunnel.PDR.PDI.SourceInterface.InterfaceValue != context.SourceInterfaceCore {
		t.Errorf("expected SourceInterface to be %v, got %v", context.SourceInterfaceCore, dp.DownLinkTunnel.PDR.PDI.SourceInterface.InterfaceValue)
	}

	if dp.DownLinkTunnel.PDR.PDI.UEIPAddress == nil {
		t.Errorf("expected pdr.PDI.UEIPAddress to be not nil")
	}

	if !dp.DownLinkTunnel.PDR.PDI.UEIPAddress.V4 {
		t.Errorf("expected pdr.PDI.UEIPAddress.V4 to be true")
	}

	if !dp.DownLinkTunnel.PDR.PDI.UEIPAddress.IPv4Address.Equal(net.IP{192, 168, 1, 1}) {
		t.Errorf("expected pdr.PDI.UEIPAddress.IPv4Address to be %v, got %v", net.IP{192, 168, 1, 1}, dp.DownLinkTunnel.PDR.PDI.UEIPAddress.IPv4Address)
	}
}
