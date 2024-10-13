package service

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/http2_util"
	logger_util "github.com/omec-project/util/logger"
	"github.com/yeastengine/config5g/proto/client"
	protos "github.com/yeastengine/config5g/proto/sdcoreConfig"
	ausf_context "github.com/yeastengine/ella/internal/ausf/context"
	"github.com/yeastengine/ella/internal/ausf/factory"
	"github.com/yeastengine/ella/internal/ausf/logger"
	"github.com/yeastengine/ella/internal/ausf/ueauthentication"
	"github.com/yeastengine/ella/internal/ausf/util"
)

type AUSF struct{}

var initLog *logrus.Entry

func init() {
	initLog = logger.InitLog
}

var ConfigPodTrigger chan bool

func init() {
	ConfigPodTrigger = make(chan bool)
}

func (ausf *AUSF) Initialize(c factory.Config) {
	factory.InitConfigFactory(c)
	ausf.setLogLevel()
	commChannel := client.ConfigWatcher(factory.AusfConfig.Configuration.WebuiUri, "ausf")
	go ausf.updateConfig(commChannel)
}

func (ausf *AUSF) setLogLevel() {
	if level, err := logrus.ParseLevel(factory.AusfConfig.Logger.AUSF.DebugLevel); err != nil {
		initLog.Warnf("AUSF Log level [%s] is invalid, set to [info] level",
			factory.AusfConfig.Logger.AUSF.DebugLevel)
		logger.SetLogLevel(logrus.InfoLevel)
	} else {
		initLog.Infof("AUSF Log level is set to [%s] level", level)
		logger.SetLogLevel(level)
	}
	logger.SetReportCaller(factory.AusfConfig.Logger.AUSF.ReportCaller)
}

func (ausf *AUSF) updateConfig(commChannel chan *protos.NetworkSliceResponse) bool {
	var minConfig bool
	context := ausf_context.GetSelf()
	for rsp := range commChannel {
		logger.GrpcLog.Infoln("Received updateConfig in the ausf app : ", rsp)
		for _, ns := range rsp.NetworkSlice {
			logger.GrpcLog.Infoln("Network Slice Name ", ns.Name)
			if ns.Site != nil {
				temp := models.PlmnId{}
				var found bool = false
				logger.GrpcLog.Infoln("Network Slice has site name present ")
				site := ns.Site
				logger.GrpcLog.Infoln("Site name ", site.SiteName)
				if site.Plmn != nil {
					temp.Mcc = site.Plmn.Mcc
					temp.Mnc = site.Plmn.Mnc
					logger.GrpcLog.Infoln("Plmn mcc ", site.Plmn.Mcc)
					for _, item := range context.PlmnList {
						if item.Mcc == temp.Mcc && item.Mnc == temp.Mnc {
							found = true
							break
						}
					}
					if !found {
						context.PlmnList = append(context.PlmnList, temp)
						logger.GrpcLog.Infoln("Plmn added in the context", context.PlmnList)
					}
				} else {
					logger.GrpcLog.Infoln("Plmn not present in the message ")
				}
			}
		}
		if !minConfig {
			// first slice Created
			if len(context.PlmnList) > 0 {
				minConfig = true
				ConfigPodTrigger <- true
				logger.GrpcLog.Infoln("Send config trigger to main routine first time config")
			}
		} else {
			// all slices deleted
			if len(context.PlmnList) == 0 {
				minConfig = false
				ConfigPodTrigger <- false
				logger.GrpcLog.Infoln("Send config trigger to main routine config deleted")
			} else {
				ConfigPodTrigger <- true
				logger.GrpcLog.Infoln("Send config trigger to main routine config updated")
			}
		}
	}
	return true
}

func (ausf *AUSF) Start() {
	initLog.Infoln("Server started")

	router := logger_util.NewGinWithLogrus(logger.GinLog)
	ueauthentication.AddService(router)

	ausf_context.Init()
	self := ausf_context.GetSelf()

	ausfLogPath := util.AusfLogPath

	addr := fmt.Sprintf("%s:%d", self.BindingIPv4, self.SBIPort)

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		ausf.Terminate()
		os.Exit(0)
	}()

	server, err := http2_util.NewServer(addr, ausfLogPath, router)
	if server == nil {
		initLog.Errorf("Initialize HTTP server failed: %+v", err)
		return
	}

	if err != nil {
		initLog.Warnf("Initialize HTTP server: +%v", err)
	}

	err = server.ListenAndServe()
	if err != nil {
		initLog.Fatalf("HTTP server setup failed: %+v", err)
	}
}

func (ausf *AUSF) Terminate() {
	logger.InitLog.Infof("AUSF terminated")
}
