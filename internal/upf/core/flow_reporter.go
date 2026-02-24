// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pfcp_dispatcher"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.uber.org/zap"
)

var bootTime = mustGetBootTime()

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

// SendFlowReport converts an eBPF flow record to a FlowReportRequest and sends it to the SMF dispatcher.
// The function is synchronous; callers should invoke it asynchronously if non-blocking behavior is desired.
// Errors are logged but do not propagate to the caller.
func SendFlowReport(ctx context.Context, flow ebpf.N3N6EntrypointFlow, stats ebpf.N3N6EntrypointFlowStats) {
	saddr := int2ip(flow.Saddr)
	daddr := int2ip(flow.Daddr)
	sport := u16NtoHS(flow.Sport)
	dport := u16NtoHS(flow.Dport)
	proto := flow.Proto
	startTime := bootTime.Add(time.Duration(stats.FirstTs))
	endTime := bootTime.Add(time.Duration(stats.LastTs))
	imsiStr := fmt.Sprintf("%015d", flow.Imsi)

	// Create flow report request
	flowReportReq := &pfcp_dispatcher.FlowReportRequest{
		IMSI:            imsiStr,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		SourceIP:        saddr.String(),
		DestinationIP:   daddr.String(),
		SourcePort:      sport,
		DestinationPort: dport,
		Protocol:        proto,
		Packets:         stats.Packets,
		Bytes:           stats.Bytes,
		StartTime:       startTime.UTC().Format(time.RFC3339),
		EndTime:         endTime.UTC().Format(time.RFC3339),
	}

	// Send to SMF via dispatcher
	err := pfcp_dispatcher.Dispatcher.SMF.SendFlowReport(ctx, flowReportReq)
	if err != nil {
		logger.UpfLog.Error(
			"Failed to send flow report to SMF",
			zap.String("imsi", imsiStr),
			zap.String("source_ip", flowReportReq.SourceIP),
			zap.String("destination_ip", flowReportReq.DestinationIP),
			zap.Error(err),
		)

		return
	}

	logger.UpfLog.Debug(
		"Flow expired and sent to SMF",
		zap.String("IMSI", imsiStr),
		zap.String("Source", saddr.String()),
		zap.String("Destination", daddr.String()),
		zap.Uint16("Source Port", sport),
		zap.Uint16("Destination Port", dport),
		zap.Uint8("Protocol", proto),
		zap.Uint64("Packets", stats.Packets),
		zap.Uint64("Bytes", stats.Bytes),
		zap.Time("Start", startTime),
		zap.Time("End", endTime),
	)
}
