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
	"github.com/yeastengine/ella/internal/nssf/consumer"
	"github.com/yeastengine/ella/internal/nssf/context"
	"github.com/yeastengine/ella/internal/nssf/factory"
	"github.com/yeastengine/ella/internal/nssf/logger"
	"github.com/yeastengine/ella/internal/nssf/nssaiavailability"
	"github.com/yeastengine/ella/internal/nssf/nsselection"
	"github.com/yeastengine/ella/internal/nssf/util"
)

type NSSF struct{}

var initLog *logrus.Entry

var (
	KeepAliveTimer      *time.Timer
	KeepAliveTimerMutex sync.Mutex
)

func init() {
	initLog = logger.InitLog
}

func (nssf *NSSF) Initialize(c factory.Config) error {
	factory.InitConfigFactory(c)
	context.InitNssfContext()
	nssf.setLogLevel()
	return nil
}

func (nssf *NSSF) setLogLevel() {
	if level, err := logrus.ParseLevel(factory.NssfConfig.Logger.NSSF.DebugLevel); err != nil {
		initLog.Warnf("NSSF Log level [%s] is invalid, set to [info] level",
			factory.NssfConfig.Logger.NSSF.DebugLevel)
		logger.SetLogLevel(logrus.InfoLevel)
	} else {
		initLog.Infof("NSSF Log level is set to [%s] level", level)
		logger.SetLogLevel(level)
	}
	logger.SetReportCaller(factory.NssfConfig.Logger.NSSF.ReportCaller)
}

func (nssf *NSSF) Start() {
	initLog.Infoln("Server started")

	router := logger_util.NewGinWithLogrus(logger.GinLog)

	nssaiavailability.AddService(router)
	nsselection.AddService(router)

	self := context.NSSF_Self()
	addr := fmt.Sprintf("%s:%d", self.BindingIPv4, self.SBIPort)

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		nssf.Terminate()
		os.Exit(0)
	}()

	go nssf.registerNF()

	server, err := http2_util.NewServer(addr, util.NSSF_LOG_PATH, router)

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

func (nssf *NSSF) Terminate() {
	logger.InitLog.Infof("Terminating NSSF...")
	// deregister with NRF
	problemDetails, err := consumer.SendDeregisterNFInstance()
	if problemDetails != nil {
		logger.InitLog.Errorf("Deregister NF instance Failed Problem[%+v]", problemDetails)
	} else if err != nil {
		logger.InitLog.Errorf("Deregister NF instance Error[%+v]", err)
	} else {
		logger.InitLog.Infof("Deregister from NRF successfully")
	}

	logger.InitLog.Infof("NSSF terminated")
}

func (nssf *NSSF) StartKeepAliveTimer(nfProfile models.NfProfile) {
	KeepAliveTimerMutex.Lock()
	defer KeepAliveTimerMutex.Unlock()
	nssf.StopKeepAliveTimer()
	if nfProfile.HeartBeatTimer == 0 {
		nfProfile.HeartBeatTimer = 60
	}
	logger.InitLog.Infof("Started KeepAlive Timer: %v sec", nfProfile.HeartBeatTimer)
	// AfterFunc starts timer and waits for KeepAliveTimer to elapse and then calls nssf.UpdateNF function
	KeepAliveTimer = time.AfterFunc(time.Duration(nfProfile.HeartBeatTimer)*time.Second, nssf.UpdateNF)
}

func (nssf *NSSF) StopKeepAliveTimer() {
	if KeepAliveTimer != nil {
		logger.InitLog.Infof("Stopped KeepAlive Timer.")
		KeepAliveTimer.Stop()
		KeepAliveTimer = nil
	}
}

func (nssf *NSSF) BuildAndSendRegisterNFInstance() (models.NfProfile, error) {
	self := context.NSSF_Self()
	profile, err := consumer.BuildNFProfile(self)
	if err != nil {
		initLog.Errorf("Build NSSF Profile Error: %v", err)
		return profile, err
	}
	initLog.Infof("Pcf Profile Registering to NRF: %v", profile)
	// Indefinite attempt to register until success
	profile, _, self.NfId, err = consumer.SendRegisterNFInstance(self.NrfUri, self.NfId, profile)
	return profile, err
}

// UpdateNF is the callback function, this is called when keepalivetimer elapsed
func (nssf *NSSF) UpdateNF() {
	KeepAliveTimerMutex.Lock()
	defer KeepAliveTimerMutex.Unlock()
	if KeepAliveTimer == nil {
		initLog.Warnf("KeepAlive timer has been stopped.")
		return
	}
	// setting default value 60 sec
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
		initLog.Errorf("NSSF update to NRF ProblemDetails[%v]", problemDetails)
		// 5xx response from NRF, 404 Not Found, 400 Bad Request
		if (problemDetails.Status >= 500 && problemDetails.Status <= 599) ||
			problemDetails.Status == 404 || problemDetails.Status == 400 {
			// register with NRF full profile
			nfProfile, err = nssf.BuildAndSendRegisterNFInstance()
			if err != nil {
				initLog.Errorf("NSSF update to NRF Error[%s]", err.Error())
			}
		}
	} else if err != nil {
		initLog.Errorf("NSSF update to NRF Error[%s]", err.Error())
		nfProfile, err = nssf.BuildAndSendRegisterNFInstance()
		if err != nil {
			initLog.Errorf("NSSF update to NRF Error[%s]", err.Error())
		}
	}

	if nfProfile.HeartBeatTimer != 0 {
		// use hearbeattimer value with received timer value from NRF
		heartBeatTimer = nfProfile.HeartBeatTimer
	}
	logger.InitLog.Debugf("Restarted KeepAlive Timer: %v sec", heartBeatTimer)
	// restart timer with received HeartBeatTimer value
	KeepAliveTimer = time.AfterFunc(time.Duration(heartBeatTimer)*time.Second, nssf.UpdateNF)
}

func (nssf *NSSF) registerNF() {
	for msg := range factory.ConfigPodTrigger {
		initLog.Infof("Minimum configuration from config pod available %v", msg)
		self := context.NSSF_Self()
		profile, err := consumer.BuildNFProfile(self)
		if err != nil {
			logger.InitLog.Errorf("Build profile failed.")
		}

		var newNrfUri string
		var prof models.NfProfile
		// send registration with updated PLMN Ids.
		prof, newNrfUri, self.NfId, err = consumer.SendRegisterNFInstance(self.NrfUri, profile.NfInstanceId, profile)
		if err == nil {
			nssf.StartKeepAliveTimer(prof)
			logger.CfgLog.Infof("Sent Register NF Instance with updated profile")
			self.NrfUri = newNrfUri
		} else {
			initLog.Errorf("Send Register NFInstance Error[%s]", err.Error())
		}
	}
}
