package context

import (
	"fmt"
	"math"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/idgenerator"
	"github.com/yeastengine/ella/internal/amf/db"
	"github.com/yeastengine/ella/internal/amf/factory"
	"github.com/yeastengine/ella/internal/amf/logger"
)

var (
	amfContext                                                = AMFContext{}
	tmsiGenerator                    *idgenerator.IDGenerator = nil
	amfUeNGAPIDGenerator             *idgenerator.IDGenerator = nil
	amfStatusSubscriptionIDGenerator *idgenerator.IDGenerator = nil
	mutex                            sync.Mutex
)

func init() {
	AMF_Self().LadnPool = make(map[string]*LADN)
	AMF_Self().EventSubscriptionIDGenerator = idgenerator.NewGenerator(1, math.MaxInt32)
	AMF_Self().Name = "amf"
	AMF_Self().UriScheme = models.UriScheme_HTTP
	AMF_Self().RelativeCapacity = 0xff
	AMF_Self().ServedGuamiList = make([]models.Guami, 0, MaxNumOfServedGuamiList)
	// AMF_Self().PlmnSupportList = make([]factory.PlmnSupportItem, 0, MaxNumOfPLMNs)
	AMF_Self().NfService = make(map[models.ServiceName]models.NfService)
	AMF_Self().NetworkName.Full = "free5GC"
	tmsiGenerator = idgenerator.NewGenerator(1, math.MaxInt32)
	amfStatusSubscriptionIDGenerator = idgenerator.NewGenerator(1, math.MaxInt32)
	amfUeNGAPIDGenerator = idgenerator.NewGenerator(1, MaxValueOfAmfUeNgapId)
}

type AMFContext struct {
	EventSubscriptionIDGenerator    *idgenerator.IDGenerator
	EventSubscriptions              sync.Map
	UePool                          sync.Map         // map[supi]*AmfUe
	RanUePool                       sync.Map         // map[AmfUeNgapID]*RanUe
	AmfRanPool                      sync.Map         // map[net.Conn]*AmfRan
	LadnPool                        map[string]*LADN // dnn as key
	ServedGuamiList                 []models.Guami
	RelativeCapacity                int64
	NfId                            string
	Name                            string
	NfService                       map[models.ServiceName]models.NfService // nfservice that amf support
	UriScheme                       models.UriScheme
	BindingIPv4                     string
	SBIPort                         int
	NgapPort                        int
	SctpGrpcPort                    int
	HttpIPv6Address                 string
	TNLWeightFactor                 int64
	SupportDnnLists                 []string
	AMFStatusSubscriptions          sync.Map // map[subscriptionID]models.SubscriptionData
	NfStatusSubscriptions           sync.Map // map[NfInstanceID]models.NrfSubscriptionData.SubscriptionId
	AusfUri                         string
	NssfUri                         string
	PcfUri                          string
	SmfUri                          string
	UdmsdmUri                       string
	UdmUecmUri                      string
	SecurityAlgorithm               SecurityAlgorithm
	NetworkName                     factory.NetworkName
	NgapIpList                      []string // NGAP Server IP
	T3502Value                      int      // unit is second
	T3512Value                      int      // unit is second
	Non3gppDeregistrationTimerValue int      // unit is second
	// read-only fields
	T3513Cfg factory.TimerValue
	T3522Cfg factory.TimerValue
	T3550Cfg factory.TimerValue
	T3560Cfg factory.TimerValue
	T3565Cfg factory.TimerValue
}

type AMFContextEventSubscription struct {
	IsAnyUe           bool
	IsGroupUe         bool
	UeSupiList        []string
	Expiry            *time.Time
	EventSubscription models.AmfEventSubscription
}

type SecurityAlgorithm struct {
	IntegrityOrder []uint8 // slice of security.AlgIntegrityXXX
	CipheringOrder []uint8 // slice of security.AlgCipheringXXX
}

func NewPlmnSupportItem() (item factory.PlmnSupportItem) {
	item.SNssaiList = make([]models.Snssai, 0, MaxNumOfSlice)
	return
}

func (context *AMFContext) TmsiAllocate() int32 {
	tmp, err := AllocateUniqueID(&tmsiGenerator, "tmsi")
	val := int32(tmp)
	if err != nil {
		logger.ContextLog.Errorf("Allocate TMSI error: %+v", err)
		return -1
	}
	logger.ContextLog.Infof("Allocate TMSI : %v", val)
	return val
}

func (context *AMFContext) AllocateAmfUeNgapID() (int64, error) {
	val, err := AllocateUniqueID(&amfUeNGAPIDGenerator, "amfUeNgapID")
	if err != nil {
		logger.ContextLog.Errorf("Allocate NgapID error: %+v", err)
		return -1, err
	}
	logger.ContextLog.Infof("Allocate AmfUeNgapID : %v", val)
	return val, nil
}

func (context *AMFContext) AllocateGutiToUe(ue *AmfUe) {
	servedGuami := context.ServedGuamiList[0]
	ue.Tmsi = context.TmsiAllocate()
	plmnID := servedGuami.PlmnId.Mcc + servedGuami.PlmnId.Mnc
	tmsiStr := fmt.Sprintf("%08x", ue.Tmsi)
	ue.Guti = plmnID + servedGuami.AmfId + tmsiStr
}

func (context *AMFContext) ReAllocateGutiToUe(ue *AmfUe) {
	servedGuami := context.ServedGuamiList[0]
	tmsiGenerator.FreeID(int64(ue.Tmsi))
	ue.Tmsi = context.TmsiAllocate()
	plmnID := servedGuami.PlmnId.Mcc + servedGuami.PlmnId.Mnc
	tmsiStr := fmt.Sprintf("%08x", ue.Tmsi)
	ue.Guti = plmnID + servedGuami.AmfId + tmsiStr
}

func (context *AMFContext) AllocateRegistrationArea(ue *AmfUe, anType models.AccessType) {
	// clear the previous registration area if need
	if len(ue.RegistrationArea[anType]) > 0 {
		ue.RegistrationArea[anType] = nil
	}

	// allocate a new tai list as a registration area to ue
	// TODO: algorithm to choose TAI list
	supportTaiList, err := db.GetSupportTaiList()
	if err != nil {
		logger.ContextLog.Errorf("Error getting support tai list: %v", err)
		return
	}
	taiList := make([]models.Tai, len(supportTaiList))
	copy(taiList, supportTaiList)
	for i := range taiList {
		tmp, err := strconv.ParseUint(taiList[i].Tac, 10, 32)
		if err != nil {
			logger.ContextLog.Errorf("Could not convert TAC to int: %v", err)
		}
		taiList[i].Tac = fmt.Sprintf("%06x", tmp)
		logger.ContextLog.Infof("Supported Tai List in AMF Plmn: %v, Tac: %v", taiList[i].PlmnId, taiList[i].Tac)
	}
	for _, supportTai := range taiList {
		if reflect.DeepEqual(supportTai, ue.Tai) {
			ue.RegistrationArea[anType] = append(ue.RegistrationArea[anType], supportTai)
			break
		}
	}
}

func (context *AMFContext) NewAMFStatusSubscription(subscriptionData models.SubscriptionData) (subscriptionID string) {
	tmp, err := amfStatusSubscriptionIDGenerator.Allocate()
	if err != nil {
		logger.ContextLog.Errorf("Allocate subscriptionID error: %+v", err)
		return ""
	}
	id := int32(tmp)
	subscriptionID = strconv.Itoa(int(id))
	context.AMFStatusSubscriptions.Store(subscriptionID, subscriptionData)
	return
}

// Return Value: (subscriptionData *models.SubScriptionData, ok bool)
func (context *AMFContext) FindAMFStatusSubscription(subscriptionID string) (*models.SubscriptionData, bool) {
	if value, ok := context.AMFStatusSubscriptions.Load(subscriptionID); ok {
		subscriptionData := value.(models.SubscriptionData)
		return &subscriptionData, ok
	} else {
		return nil, false
	}
}

func (context *AMFContext) DeleteAMFStatusSubscription(subscriptionID string) {
	context.AMFStatusSubscriptions.Delete(subscriptionID)
	id, err := strconv.ParseInt(subscriptionID, 10, 64)
	if err != nil {
		logger.ContextLog.Error(err)
		return
	}
	amfStatusSubscriptionIDGenerator.FreeID(id)
}

func (context *AMFContext) NewEventSubscription(subscriptionID string, subscription *AMFContextEventSubscription) {
	context.EventSubscriptions.Store(subscriptionID, subscription)
}

func (context *AMFContext) FindEventSubscription(subscriptionID string) (*AMFContextEventSubscription, bool) {
	if value, ok := context.EventSubscriptions.Load(subscriptionID); ok {
		return value.(*AMFContextEventSubscription), ok
	} else {
		return nil, false
	}
}

func (context *AMFContext) DeleteEventSubscription(subscriptionID string) {
	context.EventSubscriptions.Delete(subscriptionID)
	if id, err := strconv.ParseInt(subscriptionID, 10, 32); err != nil {
		logger.ContextLog.Error(err)
	} else {
		context.EventSubscriptionIDGenerator.FreeID(id)
	}
}

func (context *AMFContext) AddAmfUeToUePool(ue *AmfUe, supi string) {
	if len(supi) == 0 {
		logger.ContextLog.Errorf("Supi is nil")
	}
	ue.Supi = supi
	context.UePool.Store(ue.Supi, ue)
}

func (context *AMFContext) NewAmfUe(supi string) *AmfUe {
	mutex.Lock()
	defer mutex.Unlock()
	ue := AmfUe{}
	ue.init()

	if supi != "" {
		context.AddAmfUeToUePool(&ue, supi)
	}

	context.AllocateGutiToUe(&ue)

	return &ue
}

func (context *AMFContext) AmfUeFindByUeContextID(ueContextID string) (*AmfUe, bool) {
	if strings.HasPrefix(ueContextID, "imsi") {
		return context.AmfUeFindBySupi(ueContextID)
	}
	if strings.HasPrefix(ueContextID, "imei") {
		return context.AmfUeFindByPei(ueContextID)
	}
	if strings.HasPrefix(ueContextID, "5g-guti") {
		guti := ueContextID[strings.LastIndex(ueContextID, "-")+1:]
		return context.AmfUeFindByGuti(guti)
	}
	return nil, false
}

func (context *AMFContext) AmfUeFindBySupi(supi string) (ue *AmfUe, ok bool) {
	if value, loadOk := context.UePool.Load(supi); loadOk {
		ue = value.(*AmfUe)
		ok = loadOk
	} else {
		logger.ContextLog.Infoln("Ue with Supi not found : ", supi)
	}

	return
}

func (context *AMFContext) AmfUeFindByPei(pei string) (ue *AmfUe, ok bool) {
	context.UePool.Range(func(key, value interface{}) bool {
		candidate := value.(*AmfUe)
		if ok = (candidate.Pei == pei); ok {
			ue = candidate
			return false
		}
		return true
	})
	return
}

func (context *AMFContext) AmfUeFindBySuci(suci string) (ue *AmfUe, ok bool) {
	context.UePool.Range(func(key, value interface{}) bool {
		candidate := value.(*AmfUe)
		if ok = (candidate.Suci == suci); ok {
			ue = candidate
			return false
		}
		return true
	})
	return
}

func (context *AMFContext) AmfUeDeleteBySuci(suci string) (ue *AmfUe, ok bool) {
	context.UePool.Range(func(key, value interface{}) bool {
		candidate := value.(*AmfUe)
		if ok = (candidate.Suci == suci); ok {
			context.UePool.Delete(candidate.Supi)
			candidate.TxLog.Infof("uecontext removed based on suci")
			candidate.Remove()
			return false
		}
		return true
	})
	return
}

func (context *AMFContext) NewAmfRan(conn net.Conn) *AmfRan {
	ran := AmfRan{}
	ran.SupportedTAList = NewSupportedTAIList()
	ran.Conn = conn
	ran.GnbIp = conn.RemoteAddr().String()
	ran.Log = logger.NgapLog.WithField(logger.FieldRanAddr, conn.RemoteAddr().String())
	context.AmfRanPool.Store(conn, &ran)
	return &ran
}

// use net.Conn to find RAN context, return *AmfRan and ok bit
func (context *AMFContext) AmfRanFindByConn(conn net.Conn) (*AmfRan, bool) {
	if value, ok := context.AmfRanPool.Load(conn); ok {
		return value.(*AmfRan), ok
	}
	return nil, false
}

func (context *AMFContext) NewAmfRanAddr(remoteAddr string) *AmfRan {
	ran := AmfRan{}
	ran.SupportedTAList = NewSupportedTAIList()
	ran.GnbIp = remoteAddr
	ran.Log = logger.NgapLog.WithField(logger.FieldRanAddr, remoteAddr)
	context.AmfRanPool.Store(remoteAddr, &ran)
	return &ran
}

func (context *AMFContext) NewAmfRanId(GnbId string) *AmfRan {
	ran := AmfRan{}
	ran.SupportedTAList = NewSupportedTAIList()
	ran.GnbId = GnbId
	ran.Log = logger.NgapLog.WithField(logger.FieldRanId, GnbId)
	context.AmfRanPool.Store(GnbId, &ran)
	return &ran
}

func (context *AMFContext) AmfRanFindByGnbId(gnbId string) (*AmfRan, bool) {
	if value, ok := context.AmfRanPool.Load(gnbId); ok {
		return value.(*AmfRan), ok
	}
	return nil, false
}

// use ranNodeID to find RAN context, return *AmfRan and ok bit
func (context *AMFContext) AmfRanFindByRanID(ranNodeID models.GlobalRanNodeId) (*AmfRan, bool) {
	var ran *AmfRan
	var ok bool
	context.AmfRanPool.Range(func(key, value interface{}) bool {
		amfRan := value.(*AmfRan)
		switch amfRan.RanPresent {
		case RanPresentGNbId:
			if amfRan.RanId.GNbId.GNBValue == ranNodeID.GNbId.GNBValue {
				ran = amfRan
				ok = true
				return false
			}
		case RanPresentNgeNbId:
			if amfRan.RanId.NgeNbId == ranNodeID.NgeNbId {
				ran = amfRan
				ok = true
				return false
			}
		case RanPresentN3IwfId:
			if amfRan.RanId.N3IwfId == ranNodeID.N3IwfId {
				ran = amfRan
				ok = true
				return false
			}
		}
		return true
	})
	return ran, ok
}

func (context *AMFContext) DeleteAmfRan(conn net.Conn) {
	context.AmfRanPool.Delete(conn)
}

func (context *AMFContext) DeleteAmfRanId(gnbId string) {
	context.AmfRanPool.Delete(gnbId)
}

func (context *AMFContext) InSupportDnnList(targetDnn string) bool {
	for _, dnn := range context.SupportDnnLists {
		if dnn == targetDnn {
			return true
		}
	}
	return false
}

func (ctx *AMFContext) InPlmnSupportList(snssai models.Snssai) bool {
	dbPlmnSupportList, err := db.GetPLMNSupportList()
	if err != nil {
		logger.ContextLog.Errorf("GetPLMNSupportList error: %+v", err)
		return false
	}
	for _, plmnSupportItem := range dbPlmnSupportList {
		for _, snssaiItem := range plmnSupportItem.SNssaiList {
			if reflect.DeepEqual(snssaiItem, snssai) {
				return true
			}
		}
	}
	return false
}

func (context *AMFContext) AmfUeFindByGutiLocal(guti string) (ue *AmfUe, ok bool) {
	context.UePool.Range(func(key, value interface{}) bool {
		candidate := value.(*AmfUe)
		if ok = (candidate.Guti == guti); ok {
			ue = candidate
			return false
		}
		return true
	})

	return
}

func (context *AMFContext) AmfUeFindBySupiLocal(supi string) (ue *AmfUe, ok bool) {
	context.UePool.Range(func(key, value interface{}) bool {
		candidate := value.(*AmfUe)
		if ok = (candidate.Supi == supi); ok {
			ue = candidate
			return false
		}
		return true
	})

	return
}

func (context *AMFContext) AmfUeFindByGuti(guti string) (ue *AmfUe, ok bool) {
	ue, ok = context.AmfUeFindByGutiLocal(guti)
	if ok {
		logger.ContextLog.Infoln("Guti found locally : ", guti)
	} else {
		logger.ContextLog.Infoln("Ue with Guti not found : ", guti)
	}
	return
}

func (context *AMFContext) AmfUeFindByPolicyAssociationID(polAssoId string) (ue *AmfUe, ok bool) {
	context.UePool.Range(func(key, value interface{}) bool {
		candidate := value.(*AmfUe)
		if ok = (candidate.PolicyAssociationId == polAssoId); ok {
			ue = candidate
			return false
		}
		return true
	})
	return
}

func (context *AMFContext) RanUeFindByAmfUeNgapIDLocal(amfUeNgapID int64) *RanUe {
	if value, ok := context.RanUePool.Load(amfUeNgapID); ok {
		return value.(*RanUe)
	} else {
		return nil
	}
}

func (context *AMFContext) RanUeFindByAmfUeNgapID(amfUeNgapID int64) *RanUe {
	ranUe := context.RanUeFindByAmfUeNgapIDLocal(amfUeNgapID)
	if ranUe != nil {
		return ranUe
	}

	logger.ContextLog.Errorf("ranUe not found with AmfUeNgapID")
	return nil
}

func (context *AMFContext) GetIPv4Uri() string {
	return fmt.Sprintf("%s://%s:%d", context.UriScheme, context.BindingIPv4, context.SBIPort)
}

func (context *AMFContext) InitNFService(serivceName []string, version string) {
	tmpVersion := strings.Split(version, ".")
	versionUri := "v" + tmpVersion[0]
	for index, nameString := range serivceName {
		name := models.ServiceName(nameString)
		context.NfService[name] = models.NfService{
			ServiceInstanceId: strconv.Itoa(index),
			ServiceName:       name,
			Versions: &[]models.NfServiceVersion{
				{
					ApiFullVersion:  version,
					ApiVersionInUri: versionUri,
				},
			},
			Scheme:          context.UriScheme,
			NfServiceStatus: models.NfServiceStatus_REGISTERED,
			ApiPrefix:       context.GetIPv4Uri(),
			IpEndPoints: &[]models.IpEndPoint{
				{
					Ipv4Address: context.BindingIPv4,
					Transport:   models.TransportProtocol_TCP,
					Port:        int32(context.SBIPort),
				},
			},
		}
	}
}

// Create new AMF context
func AMF_Self() *AMFContext {
	return &amfContext
}
