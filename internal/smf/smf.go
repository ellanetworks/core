package smf

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/factory"
	"github.com/ellanetworks/core/internal/smf/pfcp"
	"github.com/ellanetworks/core/internal/smf/pfcp/message"
	"github.com/ellanetworks/core/internal/smf/pfcp/udp"
	"github.com/ellanetworks/core/internal/smf/pfcp/upf"
)

func Start() error {
	configuration := factory.Configuration{
		PFCP: &factory.PFCP{
			Addr: "0.0.0.0",
		},
		SmfName: "SMF",
	}
	factory.InitConfigFactory(configuration)
	StartPfcpServer()
	metrics.RegisterSmfMetrics()
	return nil
}

func StartPfcpServer() {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		os.Exit(0)
	}()
	context.InitSmfContext(&factory.SmfConfig)
	udp.Run(pfcp.Dispatch)
	userPlaneInformation := context.GetUserPlaneInformation()
	if userPlaneInformation.UPF != nil {
		err := message.SendPfcpAssociationSetupRequest(userPlaneInformation.UPF.NodeID, userPlaneInformation.UPF.Port)
		if err != nil {
			logger.SmfLog.Warnf("Failed to send PFCP Association Setup Request to UPF: %+v", err)
			return
		}
	}
	go upf.InitPfcpHeartbeatRequest(userPlaneInformation)
	go upf.ProbeInactiveUpfs(userPlaneInformation)
}
