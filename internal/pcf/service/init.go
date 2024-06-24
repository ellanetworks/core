package service

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/antihax/optional"
	"github.com/gin-contrib/cors"
	"github.com/omec-project/openapi/Nnrf_NFDiscovery"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/http2_util"
	"github.com/omec-project/util/idgenerator"
	logger_util "github.com/omec-project/util/logger"
	"github.com/omec-project/util/path_util"
	pathUtilLogger "github.com/omec-project/util/path_util/logger"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/yeastengine/config5g/proto/client"
	protos "github.com/yeastengine/config5g/proto/sdcoreConfig"
	"github.com/yeastengine/ella/internal/pcf/ampolicy"
	"github.com/yeastengine/ella/internal/pcf/bdtpolicy"
	"github.com/yeastengine/ella/internal/pcf/consumer"
	"github.com/yeastengine/ella/internal/pcf/context"
	"github.com/yeastengine/ella/internal/pcf/factory"
	"github.com/yeastengine/ella/internal/pcf/httpcallback"
	"github.com/yeastengine/ella/internal/pcf/internal/notifyevent"
	"github.com/yeastengine/ella/internal/pcf/logger"
	"github.com/yeastengine/ella/internal/pcf/oam"
	"github.com/yeastengine/ella/internal/pcf/policyauthorization"
	"github.com/yeastengine/ella/internal/pcf/smpolicy"
	"github.com/yeastengine/ella/internal/pcf/uepolicy"
	"github.com/yeastengine/ella/internal/pcf/util"
)

type PCF struct{}

type (
	// Config information.
	Config struct {
		pcfcfg string
	}
)

var (
	ConfigPodTrigger    chan bool
	KeepAliveTimer      *time.Timer
	KeepAliveTimerMutex sync.Mutex
)

func init() {
	ConfigPodTrigger = make(chan bool)
}

var config Config

var pcfCLi = []cli.Flag{
	cli.StringFlag{
		Name:  "free5gccfg",
		Usage: "common config file",
	},
	cli.StringFlag{
		Name:  "pcfcfg",
		Usage: "config file",
	},
}

var initLog *logrus.Entry

func init() {
	initLog = logger.InitLog
}

func (*PCF) GetCliCmd() (flags []cli.Flag) {
	return pcfCLi
}

func (pcf *PCF) Initialize(c *cli.Context) error {
	config = Config{
		pcfcfg: c.String("pcfcfg"),
	}
	if config.pcfcfg != "" {
		if err := factory.InitConfigFactory(config.pcfcfg); err != nil {
			return err
		}
	} else {
		DefaultPcfConfigPath := path_util.Free5gcPath("free5gc/config/pcfcfg.yaml")
		if err := factory.InitConfigFactory(DefaultPcfConfigPath); err != nil {
			return err
		}
	}

	pcf.setLogLevel()

	gClient := client.ConnectToConfigServer(factory.PcfConfig.Configuration.WebuiUri, "pcf")
	commChannel := gClient.PublishOnConfigChange(true)
	go pcf.updateConfig(commChannel)
	return nil
}

func (pcf *PCF) setLogLevel() {
	if factory.PcfConfig.Logger == nil {
		initLog.Warnln("PCF config without log level setting!!!")
		return
	}

	if factory.PcfConfig.Logger.PCF != nil {
		if factory.PcfConfig.Logger.PCF.DebugLevel != "" {
			if level, err := logrus.ParseLevel(factory.PcfConfig.Logger.PCF.DebugLevel); err != nil {
				initLog.Warnf("PCF Log level [%s] is invalid, set to [info] level",
					factory.PcfConfig.Logger.PCF.DebugLevel)
				logger.SetLogLevel(logrus.InfoLevel)
			} else {
				initLog.Infof("PCF Log level is set to [%s] level", level)
				logger.SetLogLevel(level)
			}
		} else {
			initLog.Infoln("PCF Log level is default set to [info] level")
			logger.SetLogLevel(logrus.InfoLevel)
		}
		logger.SetReportCaller(factory.PcfConfig.Logger.PCF.ReportCaller)
	}

	if factory.PcfConfig.Logger.PathUtil != nil {
		if factory.PcfConfig.Logger.PathUtil.DebugLevel != "" {
			if level, err := logrus.ParseLevel(factory.PcfConfig.Logger.PathUtil.DebugLevel); err != nil {
				pathUtilLogger.PathLog.Warnf("PathUtil Log level [%s] is invalid, set to [info] level",
					factory.PcfConfig.Logger.PathUtil.DebugLevel)
				pathUtilLogger.SetLogLevel(logrus.InfoLevel)
			} else {
				pathUtilLogger.SetLogLevel(level)
			}
		} else {
			pathUtilLogger.PathLog.Warnln("PathUtil Log level not set. Default set to [info] level")
			pathUtilLogger.SetLogLevel(logrus.InfoLevel)
		}
		pathUtilLogger.SetReportCaller(factory.PcfConfig.Logger.PathUtil.ReportCaller)
	}

	/*if factory.PcfConfig.Logger.OpenApi != nil {
		if factory.PcfConfig.Logger.OpenApi.DebugLevel != "" {
			if level, err := logrus.ParseLevel(factory.PcfConfig.Logger.OpenApi.DebugLevel); err != nil {
				openApiLogger.OpenApiLog.Warnf("OpenAPI Log level [%s] is invalid, set to [info] level",
					factory.PcfConfig.Logger.OpenApi.DebugLevel)
				openApiLogger.SetLogLevel(logrus.InfoLevel)
			} else {
				openApiLogger.SetLogLevel(level)
			}
		} else {
			openApiLogger.OpenApiLog.Warnln("OpenAPI Log level not set. Default set to [info] level")
			openApiLogger.SetLogLevel(logrus.InfoLevel)
		}
		openApiLogger.SetReportCaller(factory.PcfConfig.Logger.OpenApi.ReportCaller)
	}*/
}

func (pcf *PCF) FilterCli(c *cli.Context) (args []string) {
	for _, flag := range pcf.GetCliCmd() {
		name := flag.GetName()
		value := fmt.Sprint(c.Generic(name))
		if value == "" {
			continue
		}

		args = append(args, "--"+name, value)
	}
	return args
}

func (pcf *PCF) Start() {
	initLog.Infoln("Server started")
	router := logger_util.NewGinWithLogrus(logger.GinLog)

	bdtpolicy.AddService(router)
	smpolicy.AddService(router)
	ampolicy.AddService(router)
	uepolicy.AddService(router)
	policyauthorization.AddService(router)
	httpcallback.AddService(router)
	oam.AddService(router)

	router.Use(cors.New(cors.Config{
		AllowMethods: []string{"GET", "POST", "OPTIONS", "PUT", "PATCH", "DELETE"},
		AllowHeaders: []string{
			"Origin", "Content-Length", "Content-Type", "User-Agent",
			"Referrer", "Host", "Token", "X-Requested-With",
		},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		AllowAllOrigins:  true,
		MaxAge:           86400,
	}))

	if err := notifyevent.RegisterNotifyDispatcher(); err != nil {
		initLog.Error("Register NotifyDispatcher Error")
	}

	self := context.PCF_Self()
	util.InitpcfContext(self)

	addr := fmt.Sprintf("%s:%d", self.BindingIPv4, self.SBIPort)

	// Attempt NRF Registration until success
	go pcf.RegisterNF()

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		pcf.Terminate()
		os.Exit(0)
	}()

	server, err := http2_util.NewServer(addr, util.PCF_LOG_PATH, router)
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

func (pcf *PCF) Exec(c *cli.Context) error {
	initLog.Traceln("args:", c.String("pcfcfg"))
	args := pcf.FilterCli(c)
	initLog.Traceln("filter: ", args)
	command := exec.Command("./pcf", args...)

	stdout, err := command.StdoutPipe()
	if err != nil {
		initLog.Fatalln(err)
	}
	wg := sync.WaitGroup{}
	wg.Add(4)
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
		fmt.Println("PCF log start")
		for in.Scan() {
			fmt.Println(in.Text())
		}
		wg.Done()
	}()

	go func() {
		fmt.Println("PCF start")
		if err = command.Start(); err != nil {
			fmt.Printf("command.Start() error: %v", err)
		}
		fmt.Println("PCF end")
		wg.Done()
	}()

	wg.Wait()

	return err
}

func (pcf *PCF) StartKeepAliveTimer(nfProfile models.NfProfile) {
	KeepAliveTimerMutex.Lock()
	defer KeepAliveTimerMutex.Unlock()
	pcf.StopKeepAliveTimer()
	if nfProfile.HeartBeatTimer == 0 {
		// heartbeat timer value set to 60 sec
		nfProfile.HeartBeatTimer = 60
	}
	logger.InitLog.Infof("Started KeepAlive Timer: %v sec", nfProfile.HeartBeatTimer)
	// AfterFunc starts timer and waits for KeepAliveTimer to elapse and then calls pcf.UpdateNF function
	KeepAliveTimer = time.AfterFunc(time.Duration(nfProfile.HeartBeatTimer)*time.Second, pcf.UpdateNF)
}

func (pcf *PCF) StopKeepAliveTimer() {
	if KeepAliveTimer != nil {
		logger.InitLog.Infof("Stopped KeepAlive Timer.")
		KeepAliveTimer.Stop()
		KeepAliveTimer = nil
	}
}

func (pcf *PCF) Terminate() {
	logger.InitLog.Infof("Terminating PCF...")
	// deregister with NRF
	problemDetails, err := consumer.SendDeregisterNFInstance()
	if problemDetails != nil {
		logger.InitLog.Errorf("Deregister NF instance Failed Problem[%+v]", problemDetails)
	} else if err != nil {
		logger.InitLog.Errorf("Deregister NF instance Error[%+v]", err)
	} else {
		logger.InitLog.Infof("Deregister from NRF successfully")
	}
	logger.InitLog.Infof("PCF terminated")
}

func (pcf *PCF) BuildAndSendRegisterNFInstance() (models.NfProfile, error) {
	self := context.PCF_Self()
	profile, err := consumer.BuildNFInstance(self)
	if err != nil {
		initLog.Errorf("Build PCF Profile Error: %v", err)
		return profile, err
	}
	initLog.Infof("Pcf Profile Registering to NRF: %v", profile)
	// Indefinite attempt to register until success
	profile, _, self.NfId, err = consumer.SendRegisterNFInstance(self.NrfUri, self.NfId, profile)
	return profile, err
}

func (pcf *PCF) RegisterNF() {
	for {
		msg := <-ConfigPodTrigger
		// wait till Config pod updates config
		if msg {
			initLog.Infof("Config update trigger %v received in PCF App", msg)
			profile, err := pcf.BuildAndSendRegisterNFInstance()
			if err != nil {
				initLog.Errorf("PCF register to NRF Error[%s]", err.Error())
			} else {
				pcf.StartKeepAliveTimer(profile)
				// NRF Registration Successful, Trigger for UDR Discovery
				pcf.DiscoverUdr()
			}
		} else {
			// stopping keepAlive timer
			KeepAliveTimerMutex.Lock()
			pcf.StopKeepAliveTimer()
			KeepAliveTimerMutex.Unlock()
			initLog.Infof("PCF is not having Minimum Config to Register/Update to NRF")
			problemDetails, err := consumer.SendDeregisterNFInstance()
			if problemDetails != nil {
				initLog.Errorf("PCF Deregister Instance to NRF failed, Problem: [+%v]", problemDetails)
			}
			if err != nil {
				initLog.Errorf("PCF Deregister Instance to NRF Error[%s]", err.Error())
			} else {
				logger.InitLog.Infof("Deregister from NRF successfully")
			}
		}
	}
}

// UpdateNF is the callback function, this is called when keepalivetimer elapsed
func (pcf *PCF) UpdateNF() {
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
		initLog.Errorf("PCF update to NRF ProblemDetails[%v]", problemDetails)
		// 5xx response from NRF, 404 Not Found, 400 Bad Request
		if (problemDetails.Status/100) == 5 ||
			problemDetails.Status == 404 || problemDetails.Status == 400 {
			// register with NRF full profile
			nfProfile, err = pcf.BuildAndSendRegisterNFInstance()
			if err != nil {
				initLog.Errorf("PCF register to NRF Error[%s]", err.Error())
			}
		}
	} else if err != nil {
		initLog.Errorf("PCF update to NRF Error[%s]", err.Error())
		nfProfile, err = pcf.BuildAndSendRegisterNFInstance()
		if err != nil {
			initLog.Errorf("PCF register to NRF Error[%s]", err.Error())
		}
	}

	if nfProfile.HeartBeatTimer != 0 {
		// use hearbeattimer value with received timer value from NRF
		heartBeatTimer = nfProfile.HeartBeatTimer
	}
	logger.InitLog.Debugf("Restarted KeepAlive Timer: %v sec", heartBeatTimer)
	// restart timer with received HeartBeatTimer value
	KeepAliveTimer = time.AfterFunc(time.Duration(heartBeatTimer)*time.Second, pcf.UpdateNF)
}

func (pcf *PCF) DiscoverUdr() {
	self := context.PCF_Self()
	param := Nnrf_NFDiscovery.SearchNFInstancesParamOpts{
		ServiceNames: optional.NewInterface([]models.ServiceName{models.ServiceName_NUDR_DR}),
	}
	if resp, err := consumer.SendSearchNFInstances(self.NrfUri, models.NfType_UDR, models.NfType_PCF, param); err != nil {
		initLog.Errorln(err)
	} else {
		for _, nfProfile := range resp.NfInstances {
			udruri := util.SearchNFServiceUri(nfProfile, models.ServiceName_NUDR_DR, models.NfServiceStatus_REGISTERED)
			if udruri != "" {
				self.SetDefaultUdrURI(udruri)
				break
			}
		}
	}
}

func ImsiExistInDeviceGroup(devGroup *protos.DeviceGroup, imsi string) bool {
	for _, i := range devGroup.Imsi {
		if i == imsi {
			return true
		}
	}
	return false
}

func GetBitRateUnit(val int64) (int64, string) {
	unit := " Kbps"
	if val < 1000 {
		logger.GrpcLog.Warnf("configured value [%v] is lesser than 1000 bps, so setting 1 Kbps", val)
		val = 1
		return val, unit
	}
	if val >= 0xFFFF {
		val = (val / 1000)
		unit = " Kbps"
		if val >= 0xFFFF {
			val = (val / 1000)
			unit = " Mbps"
		}
		if val >= 0xFFFF {
			val = (val / 1000)
			unit = " Gbps"
		}
	} else {
		// minimum supported is kbps by SMF/UE
		val = val / 1000
	}

	return val, unit
}

func getSessionRule(devGroup *protos.DeviceGroup) (sessionRule *models.SessionRule) {
	sessionRule = &models.SessionRule{}
	qos := devGroup.IpDomainDetails.UeDnnQos
	if qos.TrafficClass != nil {
		sessionRule.AuthDefQos = &models.AuthorizedDefaultQos{
			Var5qi: qos.TrafficClass.Qci,
			Arp:    &models.Arp{PriorityLevel: qos.TrafficClass.Arp},
			// PriorityLevel:
		}
	}
	ul, uunit := GetBitRateUnit(qos.DnnMbrUplink)
	dl, dunit := GetBitRateUnit(qos.DnnMbrDownlink)
	sessionRule.AuthSessAmbr = &models.Ambr{
		Uplink:   strconv.FormatInt(ul, 10) + uunit,
		Downlink: strconv.FormatInt(dl, 10) + dunit,
	}
	return sessionRule
}

func getPccRules(slice *protos.NetworkSlice, sessionRule *models.SessionRule) (pccPolicy context.PccPolicy) {
	if slice.AppFilters == nil || slice.AppFilters.PccRuleBase == nil {
		logger.GrpcLog.Warnf("PccRules not exist in slice: %v", slice.Name)
		return context.PccPolicy{IdGenerator: idgenerator.NewGenerator(1, math.MaxInt64)}
	}
	pccPolicy.IdGenerator = idgenerator.NewGenerator(1, math.MaxInt64)
	for _, pccrule := range slice.AppFilters.PccRuleBase {
		id, err := pccPolicy.IdGenerator.Allocate()
		if err != nil {
			logger.GrpcLog.Errorf("IdGenerator allocation failed: %v", err)
		}
		var rule models.PccRule
		var qos models.QosData
		rule.PccRuleId = strconv.FormatInt(id, 10)
		rule.Precedence = pccrule.Priority
		if pccrule.Qos != nil {
			qos.QosId = strconv.FormatInt(id, 10)
			qos.Var5qi = pccrule.Qos.Var5Qi
			if pccrule.Qos.MaxbrUl != 0 {
				ul, unit := GetBitRateUnit(int64(pccrule.Qos.MaxbrUl))
				qos.MaxbrUl = strconv.FormatInt(ul, 10) + unit
			}
			if pccrule.Qos.MaxbrDl != 0 {
				dl, unit := GetBitRateUnit(int64(pccrule.Qos.MaxbrDl))
				qos.MaxbrDl = strconv.FormatInt(dl, 10) + unit
			}
			if pccrule.Qos.GbrUl != 0 {
				ul, unit := GetBitRateUnit(int64(pccrule.Qos.GbrUl))
				qos.GbrUl = strconv.FormatInt(ul, 10) + unit
			}
			if pccrule.Qos.GbrDl != 0 {
				dl, unit := GetBitRateUnit(int64(pccrule.Qos.GbrDl))
				qos.GbrDl = strconv.FormatInt(dl, 10) + unit
			}
			if pccrule.Qos.Arp != nil {
				qos.Arp = &models.Arp{PriorityLevel: pccrule.Qos.Arp.PL}
				if pccrule.Qos.Arp.PC == protos.PccArpPc_NOT_PREEMPT {
					qos.Arp.PreemptCap = models.PreemptionCapability_NOT_PREEMPT
				} else if pccrule.Qos.Arp.PC == protos.PccArpPc_MAY_PREEMPT {
					qos.Arp.PreemptCap = models.PreemptionCapability_MAY_PREEMPT
				}
				if pccrule.Qos.Arp.PV == protos.PccArpPv_NOT_PREEMPTABLE {
					qos.Arp.PreemptVuln = models.PreemptionVulnerability_NOT_PREEMPTABLE
				} else if pccrule.Qos.Arp.PV == protos.PccArpPv_PREEMPTABLE {
					qos.Arp.PreemptVuln = models.PreemptionVulnerability_PREEMPTABLE
				}
			}
			if pccrule.Qos.MaxbrUl == 0 && pccrule.Qos.MaxbrDl == 0 && pccrule.Qos.GbrUl == 0 && pccrule.Qos.GbrDl == 0 {
				// getting from sessionrule
				qos.MaxbrUl = sessionRule.AuthSessAmbr.Uplink
				qos.MaxbrDl = sessionRule.AuthSessAmbr.Downlink
			}
			//rule.RefQosData = append(rule.RefQosData, qos.QosId)
			//if pccPolicy.QosDecs == nil {
			//	pccPolicy.QosDecs = make(map[string]*models.QosData)
			//}
			//pccPolicy.QosDecs[qos.QosId] = &qos
		}
		for _, pflow := range pccrule.FlowInfos {
			var flow models.FlowInformation
			flow.FlowDescription = pflow.FlowDesc
			// flow.TosTrafficClass = pflow.TosTrafficClass
			id, err := pccPolicy.IdGenerator.Allocate()
			if err != nil {
				logger.GrpcLog.Errorf("IdGenerator allocation failed: %v", err)
			}
			flow.PackFiltId = strconv.FormatInt(id, 10)

			if pflow.FlowDir == protos.PccFlowDirection_DOWNLINK {
				flow.FlowDirection = models.FlowDirectionRm_DOWNLINK
			} else if pflow.FlowDir == protos.PccFlowDirection_UPLINK {
				flow.FlowDirection = models.FlowDirectionRm_UPLINK
			} else if pflow.FlowDir == protos.PccFlowDirection_BIDIRECTIONAL {
				flow.FlowDirection = models.FlowDirectionRm_BIDIRECTIONAL
			} else if pflow.FlowDir == protos.PccFlowDirection_UNSPECIFIED {
				flow.FlowDirection = models.FlowDirectionRm_UNSPECIFIED
			}
			if strings.HasSuffix(flow.FlowDescription, "any to assigned") ||
				strings.HasSuffix(flow.FlowDescription, "any to assigned ") {
				qos.DefQosFlowIndication = true
			}
			// traffic control info set based on flow at present
			var tcData models.TrafficControlData
			tcData.TcId = "TcId-" + strconv.FormatInt(id, 10)

			if pflow.FlowStatus == protos.PccFlowStatus_ENABLED {
				tcData.FlowStatus = models.FlowStatus_ENABLED
			} else if pflow.FlowStatus == protos.PccFlowStatus_DISABLED {
				tcData.FlowStatus = models.FlowStatus_DISABLED
			}

			rule.RefTcData = append(rule.RefTcData, tcData.TcId)
			if pccPolicy.TraffContDecs == nil {
				pccPolicy.TraffContDecs = make(map[string]*models.TrafficControlData)
			}
			pccPolicy.TraffContDecs[tcData.TcId] = &tcData

			rule.FlowInfos = append(rule.FlowInfos, flow)
		}
		if pccPolicy.QosDecs == nil {
			pccPolicy.QosDecs = make(map[string]*models.QosData)
		}
		if ok, q := findQosData(pccPolicy.QosDecs, qos); ok {
			rule.RefQosData = append(rule.RefQosData, q.QosId)
		} else {
			rule.RefQosData = append(rule.RefQosData, qos.QosId)
			pccPolicy.QosDecs[qos.QosId] = &qos
		}
		if pccPolicy.PccRules == nil {
			pccPolicy.PccRules = make(map[string]*models.PccRule)
		}
		pccPolicy.PccRules[pccrule.RuleId] = &rule
	}

	return pccPolicy
}

func findQosData(qosdecs map[string]*models.QosData, qos models.QosData) (bool, *models.QosData) {
	for _, q := range qosdecs {
		if q.Var5qi == qos.Var5qi && q.MaxbrUl == qos.MaxbrUl && q.MaxbrDl == qos.MaxbrDl &&
			q.GbrUl == qos.GbrUl && q.GbrDl == qos.GbrDl && q.Qnc == qos.Qnc &&
			q.PriorityLevel == qos.PriorityLevel && q.AverWindow == qos.AverWindow &&
			q.MaxDataBurstVol == qos.MaxDataBurstVol && q.ReflectiveQos == qos.ReflectiveQos &&
			q.SharingKeyDl == qos.SharingKeyDl && q.SharingKeyUl == qos.SharingKeyUl &&
			q.MaxPacketLossRateDl == qos.MaxPacketLossRateDl && q.MaxPacketLossRateUl == qos.MaxPacketLossRateUl &&
			q.DefQosFlowIndication == qos.DefQosFlowIndication {
			if q.Arp != nil && qos.Arp != nil && *q.Arp == *qos.Arp {
				return true, q
			}
		}
	}
	return false, nil
}

func (pcf *PCF) UpdatePcfSubsriberPolicyData(slice *protos.NetworkSlice) {
	self := context.PCF_Self()
	sliceid := slice.Nssai.Sst + slice.Nssai.Sd
	switch slice.OperationType {
	case protos.OpType_SLICE_ADD:
		logger.GrpcLog.Infoln("Received Slice with OperationType: Add from ConfigPod")
		for _, devgroup := range slice.DeviceGroup {
			var sessionrule *models.SessionRule
			var dnn string
			if devgroup.IpDomainDetails == nil || devgroup.IpDomainDetails.UeDnnQos == nil {
				logger.GrpcLog.Warnf("ip details or qos details in ipdomain not exist for device group: %v", devgroup.Name)
				continue
			}
			dnn = devgroup.IpDomainDetails.DnnName
			sessionrule = getSessionRule(devgroup)
			for _, imsi := range devgroup.Imsi {
				self.PcfSubscriberPolicyData[imsi] = &context.PcfSubscriberPolicyData{}
				policyData := self.PcfSubscriberPolicyData[imsi]
				policyData.CtxLog = logger.CtxLog.WithField(logger.FieldSupi, "imsi-"+imsi)
				policyData.PccPolicy = make(map[string]*context.PccPolicy)
				policyData.PccPolicy[sliceid] = &context.PccPolicy{
					PccRules: make(map[string]*models.PccRule),
					QosDecs:  make(map[string]*models.QosData), TraffContDecs: make(map[string]*models.TrafficControlData),
					SessionPolicy: make(map[string]*context.SessionPolicy), IdGenerator: nil,
				}
				policyData.PccPolicy[sliceid].SessionPolicy[dnn] = &context.SessionPolicy{SessionRules: make(map[string]*models.SessionRule), SessionRuleIdGenerator: idgenerator.NewGenerator(1, math.MaxInt16)}
				id, err := policyData.PccPolicy[sliceid].SessionPolicy[dnn].SessionRuleIdGenerator.Allocate()
				if err != nil {
					logger.GrpcLog.Errorf("SessionRuleIdGenerator allocation failed: %v", err)
				}
				// tcid, _ := policyData.PccPolicy[sliceid].TcIdGenerator.Allocate()
				sessionrule.SessRuleId = dnn + "-" + strconv.Itoa(int(id))
				policyData.PccPolicy[sliceid].SessionPolicy[dnn].SessionRules[sessionrule.SessRuleId] = sessionrule
				pccPolicy := getPccRules(slice, sessionrule)
				for index, element := range pccPolicy.PccRules {
					policyData.PccPolicy[sliceid].PccRules[index] = element
				}
				for index, element := range pccPolicy.QosDecs {
					policyData.PccPolicy[sliceid].QosDecs[index] = element
				}
				for index, element := range pccPolicy.TraffContDecs {
					policyData.PccPolicy[sliceid].TraffContDecs[index] = element
				}
				policyData.CtxLog.Infof("Subscriber Detals: %v", policyData)
				// self.DisplayPcfSubscriberPolicyData(imsi)
			}
		}
	case protos.OpType_SLICE_UPDATE:
		logger.GrpcLog.Infof("Received Slice with OperationType: Update from ConfigPod")
		for _, devgroup := range slice.DeviceGroup {
			var sessionrule *models.SessionRule
			var dnn string
			if devgroup.IpDomainDetails == nil || devgroup.IpDomainDetails.UeDnnQos == nil {
				logger.GrpcLog.Warnf("ip details or qos details in ipdomain not exist for device group: %v", devgroup.Name)
				continue
			}

			dnn = devgroup.IpDomainDetails.DnnName
			sessionrule = getSessionRule(devgroup)

			for _, imsi := range slice.AddUpdatedImsis {
				if ImsiExistInDeviceGroup(devgroup, imsi) {
					// TODO policy exists, so compare and get difference with existing policy then notify the subscriber
					self.PcfSubscriberPolicyData[imsi] = &context.PcfSubscriberPolicyData{}
					policyData := self.PcfSubscriberPolicyData[imsi]
					policyData.CtxLog = logger.CtxLog.WithField(logger.FieldSupi, "imsi-"+imsi)
					policyData.PccPolicy = make(map[string]*context.PccPolicy)
					policyData.PccPolicy[sliceid] = &context.PccPolicy{
						PccRules: make(map[string]*models.PccRule),
						QosDecs:  make(map[string]*models.QosData), TraffContDecs: make(map[string]*models.TrafficControlData),
						SessionPolicy: make(map[string]*context.SessionPolicy), IdGenerator: nil,
					}
					policyData.PccPolicy[sliceid].SessionPolicy[dnn] = &context.SessionPolicy{
						SessionRules:           make(map[string]*models.SessionRule),
						SessionRuleIdGenerator: idgenerator.NewGenerator(1, math.MaxInt16),
					}

					// Added session rules
					id, err := policyData.PccPolicy[sliceid].SessionPolicy[dnn].SessionRuleIdGenerator.Allocate()
					if err != nil {
						logger.GrpcLog.Errorf("SessionRuleIdGenerator allocation failed: %v", err)
					}
					sessionrule.SessRuleId = dnn + strconv.Itoa(int(id))
					policyData.PccPolicy[sliceid].SessionPolicy[dnn].SessionRules[sessionrule.SessRuleId] = sessionrule
					// Added pcc rules
					pccPolicy := getPccRules(slice, sessionrule)
					for index, element := range pccPolicy.PccRules {
						policyData.PccPolicy[sliceid].PccRules[index] = element
					}
					for index, element := range pccPolicy.QosDecs {
						policyData.PccPolicy[sliceid].QosDecs[index] = element
					}
					for index, element := range pccPolicy.TraffContDecs {
						policyData.PccPolicy[sliceid].TraffContDecs[index] = element
					}
					policyData.CtxLog.Infof("Subscriber Detals: %v", policyData)
				}
				// self.DisplayPcfSubscriberPolicyData(imsi)
			}
		}

		for _, imsi := range slice.DeletedImsis {
			policyData, ok := self.PcfSubscriberPolicyData[imsi]
			if !ok {
				logger.GrpcLog.Warnf("imsi: %v not exist in SubscriberPolicyData", imsi)
				continue
			}
			_, ok = policyData.PccPolicy[sliceid]
			if !ok {
				logger.GrpcLog.Errorf("PccPolicy for the slice: %v not exist in SubscriberPolicyData", sliceid)
				continue
			}
			// sessionrules, pccrules if exist in slice, implicitly deletes all sessionrules, pccrules for this sliceid
			policyData.CtxLog.Infof("slice: %v deleted from SubscriberPolicyData", sliceid)
			delete(policyData.PccPolicy, sliceid)
			if len(policyData.PccPolicy) == 0 {
				policyData.CtxLog.Infof("Subscriber Deleted from PcfSubscriberPolicyData map")
				delete(self.PcfSubscriberPolicyData, imsi)
			}
		}

	case protos.OpType_SLICE_DELETE:
		logger.GrpcLog.Infof("Received Slice with OperationType: Delete from ConfigPod")
		for _, imsi := range slice.DeletedImsis {
			policyData, ok := self.PcfSubscriberPolicyData[imsi]
			if !ok {
				logger.GrpcLog.Errorf("imsi: %v not exist in SubscriberPolicyData", imsi)
				continue
			}
			_, ok = policyData.PccPolicy[sliceid]
			if !ok {
				logger.GrpcLog.Errorf("PccPolicy for the slice: %v not exist in SubscriberPolicyData", sliceid)
				continue
			}
			policyData.CtxLog.Infof("slice: %v deleted from SubscriberPolicyData", sliceid)
			delete(policyData.PccPolicy, sliceid)
			if len(policyData.PccPolicy) == 0 {
				policyData.CtxLog.Infof("Subscriber Deleted from PcfSubscriberPolicyData map")
				delete(self.PcfSubscriberPolicyData, imsi)
			}
		}
	}
}

func (pcf *PCF) UpdateDnnList(ns *protos.NetworkSlice) {
	sliceid := ns.Nssai.Sst + ns.Nssai.Sd
	pcfContext := context.PCF_Self()
	pcfConfig := factory.PcfConfig.Configuration
	switch ns.OperationType {
	case protos.OpType_SLICE_ADD:
		fallthrough
	case protos.OpType_SLICE_UPDATE:
		var dnnList []string
		for _, devgroup := range ns.DeviceGroup {
			if devgroup.IpDomainDetails != nil {
				dnnList = append(dnnList, devgroup.IpDomainDetails.DnnName)
			}
		}
		if pcfConfig.DnnList == nil {
			pcfConfig.DnnList = make(map[string][]string)
		}
		pcfConfig.DnnList[sliceid] = dnnList
	case protos.OpType_SLICE_DELETE:
		delete(pcfConfig.DnnList, sliceid)
	}
	s := fmt.Sprintf("Updated Slice level DnnList[%v]: ", sliceid)
	for _, dnn := range pcfConfig.DnnList[sliceid] {
		s += fmt.Sprintf("%v ", dnn)
	}
	logger.GrpcLog.Infoln(s)

	pcfContext.DnnList = nil
	for _, slice := range pcfConfig.DnnList {
		for _, dnn := range slice {
			var found bool
			for _, d := range pcfContext.DnnList {
				if d == dnn {
					found = true
				}
			}
			if !found {
				pcfContext.DnnList = append(pcfContext.DnnList, dnn)
			}
		}
	}
	logger.GrpcLog.Infof("DnnList Present in PCF: %v", pcfContext.DnnList)
}

func (pcf *PCF) UpdatePlmnList(ns *protos.NetworkSlice) {
	sliceid := ns.Nssai.Sst + ns.Nssai.Sd
	pcfContext := context.PCF_Self()
	pcfConfig := factory.PcfConfig.Configuration
	switch ns.OperationType {
	case protos.OpType_SLICE_ADD:
		fallthrough
	case protos.OpType_SLICE_UPDATE:
		temp := factory.PlmnSupportItem{}
		if ns.Site.Plmn != nil {
			temp.PlmnId.Mcc = ns.Site.Plmn.Mcc
			temp.PlmnId.Mnc = ns.Site.Plmn.Mnc
		}
		if pcfConfig.SlicePlmn == nil {
			pcfConfig.SlicePlmn = make(map[string]factory.PlmnSupportItem)
		}
		pcfConfig.SlicePlmn[sliceid] = temp
	case protos.OpType_SLICE_DELETE:
		delete(pcfConfig.SlicePlmn, sliceid)
	}
	s := fmt.Sprintf("Updated Slice level Plmn[%v]: %v", sliceid, pcfConfig.SlicePlmn[sliceid])
	logger.GrpcLog.Infoln(s)
	pcfContext.PlmnList = nil
	for _, plmn := range pcfConfig.SlicePlmn {
		var found bool
		for _, p := range pcfContext.PlmnList {
			if p == plmn {
				found = true
				break
			}
		}
		if !found {
			pcfContext.PlmnList = append(pcfContext.PlmnList, plmn)
		}
	}
	logger.GrpcLog.Infof("PlmnList Present in PCF: %v", pcfContext.PlmnList)
}

func (pcf *PCF) updateConfig(commChannel chan *protos.NetworkSliceResponse) bool {
	var minConfig bool
	pcfContext := context.PCF_Self()
	for rsp := range commChannel {
		logger.GrpcLog.Infoln("Received updateConfig in the pcf app : ", rsp)
		for _, ns := range rsp.NetworkSlice {
			logger.GrpcLog.Infoln("Network Slice Name ", ns.Name)

			// Update Qos Info
			// Update/Create/Delete PcfSubscriberPolicyData
			pcf.UpdatePcfSubsriberPolicyData(ns)

			pcf.UpdateDnnList(ns)

			if ns.Site != nil {
				site := ns.Site
				logger.GrpcLog.Infof("Network Slice [%v] has site name: %v", ns.Nssai.Sst+ns.Nssai.Sd, site.SiteName)
				if site.Plmn != nil {
					pcf.UpdatePlmnList(ns)
				} else {
					logger.GrpcLog.Infof("Plmn not present in the sitename: %v of Slice: %v", site.SiteName, ns.Nssai.Sst+ns.Nssai.Sd)
				}
			}
		}
		// minConfig is 'true' when one slice is configured at least.
		// minConfig is 'false' when no slice configuration.
		// check PlmnList for each configuration update from Roc/Simapp.
		if !minConfig {
			// For each slice Plmn is the mandatory parameter, checking PlmnList length is greater than zero
			// setting minConfig to true
			if len(pcfContext.PlmnList) > 0 {
				minConfig = true
				ConfigPodTrigger <- true
				// Start Heart Beat timer for periodic config updates to NRF
				logger.GrpcLog.Infoln("Send config trigger to main routine first time config")
			}
		} else if minConfig { // one or more slices are configured hence minConfig is true
			// minConfig is true but PlmnList is '0' means slices were configured then deleted.
			if len(pcfContext.PlmnList) == 0 {
				minConfig = false
				ConfigPodTrigger <- false
				logger.GrpcLog.Infoln("Send config trigger to main routine config deleted")
			} else {
				// configuration update from simapp/RoC
				ConfigPodTrigger <- true
				logger.GrpcLog.Infoln("Send config trigger to main routine config updated")
			}
		}
	}
	return true
}
