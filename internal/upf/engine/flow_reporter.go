// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ellanetworks/core/internal/models"
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

func addrFromIn6(addr ebpf.N3N6EntrypointIn6Addr) netip.Addr {
	b := addr.In6U.U6Addr8
	// Check for IPv4-mapped IPv6 (::ffff:0.0.0.0/96)
	// bytes 0-9 must be zero, bytes 10-11 must be 0xFF, 0xFF
	if b[0] == 0 && b[1] == 0 && b[2] == 0 && b[3] == 0 && b[4] == 0 &&
		b[5] == 0 && b[6] == 0 && b[7] == 0 && b[8] == 0 && b[9] == 0 &&
		b[10] == 0xff && b[11] == 0xff {
		var b4 [4]byte
		copy(b4[:], b[12:16])

		return netip.AddrFrom4(b4)
	}

	var b16 [16]byte
	copy(b16[:], b[:])

	return netip.AddrFrom16(b16)
}

func u16NtoHS(n uint16) uint16 {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, n)

	return binary.NativeEndian.Uint16(b)
}

// BuildFlowReportRequest converts an eBPF flow record to a FlowReportRequest
// without sending it. Used by the batch reporting path.
func BuildFlowReportRequest(flow ebpf.N3N6EntrypointFlow, stats ebpf.N3N6EntrypointFlowStats) *models.FlowReportRequest {
	saddr := addrFromIn6(flow.Saddr)
	daddr := addrFromIn6(flow.Daddr)
	sport := u16NtoHS(flow.Sport)
	dport := u16NtoHS(flow.Dport)
	startTime := bootTime.Add(time.Duration(stats.FirstTs))
	endTime := bootTime.Add(time.Duration(stats.LastTs))

	// Determine direction: ingress on N3 means the UE originated the traffic (uplink)
	direction := models.DirectionDownlink
	if flow.IngressIfindex == n3IfIndex.Load() {
		direction = models.DirectionUplink
	}

	return &models.FlowReportRequest{
		IMSI:            fmt.Sprintf("%015d", flow.Imsi),
		SourceIP:        saddr.String(),
		DestinationIP:   daddr.String(),
		SourcePort:      sport,
		DestinationPort: dport,
		Protocol:        flow.Proto,
		Packets:         stats.Packets,
		Bytes:           stats.Bytes,
		StartTime:       startTime.UTC().Format(time.RFC3339Nano),
		EndTime:         endTime.UTC().Format(time.RFC3339Nano),
		Direction:       direction,
		Action:          models.Action(flow.Action),
	}
}
