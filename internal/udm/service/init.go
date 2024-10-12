package service

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/omec-project/util/http2_util"
	logger_util "github.com/omec-project/util/logger"
	"github.com/omec-project/util/path_util"
	"github.com/sirupsen/logrus"
	"github.com/yeastengine/config5g/proto/client"
	protos "github.com/yeastengine/config5g/proto/sdcoreConfig"
	"github.com/yeastengine/ella/internal/udm/context"
	"github.com/yeastengine/ella/internal/udm/eventexposure"
	"github.com/yeastengine/ella/internal/udm/factory"
	"github.com/yeastengine/ella/internal/udm/httpcallback"
	"github.com/yeastengine/ella/internal/udm/logger"
	"github.com/yeastengine/ella/internal/udm/parameterprovision"
	"github.com/yeastengine/ella/internal/udm/subscriberdatamanagement"
	"github.com/yeastengine/ella/internal/udm/ueauthentication"
	"github.com/yeastengine/ella/internal/udm/uecontextmanagement"
	"github.com/yeastengine/ella/internal/udm/util"
)

type UDM struct{}

var ConfigPodTrigger chan bool

func init() {
	ConfigPodTrigger = make(chan bool)
}

var (
	KeepAliveTimer      *time.Timer
	KeepAliveTimerMutex sync.Mutex
)

var initLog *logrus.Entry

func init() {
	initLog = logger.InitLog
}

func (udm *UDM) Initialize(c factory.Config) error {
	factory.InitConfigFactory(c)
	udm.setLogLevel()
	commChannel := client.ConfigWatcher(factory.UdmConfig.Configuration.WebuiUri, "udm")
	go udm.updateConfig(commChannel)
	return nil
}

func (udm *UDM) setLogLevel() {
	if level, err := logrus.ParseLevel(factory.UdmConfig.Logger.UDM.DebugLevel); err != nil {
		initLog.Warnf("UDM Log level [%s] is invalid, set to [info] level",
			factory.UdmConfig.Logger.UDM.DebugLevel)
		logger.SetLogLevel(logrus.InfoLevel)
	} else {
		initLog.Infof("UDM Log level is set to [%s] level", level)
		logger.SetLogLevel(level)
	}
	logger.SetReportCaller(factory.UdmConfig.Logger.UDM.ReportCaller)
}

func (udm *UDM) Start() {
	config := factory.UdmConfig
	configuration := config.Configuration
	serviceName := configuration.ServiceNameList

	initLog.Infof("UDM Config Info: Version[%s] Description[%s]", config.Info.Version, config.Info.Description)

	initLog.Infoln("Server started")

	router := logger_util.NewGinWithLogrus(logger.GinLog)

	eventexposure.AddService(router)
	httpcallback.AddService(router)
	parameterprovision.AddService(router)
	subscriberdatamanagement.AddService(router)
	ueauthentication.AddService(router)
	uecontextmanagement.AddService(router)

	udmLogPath := path_util.Free5gcPath("omec-project/udmsslkey.log")

	self := context.UDM_Self()
	util.InitUDMContext(self)
	context.UDM_Self().InitNFService(serviceName, config.Info.Version)

	addr := fmt.Sprintf("%s:%d", self.BindingIPv4, self.SBIPort)

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		udm.Terminate()
		os.Exit(0)
	}()

	server, err := http2_util.NewServer(addr, udmLogPath, router)
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

func (udm *UDM) Terminate() {
	logger.InitLog.Infof("UDM terminated")
}

func (udm *UDM) updateConfig(commChannel chan *protos.NetworkSliceResponse) bool {
	var minConfig bool
	self := context.UDM_Self()
	for rsp := range commChannel {
		logger.GrpcLog.Infoln("Received updateConfig in the udm app : ", rsp)
		for _, ns := range rsp.NetworkSlice {
			logger.GrpcLog.Infoln("Network Slice Name ", ns.Name)
			if ns.Site != nil {
				temp := factory.PlmnSupportItem{}
				var found bool = false
				logger.GrpcLog.Infoln("Network Slice has site name present ")
				site := ns.Site
				logger.GrpcLog.Infoln("Site name ", site.SiteName)
				if site.Plmn != nil {
					temp.PlmnId.Mcc = site.Plmn.Mcc
					temp.PlmnId.Mnc = site.Plmn.Mnc
					logger.GrpcLog.Infoln("Plmn mcc ", site.Plmn.Mcc)
					for _, item := range self.PlmnList {
						if item.PlmnId.Mcc == temp.PlmnId.Mcc && item.PlmnId.Mnc == temp.PlmnId.Mnc {
							found = true
							break
						}
					}
					if !found {
						self.PlmnList = append(self.PlmnList, temp)
						logger.GrpcLog.Infoln("Plmn added in the context", self.PlmnList)
					}
				} else {
					logger.GrpcLog.Infoln("Plmn not present in the message ")
				}
			}
		}
		if !minConfig {
			// first slice Created
			if len(self.PlmnList) > 0 {
				minConfig = true
				ConfigPodTrigger <- true
				logger.GrpcLog.Infoln("Send config trigger to main routine")
			}
		} else {
			// all slices deleted
			if len(self.PlmnList) == 0 {
				minConfig = false
				ConfigPodTrigger <- false
				logger.GrpcLog.Infoln("Send config trigger to main routine")
			} else {
				ConfigPodTrigger <- true
				logger.GrpcLog.Infoln("Send config trigger to main routine")
			}
		}
	}
	return true
}
