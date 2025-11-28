// Copyright 2024 Ella Networks

package upf

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"time"

	bpf "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/upf/core"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

const (
	PfcpAddress      = "0.0.0.0"
	SmfAddress       = "0.0.0.0"
	SmfNodeID        = "0.0.0.0"
	PfcpNodeID       = "0.0.0.0"
	FTEIDPool        = 65535
	ConnTrackTimeout = 10 * time.Minute
)

type UPF struct {
	bpfObjects         *ebpf.BpfObjects
	n3Link             link.Link
	n6Link             *link.Link
	pfcpConn           *core.PfcpConnection
	notificationReader *ringbuf.Reader

	gcCancel context.CancelFunc
}

func Start(ctx context.Context, n3Interface config.N3Interface, n3Address string, advertisedN3Address string, n6Interface config.N6Interface, xdpAttachMode string, masquerade bool) (*UPF, error) {
	var n3Vlan uint32
	var n6Vlan uint32

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

	bpfObjects := ebpf.NewBpfObjects(masquerade, n3Iface.Index, n6Iface.Index, n3Vlan, n6Vlan)

	err = ebpf.PinMaps()
	if err != nil {
		logger.UpfLog.Fatal("Creating BPF pin path failed", zap.Error(err))
		return nil, err
	}

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

	pfcpConn, err := core.CreatePfcpConnection(PfcpAddress, PfcpNodeID, n3Address, advertisedN3Address, SmfAddress, bpfObjects, resourceManager)
	if err != nil {
		return nil, fmt.Errorf("failed to create PFCP connection: %w", err)
	}

	remoteNode := core.NewNodeAssociation(SmfNodeID, SmfAddress)
	pfcpConn.SmfNodeAssociation = remoteNode

	ForwardPlaneStats := &ebpf.UpfXdpActionStatistic{
		BpfObjects: bpfObjects,
	}

	metrics.RegisterUPFMetrics(ForwardPlaneStats)

	notificationReader, err := ringbuf.NewReader(bpfObjects.NocpMap)
	if err != nil {
		return nil, fmt.Errorf("coud not start traffic notification reader: %s", err.Error())
	}

	upf := &UPF{
		bpfObjects:         bpfObjects,
		n3Link:             n3Link,
		n6Link:             n6Link,
		pfcpConn:           pfcpConn,
		notificationReader: notificationReader,
	}

	go upf.listenForTrafficNotifications()

	go upf.monitorUsage(30*time.Second, ctx.Done())

	if masquerade {
		upf.startGC()
	}

	if masquerade {
		go upf.collectCollectionTrackingGarbage(ctx)
	}

	return upf, nil
}

func (u *UPF) Close() {
	if u.n6Link != nil {
		if err := (*u.n6Link).Close(); err != nil {
			logger.UpfLog.Warn("Failed to detach eBPF from n6", zap.Error(err))
		}
	}
	if err := u.n3Link.Close(); err != nil {
		logger.UpfLog.Warn("Failed to detach eBPF from n3", zap.Error(err))
	}
	if err := u.bpfObjects.Close(); err != nil {
		logger.UpfLog.Warn("Failed to close BPF objects", zap.Error(err))
	}
	if err := u.notificationReader.Close(); err != nil {
		logger.UpfLog.Warn("Failed to close notification reader", zap.Error(err))
	}
	logger.UpfLog.Info("UPF resources released")
}

func (u *UPF) UpdateAdvertisedN3Address(newN3Addr net.IP) {
	u.pfcpConn.SetAdvertisedN3Address(newN3Addr)
}

func (u *UPF) Reload(masquerade bool) error {
	u.bpfObjects.Masquerade = masquerade

	err := u.bpfObjects.Load()
	if err != nil {
		return fmt.Errorf("couldn't load BPF objects: %w", err)
	}

	if err := u.n3Link.Update(u.bpfObjects.UpfN3N6EntrypointFunc); err != nil {
		return err
	}

	if u.n6Link != nil {
		if err := (*u.n6Link).Update(u.bpfObjects.UpfN3N6EntrypointFunc); err != nil {
			return err
		}
	}

	if masquerade {
		u.startGC()
	} else {
		u.stopGC()
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

func (u *UPF) startGC() {
	if u.gcCancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	u.gcCancel = cancel

	go u.collectCollectionTrackingGarbage(ctx)
}

func (u *UPF) stopGC() {
	if u.gcCancel != nil {
		u.gcCancel()
		u.gcCancel = nil
	}
}

func (u *UPF) collectCollectionTrackingGarbage(ctx context.Context) {
	var (
		key     ebpf.N3N6EntrypointFiveTuple
		value   ebpf.N3N6EntrypointNatEntry
		sysInfo unix.Sysinfo_t
	)
	expiredKeys := make([]ebpf.N3N6EntrypointFiveTuple, 0)

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Minute):
		}

		err := unix.Sysinfo(&sysInfo)
		if err != nil {
			logger.UpfLog.Warn("Failed to query sysinfo", zap.Error(err))
			return
		}
		nsSinceBoot := sysInfo.Uptime * time.Second.Nanoseconds()
		expiryThreshold := nsSinceBoot - ConnTrackTimeout.Nanoseconds()

		ct_entries := u.bpfObjects.N3N6EntrypointMaps.NatCt.Iterate()
		for ct_entries.Next(&key, &value) {
			if value.RefreshTs < uint64(expiryThreshold) {
				expiredKeys = append(expiredKeys, key)
			}
		}
		if err := ct_entries.Err(); err != nil {
			logger.UpfLog.Debug("Error while iterating over conntrack entries", zap.Error(err))
		}

		count, err := u.bpfObjects.N3N6EntrypointMaps.NatCt.BatchDelete(expiredKeys, &bpf.BatchOptions{})
		if err != nil {
			logger.UpfLog.Warn("Failed to delete expired conntrack entries", zap.Error(err))
		}
		logger.UpfLog.Debug("Deleted expired conntrack entries", zap.Int("count", count))
		expiredKeys = expiredKeys[:0]
	}
}

func (u *UPF) listenForTrafficNotifications() {
	var record ringbuf.Record
	var event ebpf.DataNotification
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
		if !u.bpfObjects.IsAlreadyNotified(event) {
			logger.UpfLog.Debug("Notifying SMF of downlink data", zap.Uint64("SEID", event.LocalSEID), zap.Uint16("PDRID", event.PdrID), zap.Uint8("QFI", event.QFI))
			err = core.SendPfcpSessionReportRequestForDownlinkData(context.TODO(), event.LocalSEID, event.PdrID, event.QFI)
			if err != nil {
				logger.UpfLog.Warn("Failed to send downlink data notification", zap.Error(err))
			} else {
				u.bpfObjects.MarkNotified(event)
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
	var perCPU []uint64
	var total uint64

	ncpu := runtime.NumCPU()
	zeroes := make([]uint64, ncpu)

	err := u.bpfObjects.N3N6EntrypointMaps.UrrMap.Lookup(&urrID, &perCPU)
	if err != nil {
		return 0, fmt.Errorf("failed to lookup URR: %w", err)
	}

	err = u.bpfObjects.N3N6EntrypointMaps.UrrMap.Update(&urrID, zeroes, bpf.UpdateAny)
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

	if u.pfcpConn.SmfNodeAssociation == nil {
		return fmt.Errorf("SMF node association is nil")
	}

	if u.pfcpConn.SmfNodeAssociation.Sessions == nil {
		return fmt.Errorf("SMF node association sessions is nil")
	}

	sessions := u.pfcpConn.SmfNodeAssociation.Sessions

	for localSeid, session := range sessions {
		for _, pdr := range session.PDRs {
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
