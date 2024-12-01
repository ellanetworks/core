package service

import (
	"fmt"
	_ "net/http/pprof" // Using package only for invoking initialization.
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/http2_util"
	logger_util "github.com/omec-project/util/logger"
	"github.com/sirupsen/logrus"
	"github.com/yeastengine/ella/internal/smf/callback"
	"github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/eventexposure"
	"github.com/yeastengine/ella/internal/smf/factory"
	"github.com/yeastengine/ella/internal/smf/logger"
	"github.com/yeastengine/ella/internal/smf/oam"
	"github.com/yeastengine/ella/internal/smf/pdusession"
	"github.com/yeastengine/ella/internal/smf/pfcp"
	"github.com/yeastengine/ella/internal/smf/pfcp/message"
	"github.com/yeastengine/ella/internal/smf/pfcp/udp"
	"github.com/yeastengine/ella/internal/smf/pfcp/upf"
	"github.com/yeastengine/ella/internal/smf/util"
)

type SMF struct{}

var initLog *logrus.Entry

func init() {
	initLog = logger.InitLog
}

func (smf *SMF) Initialize(smfConfig factory.Configuration) error {
	factory.InitConfigFactory(smfConfig)
	smf.setLogLevel()
	return nil
}

func (smf *SMF) setLogLevel() {
	if level, err := logrus.ParseLevel(factory.SmfConfig.Logger.SMF.DebugLevel); err != nil {
		initLog.Warnf("SMF Log level [%s] is invalid, set to [info] level",
			factory.SmfConfig.Logger.SMF.DebugLevel)
		logger.SetLogLevel(logrus.InfoLevel)
	} else {
		initLog.Infof("SMF Log level is set to [%s] level", level)
		logger.SetLogLevel(level)
	}
	logger.SetReportCaller(factory.SmfConfig.Logger.SMF.ReportCaller)
}

func (smf *SMF) Start() {
	// Initialise channel to stop SMF
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		smf.Terminate()
		os.Exit(0)
	}()

	// Init SMF Service
	context.InitSmfContext(&factory.SmfConfig)

	// // allocate id for each upf
	// context.AllocateUPFID()

	router := logger_util.NewGinWithLogrus(logger.GinLog)
	oam.AddService(router)
	callback.AddService(router)
	for _, serviceName := range factory.SmfConfig.ServiceNameList {
		switch models.ServiceName(serviceName) {
		case models.ServiceName_NSMF_PDUSESSION:
			pdusession.AddService(router)
		case models.ServiceName_NSMF_EVENT_EXPOSURE:
			eventexposure.AddService(router)
		}
	}

	udp.Run(pfcp.Dispatch)

	userPlaneInformation := context.GetUserPlaneInformation()

	if userPlaneInformation != nil {
		message.SendPfcpAssociationSetupRequest(userPlaneInformation.UPF.NodeID, userPlaneInformation.UPF.Port)

		// Trigger PFCP Heartbeat towards all connected UPFs
		go upf.InitPfcpHeartbeatRequest(userPlaneInformation)

		// Trigger PFCP association towards not associated UPFs
		go upf.ProbeInactiveUpfs(userPlaneInformation)
	}

	time.Sleep(1000 * time.Millisecond)

	HTTPAddr := fmt.Sprintf("%s:%d", context.SMF_Self().BindingIPv4, context.SMF_Self().SBIPort)
	server, err := http2_util.NewServer(HTTPAddr, util.SmfLogPath, router)

	if server == nil {
		initLog.Error("Initialize HTTP server failed:", err)
		return
	}

	if err != nil {
		initLog.Warnln("Initialize HTTP server:", err)
	}

	err = server.ListenAndServe()
	if err != nil {
		initLog.Fatalln("HTTP server setup failed:", err)
	}
}

func (smf *SMF) Terminate() {
	logger.InitLog.Infof("Terminating SMF...")
}
