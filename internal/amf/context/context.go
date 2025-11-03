// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	ctxt "context"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/idgenerator"
	"go.uber.org/zap"
)

var (
	amfContext                                    = AMFContext{}
	tmsiGenerator        *idgenerator.IDGenerator = nil
	amfUeNGAPIDGenerator *idgenerator.IDGenerator = nil
	mutex                sync.Mutex
)

func init() {
	tmsiGenerator = idgenerator.NewGenerator(1, math.MaxInt32)
	amfUeNGAPIDGenerator = idgenerator.NewGenerator(1, MaxValueOfAmfUeNgapID)
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

type Security struct {
	IntegrityOrder []string
	CipheringOrder []string
}

type PlmnSupportItem struct {
	PlmnID     models.PlmnID
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
	DBInstance                      *db.Database
	UePool                          sync.Map         // map[supi]*AmfUe
	RanUePool                       sync.Map         // map[AmfUeNgapID]*RanUe
	AmfRanPool                      sync.Map         // map[net.Conn]*AmfRan
	LadnPool                        map[string]*LADN // dnn as key
	RelativeCapacity                int64
	NfID                            string
	Name                            string
	URIScheme                       models.URIScheme
	NgapPort                        int
	NetworkFeatureSupport5GS        *NetworkFeatureSupport5GS
	SecurityAlgorithm               SecurityAlgorithm
	NetworkName                     NetworkName
	T3502Value                      int // unit is second
	T3512Value                      int // unit is second
	Non3gppDeregistrationTimerValue int // unit is second
	T3513Cfg                        TimerValue
	T3522Cfg                        TimerValue
	T3550Cfg                        TimerValue
	T3560Cfg                        TimerValue
	T3565Cfg                        TimerValue
}

type SecurityAlgorithm struct {
	IntegrityOrder []uint8 // slice of security.AlgIntegrityXXX
	CipheringOrder []uint8 // slice of security.AlgCipheringXXX
}

func (context *AMFContext) TmsiAllocate() int32 {
	val, err := tmsiGenerator.Allocate()
	if err != nil {
		logger.AmfLog.Warn("could not allocate TMSI", zap.Error(err))
		return -1
	}
	return int32(val)
}

func (context *AMFContext) AllocateAmfUeNgapID() (int64, error) {
	val, err := amfUeNGAPIDGenerator.Allocate()
	if err != nil {
		return -1, fmt.Errorf("could not allocate AmfUeNgapID: %v", err)
	}
	return val, nil
}

func (context *AMFContext) AllocateGutiToUe(ctx ctxt.Context, ue *AmfUe) {
	guamis := GetServedGuamiList(ctx)
	servedGuami := guamis[0]
	ue.Tmsi = context.TmsiAllocate()
	plmnID := servedGuami.PlmnID.Mcc + servedGuami.PlmnID.Mnc
	tmsiStr := fmt.Sprintf("%08x", ue.Tmsi)
	ue.Guti = plmnID + servedGuami.AmfID + tmsiStr
}

func (context *AMFContext) ReAllocateGutiToUe(ctx ctxt.Context, ue *AmfUe) {
	guamis := GetServedGuamiList(ctx)
	servedGuami := guamis[0]
	tmsiGenerator.FreeID(int64(ue.Tmsi))
	ue.Tmsi = context.TmsiAllocate()
	plmnID := servedGuami.PlmnID.Mcc + servedGuami.PlmnID.Mnc
	tmsiStr := fmt.Sprintf("%08x", ue.Tmsi)
	ue.Guti = plmnID + servedGuami.AmfID + tmsiStr
}

func (context *AMFContext) AllocateRegistrationArea(ctx ctxt.Context, ue *AmfUe, anType models.AccessType) {
	// clear the previous registration area if need
	if len(ue.RegistrationArea[anType]) > 0 {
		ue.RegistrationArea[anType] = nil
	}

	supportTaiList := GetSupportTaiList(ctx)
	taiList := make([]models.Tai, len(supportTaiList))
	copy(taiList, supportTaiList)
	for i := range taiList {
		tmp, err := strconv.ParseUint(taiList[i].Tac, 10, 32)
		if err != nil {
			logger.AmfLog.Error("Could not convert TAC to int", zap.Error(err))
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
		logger.AmfLog.Error("Supi is nil")
	}
	ue.Supi = supi
	context.UePool.Store(ue.Supi, ue)
}

func (context *AMFContext) NewAmfUe(ctx ctxt.Context, supi string) *AmfUe {
	mutex.Lock()
	defer mutex.Unlock()
	ue := AmfUe{}
	ue.init()

	if supi != "" {
		context.AddAmfUeToUePool(&ue, supi)
	}

	context.AllocateGutiToUe(ctx, &ue)

	return &ue
}

func (context *AMFContext) AmfUeFindByUeContextID(ueContextID string) (*AmfUe, bool) {
	return context.AmfUeFindBySupi(ueContextID)
}

func (context *AMFContext) AmfUeFindBySupi(supi string) (ue *AmfUe, ok bool) {
	if value, loadOk := context.UePool.Load(supi); loadOk {
		ue = value.(*AmfUe)
		ok = loadOk
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

func (context *AMFContext) NewAmfRan(conn *sctp.SCTPConn) *AmfRan {
	ran := AmfRan{}
	ran.SupportedTAList = NewSupportedTAIList()
	ran.Conn = conn
	ran.GnbIP = conn.RemoteAddr().String()
	ran.Log = logger.AmfLog.With(zap.String("ran_addr", conn.RemoteAddr().String()))
	context.AmfRanPool.Store(conn, &ran)
	return &ran
}

// use net.Conn to find RAN context, return *AmfRan and ok bit
func (context *AMFContext) AmfRanFindByConn(conn *sctp.SCTPConn) (*AmfRan, bool) {
	if value, ok := context.AmfRanPool.Load(conn); ok {
		return value.(*AmfRan), ok
	}
	return nil, false
}

// use ranNodeID to find RAN context, return *AmfRan and ok bit
func (context *AMFContext) AmfRanFindByRanID(ranNodeID models.GlobalRanNodeID) (*AmfRan, bool) {
	var ran *AmfRan
	var ok bool
	context.AmfRanPool.Range(func(key, value any) bool {
		amfRan := value.(*AmfRan)
		switch amfRan.RanPresent {
		case RanPresentGNbID:
			if amfRan.RanID.GNbID.GNBValue == ranNodeID.GNbID.GNBValue {
				ran = amfRan
				ok = true
				return false
			}
		case RanPresentNgeNbID:
			if amfRan.RanID.NgeNbID == ranNodeID.NgeNbID {
				ran = amfRan
				ok = true
				return false
			}
		case RanPresentN3IwfID:
			if amfRan.RanID.N3IwfID == ranNodeID.N3IwfID {
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

func (context *AMFContext) DeleteAmfRan(conn *sctp.SCTPConn) {
	context.AmfRanPool.Delete(conn)
}

func (context *AMFContext) InPlmnSupport(ctx ctxt.Context, snssai models.Snssai) bool {
	plmnSupportItem := GetSupportedPlmn(ctx)
	for _, supportSnssai := range plmnSupportItem.SNssaiList {
		if reflect.DeepEqual(supportSnssai, snssai) {
			return true
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
		logger.AmfLog.Info("Guti found locally", zap.String("guti", guti))
	} else {
		logger.AmfLog.Debug("Ue with Guti not found", zap.String("guti", guti))
	}
	return
}

func (context *AMFContext) AmfUeFindByPolicyAssociationID(polAssoID string) (ue *AmfUe, ok bool) {
	context.UePool.Range(func(key, value interface{}) bool {
		candidate := value.(*AmfUe)
		if ok = (candidate.PolicyAssociationID == polAssoID); ok {
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

	return nil
}

func (context *AMFContext) GetIPv4Uri() string {
	return fmt.Sprintf("%s://", context.URIScheme)
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
func AMFSelf() *AMFContext {
	return &amfContext
}
