package upf

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/upf/config"
	"github.com/ellanetworks/core/internal/upf/core"
	"github.com/ellanetworks/core/internal/upf/core/service"
	"github.com/ellanetworks/core/internal/upf/ebpf"

	"github.com/cilium/ebpf/link"
)

func Start(n3Interface string, n6Interface string, n3Address string, xdpAttachMode string) error {
	c := config.UpfConfig{
		N3Interface:   n3Interface,
		N6Interface:   n6Interface,
		XDPAttachMode: xdpAttachMode,
		PfcpAddress:   "0.0.0.0",
		SmfAddress:    "0.0.0.0",
		SmfNodeId:     "0.0.0.0",
		PfcpNodeId:    "0.0.0.0",
		N3Address:     n3Address,
		QerMapSize:    1024,
		FarMapSize:    1024,
		PdrMapSize:    1024,
		FTEIDPool:     65535,
	}
	config.Init(c)

	stopper := make(chan os.Signal, 1)
	signal.Notify(stopper, os.Interrupt, syscall.SIGTERM)

	if err := ebpf.IncreaseResourceLimits(); err != nil {
		logger.UpfLog.Fatalf("Can't increase resource limits: %s", err.Error())
	}

	bpfObjects := ebpf.NewBpfObjects()
	if err := bpfObjects.Load(); err != nil {
		logger.UpfLog.Fatalf("Loading bpf objects failed: %s", err.Error())
		return err
	}
	defer func() {
		if err := bpfObjects.Close(); err != nil {
			logger.UpfLog.Warnf("Failed to detach XDP program: %s", err)
		}
	}()

	n3Iface, err := net.InterfaceByName(config.Conf.N3Interface)
	if err != nil {
		logger.UpfLog.Fatalf("Lookup network iface %q: %s", config.Conf.N3Interface, err.Error())
		return err
	}
	n6Iface, err := net.InterfaceByName(config.Conf.N6Interface)
	if err != nil {
		logger.UpfLog.Fatalf("Lookup network iface %q: %s", config.Conf.N6Interface, err.Error())
		return err
	}

	// Attach the N3 XDP program to the N3 interface.
	n3Link, err := link.AttachXDP(link.XDPOptions{
		Program:   bpfObjects.UpfN3EntrypointFunc,
		Interface: n3Iface.Index,
		Flags:     StringToXDPAttachMode(config.Conf.XDPAttachMode),
	})
	if err != nil {
		return fmt.Errorf("failed to attach N3 XDP program on iface %q: %s", config.Conf.N3Interface, err)
	}
	defer func() {
		if err := n3Link.Close(); err != nil {
			logger.UpfLog.Warnf("Failed to detach N3 XDP program: %s", err)
		}
	}()
	logger.UpfLog.Debugf("Attached N3 XDP program to iface %q", config.Conf.N3Interface)

	n6Link, err := link.AttachXDP(link.XDPOptions{
		Program:   bpfObjects.UpfN6EntrypointFunc,
		Interface: n6Iface.Index,
		Flags:     StringToXDPAttachMode(config.Conf.XDPAttachMode),
	})
	if err != nil {
		return fmt.Errorf("failed to attach N6 XDP program on iface %q: %s", config.Conf.N6Interface, err)
	}
	defer func() {
		if err := n6Link.Close(); err != nil {
			logger.UpfLog.Warnf("Failed to detach N6 XDP program: %s", err)
		}
	}()
	logger.UpfLog.Debugf("Attached N6 XDP program to iface %q", config.Conf.N6Interface)

	resourceManager, err := service.NewResourceManager(config.Conf.FTEIDPool)
	if err != nil {
		logger.UpfLog.Errorf("failed to create ResourceManager - err: %v", err)
	}

	pfcpConn, err := core.CreatePfcpConnection(config.Conf.PfcpAddress, config.Conf.PfcpNodeId, config.Conf.N3Address, bpfObjects, resourceManager)
	if err != nil {
		logger.UpfLog.Fatalf("Could not create PFCP connection: %s", err.Error())
	}

	remoteNode := core.NewNodeAssociation(config.Conf.SmfNodeId, config.Conf.SmfAddress)
	pfcpConn.NodeAssociations[config.Conf.SmfAddress] = remoteNode

	ForwardPlaneStats := ebpf.UpfXdpActionStatistic{
		BpfObjects: bpfObjects,
	}
	metrics.RegisterUPFMetrics(ForwardPlaneStats, pfcpConn)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
		case <-stopper:
			logger.UpfLog.Infof("Received signal, exiting program...")
			return nil
		}
	}
}

func StringToXDPAttachMode(mode string) link.XDPAttachFlags {
	switch mode {
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
