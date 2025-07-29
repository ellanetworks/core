// Copyright 2024 Ella Networks

package upf

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/cilium/ebpf/link"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/upf/core"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.uber.org/zap"
)

const (
	PfcpAddress = "0.0.0.0"
	SmfAddress  = "0.0.0.0"
	SmfNodeID   = "0.0.0.0"
	PfcpNodeID  = "0.0.0.0"
	QerMapSize  = 1024
	FarMapSize  = 1024
	FTEIDPool   = 65535
)

func Start(ctx context.Context, n3Address string, n3Interface string, n6Interface string, xdpAttachMode string) error {
	if err := ebpf.IncreaseResourceLimits(); err != nil {
		logger.UpfLog.Fatal("Can't increase resource limits", zap.Error(err))
	}

	bpfObjects := ebpf.NewBpfObjects(FarMapSize, QerMapSize)
	if err := bpfObjects.Load(); err != nil {
		logger.UpfLog.Fatal("Loading bpf objects failed", zap.Error(err))
		return err
	}

	n3Iface, err := net.InterfaceByName(n3Interface)
	if err != nil {
		logger.UpfLog.Fatal("Lookup network iface", zap.String("iface", n3Interface), zap.Error(err))
		return err
	}
	n3Link, err := link.AttachXDP(link.XDPOptions{
		Program:   bpfObjects.UpfN3EntrypointFunc,
		Interface: n3Iface.Index,
		Flags:     StringToXDPAttachMode(xdpAttachMode),
	})
	if err != nil {
		return fmt.Errorf("failed to attach eBPF program on n3 interface %q: %s", n3Interface, err)
	}

	n6Iface, err := net.InterfaceByName(n6Interface)
	if err != nil {
		logger.UpfLog.Fatal("Lookup network iface", zap.String("iface", n6Interface), zap.Error(err))
		return err
	}
	n6Link, err := link.AttachXDP(link.XDPOptions{
		Program:   bpfObjects.UpfN6EntrypointFunc,
		Interface: n6Iface.Index,
		Flags:     StringToXDPAttachMode(xdpAttachMode),
	})
	if err != nil {
		return fmt.Errorf("failed to attach eBPF program on n6 interface %q: %s", n6Interface, err)
	}

	resourceManager, err := core.NewFteIDResourceManager(FTEIDPool)
	if err != nil {
		logger.UpfLog.Error("failed to create Resource Manager", zap.Error(err))
	}

	pfcpConn, err := core.CreatePfcpConnection(PfcpAddress, PfcpNodeID, n3Address, SmfAddress, bpfObjects, resourceManager)
	if err != nil {
		logger.UpfLog.Fatal("Could not create PFCP connection", zap.Error(err))
	}

	remoteNode := core.NewNodeAssociation(SmfNodeID, SmfAddress)
	pfcpConn.SmfNodeAssociation = remoteNode

	ForwardPlaneStats := ebpf.UpfXdpActionStatistic{
		BpfObjects: bpfObjects,
	}
	metrics.RegisterUPFMetrics(ForwardPlaneStats, pfcpConn)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Wait until context is canceled
	<-ctx.Done()

	logger.UpfLog.Info("Received shutdown signal, cleaning up UPF...")

	// Explicit cleanup instead of relying on defer
	if err := n6Link.Close(); err != nil {
		logger.UpfLog.Warn("Failed to detach eBPF program from n6 interface", zap.Error(err))
	} else {
		logger.UpfLog.Info("eBPF program detached from n6 interface")
	}

	if err := n3Link.Close(); err != nil {
		logger.UpfLog.Warn("Failed to detach eBPF program from n3 interface", zap.Error(err))
	} else {
		logger.UpfLog.Info("eBPF program detached from n3 interface")
	}

	if err := bpfObjects.Close(); err != nil {
		logger.UpfLog.Warn("Failed to close BPF objects", zap.Error(err))
	} else {
		logger.UpfLog.Info("BPF objects closed")
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
