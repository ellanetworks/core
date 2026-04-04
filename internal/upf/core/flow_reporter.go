// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/pfcp_dispatcher"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

var bootTime = mustGetBootTime()

// n3IfIndex stores the N3 interface index, used to determine flow direction.
// Set once during UPF startup via SetN3InterfaceIndex.
var n3IfIndex atomic.Uint32

// SetN3InterfaceIndex records the N3 (radio-side) network interface index so that
// flow direction can be derived: ingress on N3 means uplink, otherwise downlink.
func SetN3InterfaceIndex(idx int) {
	n3IfIndex.Store(uint32(idx))
}

func mustGetBootTime() time.Time {
	var info syscall.Sysinfo_t
	if err := syscall.Sysinfo(&info); err != nil {
		panic(err)
	}

	bootTime := time.Now().Add(-time.Duration(info.Uptime) * time.Second)

	return bootTime
}

func int2ip(nn uint32) net.IP {
	ip := make(net.IP, 4)
	binary.NativeEndian.PutUint32(ip, nn)

	return ip
}

func u16NtoHS(n uint16) uint16 {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, n)

	return binary.NativeEndian.Uint16(b)
}

// BuildFlowReportRequest converts an eBPF flow record to a FlowReportRequest
// without sending it. Used by the batch reporting path.
func BuildFlowReportRequest(flow ebpf.N3N6EntrypointFlow, stats ebpf.N3N6EntrypointFlowStats) *pfcp_dispatcher.FlowReportRequest {
	saddr := int2ip(flow.Saddr)
	daddr := int2ip(flow.Daddr)
	sport := u16NtoHS(flow.Sport)
	dport := u16NtoHS(flow.Dport)
	startTime := bootTime.Add(time.Duration(stats.FirstTs))
	endTime := bootTime.Add(time.Duration(stats.LastTs))

	// Determine direction: ingress on N3 means the UE originated the traffic (uplink)
	direction := models.DirectionDownlink
	if flow.IngressIfindex == n3IfIndex.Load() {
		direction = models.DirectionUplink
	}

	return &pfcp_dispatcher.FlowReportRequest{
		IMSI:            fmt.Sprintf("%015d", flow.Imsi),
		SourceIP:        saddr.String(),
		DestinationIP:   daddr.String(),
		SourcePort:      sport,
		DestinationPort: dport,
		Protocol:        flow.Proto,
		Packets:         stats.Packets,
		Bytes:           stats.Bytes,
		StartTime:       startTime.UTC().Format(time.RFC3339),
		EndTime:         endTime.UTC().Format(time.RFC3339),
		Direction:       direction,
		Action:          models.Action(flow.Action),
	}
}
