// Copyright 2024 Ella Networks

package upf

import (
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

func Start(n3Address string, n3Interface string, n6Interface string) error {
	stopper := make(chan os.Signal, 1)
	signal.Notify(stopper, os.Interrupt, syscall.SIGTERM)
	interfaces := []string{n3Interface, n6Interface}
	c := config.UpfConfig{
		InterfaceName: interfaces,
		XDPAttachMode: "generic",
		PfcpAddress:   "0.0.0.0",
		SmfAddress:    "0.0.0.0",
		SmfNodeId:     "0.0.0.0",
		PfcpNodeId:    "0.0.0.0",
		N3Address:     n3Address,
		EchoInterval:  10,
		QerMapSize:    1024,
		FarMapSize:    1024,
		PdrMapSize:    1024,
		EbpfMapResize: false,
		FeatureFTUP:   true,
		FTEIDPool:     65535,
	}
	config.Init(c)

	if err := ebpf.IncreaseResourceLimits(); err != nil {
		logger.UpfLog.Fatalf("Can't increase resource limits: %s", err.Error())
	}

	bpfObjects := ebpf.NewBpfObjects()
	if err := bpfObjects.Load(); err != nil {
		logger.UpfLog.Fatalf("Loading bpf objects failed: %s", err.Error())
		return err
	}

	if config.Conf.EbpfMapResize {
		if err := bpfObjects.ResizeAllMaps(config.Conf.QerMapSize, config.Conf.FarMapSize, config.Conf.PdrMapSize); err != nil {
			logger.UpfLog.Fatalf("Failed to set ebpf map sizes: %s", err)
			return err
		}
	}

	defer func() {
		if err := bpfObjects.Close(); err != nil {
			logger.UpfLog.Warnf("Failed to detach XDP program: %s", err)
		}
	}()

	for _, ifaceName := range config.Conf.InterfaceName {
		iface, err := net.InterfaceByName(ifaceName)
		if err != nil {
			logger.UpfLog.Fatalf("Lookup network iface %q: %s", ifaceName, err.Error())
			return err
		}

		// Attach the program.
		l, err := link.AttachXDP(link.XDPOptions{
			Program:   bpfObjects.UpfIpEntrypointFunc,
			Interface: iface.Index,
			Flags:     StringToXDPAttachMode(config.Conf.XDPAttachMode),
		})
		if err != nil {
			logger.UpfLog.Fatalf("Could not attach XDP program: %s", err.Error())
		}
		defer func() {
			if err := l.Close(); err != nil {
				logger.UpfLog.Warnf("Failed to detach XDP program: %s", err)
			}
		}()

		logger.UpfLog.Infof("Attached XDP program to iface %q", iface.Name)
	}

	var err error
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

	gtpPathManager := core.NewGtpPathManager(config.Conf.N3Address+":2152", time.Duration(config.Conf.EchoInterval)*time.Second)
	for _, peer := range config.Conf.GtpPeer {
		gtpPathManager.AddGtpPath(peer)
	}
	gtpPathManager.Run()

	defer gtpPathManager.Stop()
	logger.UpfLog.Infof("GTP server started on %s", config.Conf.N3Address+":2152")

	// Print the contents of the BPF hash map (source IP address -> packet count).
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// s, err := FormatMapContents(bpfObjects.UpfXdpObjects.UpfPipeline)
			// if err != nil {
			// 	logger.UpfLog.Printf("Error reading map: %s", err)
			// 	continue
			// }
			// logger.UpfLog.Printf("Pipeline map contents:\n%s", s)
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
