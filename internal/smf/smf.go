package smf

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/factory"
	"github.com/yeastengine/ella/internal/smf/logger"
	"github.com/yeastengine/ella/internal/smf/pfcp"
	"github.com/yeastengine/ella/internal/smf/pfcp/message"
	"github.com/yeastengine/ella/internal/smf/pfcp/udp"
	"github.com/yeastengine/ella/internal/smf/pfcp/upf"
	"go.uber.org/zap/zapcore"
)

func Start(amfURL string, udmURL string) error {
	configuration := factory.Configuration{
		PFCP: &factory.PFCP{
			Addr: "0.0.0.0",
		},
		AmfUri:  amfURL,
		UdmUri:  udmURL,
		SmfName: "SMF",
	}

	factory.InitConfigFactory(configuration)
	level, err := zapcore.ParseLevel("debug")
	if err != nil {
		return err
	}
	logger.SetLogLevel(level)
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
