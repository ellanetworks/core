// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

type resolvedNetworkInstance struct {
	name          string
	attachName    string
	interfaceName string
	datapath      ebpf.NetworkInstance
}

var interfaceByName = net.InterfaceByName

func resolveNetworkInstances(instances []config.NetworkInstance, n3Ifindex, n6Ifindex int) ([]resolvedNetworkInstance, []datapathIface, error) {
	if len(instances) >= ebpf.MaxNetworkInstances {
		return nil, nil, fmt.Errorf("at most %d network instances besides interfaces.n6 are supported, %d are configured", ebpf.MaxNetworkInstances-1, len(instances))
	}

	attached := map[int]bool{n3Ifindex: true, n6Ifindex: true}
	resolved := make([]resolvedNetworkInstance, 0, len(instances))

	var extra []datapathIface

	for i, ni := range instances {
		attachName := ni.N6.Name

		var vlan uint32

		if ni.N6.VlanConfig != nil {
			attachName = ni.N6.VlanConfig.MasterInterface
			vlan = uint32(ni.N6.VlanConfig.VlanId)
		}

		attachIface, err := interfaceByName(attachName)
		if err != nil {
			return nil, nil, fmt.Errorf("network instance %s: lookup interface %s: %w", ni.Name, attachName, err)
		}

		routingIface, err := interfaceByName(ni.N6.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("network instance %s: lookup interface %s: %w", ni.Name, ni.N6.Name, err)
		}

		if !attached[attachIface.Index] {
			attached[attachIface.Index] = true
			extra = append(extra, datapathIface{index: attachIface.Index, name: attachName})
		}

		resolved = append(resolved, resolvedNetworkInstance{
			name:          ni.Name,
			attachName:    attachName,
			interfaceName: ni.N6.Name,
			datapath: ebpf.NetworkInstance{
				ID:             uint32(i + 1),
				Ifindex:        uint32(attachIface.Index),
				RoutingIfindex: uint32(routingIface.Index),
				Vlan:           vlan,
			},
		})
	}

	return resolved, extra, nil
}
