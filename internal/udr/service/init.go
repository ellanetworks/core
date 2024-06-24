package service

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/http2_util"
	logger_util "github.com/omec-project/util/logger"
	"github.com/sirupsen/logrus"
	"github.com/yeastengine/ella/internal/udr/consumer"
	"github.com/yeastengine/ella/internal/udr/context"
	udr_context "github.com/yeastengine/ella/internal/udr/context"
	"github.com/yeastengine/ella/internal/udr/datarepository"
	"github.com/yeastengine/ella/internal/udr/factory"
	"github.com/yeastengine/ella/internal/udr/logger"
	"github.com/yeastengine/ella/internal/udr/producer"
	"github.com/yeastengine/ella/internal/udr/util"
)

type UDR struct{}

var initLog *logrus.Entry

var (
	KeepAliveTimer      *time.Timer
	KeepAliveTimerMutex sync.Mutex
)

func init() {
	initLog = logger.InitLog
}

func (udr *UDR) Initialize(c factory.Config) error {
	factory.InitConfigFactory(c)
	udr.setLogLevel()
	return nil
}

func (udr *UDR) setLogLevel() {
	if level, err := logrus.ParseLevel(factory.UdrConfig.Logger.UDR.DebugLevel); err != nil {
		initLog.Warnf("UDR Log level [%s] is invalid, set to [info] level",
			factory.UdrConfig.Logger.UDR.DebugLevel)
		logger.SetLogLevel(logrus.InfoLevel)
	} else {
		initLog.Infof("UDR Log level is set to [%s] level", level)
		logger.SetLogLevel(level)
	}
	logger.SetReportCaller(factory.UdrConfig.Logger.UDR.ReportCaller)
}

func (udr *UDR) Start() {
	// get config file info
	config := factory.UdrConfig
	mongodb := config.Configuration.Mongodb
	initLog.Infof("UDR Config Info: Version[%s] Description[%s]", config.Info.Version, config.Info.Description)

	// Connect to MongoDB
	producer.ConnectMongo(mongodb.Url, mongodb.Name, mongodb.AuthUrl, mongodb.AuthKeysDbName)
	initLog.Infoln("Server started")

	router := logger_util.NewGinWithLogrus(logger.GinLog)

	datarepository.AddService(router)

	udrLogPath := util.UdrLogPath

	self := udr_context.UDR_Self()
	util.InitUdrContext(self)

	addr := fmt.Sprintf("%s:%d", self.BindingIPv4, self.SBIPort)

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		udr.Terminate()
		os.Exit(0)
	}()

	go udr.registerNF()
	go udr.configUpdateDb()

	server, err := http2_util.NewServer(addr, udrLogPath, router)
	if server == nil {
		initLog.Errorf("Initialize HTTP server failed: %+v", err)
		return
	}

	if err != nil {
		initLog.Warnf("Initialize HTTP server: %+v", err)
	}

	err = server.ListenAndServe()
	if err != nil {
		initLog.Fatalf("HTTP server setup failed: %+v", err)
	}
}

func (udr *UDR) Terminate() {
	logger.InitLog.Infof("Terminating UDR...")
	// deregister with NRF
	problemDetails, err := consumer.SendDeregisterNFInstance()
	if problemDetails != nil {
		logger.InitLog.Errorf("Deregister NF instance Failed Problem[%+v]", problemDetails)
	} else if err != nil {
		logger.InitLog.Errorf("Deregister NF instance Error[%+v]", err)
	} else {
		logger.InitLog.Infof("Deregister from NRF successfully")
	}
	logger.InitLog.Infof("UDR terminated")
}

func (udr *UDR) configUpdateDb() {
	for msg := range factory.ConfigUpdateDbTrigger {
		initLog.Infof("Config update DB trigger")
		err := producer.AddEntrySmPolicyTable(
			msg.SmPolicyTable.Imsi,
			msg.SmPolicyTable.Dnn,
			msg.SmPolicyTable.Snssai)
		if err == nil {
			initLog.Infof("added entry to sm policy table success")
		} else {
			initLog.Errorf("entry add failed %+v", err)
		}
	}
}

func (udr *UDR) StartKeepAliveTimer(nfProfile models.NfProfile) {
	KeepAliveTimerMutex.Lock()
	defer KeepAliveTimerMutex.Unlock()
	udr.StopKeepAliveTimer()
	if nfProfile.HeartBeatTimer == 0 {
		nfProfile.HeartBeatTimer = 60
	}
	logger.InitLog.Infof("Started KeepAlive Timer: %v sec", nfProfile.HeartBeatTimer)
	// AfterFunc starts timer and waits for KeepAliveTimer to elapse and then calls udr.UpdateNF function
	KeepAliveTimer = time.AfterFunc(time.Duration(nfProfile.HeartBeatTimer)*time.Second, udr.UpdateNF)
}

func (udr *UDR) StopKeepAliveTimer() {
	if KeepAliveTimer != nil {
		logger.InitLog.Infof("Stopped KeepAlive Timer.")
		KeepAliveTimer.Stop()
		KeepAliveTimer = nil
	}
}

func (udr *UDR) BuildAndSendRegisterNFInstance() (prof models.NfProfile, err error) {
	self := context.UDR_Self()
	profile := consumer.BuildNFInstance(self)
	initLog.Infof("Pcf Profile Registering to NRF: %v", profile)
	// Indefinite attempt to register until success
	profile, _, self.NfId, err = consumer.SendRegisterNFInstance(self.NrfUri, self.NfId, profile)
	return profile, err
}

// UpdateNF is the callback function, this is called when keepalivetimer elapsed
func (udr *UDR) UpdateNF() {
	KeepAliveTimerMutex.Lock()
	defer KeepAliveTimerMutex.Unlock()
	if KeepAliveTimer == nil {
		initLog.Warnf("KeepAlive timer has been stopped.")
		return
	}
	// setting default value 30 sec
	var heartBeatTimer int32 = 60
	pitem := models.PatchItem{
		Op:    "replace",
		Path:  "/nfStatus",
		Value: "REGISTERED",
	}
	var patchItem []models.PatchItem
	patchItem = append(patchItem, pitem)
	nfProfile, problemDetails, err := consumer.SendUpdateNFInstance(patchItem)
	if problemDetails != nil {
		initLog.Errorf("UDR update to NRF ProblemDetails[%v]", problemDetails)
		// 5xx response from NRF, 404 Not Found, 400 Bad Request
		if (problemDetails.Status/100) == 5 ||
			problemDetails.Status == 404 || problemDetails.Status == 400 {
			// register with NRF full profile
			nfProfile, err = udr.BuildAndSendRegisterNFInstance()
			if err != nil {
				initLog.Errorf("UDR register to NRF Error[%s]", err.Error())
			}
		}
	} else if err != nil {
		initLog.Errorf("UDR update to NRF Error[%s]", err.Error())
		nfProfile, err = udr.BuildAndSendRegisterNFInstance()
		if err != nil {
			initLog.Errorf("UDR register to NRF Error[%s]", err.Error())
		}
	}

	if nfProfile.HeartBeatTimer != 0 {
		// use hearbeattimer value with received timer value from NRF
		heartBeatTimer = nfProfile.HeartBeatTimer
	}
	logger.InitLog.Debugf("Restarted KeepAlive Timer: %v sec", heartBeatTimer)
	// restart timer with received HeartBeatTimer value
	KeepAliveTimer = time.AfterFunc(time.Duration(heartBeatTimer)*time.Second, udr.UpdateNF)
}

func (udr *UDR) registerNF() {
	for msg := range factory.ConfigPodTrigger {
		initLog.Infof("Minimum configuration from config pod available %v", msg)
		self := udr_context.UDR_Self()
		profile := consumer.BuildNFInstance(self)
		var err error
		var prof models.NfProfile
		// send registration with updated PLMN Ids.
		prof, _, self.NfId, err = consumer.SendRegisterNFInstance(self.NrfUri, profile.NfInstanceId, profile)
		if err == nil {
			udr.StartKeepAliveTimer(prof)
			logger.CfgLog.Infoln("Sent Register NF Instance with updated profile")
		} else {
			initLog.Errorf("Send Register NFInstance Error[%s]", err.Error())
		}
	}
}
