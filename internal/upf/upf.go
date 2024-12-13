package upf

import (
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wmnsk/go-pfcp/message"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/metrics"
	"github.com/yeastengine/ella/internal/upf/config"
	"github.com/yeastengine/ella/internal/upf/core"
	"github.com/yeastengine/ella/internal/upf/core/service"
	"github.com/yeastengine/ella/internal/upf/ebpf"

	"github.com/cilium/ebpf/link"
)

func Start(interfaces []string, n3_address string) error {
	stopper := make(chan os.Signal, 1)
	signal.Notify(stopper, os.Interrupt, syscall.SIGTERM)

	c := config.UpfConfig{
		InterfaceName:     interfaces,
		XDPAttachMode:     "generic",
		ApiAddress:        ":8080",
		PfcpAddress:       "0.0.0.0:8806",
		PfcpNodeId:        "0.0.0.0",
		N3Address:         n3_address,
		EchoInterval:      10,
		QerMapSize:        1024,
		FarMapSize:        1024,
		PdrMapSize:        1024,
		EbpfMapResize:     false,
		HeartbeatRetries:  3,
		HeartbeatInterval: 5,
		HeartbeatTimeout:  5,
		LoggingLevel:      "debug",
		FeatureFTUP:       true,
		FTEIDPool:         65535,
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

	defer bpfObjects.Close()

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
		defer l.Close()

		logger.UpfLog.Infof("Attached XDP program to iface %q (index %d)", iface.Name, iface.Index)
	}

	logger.UpfLog.Infof("Initialize resources: UEIP pool (CIDR: \"%s\"), TEID pool (size: %d)", config.Conf.UEIPPool, config.Conf.FTEIDPool)
	var err error
	resourceManager, err := service.NewResourceManager(config.Conf.UEIPPool, config.Conf.FTEIDPool)
	if err != nil {
		logger.UpfLog.Errorf("failed to create ResourceManager - err: %v", err)
	}

	// Create PFCP connection
	pfcpHandlers := core.PfcpHandlerMap{
		message.MsgTypeHeartbeatRequest:            core.HandlePfcpHeartbeatRequest,
		message.MsgTypeHeartbeatResponse:           core.HandlePfcpHeartbeatResponse,
		message.MsgTypeAssociationSetupRequest:     core.HandlePfcpAssociationSetupRequest,
		message.MsgTypeSessionEstablishmentRequest: core.HandlePfcpSessionEstablishmentRequest,
		message.MsgTypeSessionDeletionRequest:      core.HandlePfcpSessionDeletionRequest,
		message.MsgTypeSessionModificationRequest:  core.HandlePfcpSessionModificationRequest,
	}

	pfcpConn, err := core.CreatePfcpConnection(config.Conf.PfcpAddress, pfcpHandlers, config.Conf.PfcpNodeId, config.Conf.N3Address, bpfObjects, resourceManager)
	if err != nil {
		logger.UpfLog.Fatalf("Could not create PFCP connection: %s", err.Error())
	}
	go pfcpConn.Run()
	defer pfcpConn.Close()

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
