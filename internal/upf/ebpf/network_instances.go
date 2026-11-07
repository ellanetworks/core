// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ebpf

import (
	"fmt"
)

const MaxNetworkInstances = 16

type NetworkInstance struct {
	ID             uint32
	Ifindex        uint32
	RoutingIfindex uint32
	Vlan           uint32
}

func (bpfObjects *BpfObjects) PutNetworkInstance(ni NetworkInstance) error {
	if ni.ID == 0 || ni.ID >= MaxNetworkInstances {
		return fmt.Errorf("network instance id %d is outside 1-%d", ni.ID, MaxNetworkInstances-1)
	}

	value := N3N6EntrypointNetworkInstance{Ifindex: ni.Ifindex, RoutingIfindex: ni.RoutingIfindex, Vlan: ni.Vlan}
	if err := bpfObjects.NetworkInstances.Put(ni.ID, value); err != nil {
		return fmt.Errorf("put network instance %d: %w", ni.ID, err)
	}

	key := N3N6EntrypointNetworkInstanceKey{Ifindex: ni.Ifindex, Vlan: ni.Vlan}
	if err := bpfObjects.NetworkInstanceClassifier.Put(key, ni.ID); err != nil {
		return fmt.Errorf("classify network instance %d: %w", ni.ID, err)
	}

	return nil
}
