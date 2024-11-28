package service

import (
	"fmt"
	_ "net/http/pprof" // Using package only for invoking initialization.
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/gin-contrib/cors"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/fsm"
	"github.com/omec-project/util/http2_util"
	logger_util "github.com/omec-project/util/logger"
	"github.com/sirupsen/logrus"
	protos "github.com/yeastengine/config5g/proto/sdcoreConfig"
	"github.com/yeastengine/ella/internal/amf/communication"
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
)

type AMF struct{}

const IMSI_PREFIX = "imsi-"

var initLog *logrus.Entry

func init() {
	initLog = logger.InitLog
}

func (amf *AMF) Initialize(c factory.Config) {
	factory.InitConfigFactory(c)
	amf.setLogLevel()
}

func (amf *AMF) setLogLevel() {
	if level, err := logrus.ParseLevel(factory.AmfConfig.Logger.AMF.DebugLevel); err != nil {
		initLog.Warnf("AMF Log level [%s] is invalid, set to [info] level",
			factory.AmfConfig.Logger.AMF.DebugLevel)
		logger.SetLogLevel(logrus.InfoLevel)
	} else {
		initLog.Infof("AMF Log level is set to [%s] level", level)
		logger.SetLogLevel(level)
	}
	logger.SetReportCaller(factory.AmfConfig.Logger.AMF.ReportCaller)
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

	addr := fmt.Sprintf("%s:%d", self.BindingIPv4, self.SBIPort)

	ngapHandler := ngap_service.NGAPHandler{
		HandleMessage:      ngap.Dispatch,
		HandleNotification: ngap.HandleSCTPNotification,
	}
	ngap_service.Run(self.NgapIpList, self.NgapPort, ngapHandler)

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

	err = server.ListenAndServe()
	if err != nil {
		initLog.Fatalf("HTTP server setup failed: %+v", err)
	}
}

// Used in AMF planned removal procedure
func (amf *AMF) Terminate() {
	logger.InitLog.Infof("Terminating AMF...")
	amfSelf := context.AMF_Self()

	// send AMF status indication to ran to notify ran that this AMF will be unavailable
	logger.InitLog.Infof("Send AMF Status Indication to Notify RANs due to AMF terminating")
	guamiList := context.GetServedGuamiList()
	unavailableGuamiList := ngap_message.BuildUnavailableGUAMIList(guamiList)
	amfSelf.AmfRanPool.Range(func(key, value interface{}) bool {
		ran := value.(*context.AmfRan)
		ngap_message.SendAMFStatusIndication(ran, unavailableGuamiList)
		return true
	})

	ngap_service.Stop()

	callback.SendAmfStatusChangeNotify((string)(models.StatusChange_UNAVAILABLE), guamiList)

	logger.InitLog.Infof("AMF terminated")
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
