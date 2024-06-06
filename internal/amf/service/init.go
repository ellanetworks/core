package service

import (
	"bufio"
	"fmt"
	"net/http"
	_ "net/http/pprof" // Using package only for invoking initialization.
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gin-contrib/cors"
	aperLogger "github.com/omec-project/aper/logger"
	gClient "github.com/omec-project/config5g/proto/client"
	protos "github.com/omec-project/config5g/proto/sdcoreConfig"
	nasLogger "github.com/omec-project/nas/logger"
	ngapLogger "github.com/omec-project/ngap/logger"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/fsm"
	fsmLogger "github.com/omec-project/util/fsm/logger"
	"github.com/omec-project/util/http2_util"
	logger_util "github.com/omec-project/util/logger"
	"github.com/omec-project/util/path_util"
	pathUtilLogger "github.com/omec-project/util/path_util/logger"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/urfave/cli"
	"github.com/yeastengine/ella/internal/amf/communication"
	"github.com/yeastengine/ella/internal/amf/consumer"
	"github.com/yeastengine/ella/internal/amf/context"
	"github.com/yeastengine/ella/internal/amf/eventexposure"
	"github.com/yeastengine/ella/internal/amf/factory"
	"github.com/yeastengine/ella/internal/amf/gmm"
	"github.com/yeastengine/ella/internal/amf/httpcallback"
	"github.com/yeastengine/ella/internal/amf/location"
	"github.com/yeastengine/ella/internal/amf/logger"
	"github.com/yeastengine/ella/internal/amf/mt"
	"github.com/yeastengine/ella/internal/amf/ngap"
	ngap_message "github.com/yeastengine/ella/internal/amf/ngap/message"
	ngap_service "github.com/yeastengine/ella/internal/amf/ngap/service"
	"github.com/yeastengine/ella/internal/amf/oam"
	"github.com/yeastengine/ella/internal/amf/producer/callback"
	"github.com/yeastengine/ella/internal/amf/util"
	nrf_cache "github.com/yeastengine/ella/internal/nrf/nrfcache"
)

type AMF struct{}

const IMSI_PREFIX = "imsi-"

var RocUpdateConfigChannel chan bool

type (
	// Config information.
	Config struct {
		amfcfg string
	}
)

var config Config

var amfCLi = []cli.Flag{
	cli.StringFlag{
		Name:  "free5gccfg",
		Usage: "common config file",
	},
	cli.StringFlag{
		Name:  "amfcfg",
		Usage: "amf config file",
	},
}

var initLog *logrus.Entry

var (
	KeepAliveTimer      *time.Timer
	KeepAliveTimerMutex sync.Mutex
)

func init() {
	initLog = logger.InitLog
	RocUpdateConfigChannel = make(chan bool)
}

func (*AMF) GetCliCmd() (flags []cli.Flag) {
	return amfCLi
}

func (amf *AMF) Initialize(c *cli.Context) error {
	config = Config{
		amfcfg: c.String("amfcfg"),
	}

	if config.amfcfg != "" {
		if err := factory.InitConfigFactory(config.amfcfg); err != nil {
			return err
		}
	} else {
		DefaultAmfConfigPath := path_util.Free5gcPath("free5gc/config/amfcfg.yaml")
		if err := factory.InitConfigFactory(DefaultAmfConfigPath); err != nil {
			return err
		}
	}

	amf.setLogLevel()

	// Initiating a server for profiling
	if factory.AmfConfig.Configuration.DebugProfilePort != 0 {
		addr := fmt.Sprintf(":%d", factory.AmfConfig.Configuration.DebugProfilePort)
		go func() {
			if err := http.ListenAndServe(addr, nil); err != nil {
				initLog.Errorln(err)
			}
		}()
	}

	if err := factory.CheckConfigVersion(); err != nil {
		return err
	}

	if _, err := os.Stat("/free5gc/config/amfcfg.conf"); err == nil {
		viper.SetConfigName("amfcfg.conf")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("/free5gc/config")
		err = viper.ReadInConfig() // Find and read the config file
		if err != nil {            // Handle errors reading the config file
			return err
		}
	} else if os.IsNotExist(err) {
		fmt.Println("amfcfg does not exists in /free5gc/config")
	}

	if os.Getenv("MANAGED_BY_CONFIG_POD") == "true" {
		factory.AmfConfig.Configuration.ServedGumaiList = nil
		factory.AmfConfig.Configuration.SupportTAIList = nil
		factory.AmfConfig.Configuration.PlmnSupportList = nil
		initLog.Infoln("Reading Amf related configuration from ROC")
		client := gClient.ConnectToConfigServer(factory.AmfConfig.Configuration.WebuiUri)
		configChannel := client.PublishOnConfigChange(true)
		go amf.UpdateConfig(configChannel)
	} else {
		go func() {
			logger.GrpcLog.Infoln("Reading Amf Configuration from Helm")
			// sending true to the channel for sending NFRegistration to NRF
			RocUpdateConfigChannel <- true
		}()
	}

	return nil
}

func (amf *AMF) WatchConfig() {
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed:", e.Name)
		if err := factory.UpdateAmfConfig("/free5gc/config/amfcfg.conf"); err != nil {
			fmt.Println("error in loading updated configuration")
		} else {
			self := context.AMF_Self()
			util.InitAmfContext(self)
			fmt.Println("successfully updated configuration")
		}
	})
}

func (amf *AMF) setLogLevel() {
	if factory.AmfConfig.Logger == nil {
		initLog.Warnln("AMF config without log level setting!!!")
		return
	}

	if factory.AmfConfig.Logger.AMF != nil {
		if factory.AmfConfig.Logger.AMF.DebugLevel != "" {
			if level, err := logrus.ParseLevel(factory.AmfConfig.Logger.AMF.DebugLevel); err != nil {
				initLog.Warnf("AMF Log level [%s] is invalid, set to [info] level",
					factory.AmfConfig.Logger.AMF.DebugLevel)
				logger.SetLogLevel(logrus.InfoLevel)
			} else {
				initLog.Infof("AMF Log level is set to [%s] level", level)
				logger.SetLogLevel(level)
			}
		} else {
			initLog.Warnln("AMF Log level not set. Default set to [info] level")
			logger.SetLogLevel(logrus.InfoLevel)
		}
		logger.SetReportCaller(factory.AmfConfig.Logger.AMF.ReportCaller)
	}

	if factory.AmfConfig.Logger.NAS != nil {
		if factory.AmfConfig.Logger.NAS.DebugLevel != "" {
			if level, err := logrus.ParseLevel(factory.AmfConfig.Logger.NAS.DebugLevel); err != nil {
				nasLogger.NasLog.Warnf("NAS Log level [%s] is invalid, set to [info] level",
					factory.AmfConfig.Logger.NAS.DebugLevel)
				logger.SetLogLevel(logrus.InfoLevel)
			} else {
				nasLogger.SetLogLevel(level)
			}
		} else {
			nasLogger.NasLog.Warnln("NAS Log level not set. Default set to [info] level")
			nasLogger.SetLogLevel(logrus.InfoLevel)
		}
		nasLogger.SetReportCaller(factory.AmfConfig.Logger.NAS.ReportCaller)
	}

	if factory.AmfConfig.Logger.NGAP != nil {
		if factory.AmfConfig.Logger.NGAP.DebugLevel != "" {
			if level, err := logrus.ParseLevel(factory.AmfConfig.Logger.NGAP.DebugLevel); err != nil {
				ngapLogger.NgapLog.Warnf("NGAP Log level [%s] is invalid, set to [info] level",
					factory.AmfConfig.Logger.NGAP.DebugLevel)
				ngapLogger.SetLogLevel(logrus.InfoLevel)
			} else {
				ngapLogger.SetLogLevel(level)
			}
		} else {
			ngapLogger.NgapLog.Warnln("NGAP Log level not set. Default set to [info] level")
			ngapLogger.SetLogLevel(logrus.InfoLevel)
		}
		ngapLogger.SetReportCaller(factory.AmfConfig.Logger.NGAP.ReportCaller)
	}

	if factory.AmfConfig.Logger.FSM != nil {
		if factory.AmfConfig.Logger.FSM.DebugLevel != "" {
			if level, err := logrus.ParseLevel(factory.AmfConfig.Logger.FSM.DebugLevel); err != nil {
				fsmLogger.FsmLog.Warnf("FSM Log level [%s] is invalid, set to [info] level",
					factory.AmfConfig.Logger.FSM.DebugLevel)
				fsmLogger.SetLogLevel(logrus.InfoLevel)
			} else {
				fsmLogger.SetLogLevel(level)
			}
		} else {
			fsmLogger.FsmLog.Warnln("FSM Log level not set. Default set to [info] level")
			fsmLogger.SetLogLevel(logrus.InfoLevel)
		}
		fsmLogger.SetReportCaller(factory.AmfConfig.Logger.FSM.ReportCaller)
	}

	if factory.AmfConfig.Logger.Aper != nil {
		if factory.AmfConfig.Logger.Aper.DebugLevel != "" {
			if level, err := logrus.ParseLevel(factory.AmfConfig.Logger.Aper.DebugLevel); err != nil {
				aperLogger.AperLog.Warnf("Aper Log level [%s] is invalid, set to [info] level",
					factory.AmfConfig.Logger.Aper.DebugLevel)
				aperLogger.SetLogLevel(logrus.InfoLevel)
			} else {
				aperLogger.SetLogLevel(level)
			}
		} else {
			aperLogger.AperLog.Warnln("Aper Log level not set. Default set to [info] level")
			aperLogger.SetLogLevel(logrus.InfoLevel)
		}
		aperLogger.SetReportCaller(factory.AmfConfig.Logger.Aper.ReportCaller)
	}

	if factory.AmfConfig.Logger.PathUtil != nil {
		if factory.AmfConfig.Logger.PathUtil.DebugLevel != "" {
			if level, err := logrus.ParseLevel(factory.AmfConfig.Logger.PathUtil.DebugLevel); err != nil {
				pathUtilLogger.PathLog.Warnf("PathUtil Log level [%s] is invalid, set to [info] level",
					factory.AmfConfig.Logger.PathUtil.DebugLevel)
				pathUtilLogger.SetLogLevel(logrus.InfoLevel)
			} else {
				pathUtilLogger.SetLogLevel(level)
			}
		} else {
			pathUtilLogger.PathLog.Warnln("PathUtil Log level not set. Default set to [info] level")
			pathUtilLogger.SetLogLevel(logrus.InfoLevel)
		}
		pathUtilLogger.SetReportCaller(factory.AmfConfig.Logger.PathUtil.ReportCaller)
	}
}

func (amf *AMF) FilterCli(c *cli.Context) (args []string) {
	for _, flag := range amf.GetCliCmd() {
		name := flag.GetName()
		value := fmt.Sprint(c.Generic(name))
		if value == "" {
			continue
		}

		args = append(args, "--"+name, value)
	}
	return args
}

func (amf *AMF) Start() {
	initLog.Infoln("Server started")
	var err error

	router := logger_util.NewGinWithLogrus(logger.GinLog)
	router.Use(cors.New(cors.Config{
		AllowMethods: []string{"GET", "POST", "OPTIONS", "PUT", "PATCH", "DELETE"},
		AllowHeaders: []string{
			"Origin", "Content-Length", "Content-Type", "User-Agent", "Referrer", "Host",
			"Token", "X-Requested-With",
		},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		AllowAllOrigins:  true,
		MaxAge:           86400,
	}))

	httpcallback.AddService(router)
	oam.AddService(router)
	for _, serviceName := range factory.AmfConfig.Configuration.ServiceNameList {
		switch models.ServiceName(serviceName) {
		case models.ServiceName_NAMF_COMM:
			communication.AddService(router)
		case models.ServiceName_NAMF_EVTS:
			eventexposure.AddService(router)
		case models.ServiceName_NAMF_MT:
			mt.AddService(router)
		case models.ServiceName_NAMF_LOC:
			location.AddService(router)
		}
	}

	self := context.AMF_Self()
	util.InitAmfContext(self)
	self.Drsm, err = util.InitDrsm()
	if err != nil {
		initLog.Errorf("initialise DRSM failed, %v", err.Error())
	}

	addr := fmt.Sprintf("%s:%d", self.BindingIPv4, self.SBIPort)

	ngapHandler := ngap_service.NGAPHandler{
		HandleMessage:      ngap.Dispatch,
		HandleNotification: ngap.HandleSCTPNotification,
	}
	ngap_service.Run(self.NgapIpList, self.NgapPort, ngapHandler)

	go amf.SendNFProfileUpdateToNrf()

	if self.EnableNrfCaching {
		initLog.Infoln("Enable NRF caching feature")
		nrf_cache.InitNrfCaching(self.NrfCacheEvictionInterval*time.Second, consumer.SendNfDiscoveryToNrf)
	}

	if self.EnableSctpLb {
		go StartGrpcServer(self.SctpGrpcPort)
	}

	if self.EnableDbStore {
		go context.SetupAmfCollection()
	}

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		amf.Terminate()
		os.Exit(0)
	}()

	server, err := http2_util.NewServer(addr, util.AmfLogPath, router)

	if server == nil {
		initLog.Errorf("Initialize HTTP server failed: %+v", err)
		return
	}

	if err != nil {
		initLog.Warnf("Initialize HTTP server: %+v", err)
	}

	serverScheme := factory.AmfConfig.Configuration.Sbi.Scheme
	if serverScheme == "http" {
		err = server.ListenAndServe()
	} else if serverScheme == "https" {
		err = server.ListenAndServeTLS(util.AmfPemPath, util.AmfKeyPath)
	}

	if err != nil {
		initLog.Fatalf("HTTP server setup failed: %+v", err)
	}
}

func (amf *AMF) Exec(c *cli.Context) error {
	// AMF.Initialize(cfgPath, c)

	initLog.Traceln("args:", c.String("amfcfg"))
	args := amf.FilterCli(c)
	initLog.Traceln("filter: ", args)
	command := exec.Command("./amf", args...)

	stdout, err := command.StdoutPipe()
	if err != nil {
		initLog.Fatalln(err)
	}
	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() {
		in := bufio.NewScanner(stdout)
		for in.Scan() {
			fmt.Println(in.Text())
		}
		wg.Done()
	}()

	stderr, err := command.StderrPipe()
	if err != nil {
		initLog.Fatalln(err)
	}
	go func() {
		in := bufio.NewScanner(stderr)
		for in.Scan() {
			fmt.Println(in.Text())
		}
		wg.Done()
	}()

	go func() {
		if err = command.Start(); err != nil {
			initLog.Errorf("AMF Start error: %+v", err)
		}
		wg.Done()
	}()

	wg.Wait()

	return err
}

// Used in AMF planned removal procedure
func (amf *AMF) Terminate() {
	logger.InitLog.Infof("Terminating AMF...")
	amfSelf := context.AMF_Self()

	// TODO: forward registered UE contexts to target AMF in the same AMF set if there is one

	// deregister with NRF
	problemDetails, err := consumer.SendDeregisterNFInstance()
	if problemDetails != nil {
		logger.InitLog.Errorf("Deregister NF instance Failed Problem[%+v]", problemDetails)
	} else if err != nil {
		logger.InitLog.Errorf("Deregister NF instance Error[%+v]", err)
	} else {
		logger.InitLog.Infof("[AMF] Deregister from NRF successfully")
	}

	// send AMF status indication to ran to notify ran that this AMF will be unavailable
	logger.InitLog.Infof("Send AMF Status Indication to Notify RANs due to AMF terminating")
	unavailableGuamiList := ngap_message.BuildUnavailableGUAMIList(amfSelf.ServedGuamiList)
	amfSelf.AmfRanPool.Range(func(key, value interface{}) bool {
		ran := value.(*context.AmfRan)
		ngap_message.SendAMFStatusIndication(ran, unavailableGuamiList)
		return true
	})

	ngap_service.Stop()

	callback.SendAmfStatusChangeNotify((string)(models.StatusChange_UNAVAILABLE), amfSelf.ServedGuamiList)

	amfSelf.NfStatusSubscriptions.Range(func(nfInstanceId, v interface{}) bool {
		if subscriptionId, ok := amfSelf.NfStatusSubscriptions.Load(nfInstanceId); ok {
			logger.InitLog.Debugf("SubscriptionId is %v", subscriptionId.(string))
			problemDetails, err := consumer.SendRemoveSubscription(subscriptionId.(string))
			if problemDetails != nil {
				logger.InitLog.Errorf("Remove NF Subscription Failed Problem[%+v]", problemDetails)
			} else if err != nil {
				logger.InitLog.Errorf("Remove NF Subscription Error[%+v]", err)
			} else {
				logger.InitLog.Infoln("[AMF] Remove NF Subscription successful")
			}
		}
		return true
	})

	logger.InitLog.Infof("AMF terminated")
}

func (amf *AMF) StartKeepAliveTimer(nfProfile models.NfProfile) {
	KeepAliveTimerMutex.Lock()
	defer KeepAliveTimerMutex.Unlock()
	amf.StopKeepAliveTimer()
	if nfProfile.HeartBeatTimer == 0 {
		nfProfile.HeartBeatTimer = 60
	}
	logger.InitLog.Infof("Started KeepAlive Timer: %v sec", nfProfile.HeartBeatTimer)
	// AfterFunc starts timer and waits for KeepAliveTimer to elapse and then calls amf.UpdateNF function
	KeepAliveTimer = time.AfterFunc(time.Duration(nfProfile.HeartBeatTimer)*time.Second, amf.UpdateNF)
}

func (amf *AMF) StopKeepAliveTimer() {
	if KeepAliveTimer != nil {
		logger.InitLog.Infof("Stopped KeepAlive Timer.")
		KeepAliveTimer.Stop()
		KeepAliveTimer = nil
	}
}

func (amf *AMF) BuildAndSendRegisterNFInstance() (models.NfProfile, error) {
	self := context.AMF_Self()
	profile, err := consumer.BuildNFInstance(self)
	if err != nil {
		initLog.Errorf("Build AMF Profile Error: %v", err)
		return profile, err
	}
	initLog.Infof("Pcf Profile Registering to NRF: %v", profile)
	// Indefinite attempt to register until success
	profile, _, self.NfId, err = consumer.SendRegisterNFInstance(self.NrfUri, self.NfId, profile)
	return profile, err
}

// UpdateNF is the callback function, this is called when keepalivetimer elapsed
func (amf *AMF) UpdateNF() {
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
		initLog.Errorf("AMF update to NRF ProblemDetails[%v]", problemDetails)
		// 5xx response from NRF, 404 Not Found, 400 Bad Request
		if (problemDetails.Status/100) == 5 ||
			problemDetails.Status == 404 || problemDetails.Status == 400 {
			// register with NRF full profile
			nfProfile, err = amf.BuildAndSendRegisterNFInstance()
			if err != nil {
				initLog.Errorf("Could not register to NRF Error[%s]", err.Error())
			}
		}
	} else if err != nil {
		initLog.Errorf("AMF update to NRF Error[%s]", err.Error())
		nfProfile, err = amf.BuildAndSendRegisterNFInstance()
		if err != nil {
			initLog.Errorf("Could not register to NRF Error[%s]", err.Error())
		}
	}

	if nfProfile.HeartBeatTimer != 0 {
		// use hearbeattimer value with received timer value from NRF
		heartBeatTimer = nfProfile.HeartBeatTimer
	}
	logger.InitLog.Debugf("Restarted KeepAlive Timer: %v sec", heartBeatTimer)
	// restart timer with received HeartBeatTimer value
	KeepAliveTimer = time.AfterFunc(time.Duration(heartBeatTimer)*time.Second, amf.UpdateNF)
}

func (amf *AMF) UpdateAmfConfiguration(plmn factory.PlmnSupportItem, taiList []models.Tai, opType protos.OpType) {
	var plmnFound bool
	for plmnindex, p := range factory.AmfConfig.Configuration.PlmnSupportList {
		if p.PlmnId == plmn.PlmnId {
			plmnFound = true
			var found bool
			nssai_r := plmn.SNssaiList[0]
			for i, nssai := range p.SNssaiList {
				if nssai_r == nssai {
					found = true
					if opType == protos.OpType_SLICE_DELETE {
						factory.AmfConfig.Configuration.PlmnSupportList[plmnindex].SNssaiList = append(factory.AmfConfig.Configuration.PlmnSupportList[plmnindex].SNssaiList[:i], p.SNssaiList[i+1:]...)
						if len(factory.AmfConfig.Configuration.PlmnSupportList[plmnindex].SNssaiList) == 0 {
							factory.AmfConfig.Configuration.PlmnSupportList = append(factory.AmfConfig.Configuration.PlmnSupportList[:plmnindex],
								factory.AmfConfig.Configuration.PlmnSupportList[plmnindex+1:]...)

							factory.AmfConfig.Configuration.ServedGumaiList = append(factory.AmfConfig.Configuration.ServedGumaiList[:plmnindex],
								factory.AmfConfig.Configuration.ServedGumaiList[plmnindex+1:]...)
						}
					}
					break
				}
			}

			if !found && opType != protos.OpType_SLICE_DELETE {
				logger.GrpcLog.Infof("plmn found but slice not found in AMF Configuration")
				factory.AmfConfig.Configuration.PlmnSupportList[plmnindex].SNssaiList = append(factory.AmfConfig.Configuration.PlmnSupportList[plmnindex].SNssaiList, nssai_r)
			}
			break
		}
	}

	guami := models.Guami{PlmnId: &plmn.PlmnId, AmfId: "cafe00"}
	if !plmnFound && opType != protos.OpType_SLICE_DELETE {
		factory.AmfConfig.Configuration.PlmnSupportList = append(factory.AmfConfig.Configuration.PlmnSupportList, plmn)
		factory.AmfConfig.Configuration.ServedGumaiList = append(factory.AmfConfig.Configuration.ServedGumaiList, guami)
	}
	logger.GrpcLog.Infof("SupportedPlmnLIst: %v, SupportGuamiLIst: %v received fromRoc\n", plmn, guami)
	logger.GrpcLog.Infof("SupportedPlmnLIst: %v, SupportGuamiLIst: %v in AMF\n", factory.AmfConfig.Configuration.PlmnSupportList,
		factory.AmfConfig.Configuration.ServedGumaiList)
	// same plmn received but Tacs in gnb updated
	nssai_r := plmn.SNssaiList[0]
	slice := strconv.FormatInt(int64(nssai_r.Sst), 10) + nssai_r.Sd
	delete(factory.AmfConfig.Configuration.SliceTaiList, slice)
	if opType != protos.OpType_SLICE_DELETE {
		// maintaining slice level tai List
		if factory.AmfConfig.Configuration.SliceTaiList == nil {
			factory.AmfConfig.Configuration.SliceTaiList = make(map[string][]models.Tai)
		}
		factory.AmfConfig.Configuration.SliceTaiList[slice] = taiList
	}

	amf.UpdateSupportedTaiList()
	logger.GrpcLog.Infoln("Gnb Updated in existing Plmn, SupportTAILIst received from Roc: ", taiList)
	logger.GrpcLog.Infoln("SupportTAILIst in AMF", factory.AmfConfig.Configuration.SupportTAIList)
}

func (amf *AMF) UpdateSupportedTaiList() {
	factory.AmfConfig.Configuration.SupportTAIList = nil
	for _, slice := range factory.AmfConfig.Configuration.SliceTaiList {
		for _, tai := range slice {
			logger.GrpcLog.Infoln("Tai list present in Slice", tai, factory.AmfConfig.Configuration.SupportTAIList)
			factory.AmfConfig.Configuration.SupportTAIList = append(factory.AmfConfig.Configuration.SupportTAIList, tai)
		}
	}
}

func (amf *AMF) UpdateConfig(commChannel chan *protos.NetworkSliceResponse) bool {
	for rsp := range commChannel {
		logger.GrpcLog.Infof("Received updateConfig in the amf app : %v", rsp)
		var tai []models.Tai
		for _, ns := range rsp.NetworkSlice {
			var snssai *models.Snssai
			logger.GrpcLog.Infoln("Network Slice Name ", ns.Name)
			if ns.Nssai != nil {
				snssai = new(models.Snssai)
				val, err := strconv.ParseInt(ns.Nssai.Sst, 10, 64)
				if err != nil {
					logger.GrpcLog.Errorln(err)
				}
				snssai.Sst = int32(val)
				snssai.Sd = ns.Nssai.Sd
			}
			// inform connected UEs with update slices
			if len(ns.DeletedImsis) > 0 {
				HandleImsiDeleteFromNetworkSlice(ns)
			}
			//TODO Inform connected UEs with update Slice
			/*if len(ns.AddUpdatedImsis) > 0 {
				HandleImsiAddInNetworkSlice(ns)
			}*/

			if ns.Site != nil {
				site := ns.Site
				logger.GrpcLog.Infoln("Network Slice has site name: ", site.SiteName)
				if site.Plmn != nil {
					plmn := new(factory.PlmnSupportItem)

					logger.GrpcLog.Infoln("Plmn mcc ", site.Plmn.Mcc)
					plmn.PlmnId.Mnc = site.Plmn.Mnc
					plmn.PlmnId.Mcc = site.Plmn.Mcc

					if ns.Nssai != nil {
						plmn.SNssaiList = append(plmn.SNssaiList, *snssai)
					}
					if site.Gnb != nil {
						for _, gnb := range site.Gnb {
							var t models.Tai
							t.PlmnId = new(models.PlmnId)
							t.PlmnId.Mnc = site.Plmn.Mnc
							t.PlmnId.Mcc = site.Plmn.Mcc
							t.Tac = strconv.Itoa(int(gnb.Tac))
							tai = append(tai, t)
						}
					}

					amf.UpdateAmfConfiguration(*plmn, tai, ns.OperationType)
				} else {
					logger.GrpcLog.Infoln("Plmn not present in the message ")
				}
			}
		}

		// Update PlmnSupportList/ServedGuamiList/ServedTAIList in Amf Config
		// factory.AmfConfig.Configuration.ServedGumaiList = nil
		// factory.AmfConfig.Configuration.PlmnSupportList = nil
		if len(factory.AmfConfig.Configuration.ServedGumaiList) > 0 {
			RocUpdateConfigChannel <- true
		}
	}
	return true
}

func (amf *AMF) SendNFProfileUpdateToNrf() {
	// for rocUpdateConfig := range RocUpdateConfigChannel {
	for rocUpdateConfig := range RocUpdateConfigChannel {
		if rocUpdateConfig {
			self := context.AMF_Self()
			util.InitAmfContext(self)

			// Register to NRF with Updated Profile
			var profile models.NfProfile
			if profileTmp, err := consumer.BuildNFInstance(self); err != nil {
				logger.CfgLog.Errorf("Build AMF Profile Error: %v", err)
				continue
			} else {
				profile = profileTmp
			}

			if prof, _, nfId, err := consumer.SendRegisterNFInstance(self.NrfUri, self.NfId, profile); err != nil {
				logger.CfgLog.Warnf("Send Register NF Instance with updated profile failed: %+v", err)
			} else {
				// stop keepAliveTimer if its running and start the timer
				amf.StartKeepAliveTimer(prof)
				self.NfId = nfId
				logger.CfgLog.Infof("Sent Register NF Instance with updated profile")
			}
		}
	}
}

func UeConfigSliceDeleteHandler(supi, sst, sd string, msg interface{}) {
	amfSelf := context.AMF_Self()
	ue, _ := amfSelf.AmfUeFindBySupi(IMSI_PREFIX + supi)

	// Triggers for NwInitiatedDeRegistration
	// - Only 1 Allowed Nssai is exist and its slice information matched
	ns := msg.(*protos.NetworkSlice)
	if len(ue.AllowedNssai[models.AccessType__3_GPP_ACCESS]) == 1 {
		st, err := strconv.Atoi(ns.Nssai.Sst)
		if err != nil {
			logger.CfgLog.Errorln(err)
		}
		if ue.AllowedNssai[models.AccessType__3_GPP_ACCESS][0].AllowedSnssai.Sst == int32(st) &&
			ue.AllowedNssai[models.AccessType__3_GPP_ACCESS][0].AllowedSnssai.Sd == ns.Nssai.Sd {
			err := gmm.GmmFSM.SendEvent(ue.State[models.AccessType__3_GPP_ACCESS], gmm.NwInitiatedDeregistrationEvent, fsm.ArgsType{
				gmm.ArgAmfUe:      ue,
				gmm.ArgAccessType: models.AccessType__3_GPP_ACCESS,
			})
			if err != nil {
				logger.CfgLog.Errorln(err)
			}
		} else {
			logger.CfgLog.Infof("Deleted slice not matched with slice info in UEContext")
		}
	} else {
		var Nssai models.Snssai
		st, err := strconv.Atoi(ns.Nssai.Sst)
		if err != nil {
			logger.CfgLog.Errorln(err)
		}
		Nssai.Sst = int32(st)
		Nssai.Sd = ns.Nssai.Sd
		err = gmm.GmmFSM.SendEvent(ue.State[models.AccessType__3_GPP_ACCESS], gmm.SliceInfoDeleteEvent, fsm.ArgsType{
			gmm.ArgAmfUe:      ue,
			gmm.ArgAccessType: models.AccessType__3_GPP_ACCESS,
			gmm.ArgNssai:      Nssai,
		})
		if err != nil {
			logger.CfgLog.Errorln(err)
		}
	}
}

func UeConfigSliceAddHandler(supi, sst, sd string, msg interface{}) {
	amfSelf := context.AMF_Self()
	ue, _ := amfSelf.AmfUeFindBySupi(IMSI_PREFIX + supi)

	ns := msg.(*protos.NetworkSlice)
	var Nssai models.Snssai
	st, err := strconv.Atoi(ns.Nssai.Sst)
	if err != nil {
		logger.CfgLog.Errorln(err)
	}
	Nssai.Sst = int32(st)
	Nssai.Sd = ns.Nssai.Sd
	err = gmm.GmmFSM.SendEvent(ue.State[models.AccessType__3_GPP_ACCESS], gmm.SliceInfoAddEvent, fsm.ArgsType{
		gmm.ArgAmfUe:      ue,
		gmm.ArgAccessType: models.AccessType__3_GPP_ACCESS,
		gmm.ArgNssai:      Nssai,
	})
	if err != nil {
		logger.CfgLog.Errorln(err)
	}
}

func HandleImsiDeleteFromNetworkSlice(slice *protos.NetworkSlice) {
	var ue *context.AmfUe
	var ok bool
	logger.CfgLog.Infof("[AMF] Handle Subscribers Delete From Network Slice [sst:%v sd:%v]", slice.Nssai.Sst, slice.Nssai.Sd)

	for _, supi := range slice.DeletedImsis {
		amfSelf := context.AMF_Self()
		ue, ok = amfSelf.AmfUeFindBySupi(IMSI_PREFIX + supi)
		if !ok {
			logger.CfgLog.Infof("the UE [%v] is not Registered with the 5G-Core", supi)
			continue
		}
		// publish the event to ue channel
		configMsg := context.ConfigMsg{
			Supi: supi,
			Msg:  slice,
			Sst:  slice.Nssai.Sst,
			Sd:   slice.Nssai.Sd,
		}
		ue.SetEventChannel(nil)
		ue.EventChannel.UpdateConfigHandler(UeConfigSliceDeleteHandler)
		ue.EventChannel.SubmitMessage(configMsg)
	}
}

func HandleImsiAddInNetworkSlice(slice *protos.NetworkSlice) {
	var ue *context.AmfUe
	var ok bool
	logger.CfgLog.Infof("[AMF] Handle Subscribers Added in Network Slice [sst:%v sd:%v]", slice.Nssai.Sst, slice.Nssai.Sd)

	for _, supi := range slice.AddUpdatedImsis {
		amfSelf := context.AMF_Self()
		ue, ok = amfSelf.AmfUeFindBySupi(IMSI_PREFIX + supi)
		if !ok {
			logger.CfgLog.Infof("the UE [%v] is not Registered with the 5G-Core", supi)
			continue
		}
		// publish the event to ue channel
		configMsg := context.ConfigMsg{
			Supi: supi,
			Msg:  slice,
			Sst:  slice.Nssai.Sst,
			Sd:   slice.Nssai.Sd,
		}

		ue.EventChannel.UpdateConfigHandler(UeConfigSliceAddHandler)
		ue.EventChannel.SubmitMessage(configMsg)
	}
}
