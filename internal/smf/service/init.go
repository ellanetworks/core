package service

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/factory"
	"github.com/yeastengine/ella/internal/smf/logger"
	"github.com/yeastengine/ella/internal/smf/pfcp"
	"github.com/yeastengine/ella/internal/smf/pfcp/message"
	"github.com/yeastengine/ella/internal/smf/pfcp/udp"
	"github.com/yeastengine/ella/internal/smf/pfcp/upf"
	"go.uber.org/zap/zapcore"
)

type SMF struct{}

func (smf *SMF) Initialize(smfConfig factory.Configuration) error {
	factory.InitConfigFactory(smfConfig)
	level, err := zapcore.ParseLevel("debug")
	if err != nil {
		return err
	}
	logger.SetLogLevel(level)
	return nil
}

func (smf *SMF) Start() {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		smf.Terminate()
		os.Exit(0)
	}()

	context.InitSmfContext(&factory.SmfConfig)

	udp.Run(pfcp.Dispatch)

	userPlaneInformation := context.GetUserPlaneInformation()

	if userPlaneInformation.UPF != nil {
		message.SendPfcpAssociationSetupRequest(userPlaneInformation.UPF.NodeID, userPlaneInformation.UPF.Port)
	}

	// Trigger PFCP Heartbeat towards all connected UPFs
	go upf.InitPfcpHeartbeatRequest(userPlaneInformation)

	// Trigger PFCP association towards not associated UPFs
	go upf.ProbeInactiveUpfs(userPlaneInformation)

	time.Sleep(1000 * time.Millisecond)
}

func (smf *SMF) Terminate() {
	logger.InitLog.Infof("Terminating SMF...")
}
