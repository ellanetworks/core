// Copyright 2024 Ella Networks

package upf

import (
	"context"
	"fmt"
	"net"
	"time"

	bpf "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
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
	bpfObjects *ebpf.BpfObjects
	n3Link     link.Link
	n6Link     link.Link
	pfcpConn   *core.PfcpConnection

	gcCancel context.CancelFunc
}

func Start(ctx context.Context, n3Address string, n3Interface string, n6Interface string, xdpAttachMode string, masquerade bool) (*UPF, error) {
	if err := ebpf.IncreaseResourceLimits(); err != nil {
		logger.UpfLog.Fatal("Can't increase resource limits", zap.Error(err))
	}

	bpfObjects := ebpf.NewBpfObjects(masquerade)

	err := ebpf.PinMaps()
	if err != nil {
		logger.UpfLog.Fatal("Creating BPF pin path failed", zap.Error(err))
		return nil, err
	}

	if err := bpfObjects.Load(); err != nil {
		logger.UpfLog.Fatal("Loading bpf objects failed", zap.Error(err))
		return nil, err
	}

	n3Iface, err := net.InterfaceByName(n3Interface)
	if err != nil {
		logger.UpfLog.Fatal("Lookup network iface", zap.String("iface", n3Interface), zap.Error(err))
		return nil, err
	}

	n3Link, err := link.AttachXDP(link.XDPOptions{
		Program:   bpfObjects.UpfN3EntrypointFunc,
		Interface: n3Iface.Index,
		Flags:     StringToXDPAttachMode(xdpAttachMode),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach eBPF program on n3 interface %q: %s", n3Interface, err)
	}

	n6Iface, err := net.InterfaceByName(n6Interface)
	if err != nil {
		logger.UpfLog.Fatal("Lookup network iface", zap.String("iface", n6Interface), zap.Error(err))
		return nil, err
	}

	n6Link, err := link.AttachXDP(link.XDPOptions{
		Program:   bpfObjects.UpfN6EntrypointFunc,
		Interface: n6Iface.Index,
		Flags:     StringToXDPAttachMode(xdpAttachMode),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach eBPF program on n6 interface %q: %s", n6Interface, err)
	}

	resourceManager, err := core.NewFteIDResourceManager(FTEIDPool)
	if err != nil {
		return nil, fmt.Errorf("failed to create Resource Manager: %w", err)
	}

	pfcpConn, err := core.CreatePfcpConnection(PfcpAddress, PfcpNodeID, n3Address, SmfAddress, bpfObjects, resourceManager)
	if err != nil {
		return nil, fmt.Errorf("failed to create PFCP connection: %w", err)
	}

	remoteNode := core.NewNodeAssociation(SmfNodeID, SmfAddress)
	pfcpConn.SmfNodeAssociation = remoteNode

	ForwardPlaneStats := &ebpf.UpfXdpActionStatistic{
		BpfObjects: bpfObjects,
	}

	metrics.RegisterUPFMetrics(ForwardPlaneStats)

	upf := &UPF{
		bpfObjects: bpfObjects,
		n3Link:     n3Link,
		n6Link:     n6Link,
		pfcpConn:   pfcpConn,
	}

	if masquerade {
		upf.startGC()
	}

	if masquerade {
		go upf.collectCollectionTrackingGarbage(ctx)
	}

	return upf, nil
}

func (u *UPF) Close() {
	if err := u.n6Link.Close(); err != nil {
		logger.UpfLog.Warn("Failed to detach eBPF from n6", zap.Error(err))
	}
	if err := u.n3Link.Close(); err != nil {
		logger.UpfLog.Warn("Failed to detach eBPF from n3", zap.Error(err))
	}
	if err := u.bpfObjects.Close(); err != nil {
		logger.UpfLog.Warn("Failed to close BPF objects", zap.Error(err))
	}
	logger.UpfLog.Info("UPF resources released")
}

func (u *UPF) Reload(masquerade bool) error {
	u.bpfObjects.Masquerade = masquerade

	err := u.bpfObjects.Load()
	if err != nil {
		return fmt.Errorf("couldn't load BPF objects: %w", err)
	}

	if err := u.n3Link.Update(u.bpfObjects.UpfN3EntrypointFunc); err != nil {
		return err
	}

	if err := u.n6Link.Update(u.bpfObjects.UpfN6EntrypointFunc); err != nil {
		return err
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
		key     ebpf.N3EntrypointFiveTuple
		value   ebpf.N3EntrypointNatEntry
		sysInfo unix.Sysinfo_t
	)
	expiredKeys := make([]ebpf.N3EntrypointFiveTuple, 0)

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

		ct_entries := u.bpfObjects.N3EntrypointMaps.NatCt.Iterate()
		for ct_entries.Next(&key, &value) {
			if value.RefreshTs < uint64(expiryThreshold) {
				expiredKeys = append(expiredKeys, key)
			}
		}
		if err := ct_entries.Err(); err != nil {
			logger.UpfLog.Debug("Error while iterating over conntrack entries", zap.Error(err))
		}

		count, err := u.bpfObjects.N3EntrypointMaps.NatCt.BatchDelete(expiredKeys, &bpf.BatchOptions{})
		if err != nil {
			logger.UpfLog.Warn("Failed to delete expired conntrack entries", zap.Error(err))
		}
		logger.UpfLog.Debug("Deleted expired conntrack entries", zap.Int("count", count))
		expiredKeys = expiredKeys[:0]
	}
}
