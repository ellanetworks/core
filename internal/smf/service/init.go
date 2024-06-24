package service

import (
	"fmt"
	"net/http"
	_ "net/http/pprof" // Using package only for invoking initialization.
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	aperLogger "github.com/omec-project/aper/logger"
	nasLogger "github.com/omec-project/nas/logger"
	ngapLogger "github.com/omec-project/ngap/logger"
	"github.com/omec-project/openapi/models"
	pfcpLogger "github.com/omec-project/pfcp/logger"
	"github.com/omec-project/pfcp/pfcpType"
	"github.com/omec-project/util/http2_util"
	logger_util "github.com/omec-project/util/logger"
	"github.com/omec-project/util/path_util"
	pathUtilLogger "github.com/omec-project/util/path_util/logger"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	nrf_cache "github.com/yeastengine/ella/internal/nrf/nrfcache"
	"github.com/yeastengine/ella/internal/smf/callback"
	"github.com/yeastengine/ella/internal/smf/consumer"
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

type (
	// Config information.
	Config struct {
		smfcfg    string
		uerouting string
	}
)

var refreshNrfRegistration bool

var config Config

var smfCLi = []cli.Flag{
	cli.StringFlag{
		Name:  "cfg",
		Usage: "common config file",
	},
	cli.StringFlag{
		Name:  "smfcfg",
		Usage: "config file",
	},
	cli.StringFlag{
		Name:  "uerouting",
		Usage: "config file",
	},
}

var (
	KeepAliveTimer      *time.Timer
	KeepAliveTimerMutex sync.Mutex
)

type OneInstance struct {
	m    sync.Mutex
	done uint32
}

var nrfRegInProgress OneInstance

var initLog *logrus.Entry

func init() {
	initLog = logger.InitLog
	nrfRegInProgress = OneInstance{}
}

func (*SMF) GetCliCmd() (flags []cli.Flag) {
	return smfCLi
}

func (smf *SMF) Initialize(c *cli.Context) error {
	config = Config{
		smfcfg:    c.String("smfcfg"),
		uerouting: c.String("uerouting"),
	}

	if config.smfcfg != "" {
		if err := factory.InitConfigFactory(config.smfcfg); err != nil {
			return err
		}
	} else {
		DefaultSmfConfigPath := path_util.Free5gcPath("omec-project/smf/config/smfcfg.yaml")
		if err := factory.InitConfigFactory(DefaultSmfConfigPath); err != nil {
			return err
		}
	}

	if config.uerouting != "" {
		if err := factory.InitRoutingConfigFactory(config.uerouting); err != nil {
			return err
		}
	} else {
		DefaultUERoutingPath := path_util.Free5gcPath("omec-project/smf/config/uerouting.yaml")
		if err := factory.InitRoutingConfigFactory(DefaultUERoutingPath); err != nil {
			return err
		}
	}

	smf.setLogLevel()

	// Initiating a server for profiling
	if factory.SmfConfig.Configuration.DebugProfilePort != 0 {
		addr := fmt.Sprintf(":%d", factory.SmfConfig.Configuration.DebugProfilePort)
		go func() {
			http.ListenAndServe(addr, nil)
		}()
	}

	return nil
}

func (smf *SMF) setLogLevel() {
	if factory.SmfConfig.Logger == nil {
		initLog.Warnln("SMF config without log level setting!!!")
		return
	}

	if factory.SmfConfig.Logger.SMF != nil {
		if factory.SmfConfig.Logger.SMF.DebugLevel != "" {
			if level, err := logrus.ParseLevel(factory.SmfConfig.Logger.SMF.DebugLevel); err != nil {
				initLog.Warnf("SMF Log level [%s] is invalid, set to [info] level",
					factory.SmfConfig.Logger.SMF.DebugLevel)
				logger.SetLogLevel(logrus.InfoLevel)
			} else {
				initLog.Infof("SMF Log level is set to [%s] level", level)
				logger.SetLogLevel(level)
			}
		} else {
			initLog.Infoln("SMF Log level is default set to [info] level")
			logger.SetLogLevel(logrus.InfoLevel)
		}
		logger.SetReportCaller(factory.SmfConfig.Logger.SMF.ReportCaller)
	}

	if factory.SmfConfig.Logger.NAS != nil {
		if factory.SmfConfig.Logger.NAS.DebugLevel != "" {
			if level, err := logrus.ParseLevel(factory.SmfConfig.Logger.NAS.DebugLevel); err != nil {
				nasLogger.NasLog.Warnf("NAS Log level [%s] is invalid, set to [info] level",
					factory.SmfConfig.Logger.NAS.DebugLevel)
				logger.SetLogLevel(logrus.InfoLevel)
			} else {
				nasLogger.SetLogLevel(level)
			}
		} else {
			nasLogger.NasLog.Warnln("NAS Log level not set. Default set to [info] level")
			nasLogger.SetLogLevel(logrus.InfoLevel)
		}
		nasLogger.SetReportCaller(factory.SmfConfig.Logger.NAS.ReportCaller)
	}

	if factory.SmfConfig.Logger.NGAP != nil {
		if factory.SmfConfig.Logger.NGAP.DebugLevel != "" {
			if level, err := logrus.ParseLevel(factory.SmfConfig.Logger.NGAP.DebugLevel); err != nil {
				ngapLogger.NgapLog.Warnf("NGAP Log level [%s] is invalid, set to [info] level",
					factory.SmfConfig.Logger.NGAP.DebugLevel)
				ngapLogger.SetLogLevel(logrus.InfoLevel)
			} else {
				ngapLogger.SetLogLevel(level)
			}
		} else {
			ngapLogger.NgapLog.Warnln("NGAP Log level not set. Default set to [info] level")
			ngapLogger.SetLogLevel(logrus.InfoLevel)
		}
		ngapLogger.SetReportCaller(factory.SmfConfig.Logger.NGAP.ReportCaller)
	}

	if factory.SmfConfig.Logger.Aper != nil {
		if factory.SmfConfig.Logger.Aper.DebugLevel != "" {
			if level, err := logrus.ParseLevel(factory.SmfConfig.Logger.Aper.DebugLevel); err != nil {
				aperLogger.AperLog.Warnf("Aper Log level [%s] is invalid, set to [info] level",
					factory.SmfConfig.Logger.Aper.DebugLevel)
				aperLogger.SetLogLevel(logrus.InfoLevel)
			} else {
				aperLogger.SetLogLevel(level)
			}
		} else {
			aperLogger.AperLog.Warnln("Aper Log level not set. Default set to [info] level")
			aperLogger.SetLogLevel(logrus.InfoLevel)
		}
		aperLogger.SetReportCaller(factory.SmfConfig.Logger.Aper.ReportCaller)
	}

	if factory.SmfConfig.Logger.PathUtil != nil {
		if factory.SmfConfig.Logger.PathUtil.DebugLevel != "" {
			if level, err := logrus.ParseLevel(factory.SmfConfig.Logger.PathUtil.DebugLevel); err != nil {
				pathUtilLogger.PathLog.Warnf("PathUtil Log level [%s] is invalid, set to [info] level",
					factory.SmfConfig.Logger.PathUtil.DebugLevel)
				pathUtilLogger.SetLogLevel(logrus.InfoLevel)
			} else {
				pathUtilLogger.SetLogLevel(level)
			}
		} else {
			pathUtilLogger.PathLog.Warnln("PathUtil Log level not set. Default set to [info] level")
			pathUtilLogger.SetLogLevel(logrus.InfoLevel)
		}
		pathUtilLogger.SetReportCaller(factory.SmfConfig.Logger.PathUtil.ReportCaller)
	}

	if factory.SmfConfig.Logger.PFCP != nil {
		if factory.SmfConfig.Logger.PFCP.DebugLevel != "" {
			if level, err := logrus.ParseLevel(factory.SmfConfig.Logger.PFCP.DebugLevel); err != nil {
				pfcpLogger.PFCPLog.Warnf("PFCP Log level [%s] is invalid, set to [info] level",
					factory.SmfConfig.Logger.PFCP.DebugLevel)
				pfcpLogger.SetLogLevel(logrus.InfoLevel)
			} else {
				pfcpLogger.SetLogLevel(level)
			}
		} else {
			pfcpLogger.PFCPLog.Warnln("PFCP Log level not set. Default set to [info] level")
			pfcpLogger.SetLogLevel(logrus.InfoLevel)
		}
		pfcpLogger.SetReportCaller(factory.SmfConfig.Logger.PFCP.ReportCaller)
	}
}

func (smf *SMF) FilterCli(c *cli.Context) (args []string) {
	for _, flag := range smf.GetCliCmd() {
		name := flag.GetName()
		value := fmt.Sprint(c.Generic(name))
		if value == "" {
			continue
		}

		args = append(args, "--"+name, value)
	}
	return args
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
	smfCtxt := context.InitSmfContext(&factory.SmfConfig)

	// allocate id for each upf
	context.AllocateUPFID()

	// Init UE Specific Config
	context.InitSMFUERouting(&factory.UERoutingConfig)

	// Wait for additional/updated config from config pod
	initLog.Infof("Configuration is managed by Config Pod")
	initLog.Infof("waiting for initial configuration from config pod")

	// Main thread should be blocked for config update from ROC
	// Future config update from ROC can be handled via background go-routine.
	if <-factory.ConfigPodTrigger {
		initLog.Infof("minimum configuration from config pod available")
		context.ProcessConfigUpdate()
	}

	// Trigger background goroutine to handle further config updates
	go func() {
		initLog.Infof("Dynamic config update task initialised")
		for {
			if <-factory.ConfigPodTrigger {
				if context.ProcessConfigUpdate() {
					// Let NRF registration happen in background
					go smf.SendNrfRegistration()
				}
			}
		}
	}()

	// Send NRF Registration
	smf.SendNrfRegistration()

	if smfCtxt.EnableNrfCaching {
		initLog.Infof("Enable NRF caching feature for %d seconds", smfCtxt.NrfCacheEvictionInterval)
		nrf_cache.InitNrfCaching(smfCtxt.NrfCacheEvictionInterval*time.Second, consumer.SendNrfForNfInstance)
	}

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

	if factory.SmfConfig.Configuration.EnableDbStore {
		initLog.Infof("SetupSmfCollection")
		context.SetupSmfCollection()
	}

	// Init DRSM for unique FSEID/FTEID/IP-Addr
	if err := smfCtxt.InitDrsm(); err != nil {
		initLog.Errorf("initialse drsm failed, %v ", err.Error())
	}

	udp.Run(pfcp.Dispatch)

	for _, upf := range context.SMF_Self().UserPlaneInformation.UPFs {
		if upf.NodeID.NodeIdType == pfcpType.NodeIdTypeFqdn {
			logger.AppLog.Infof("Send PFCP Association Request to UPF[%s](%s)\n", upf.NodeID.NodeIdValue,
				upf.NodeID.ResolveNodeIdToIp().String())
		} else {
			logger.AppLog.Infof("Send PFCP Association Request to UPF[%s]\n", upf.NodeID.ResolveNodeIdToIp().String())
		}
		message.SendPfcpAssociationSetupRequest(upf.NodeID, upf.Port)
	}

	// Trigger PFCP Heartbeat towards all connected UPFs
	go upf.InitPfcpHeartbeatRequest(context.SMF_Self().UserPlaneInformation)

	// Trigger PFCP association towards not associated UPFs
	go upf.ProbeInactiveUpfs(context.SMF_Self().UserPlaneInformation)

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

	serverScheme := factory.SmfConfig.Configuration.Sbi.Scheme
	if serverScheme == "http" {
		err = server.ListenAndServe()
	} else if serverScheme == "https" {
		err = server.ListenAndServeTLS(util.SmfPemPath, util.SmfKeyPath)
	}

	if err != nil {
		initLog.Fatalln("HTTP server setup failed:", err)
	}
}

func (smf *SMF) Terminate() {
	logger.InitLog.Infof("Terminating SMF...")
	// deregister with NRF
	problemDetails, err := consumer.SendDeregisterNFInstance()
	if problemDetails != nil {
		logger.InitLog.Errorf("Deregister NF instance Failed Problem[%+v]", problemDetails)
	} else if err != nil {
		logger.InitLog.Errorf("Deregister NF instance Error[%+v]", err)
	} else {
		logger.InitLog.Infof("Deregister from NRF successfully")
	}
}

func (smf *SMF) Exec(c *cli.Context) error {
	return nil
}

func StartKeepAliveTimer(nfProfile *models.NfProfile) {
	KeepAliveTimerMutex.Lock()
	defer KeepAliveTimerMutex.Unlock()
	StopKeepAliveTimer()
	if nfProfile.HeartBeatTimer == 0 {
		nfProfile.HeartBeatTimer = 30
	}
	logger.InitLog.Infof("Started KeepAlive Timer: %v sec", nfProfile.HeartBeatTimer)
	// AfterFunc starts timer and waits for KeepAliveTimer to elapse and then calls smf.UpdateNF function
	KeepAliveTimer = time.AfterFunc(time.Duration(nfProfile.HeartBeatTimer)*time.Second, UpdateNF)
}

func StopKeepAliveTimer() {
	if KeepAliveTimer != nil {
		logger.InitLog.Infof("Stopped KeepAlive Timer.")
		KeepAliveTimer.Stop()
		KeepAliveTimer = nil
	}
}

// UpdateNF is the callback function, this is called when keepalivetimer elapsed
func UpdateNF() {
	KeepAliveTimerMutex.Lock()
	defer KeepAliveTimerMutex.Unlock()
	if KeepAliveTimer == nil {
		initLog.Warnf("KeepAlive timer has been stopped.")
		return
	}
	// setting default value 30 sec
	var heartBeatTimer int32 = 30
	pitem := models.PatchItem{
		Op:    "replace",
		Path:  "/nfStatus",
		Value: "REGISTERED",
	}
	var patchItem []models.PatchItem
	patchItem = append(patchItem, pitem)
	nfProfile, problemDetails, err := consumer.SendUpdateNFInstance(patchItem)
	if problemDetails != nil {
		initLog.Errorf("SMF update to NRF ProblemDetails[%v]", problemDetails)
		// 5xx response from NRF, 404 Not Found, 400 Bad Request
		if (problemDetails.Status/100) == 5 ||
			problemDetails.Status == 404 || problemDetails.Status == 400 {
			// register with NRF full profile
			nfProfile, err = consumer.SendNFRegistration()
		}
	} else if err != nil {
		initLog.Errorf("SMF update to NRF Error[%s]", err.Error())
		nfProfile, err = consumer.SendNFRegistration()
	}

	if nfProfile.HeartBeatTimer != 0 {
		// use hearbeattimer value with received timer value from NRF
		heartBeatTimer = nfProfile.HeartBeatTimer
	}
	logger.InitLog.Debugf("Restarted KeepAlive Timer: %v sec", heartBeatTimer)
	// restart timer with received HeartBeatTimer value
	KeepAliveTimer = time.AfterFunc(time.Duration(heartBeatTimer)*time.Second, UpdateNF)
}

func (smf *SMF) SendNrfRegistration() {
	// If NRF registration is ongoing then don't start another in parallel
	// Just mark it so that once ongoing finishes then resend another
	if nrfRegInProgress.intanceRun(consumer.ReSendNFRegistration) {
		logger.InitLog.Infof("NRF Registration already in progress...")
		refreshNrfRegistration = true
		return
	}

	// Once the first goroutine which was sending NRF registration returns,
	// Check if another fresh NRF registration is required
	if refreshNrfRegistration {
		refreshNrfRegistration = false
		if prof, err := consumer.SendNFRegistration(); err != nil {
			logger.InitLog.Infof("NRF Registration failure, %v", err.Error())
		} else {
			StartKeepAliveTimer(prof)
			logger.CfgLog.Infof("Sent Register NF Instance with updated profile")
		}
	}
}

// Run only single instance of func f at a time
func (o *OneInstance) intanceRun(f func() *models.NfProfile) bool {
	// Instance already running ?
	if atomic.LoadUint32(&o.done) == 1 {
		return true
	}

	// Slow-path.
	o.m.Lock()
	defer o.m.Unlock()
	if o.done == 0 {
		atomic.StoreUint32(&o.done, 1)
		defer atomic.StoreUint32(&o.done, 0)
		nfProfile := f()
		StartKeepAliveTimer(nfProfile)
	}
	return false
}
