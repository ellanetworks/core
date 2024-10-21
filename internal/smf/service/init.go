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
	"github.com/omec-project/util/path_util"
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
)

type SMF struct{}

var initLog *logrus.Entry

var SmfLogPath = path_util.Free5gcPath("free5gc/smfsslkey.log")

func init() {
	initLog = logger.InitLog
}

func (smf *SMF) Initialize(smfConfig factory.Config, ueRoutingConfig factory.RoutingConfig) error {
	factory.InitConfigFactory(smfConfig)
	factory.InitRoutingConfigFactory(ueRoutingConfig)
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
	initLog.Infoln("SMF app initialising...")

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

	// allocate id for each upf
	context.AllocateUPFID()

	// Init UE Specific Config
	context.InitSMFUERouting(&factory.UERoutingConfig)

	// Wait for additional/updated config from config pod
	initLog.Infof("Configuration is managed by Config Pod")
	initLog.Infof("waiting for initial configuration from config pod")

	// Main thread should be blocked for config update from ROC
	// Future config update from ROC can be handled via background go-routine.
	// if <-factory.ConfigPodTrigger {
	// 	initLog.Infof("minimum configuration from config pod available")
	// 	context.ProcessConfigUpdate()
	// }

	router := logger_util.NewGinWithLogrus(logger.GinLog)
	oam.AddService(router)
	callback.AddService(router)
	for _, serviceName := range factory.SmfConfig.Configuration.ServiceNameList {
		switch models.ServiceName(serviceName) {
		case models.ServiceName_NSMF_PDUSESSION:
			pdusession.AddService(router)
		case models.ServiceName_NSMF_EVENT_EXPOSURE:
			eventexposure.AddService(router)
		}
	}

	udp.Run(pfcp.Dispatch)

	for _, upf := range context.SMF_Self().UserPlaneInformation.UPFs {
		logger.AppLog.Infof("Send PFCP Association Request to UPF[%s]\n", upf.NodeID.ResolveNodeIdToIp().String())
		message.SendPfcpAssociationSetupRequest(upf.NodeID, upf.Port)
	}

	// Trigger PFCP Heartbeat towards all connected UPFs
	go upf.InitPfcpHeartbeatRequest(context.SMF_Self().UserPlaneInformation)

	// Trigger PFCP association towards not associated UPFs
	go upf.ProbeInactiveUpfs(context.SMF_Self().UserPlaneInformation)

	time.Sleep(1000 * time.Millisecond)

	HTTPAddr := fmt.Sprintf("%s:%d", context.SMF_Self().BindingIPv4, context.SMF_Self().SBIPort)
	server, err := http2_util.NewServer(HTTPAddr, SmfLogPath, router)

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
