package smf

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/factory"
	"github.com/yeastengine/ella/internal/smf/pfcp"
	"github.com/yeastengine/ella/internal/smf/pfcp/message"
	"github.com/yeastengine/ella/internal/smf/pfcp/udp"
	"github.com/yeastengine/ella/internal/smf/pfcp/upf"
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
		message.SendPfcpAssociationSetupRequest(userPlaneInformation.UPF.NodeID, userPlaneInformation.UPF.Port)
	}
	go upf.InitPfcpHeartbeatRequest(userPlaneInformation)
	go upf.ProbeInactiveUpfs(userPlaneInformation)
}
