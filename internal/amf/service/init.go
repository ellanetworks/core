package service

import (
	"fmt"
	_ "net/http/pprof" // Using package only for invoking initialization.
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/fsm"
	"github.com/omec-project/util/http2_util"
	logger_util "github.com/omec-project/util/logger"
	"github.com/sirupsen/logrus"
	gClient "github.com/yeastengine/config5g/proto/client"
	protos "github.com/yeastengine/config5g/proto/sdcoreConfig"
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
)

type AMF struct{}

const IMSI_PREFIX = "imsi-"

var RocUpdateConfigChannel chan bool

var initLog *logrus.Entry

var (
	KeepAliveTimer      *time.Timer
	KeepAliveTimerMutex sync.Mutex
)

func init() {
	initLog = logger.InitLog
	RocUpdateConfigChannel = make(chan bool)
}

func (amf *AMF) Initialize(c factory.Config) {
	factory.InitConfigFactory(c)
	amf.setLogLevel()
	client := gClient.ConnectToConfigServer(factory.AmfConfig.Configuration.WebuiUri, "amf")
	configChannel := client.PublishOnConfigChange(true)
	go amf.UpdateConfig(configChannel)
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

	// go amf.SendNFProfileUpdateToNrf()

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

// func (amf *AMF) SendNFProfileUpdateToNrf() {
// 	// for rocUpdateConfig := range RocUpdateConfigChannel {
// 	for rocUpdateConfig := range RocUpdateConfigChannel {
// 		if rocUpdateConfig {
// 			self := context.AMF_Self()
// 			util.InitAmfContext(self)

// 			var profile models.NfProfile
// 			if profileTmp, err := consumer.BuildNFInstance(self); err != nil {
// 				logger.CfgLog.Errorf("Build AMF Profile Error: %v", err)
// 				continue
// 			} else {
// 				profile = profileTmp
// 			}

// 			amf.StartKeepAliveTimer(profile)
// 			logger.CfgLog.Infof("Sent Register NF Instance with updated profile")
// 		}
// 	}
// }

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
