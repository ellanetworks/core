// Copyright 2024 Ella Networks

package upf

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"runtime"
	"sync"
	"time"

	bpf "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/core"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

const (
	PfcpAddress         = "0.0.0.0"
	PfcpNodeID          = "0.0.0.0"
	FTEIDPool           = 65535
	ConnTrackTimeout    = 10 * time.Minute
	InactiveFlowTimeout = 30 * time.Second
	ActiveFlowTimeout   = 30 * time.Minute
	maxInFlightFlows    = 2000
	flowReportTimeout   = 5 * time.Second
)

var bpfObjects *ebpf.BpfObjects

type flowReport struct {
	flow  ebpf.N3N6EntrypointFlow
	stats ebpf.N3N6EntrypointFlowStats
}

type UPF struct {
	n3Link             link.Link
	n6Link             *link.Link
	pfcpConn           *core.PfcpConnection
	notificationReader *ringbuf.Reader
	noNeighReader      *ringbuf.Reader

	ctx      context.Context
	gcCancel context.CancelFunc

	// fcMu serialises concurrent calls to startFlowCollection / stopFlowCollection
	// (e.g. from ReloadFlowAccounting and Close running in different goroutines).
	fcMu       sync.Mutex
	fcCancel   context.CancelFunc
	fcScanDone chan struct{} // closed when collectExpiredFlows exits
	fcDone     chan struct{} // closed when reportFlows exits (all flows reported)
}

func Start(ctx context.Context, n3Interface config.N3Interface, n3Address string, advertisedN3Address string, n6Interface config.N6Interface, xdpAttachMode string, masquerade bool, flowact bool) (*UPF, error) {
	var (
		n3Vlan uint32
		n6Vlan uint32
	)

	if err := ebpf.IncreaseResourceLimits(); err != nil {
		logger.UpfLog.Fatal("Can't increase resource limits", zap.Error(err))
	}

	n3AttachmentInterface := n3Interface.Name
	if n3Interface.VlanConfig != nil {
		n3AttachmentInterface = n3Interface.VlanConfig.MasterInterface
		n3Vlan = uint32(n3Interface.VlanConfig.VlanId)
	}

	n3Iface, err := net.InterfaceByName(n3AttachmentInterface)
	if err != nil {
		logger.UpfLog.Fatal("Lookup network iface", zap.String("iface", n3AttachmentInterface), zap.Error(err))
		return nil, err
	}

	n6AttachmentInterface := n6Interface.Name
	if n6Interface.VlanConfig != nil {
		n6AttachmentInterface = n6Interface.VlanConfig.MasterInterface
		n6Vlan = uint32(n6Interface.VlanConfig.VlanId)
	}

	n6Iface, err := net.InterfaceByName(n6AttachmentInterface)
	if err != nil {
		logger.UpfLog.Fatal("Lookup network iface", zap.String("iface", n6AttachmentInterface), zap.Error(err))
		return nil, err
	}

	bpfObjects = ebpf.NewBpfObjects(flowact, masquerade, n3Iface.Index, n6Iface.Index, n3Vlan, n6Vlan)

	if err := bpfObjects.Load(); err != nil {
		logger.UpfLog.Fatal("Loading bpf objects failed", zap.Error(err))
		return nil, err
	}

	n3Link, err := link.AttachXDP(link.XDPOptions{
		Program:   bpfObjects.UpfN3N6EntrypointFunc,
		Interface: n3Iface.Index,
		Flags:     StringToXDPAttachMode(xdpAttachMode),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach eBPF program on n3 interface %q: %s", n3AttachmentInterface, err)
	}

	var n6Link *link.Link

	if n6Iface.Index != n3Iface.Index {
		n6, err := link.AttachXDP(link.XDPOptions{
			Program:   bpfObjects.UpfN3N6EntrypointFunc,
			Interface: n6Iface.Index,
			Flags:     StringToXDPAttachMode(xdpAttachMode),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to attach eBPF program on n6 interface %q: %s", n6AttachmentInterface, err)
		}

		n6Link = &n6
	}

	resourceManager, err := core.NewFteIDResourceManager(FTEIDPool)
	if err != nil {
		return nil, fmt.Errorf("failed to create Resource Manager: %w", err)
	}

	pfcpConn, err := core.CreatePfcpConnection(PfcpAddress, PfcpNodeID, n3Address, advertisedN3Address, bpfObjects, resourceManager)
	if err != nil {
		return nil, fmt.Errorf("failed to create PFCP connection: %w", err)
	}

	notificationReader, err := ringbuf.NewReader(bpfObjects.NocpMap)
	if err != nil {
		return nil, fmt.Errorf("coud not start traffic notification reader: %s", err.Error())
	}

	noNeighReader, err := ringbuf.NewReader(bpfObjects.NoNeighMap)
	if err != nil {
		return nil, fmt.Errorf("coud not start missing neighbour reader: %s", err.Error())
	}

	upf := &UPF{
		n3Link:             n3Link,
		n6Link:             n6Link,
		pfcpConn:           pfcpConn,
		notificationReader: notificationReader,
		noNeighReader:      noNeighReader,
		ctx:                ctx,
	}

	go upf.listenForTrafficNotifications()

	go upf.monitorUsage(30*time.Second, ctx.Done())

	go upf.listenForMissingNeighbours()

	if masquerade {
		upf.startGC(ctx)
	}

	if flowact {
		upf.startFlowCollection(ctx)
	}

	return upf, nil
}

func (u *UPF) Close() {
	u.stopGC()
	u.stopFlowCollection()

	if u.n6Link != nil {
		if err := (*u.n6Link).Close(); err != nil {
			logger.UpfLog.Warn("Failed to detach eBPF from n6", zap.Error(err))
		}
	}

	if err := u.n3Link.Close(); err != nil {
		logger.UpfLog.Warn("Failed to detach eBPF from n3", zap.Error(err))
	}

	if err := u.pfcpConn.BpfObjects.Close(); err != nil {
		logger.UpfLog.Warn("Failed to close BPF objects", zap.Error(err))
	}

	if err := u.notificationReader.Close(); err != nil {
		logger.UpfLog.Warn("Failed to close notification reader", zap.Error(err))
	}

	if err := u.noNeighReader.Close(); err != nil {
		logger.UpfLog.Warn("Failed to close missing neighbour reader", zap.Error(err))
	}

	logger.UpfLog.Info("UPF resources released")
}

func (u *UPF) UpdateAdvertisedN3Address(newN3Addr net.IP) {
	u.pfcpConn.SetAdvertisedN3Address(newN3Addr)
}

func (u *UPF) ReloadNAT(masquerade bool) error {
	u.pfcpConn.BpfObjects.Masquerade = masquerade

	err := u.pfcpConn.BpfObjects.LoadWithMapReplacements()
	if err != nil {
		return fmt.Errorf("couldn't load BPF objects: %w", err)
	}

	if err := u.n3Link.Update(u.pfcpConn.BpfObjects.UpfN3N6EntrypointFunc); err != nil {
		return err
	}

	if u.n6Link != nil {
		if err := (*u.n6Link).Update(u.pfcpConn.BpfObjects.UpfN3N6EntrypointFunc); err != nil {
			return err
		}
	}

	if masquerade {
		u.startGC(u.ctx)
	} else {
		u.stopGC()
	}

	return nil
}

func (u *UPF) ReloadFlowAccounting(flowact bool) error {
	u.pfcpConn.BpfObjects.FlowAccounting = flowact

	err := u.pfcpConn.BpfObjects.LoadWithMapReplacements()
	if err != nil {
		return fmt.Errorf("couldn't load BPF objects: %w", err)
	}

	if err := u.n3Link.Update(u.pfcpConn.BpfObjects.UpfN3N6EntrypointFunc); err != nil {
		return err
	}

	if u.n6Link != nil {
		if err := (*u.n6Link).Update(u.pfcpConn.BpfObjects.UpfN3N6EntrypointFunc); err != nil {
			return err
		}
	}

	if flowact {
		u.startFlowCollection(u.ctx)
	} else {
		u.stopFlowCollection()
	}

	return nil
}

func StringToXDPAttachMode(Mode string) link.XDPAttachFlags {
	switch Mode {
	case "generic":
		return link.XDPGenericMode
	case "native":
		return link.XDPDriverMode
	case "offload":
		return link.XDPOffloadMode
	default:
		return link.XDPGenericMode
	}
}

func (u *UPF) startGC(ctx context.Context) {
	if u.gcCancel != nil {
		return
	}

	cctx, cancel := context.WithCancel(ctx)

	u.gcCancel = cancel

	go u.collectCollectionTrackingGarbage(cctx)
}

func (u *UPF) stopGC() {
	if u.gcCancel != nil {
		u.gcCancel()
		u.gcCancel = nil
	}
}

func (u *UPF) startFlowCollection(ctx context.Context) {
	u.fcMu.Lock()
	defer u.fcMu.Unlock()

	if u.fcCancel != nil {
		return
	}

	cctx, cancel := context.WithCancel(ctx)

	u.fcCancel = cancel
	flows := make(chan flowReport, maxInFlightFlows)
	u.fcScanDone = make(chan struct{})
	u.fcDone = make(chan struct{})

	// Capture channels in local variables so the goroutine closures close the
	// channels allocated above, not whatever u.fcScanDone / u.fcDone point to
	// at the time the deferred close executes (which may have been reassigned
	// by a subsequent startFlowCollection call).
	scanDone := u.fcScanDone
	done := u.fcDone

	go func() {
		defer close(scanDone)

		u.collectExpiredFlows(cctx, flows)
	}()
	go func() {
		defer close(done)

		u.reportFlows(flows)
	}()
}

// stopFlowCollection cancels the flow-collection goroutines and waits for both
// to exit before returning.
//
// Sequencing:
//  1. cancel() unblocks collectExpiredFlows, which does a final scan, closes
//     flowch, then closes fcScanDone.
//  2. We wait on fcScanDone — at this point it is safe to close BPF objects,
//     because collectExpiredFlows will no longer touch the BPF map.
//  3. Closing flowch causes reportFlows to drain any remaining buffered entries
//     and exit, closing fcDone.
//  4. We wait on fcDone — all flows have been forwarded to the SMF.
//
// fcCancel is cleared while holding fcMu, before the wait. This eliminates the
// ABA window where a concurrent startFlowCollection would see a non-nil fcCancel
// (believing the pipeline is already running) while stopFlowCollection is
// actively tearing it down.
func (u *UPF) stopFlowCollection() {
	u.fcMu.Lock()

	if u.fcCancel == nil {
		u.fcMu.Unlock()
		return
	}

	cancel := u.fcCancel
	scanDone := u.fcScanDone
	done := u.fcDone
	u.fcCancel = nil // clear under the lock; startFlowCollection may now proceed
	u.fcMu.Unlock()

	cancel()
	<-scanDone // wait for producer: BPF map is no longer accessed after this

	select {
	case <-done: // wait for consumer: all buffered flows have been reported
	case <-time.After(30 * time.Second):
		logger.UpfLog.Warn("Flow reporter drain timed out; some flows may be lost")
	}
}

func (u *UPF) collectCollectionTrackingGarbage(ctx context.Context) {
	var (
		key     ebpf.N3N6EntrypointFiveTuple
		value   ebpf.N3N6EntrypointNatEntry
		sysInfo unix.Sysinfo_t
	)

	expiredKeys := make([]ebpf.N3N6EntrypointFiveTuple, 0)

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		err := unix.Sysinfo(&sysInfo)
		if err != nil {
			logger.UpfLog.Warn("Failed to query sysinfo", zap.Error(err))
			return
		}

		nsSinceBoot := sysInfo.Uptime * time.Second.Nanoseconds()
		expiryThreshold := nsSinceBoot - ConnTrackTimeout.Nanoseconds()

		ct_entries := u.pfcpConn.BpfObjects.NatCt.Iterate()
		for ct_entries.Next(&key, &value) {
			if value.RefreshTs < uint64(expiryThreshold) {
				expiredKeys = append(expiredKeys, key)
			}
		}

		if err := ct_entries.Err(); err != nil {
			logger.UpfLog.Debug("Error while iterating over conntrack entries", zap.Error(err))
		}

		count, err := u.pfcpConn.BpfObjects.NatCt.BatchDelete(expiredKeys, &bpf.BatchOptions{})
		if err != nil {
			logger.UpfLog.Warn("Failed to delete expired conntrack entries", zap.Error(err))
		}

		logger.UpfLog.Debug("Deleted expired conntrack entries", zap.Int("count", count))

		expiredKeys = expiredKeys[:0]
	}
}

func (u *UPF) listenForTrafficNotifications() {
	var (
		record ringbuf.Record
		event  ebpf.DataNotification
	)

	for {
		err := u.notificationReader.ReadInto(&record)
		if errors.Is(err, os.ErrClosed) {
			return
		}

		if err = binary.Read(bytes.NewBuffer(record.RawSample), binary.NativeEndian, &event); err != nil {
			logger.UpfLog.Error("Failed to decode data notification", zap.Error(err))
			continue
		}

		logger.UpfLog.Debug("Received notification for", zap.Uint64("SEID", event.LocalSEID), zap.Uint16("PDRID", event.PdrID), zap.Uint8("QFI", event.QFI))

		if !u.pfcpConn.BpfObjects.IsAlreadyNotified(event) {
			logger.UpfLog.Debug("Notifying SMF of downlink data", zap.Uint64("SEID", event.LocalSEID), zap.Uint16("PDRID", event.PdrID), zap.Uint8("QFI", event.QFI))

			err = core.SendPfcpSessionReportRequestForDownlinkData(context.TODO(), event.LocalSEID, event.PdrID, event.QFI)
			if err != nil {
				logger.UpfLog.Warn("Failed to send downlink data notification", zap.Error(err))
			} else {
				u.pfcpConn.BpfObjects.MarkNotified(event)
			}
		}
	}
}

func (u *UPF) monitorUsage(interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := u.pollUsageAndResetCounters()
			if err != nil {
				logger.UpfLog.Warn("Failed to poll usage and reset counters", zap.Error(err))
			}
		case <-stop:
			return
		}
	}
}

func (u *UPF) getAndResetUsageForURR(urrID uint32) (uint64, error) {
	var (
		perCPU []uint64
		total  uint64
	)

	ncpu := runtime.NumCPU()
	zeroes := make([]uint64, ncpu)

	err := u.pfcpConn.BpfObjects.UrrMap.Lookup(&urrID, &perCPU)
	if err != nil {
		return 0, fmt.Errorf("failed to lookup URR: %w", err)
	}

	err = u.pfcpConn.BpfObjects.UrrMap.Update(&urrID, zeroes, bpf.UpdateAny)
	if err != nil {
		return 0, fmt.Errorf("failed to reset URR: %w", err)
	}

	for _, v := range perCPU {
		total += v
	}

	return total, nil
}

func (u *UPF) pollUsageAndResetCounters() error {
	if u.pfcpConn == nil {
		return fmt.Errorf("PFCP connection is nil")
	}

	for localSeid, session := range u.pfcpConn.ListSessions() {
		for _, pdr := range session.ListPDRs() {
			urrID := pdr.PdrInfo.UrrID
			if urrID == 0 {
				logger.UpfLog.Debug("URR ID is 0, skipping usage report", zap.Uint64("local_seid", localSeid), zap.Uint32("pdr_id", pdr.PdrInfo.PdrID))
				continue
			}

			uvol := uint64(0)
			dvol := uint64(0)

			var err error

			// Downlink PDR
			if pdr.Ipv4 != nil || pdr.Ipv6 != nil {
				dvol, err = u.getAndResetUsageForURR(urrID)
				if err != nil {
					logger.UpfLog.Warn("could not get usage for URR - downlink", zap.Uint32("urr_id", urrID), zap.Error(err), zap.Uint64("local_seid", localSeid), zap.Uint32("pdr_id", pdr.PdrInfo.PdrID))
					continue
				}
			} else { // Uplink PDR
				uvol, err = u.getAndResetUsageForURR(urrID)
				if err != nil {
					logger.UpfLog.Warn("could not get usage for URR - uplink", zap.Uint32("urr_id", urrID), zap.Error(err), zap.Uint64("local_seid", localSeid))
					continue
				}
			}

			err = core.SendPfcpSessionReportRequestForUsage(context.TODO(), localSeid, urrID, uvol, dvol)
			if err != nil {
				logger.UpfLog.Warn("could not send PFCP session report request for usage", zap.Error(err), zap.Uint64("local_seid", localSeid), zap.Uint32("urr_id", urrID))
				continue
			}

			logger.UpfLog.Debug(
				"Sent usage report",
				zap.Uint64("local_seid", localSeid),
				zap.Uint32("urr_id", urrID),
				zap.Uint64("uplink_volume", uvol),
				zap.Uint64("downlink_volume", dvol),
			)
		}
	}

	return nil
}

func (u *UPF) listenForMissingNeighbours() {
	var record ringbuf.Record
	for {
		err := u.noNeighReader.ReadInto(&record)
		if errors.Is(err, os.ErrClosed) {
			return
		}

		ip, ok := netip.AddrFromSlice(record.RawSample)
		if !ok {
			logger.UpfLog.Debug("could not parse IP from bytes", zap.Binary("bytes", record.RawSample))
			continue
		}

		if err := kernel.AddNeighbour(context.TODO(), ip.AsSlice()); err != nil {
			logger.UpfLog.Warn("could not add neighbour", zap.String("destination", ip.String()), zap.Error(err))
			continue
		}
	}
}

// scanAndEnqueueExpiredFlows iterates over the FlowStats BPF map, deletes
// entries that have expired (inactive or max-active-lifetime exceeded), and
// forwards them to flowch for reporting.
//
// Scan speed is the priority: sends to flowch are non-blocking. If the channel
// is full the report is dropped and counted. This ensures that BPF-map cleanup
// is never delayed by a slow reporter.
//
// NOTE (TOCTOU): There is an inherent race between the Iterate snapshot and
// the subsequent BatchDelete. The eBPF XDP program may update a flow's byte or
// packet counters after the value has been read by Iterate but before it is
// removed by BatchDelete. The deleted entry will therefore contain a slightly
// stale counter. This inaccuracy is accepted as a reasonable trade-off to
// avoid per-key Lookup+Delete syscall pairs that would double the map load.
func (u *UPF) scanAndEnqueueExpiredFlows(expiryThreshold int64, flowch chan flowReport) {
	var (
		key          ebpf.N3N6EntrypointFlow
		value        ebpf.N3N6EntrypointFlowStats
		expiredKeys  []ebpf.N3N6EntrypointFlow
		expiredFlows []flowReport
		dropped      int
	)

	iter := u.pfcpConn.BpfObjects.FlowStats.Iterate()
	for iter.Next(&key, &value) {
		if value.LastTs < uint64(expiryThreshold) || (value.LastTs-value.FirstTs) > uint64(ActiveFlowTimeout.Nanoseconds()) {
			expiredKeys = append(expiredKeys, key)
			expiredFlows = append(expiredFlows, flowReport{flow: key, stats: value})
		}
	}

	if err := iter.Err(); err != nil {
		logger.UpfLog.Debug("Error while iterating over flow entries", zap.Error(err))
	}

	if len(expiredKeys) == 0 {
		return
	}

	// Delete from the BPF map immediately so the kernel can reuse the slots
	// as fast as possible, before we spend time forwarding reports.
	count, err := u.pfcpConn.BpfObjects.FlowStats.BatchDelete(expiredKeys, &bpf.BatchOptions{})
	if err != nil {
		logger.UpfLog.Warn("Failed to delete expired flow entries", zap.Error(err))
	}

	logger.UpfLog.Debug("Deleted expired flow entries", zap.Int("count", count))

	// Forward collected reports to the reporter goroutine with a non-blocking
	// send. Dropping a report is preferable to stalling the scan loop and
	// delaying BPF-map cleanup.
	for _, f := range expiredFlows {
		select {
		case flowch <- f:
		default:
			dropped++
		}
	}

	if dropped > 0 {
		logger.UpfLog.Warn("Dropped flow reports: reporter channel full", zap.Int("dropped", dropped))
	}
}

func (u *UPF) collectExpiredFlows(ctx context.Context, flowch chan flowReport) {
	var sysInfo unix.Sysinfo_t

	ticker := time.NewTicker(InactiveFlowTimeout / 2)
	defer ticker.Stop()
	defer close(flowch)

	for {
		select {
		case <-ctx.Done():
			// Perform one final scan so flows that expired since the last tick
			// are not silently lost on a graceful shutdown.
			if err := unix.Sysinfo(&sysInfo); err != nil {
				logger.UpfLog.Error("Failed to query sysinfo during final scan", zap.Error(err))
				return
			}

			nsSinceBoot := sysInfo.Uptime * time.Second.Nanoseconds()
			u.scanAndEnqueueExpiredFlows(nsSinceBoot-InactiveFlowTimeout.Nanoseconds(), flowch)

			return
		case <-ticker.C:
		}

		if err := unix.Sysinfo(&sysInfo); err != nil {
			// The only error returned by the sysinfo syscall is EFAULT if
			// the pointer is invalid. This should never occur here.
			logger.UpfLog.Error("Failed to query sysinfo", zap.Error(err))
			return
		}

		nsSinceBoot := sysInfo.Uptime * time.Second.Nanoseconds()
		u.scanAndEnqueueExpiredFlows(nsSinceBoot-InactiveFlowTimeout.Nanoseconds(), flowch)
	}
}

// reportFlows drains flowch and forwards each entry to the SMF via
// SendFlowReport. Each call is given its own short-lived context with a
// deadline so that a slow or unresponsive SMF cannot stall the reporter
// goroutine indefinitely.
//
// The function returns only when flowch is closed by collectExpiredFlows,
// ensuring that all flows buffered before shutdown are reported.
func (u *UPF) reportFlows(flowch chan flowReport) {
	for f := range flowch {
		rctx, cancel := context.WithTimeout(context.Background(), flowReportTimeout)
		core.SendFlowReport(rctx, f.flow, f.stats)
		cancel()
	}
}
