// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package ebpf

import (
	"errors"

	"github.com/cilium/ebpf"
)

const (
	MapNatCt             = "nat_ct"
	MapFlowStats         = "flow_stats"
	MapPdrsUplink        = "pdrs_uplink"
	MapPdrsDownlinkIP4   = "pdrs_downlink_ip4"
	MapPdrsDownlinkIP6   = "pdrs_downlink_ip6"
	MapFramedDownlinkIP4 = "framed_downlink_ip4"
	MapFramedDownlinkIP6 = "framed_downlink_ip6"
	MapURR               = "urr_map"
)

var TrackedMaps = []string{
	MapNatCt,
	MapFlowStats,
	MapPdrsUplink,
	MapPdrsDownlinkIP4,
	MapPdrsDownlinkIP6,
	MapFramedDownlinkIP4,
	MapFramedDownlinkIP6,
	MapURR,
}

func (bpfObjects *BpfObjects) mapByName(name string) *ebpf.Map {
	switch name {
	case MapNatCt:
		return bpfObjects.NatCt
	case MapFlowStats:
		return bpfObjects.FlowStats
	case MapPdrsUplink:
		return bpfObjects.PdrsUplink
	case MapPdrsDownlinkIP4:
		return bpfObjects.PdrsDownlinkIp4
	case MapPdrsDownlinkIP6:
		return bpfObjects.PdrsDownlinkIp6
	case MapFramedDownlinkIP4:
		return bpfObjects.FramedDownlinkIp4
	case MapFramedDownlinkIP6:
		return bpfObjects.FramedDownlinkIp6
	case MapURR:
		return bpfObjects.UrrMap
	}

	return nil
}

func (bpfObjects *BpfObjects) SetOccupancy(name string, entries int) {
	bpfObjects.occupancyMu.Lock()
	defer bpfObjects.occupancyMu.Unlock()

	bpfObjects.occupancy[name] = entries
}

func (bpfObjects *BpfObjects) addOccupancy(name string, delta int) {
	bpfObjects.occupancyMu.Lock()
	defer bpfObjects.occupancyMu.Unlock()

	n := bpfObjects.occupancy[name] + delta
	if n < 0 {
		n = 0
	}

	bpfObjects.occupancy[name] = n
}

func (bpfObjects *BpfObjects) MapPressure() map[string]float64 {
	if bpfObjects == nil {
		return nil
	}

	bpfObjects.occupancyMu.Lock()
	occupancy := make(map[string]int, len(bpfObjects.occupancy))

	for name, n := range bpfObjects.occupancy {
		occupancy[name] = n
	}
	bpfObjects.occupancyMu.Unlock()

	out := make(map[string]float64, len(TrackedMaps))

	for _, name := range TrackedMaps {
		m := bpfObjects.mapByName(name)
		if m == nil {
			continue
		}

		maxEntries := m.MaxEntries()
		if maxEntries == 0 {
			continue
		}

		ratio := float64(occupancy[name]) / float64(maxEntries)
		if ratio > 1 {
			ratio = 1
		}

		out[name] = ratio
	}

	return out
}

func (bpfObjects *BpfObjects) putTracked(m *ebpf.Map, name string, key, value any) error {
	err := m.Update(key, value, ebpf.UpdateNoExist)
	if err == nil {
		bpfObjects.addOccupancy(name, 1)
		return nil
	}

	if !errors.Is(err, ebpf.ErrKeyExist) {
		return err
	}

	return m.Update(key, value, ebpf.UpdateAny)
}

func (bpfObjects *BpfObjects) deleteTracked(m *ebpf.Map, name string, key any) error {
	if err := m.Delete(key); err != nil {
		return err
	}

	bpfObjects.addOccupancy(name, -1)

	return nil
}
