// Copyright 2024 Ella Networks

package upf

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cilium/ebpf/link"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/upf/core"
	"github.com/ellanetworks/core/internal/upf/ebpf"
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

func Start(n3Address string, n3Interface string, n6Interface string, xdpAttachMode string) error {
	stopper := make(chan os.Signal, 1)
	signal.Notify(stopper, os.Interrupt, syscall.SIGTERM)

	if err := ebpf.IncreaseResourceLimits(); err != nil {
		logger.UpfLog.Fatalf("Can't increase resource limits: %s", err.Error())
	}

	bpfObjects := ebpf.NewBpfObjects(FarMapSize, QerMapSize)
	if err := bpfObjects.Load(); err != nil {
		logger.UpfLog.Fatalf("Loading bpf objects failed: %s", err.Error())
		return err
	}

	defer func() {
		if err := bpfObjects.Close(); err != nil {
			logger.UpfLog.Warnf("Failed to detach eBPF program: %s", err)
		}
	}()

	n3Iface, err := net.InterfaceByName(n3Interface)
	if err != nil {
		logger.UpfLog.Fatalf("Lookup network iface %q: %s", n3Interface, err.Error())
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
	defer func() {
		if err := n3Link.Close(); err != nil {
			logger.UpfLog.Warnf("Failed to detach eBPF program from n3 interface: %s", err)
		}
	}()

	logger.UpfLog.Infof("Attached eBPF program to n3 interface %q in mode %q", n3Interface, xdpAttachMode)

	n6Iface, err := net.InterfaceByName(n6Interface)
	if err != nil {
		logger.UpfLog.Fatalf("Lookup network iface %q: %s", n6Interface, err.Error())
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
	defer func() {
		if err := n6Link.Close(); err != nil {
			logger.UpfLog.Warnf("Failed to detach eBPF program from n6 interface: %s", err)
		}
	}()

	logger.UpfLog.Infof("Attached eBPF program to n6 interface %q in mode %q", n6Interface, xdpAttachMode)

	resourceManager, err := core.NewFteIDResourceManager(FTEIDPool)
	if err != nil {
		logger.UpfLog.Errorf("failed to create ResourceManager - err: %v", err)
	}

	pfcpConn, err := core.CreatePfcpConnection(PfcpAddress, PfcpNodeID, n3Address, SmfAddress, bpfObjects, resourceManager)
	if err != nil {
		logger.UpfLog.Fatalf("Could not create PFCP connection: %s", err.Error())
	}

	remoteNode := core.NewNodeAssociation(SmfNodeID, SmfAddress)
	pfcpConn.SmfNodeAssociation = remoteNode

	ForwardPlaneStats := ebpf.UpfXdpActionStatistic{
		BpfObjects: bpfObjects,
	}

	metrics.RegisterUPFMetrics(ForwardPlaneStats, pfcpConn)

	// Print the contents of the BPF hash map (source IP address -> packet count).
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
		case <-stopper:
			logger.UpfLog.Infof("Received signal, exiting program..")
			return nil
		}
	}
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
