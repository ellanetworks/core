// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/idgenerator"
	"go.uber.org/zap"
)

const (
	MaxValueOfAmfUeNgapID int64 = 1099511627775
)

var (
	amfContext                                    = AMF{}
	tmsiGenerator        *idgenerator.IDGenerator = nil
	amfUeNGAPIDGenerator *idgenerator.IDGenerator = nil
)

func init() {
	amfContext = AMF{
		UEs:    make(map[string]*AmfUe),
		Radios: make(map[*sctp.SCTPConn]*Radio),
	}
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

type NetworkName struct {
	Full  string
	Short string
}

type TimerValue struct {
	Enable        bool
	ExpireTime    time.Duration
	MaxRetryTimes int32
}

type DBer interface {
	GetOperator(ctx context.Context) (*db.Operator, error)
	GetSubscriber(ctx context.Context, imsi string) (*db.Subscriber, error)
	GetPolicyByID(ctx context.Context, id int) (*db.Policy, error)
	GetDataNetworkByID(ctx context.Context, id int) (*db.DataNetwork, error)
}

type AMF struct {
	Mutex sync.Mutex

	DBInstance               DBer
	UEs                      map[string]*AmfUe // Key: supi
	Radios                   map[*sctp.SCTPConn]*Radio
	RelativeCapacity         int64
	Name                     string
	NetworkFeatureSupport5GS *NetworkFeatureSupport5GS
	SecurityAlgorithm        SecurityAlgorithm
	NetworkName              NetworkName
	T3502Value               int    // unit is second
	T3512Value               int    // unit is second
	TimeZone                 string // "[+-]HH:MM[+][1-2]", Refer to TS 29.571 - 5.2.2 Simple Data Types
	T3513Cfg                 TimerValue
	T3522Cfg                 TimerValue
	T3550Cfg                 TimerValue
	T3555Cfg                 TimerValue
	T3560Cfg                 TimerValue
	T3565Cfg                 TimerValue
}

type SecurityAlgorithm struct {
	IntegrityOrder []uint8 // slice of security.AlgIntegrityXXX
	CipheringOrder []uint8 // slice of security.AlgCipheringXXX
}

func allocateTMSI() (int32, error) {
	val, err := tmsiGenerator.Allocate()
	if err != nil {
		return -1, fmt.Errorf("could not allocate TMSI: %v", err)
	}

	return int32(val), nil
}

func allocateAmfUeNgapID() (int64, error) {
	val, err := amfUeNGAPIDGenerator.Allocate()
	if err != nil {
		return -1, fmt.Errorf("could not allocate AmfUeNgapID: %v", err)
	}

	return val, nil
}

func (amf *AMF) AddAmfUeToUePool(ue *AmfUe) error {
	if len(ue.Supi) == 0 {
		return fmt.Errorf("supi is empty")
	}

	amf.Mutex.Lock()
	defer amf.Mutex.Unlock()

	amf.UEs[ue.Supi] = ue

	return nil
}

func (amf *AMF) RemoveAMFUE(ue *AmfUe) {
	if ue.RanUe != nil {
		err := ue.RanUe.Remove()
		if err != nil {
			logger.AmfLog.Error("failed to remove RAN UE", zap.Error(err))
		}
	}

	tmsiGenerator.FreeID(int64(ue.Tmsi))

	if ue.Supi == "" {
		return
	}

	amf.Mutex.Lock()
	delete(amf.UEs, ue.Supi)
	amf.Mutex.Unlock()
}

func (amf *AMF) FindAMFUEBySupi(supi string) (*AmfUe, bool) {
	amf.Mutex.Lock()
	defer amf.Mutex.Unlock()

	value, ok := amf.UEs[supi]
	if !ok {
		return nil, false
	}

	return value, true
}

func (amf *AMF) FindAMFUEBySuci(suci string) (*AmfUe, bool) {
	amf.Mutex.Lock()
	defer amf.Mutex.Unlock()

	for _, ue := range amf.UEs {
		if ue.Suci == suci {
			return ue, true
		}
	}

	return nil, false
}

func (amf *AMF) NewRadio(conn *sctp.SCTPConn) (*Radio, error) {
	if conn == nil {
		return nil, fmt.Errorf("SCTP connection is not available")
	}

	remoteAddr := conn.RemoteAddr()

	if remoteAddr == nil {
		return nil, fmt.Errorf("remote address is not available")
	}

	radio := Radio{
		NGAPSender: &send.RealNGAPSender{
			Conn: conn,
		},
		RanUEs:        make(map[int64]*RanUe),
		SupportedTAIs: make([]SupportedTAI, 0),
		Conn:          conn,
		GnbIP:         remoteAddr.String(),
		Log:           logger.AmfLog.With(zap.String("ran_addr", remoteAddr.String())),
	}

	amf.Mutex.Lock()
	defer amf.Mutex.Unlock()

	amf.Radios[conn] = &radio

	return &radio, nil
}

func (amf *AMF) FindRadioByConn(conn *sctp.SCTPConn) (*Radio, bool) {
	amf.Mutex.Lock()
	defer amf.Mutex.Unlock()

	ran, ok := amf.Radios[conn]
	if !ok {
		return nil, false
	}

	return ran, true
}

// use ranNodeID to find RAN context, return *AmfRan and ok bit
func (amf *AMF) FindRadioByRanID(ranNodeID models.GlobalRanNodeID) (*Radio, bool) {
	amf.Mutex.Lock()
	defer amf.Mutex.Unlock()

	for _, amfRan := range amf.Radios {
		switch amfRan.RanPresent {
		case RanPresentGNbID:
			if amfRan.RanID.GNbID.GNBValue == ranNodeID.GNbID.GNBValue {
				return amfRan, true
			}
		case RanPresentNgeNbID:
			if amfRan.RanID.NgeNbID == ranNodeID.NgeNbID {
				return amfRan, true
			}
		case RanPresentN3IwfID:
			if amfRan.RanID.N3IwfID == ranNodeID.N3IwfID {
				return amfRan, true
			}
		}
	}

	return nil, false
}

func (amf *AMF) ListRadios() []Radio {
	ranList := make([]Radio, 0)

	amf.Mutex.Lock()
	defer amf.Mutex.Unlock()

	for _, ran := range amf.Radios {
		ranList = append(ranList, *ran)
	}

	return ranList
}

func (amf *AMF) RemoveRadio(ran *Radio) {
	ran.RemoveAllUeInRan()

	amf.Mutex.Lock()
	defer amf.Mutex.Unlock()

	delete(amf.Radios, ran.Conn)
}

func (amf *AMF) FindAmfUeByGuti(guti string) (*AmfUe, bool) {
	if guti == "" {
		return nil, false
	}

	amf.Mutex.Lock()
	defer amf.Mutex.Unlock()

	for _, ue := range amf.UEs {
		if ue.Guti == guti || ue.OldGuti == guti {
			return ue, true
		}
	}

	return nil, false
}

func (amf *AMF) FindRanUeByAmfUeNgapID(amfUeNgapID int64) *RanUe {
	amf.Mutex.Lock()
	defer amf.Mutex.Unlock()

	for _, ran := range amf.Radios {
		for _, ranUe := range ran.RanUEs {
			if ranUe.AmfUeNgapID == amfUeNgapID {
				return ranUe
			}
		}
	}

	return nil
}

func (amf *AMF) Get5gsNwFeatSuppImsVoPS() uint8 {
	if amf.NetworkFeatureSupport5GS != nil {
		return amf.NetworkFeatureSupport5GS.ImsVoPS
	}

	return 0
}

func (amf *AMF) Get5gsNwFeatSuppEnable() bool {
	if amf.NetworkFeatureSupport5GS != nil {
		return amf.NetworkFeatureSupport5GS.Enable
	}

	return true
}

func (amf *AMF) Get5gsNwFeatSuppEmcN3() uint8 {
	if amf.NetworkFeatureSupport5GS != nil {
		return amf.NetworkFeatureSupport5GS.EmcN3
	}

	return 0
}

func (amf *AMF) Get5gsNwFeatSuppEmc() uint8 {
	if amf.NetworkFeatureSupport5GS != nil {
		return amf.NetworkFeatureSupport5GS.Emc
	}

	return 0
}

func (amf *AMF) Get5gsNwFeatSuppEmf() uint8 {
	if amf.NetworkFeatureSupport5GS != nil {
		return amf.NetworkFeatureSupport5GS.Emf
	}

	return 0
}

func (amf *AMF) Get5gsNwFeatSuppIwkN26() uint8 {
	if amf.NetworkFeatureSupport5GS != nil {
		return amf.NetworkFeatureSupport5GS.IwkN26
	}

	return 0
}

func (amf *AMF) Get5gsNwFeatSuppMpsi() uint8 {
	if amf.NetworkFeatureSupport5GS != nil {
		return amf.NetworkFeatureSupport5GS.Mpsi
	}

	return 0
}

func (amf *AMF) Get5gsNwFeatSuppMcsi() uint8 {
	if amf.NetworkFeatureSupport5GS != nil {
		return amf.NetworkFeatureSupport5GS.Mcsi
	}

	return 0
}

func AMFSelf() *AMF {
	return &amfContext
}

func (amf *AMF) StmsiToGuti(ctx context.Context, buf [7]byte) (string, error) {
	operatorInfo, err := amf.GetOperatorInfo(ctx)
	if err != nil {
		return "", fmt.Errorf("could not get operator info: %v", err)
	}

	tmpReginID := operatorInfo.Guami.AmfID[:2]
	amfID := hex.EncodeToString(buf[1:3])
	tmsi5G := hex.EncodeToString(buf[3:])

	guti := operatorInfo.Guami.PlmnID.Mcc + operatorInfo.Guami.PlmnID.Mnc + tmpReginID + amfID + tmsi5G

	return guti, nil
}
