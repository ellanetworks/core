// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"fmt"
	"math"
	"net"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/util/idgenerator"
	"github.com/omec-project/openapi/models"
)

var (
	amfContext                                    = AMFContext{}
	tmsiGenerator        *idgenerator.IDGenerator = nil
	amfUeNGAPIDGenerator *idgenerator.IDGenerator = nil
	mutex                sync.Mutex
)

func init() {
	AMF_Self().LadnPool = make(map[string]*LADN)
	AMF_Self().EventSubscriptionIDGenerator = idgenerator.NewGenerator(1, math.MaxInt32)
	AMF_Self().Name = "amf"
	AMF_Self().UriScheme = models.UriScheme_HTTP
	AMF_Self().RelativeCapacity = 0xff
	AMF_Self().NetworkName.Full = "free5GC"
	tmsiGenerator = idgenerator.NewGenerator(1, math.MaxInt32)
	amfUeNGAPIDGenerator = idgenerator.NewGenerator(1, MaxValueOfAmfUeNgapId)
}

type NetworkFeatureSupport5GS struct {
	Enable  bool
	ImsVoPS uint8
	Emc     uint8
	Emf     uint8
	IwkN26  uint8
	Mpsi    uint8
	EmcN3   uint8
	Mcsi    uint8
}

type Sbi struct {
	BindingIPv4 string
	Port        int
}

type Security struct {
	IntegrityOrder []string
	CipheringOrder []string
}

type PlmnSupportItem struct {
	PlmnId     models.PlmnId
	SNssaiList []models.Snssai
}

type NetworkName struct {
	Full  string
	Short string
}

type TimerValue struct {
	Enable        bool
	ExpireTime    time.Duration
	MaxRetryTimes int32
}

type AMFContext struct {
	DbInstance                      *db.Database
	EventSubscriptionIDGenerator    *idgenerator.IDGenerator
	EventSubscriptions              sync.Map
	UePool                          sync.Map         // map[supi]*AmfUe
	RanUePool                       sync.Map         // map[AmfUeNgapID]*RanUe
	AmfRanPool                      sync.Map         // map[net.Conn]*AmfRan
	LadnPool                        map[string]*LADN // dnn as key
	RelativeCapacity                int64
	NfId                            string
	Name                            string
	UriScheme                       models.UriScheme
	NgapPort                        uint16
	NetworkFeatureSupport5GS        *NetworkFeatureSupport5GS
	SctpGrpcPort                    int
	HttpIPv6Address                 string
	TNLWeightFactor                 int64
	SupportDnnLists                 []string
	SecurityAlgorithm               SecurityAlgorithm
	NetworkName                     NetworkName
	NgapIpList                      []string // NGAP Server IP
	T3502Value                      int      // unit is second
	T3512Value                      int      // unit is second
	Non3gppDeregistrationTimerValue int      // unit is second
	// read-only fields
	T3513Cfg TimerValue
	T3522Cfg TimerValue
	T3550Cfg TimerValue
	T3560Cfg TimerValue
	T3565Cfg TimerValue
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

func NewPlmnSupportItem() (item PlmnSupportItem) {
	item.SNssaiList = make([]models.Snssai, 0, MaxNumOfSlice)
	return
}

func (context *AMFContext) TmsiAllocate() int32 {
	tmp, err := AllocateUniqueID(&tmsiGenerator, "tmsi")
	val := int32(tmp)
	if err != nil {
		logger.AmfLog.Errorf("Allocate TMSI error: %+v", err)
		return -1
	}
	return val
}

func (context *AMFContext) AllocateAmfUeNgapID() (int64, error) {
	val, err := AllocateUniqueID(&amfUeNGAPIDGenerator, "amfUeNgapID")
	if err != nil {
		logger.AmfLog.Errorf("Allocate NgapID error: %+v", err)
		return -1, err
	}
	logger.AmfLog.Infof("allocated AmfUeNgapID: %v", val)
	return val, nil
}

func (context *AMFContext) AllocateGutiToUe(ue *AmfUe) {
	guamis := GetServedGuamiList()
	servedGuami := guamis[0]
	ue.Tmsi = context.TmsiAllocate()
	plmnID := servedGuami.PlmnId.Mcc + servedGuami.PlmnId.Mnc
	tmsiStr := fmt.Sprintf("%08x", ue.Tmsi)
	ue.Guti = plmnID + servedGuami.AmfId + tmsiStr
}

func (context *AMFContext) ReAllocateGutiToUe(ue *AmfUe) {
	guamis := GetServedGuamiList()
	servedGuami := guamis[0]
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

	supportTaiList := GetSupportTaiList()
	taiList := make([]models.Tai, len(supportTaiList))
	copy(taiList, supportTaiList)
	for i := range taiList {
		tmp, err := strconv.ParseUint(taiList[i].Tac, 10, 32)
		if err != nil {
			logger.AmfLog.Errorf("Could not convert TAC to int: %v", err)
		}
		taiList[i].Tac = fmt.Sprintf("%06x", tmp)
	}
	for _, supportTai := range taiList {
		if reflect.DeepEqual(supportTai, ue.Tai) {
			ue.RegistrationArea[anType] = append(ue.RegistrationArea[anType], supportTai)
			break
		}
	}
}

func (context *AMFContext) AddAmfUeToUePool(ue *AmfUe, supi string) {
	if len(supi) == 0 {
		logger.AmfLog.Errorf("Supi is nil")
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
	return context.AmfUeFindBySupi(ueContextID)
}

func (context *AMFContext) AmfUeFindBySupi(supi string) (ue *AmfUe, ok bool) {
	if value, loadOk := context.UePool.Load(supi); loadOk {
		ue = value.(*AmfUe)
		ok = loadOk
	} else {
		logger.AmfLog.Infoln("Ue with Supi not found : ", supi)
	}

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
	ran.Log = logger.AmfLog.With(logger.FieldRanAddr, conn.RemoteAddr().String())
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

func (context *AMFContext) NewAmfRanId(GnbId string) *AmfRan {
	ran := AmfRan{}
	ran.SupportedTAList = NewSupportedTAIList()
	ran.GnbId = GnbId
	ran.Log = logger.AmfLog.With(logger.FieldRanId, GnbId)
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

func (context *AMFContext) ListAmfRan() []AmfRan {
	ranList := make([]AmfRan, 0)
	context.AmfRanPool.Range(func(key, value interface{}) bool {
		ran := value.(*AmfRan)
		ranList = append(ranList, *ran)
		return true
	})
	return ranList
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

func (context *AMFContext) InPlmnSupportList(snssai models.Snssai) bool {
	plmnSupportList := GetPlmnSupportList()
	for _, plmnSupportItem := range plmnSupportList {
		for _, supportSnssai := range plmnSupportItem.SNssaiList {
			if reflect.DeepEqual(supportSnssai, snssai) {
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
		logger.AmfLog.Infoln("Guti found locally : ", guti)
	} else {
		logger.AmfLog.Infoln("Ue with Guti not found : ", guti)
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

	logger.AmfLog.Errorf("ranUe not found with AmfUeNgapID")
	return nil
}

func (context *AMFContext) GetIPv4Uri() string {
	return fmt.Sprintf("%s://", context.UriScheme)
}

func (context *AMFContext) Get5gsNwFeatSuppImsVoPS() uint8 {
	if context.NetworkFeatureSupport5GS != nil {
		return context.NetworkFeatureSupport5GS.ImsVoPS
	}
	return 0
}

func (context *AMFContext) Get5gsNwFeatSuppEnable() bool {
	if context.NetworkFeatureSupport5GS != nil {
		return context.NetworkFeatureSupport5GS.Enable
	}
	return true
}

func (context *AMFContext) Get5gsNwFeatSuppEmcN3() uint8 {
	if context.NetworkFeatureSupport5GS != nil {
		return context.NetworkFeatureSupport5GS.EmcN3
	}
	return 0
}

func (context *AMFContext) Get5gsNwFeatSuppEmc() uint8 {
	if context.NetworkFeatureSupport5GS != nil {
		return context.NetworkFeatureSupport5GS.Emc
	}
	return 0
}

func (context *AMFContext) Get5gsNwFeatSuppEmf() uint8 {
	if context.NetworkFeatureSupport5GS != nil {
		return context.NetworkFeatureSupport5GS.Emf
	}
	return 0
}

func (context *AMFContext) Get5gsNwFeatSuppIwkN26() uint8 {
	if context.NetworkFeatureSupport5GS != nil {
		return context.NetworkFeatureSupport5GS.IwkN26
	}
	return 0
}

func (context *AMFContext) Get5gsNwFeatSuppMpsi() uint8 {
	if context.NetworkFeatureSupport5GS != nil {
		return context.NetworkFeatureSupport5GS.Mpsi
	}
	return 0
}

func (context *AMFContext) Get5gsNwFeatSuppMcsi() uint8 {
	if context.NetworkFeatureSupport5GS != nil {
		return context.NetworkFeatureSupport5GS.Mcsi
	}
	return 0
}

// Create new AMF context
func AMF_Self() *AMFContext {
	return &amfContext
}
