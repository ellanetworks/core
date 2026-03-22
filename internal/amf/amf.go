// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package amf

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/util/idgenerator"
	"github.com/free5gc/nas/nasConvert"
	"go.uber.org/zap"
)

// Authenticator is the interface the AMF requires from the AUSF.
// *ausf.AUSF satisfies this interface directly.
type Authenticator interface {
	Authenticate(ctx context.Context, suci, servingNetwork string, resync *ausf.ResyncInfo) (*ausf.AuthResult, error)
	Confirm(ctx context.Context, resStar, suci string) (etsi.SUPI, string, error)
}

const (
	MaxValueOfAmfUeNgapID int64 = 1099511627775
	PreallocateTmsi       uint  = 20
)

type SmfSbi interface {
	smf.SessionQuerier
	CreateSmContext(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, dnn string, snssai *models.Snssai, n1Msg []byte) (string, []byte, error)
	ActivateSmContext(ctx context.Context, smContextRef string) ([]byte, error)
	DeactivateSmContext(ctx context.Context, smContextRef string) error
	ReleaseSmContext(ctx context.Context, smContextRef string) error
	UpdateSmContextN1Msg(ctx context.Context, smContextRef string, n1Msg []byte) (*smf.UpdateResult, error)
	UpdateSmContextN2InfoPduResSetupRsp(ctx context.Context, smContextRef string, n2Data []byte) error
	UpdateSmContextN2InfoPduResSetupFail(ctx context.Context, smContextRef string, n2Data []byte) error
	UpdateSmContextN2InfoPduResRelRsp(ctx context.Context, smContextRef string) error
	UpdateSmContextCauseDuplicatePDUSessionID(ctx context.Context, smContextRef string) ([]byte, error)
	UpdateSmContextN2HandoverPreparing(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error)
	UpdateSmContextN2HandoverPrepared(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error)
	UpdateSmContextXnHandoverPathSwitchReq(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error)
	UpdateSmContextHandoverFailed(ctx context.Context, smContextRef string, n2Data []byte) error
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

	// Allocators (owned, not exported)
	tmsi    *etsi.TmsiAllocator
	ngapIDs *idgenerator.IDGenerator

	DBInstance               DBer
	Ausf                     Authenticator
	UEs                      map[etsi.SUPI]*AmfUe
	Radios                   map[*sctp.SCTPConn]*Radio
	RelativeCapacity         int64
	Name                     string
	NetworkFeatureSupport5GS *NetworkFeatureSupport5GS
	T3502Value               time.Duration
	T3512Value               time.Duration
	TimeZone                 string // "[+-]HH:MM[+][1-2]", Refer to TS 29.571 - 5.2.2 Simple Data Types
	T3513Cfg                 TimerValue
	T3522Cfg                 TimerValue
	T3550Cfg                 TimerValue
	T3555Cfg                 TimerValue
	T3560Cfg                 TimerValue
	T3565Cfg                 TimerValue
	Smf                      SmfSbi
}

func (a *AMF) allocateTMSI(ctx context.Context) (etsi.TMSI, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	val, err := a.tmsi.Allocate(ctx)
	if err != nil {
		return val, fmt.Errorf("could not allocate TMSI: %v", err)
	}

	return val, nil
}

func (a *AMF) allocateAmfUeNgapID() (int64, error) {
	val, err := a.ngapIDs.Allocate()
	if err != nil {
		return -1, fmt.Errorf("could not allocate AmfUeNgapID: %v", err)
	}

	return val, nil
}

func (amf *AMF) AddAmfUeToUePool(ue *AmfUe) error {
	if !ue.Supi.IsValid() {
		return fmt.Errorf("supi is empty")
	}

	amf.Mutex.Lock()
	defer amf.Mutex.Unlock()

	amf.UEs[ue.Supi] = ue
	ue.smf = amf.Smf

	return nil
}

func (amf *AMF) DeregisterAndRemoveAMFUE(ctx context.Context, ue *AmfUe) {
	ue.Deregister(ctx)

	if ue.RanUe != nil {
		err := ue.RanUe.Remove()
		if err != nil {
			logger.AmfLog.Error("failed to remove RAN UE", zap.Error(err))
		}
	}

	amf.tmsi.Free(ue.Tmsi)

	if ue.implicitDeregistrationTimer != nil {
		ue.implicitDeregistrationTimer.Stop()
		ue.implicitDeregistrationTimer = nil
	}

	if ue.mobileReachableTimer != nil {
		ue.mobileReachableTimer.Stop()
		ue.mobileReachableTimer = nil
	}

	if !ue.Supi.IsValid() {
		return
	}

	amf.Mutex.Lock()
	delete(amf.UEs, ue.Supi)
	amf.Mutex.Unlock()
}

func (amf *AMF) DeregisterSubscriber(ctx context.Context, supi etsi.SUPI) {
	ue, ok := amf.FindAMFUEBySupi(supi)
	if !ok {
		logger.AmfLog.Debug("UE with SUPI not found", logger.SUPI(supi.String()))
		return
	}

	amf.DeregisterAndRemoveAMFUE(ctx, ue)
	logger.AmfLog.Info("removed ue context", logger.SUPI(supi.String()))
}

func (amf *AMF) FindAMFUEBySupi(supi etsi.SUPI) (*AmfUe, bool) {
	amf.Mutex.Lock()
	defer amf.Mutex.Unlock()

	value, ok := amf.UEs[supi]
	if !ok {
		return nil, false
	}

	return value, true
}

// GetUESnapshot atomically looks up the UE by SUPI and returns a
// point-in-time snapshot of its connection state. Returns the snapshot
// and true if the UE exists, or a zero-value snapshot and false otherwise.
func (amf *AMF) GetUESnapshot(supi etsi.SUPI) (UESnapshot, bool) {
	ue, ok := amf.FindAMFUEBySupi(supi)
	if !ok {
		return UESnapshot{}, false
	}

	return ue.Snapshot(), true
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

	now := time.Now()
	radio := Radio{
		NGAPSender: &send.RealNGAPSender{
			Conn: conn,
		},
		RanUEs:        make(map[int64]*RanUe),
		SupportedTAIs: make([]SupportedTAI, 0),
		Conn:          conn,
		ConnectedAt:   now,
		LastSeenAt:    now,
		Log:           logger.AmfLog.With(logger.RanAddr(remoteAddr.String())),
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

func (amf *AMF) CountRadios() int {
	amf.Mutex.Lock()
	defer amf.Mutex.Unlock()

	return len(amf.Radios)
}

func (amf *AMF) CountRegisteredSubscribers() int {
	amf.Mutex.Lock()
	defer amf.Mutex.Unlock()

	count := 0

	for _, ue := range amf.UEs {
		if ue.GetState() == Registered {
			count++
		}
	}

	return count
}

func (amf *AMF) RemoveRadio(ran *Radio) {
	ran.RemoveAllUeInRan()

	amf.Mutex.Lock()
	defer amf.Mutex.Unlock()

	delete(amf.Radios, ran.Conn)
}

func (amf *AMF) FindAmfUeByGuti(guti etsi.GUTI) (*AmfUe, bool) {
	if guti == etsi.InvalidGUTI {
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

// GetNetworkFeatureSupport returns the 5GS network feature support config.
// If not configured, returns a zero-value struct with Enable set to true (the default).
func (amf *AMF) GetNetworkFeatureSupport() NetworkFeatureSupport5GS {
	if amf.NetworkFeatureSupport5GS != nil {
		return *amf.NetworkFeatureSupport5GS
	}

	return NetworkFeatureSupport5GS{Enable: true}
}

// New creates a fully initialized AMF. All dependencies are explicit
// parameters; allocators are owned and zeroed. Call Start to open the
// N2 listener.
func New(db DBer, ausf Authenticator, smf SmfSbi) *AMF {
	a := &AMF{
		UEs:                      make(map[etsi.SUPI]*AmfUe),
		Radios:                   make(map[*sctp.SCTPConn]*Radio),
		DBInstance:               db,
		Ausf:                     ausf,
		Smf:                      smf,
		tmsi:                     etsi.NewTMSIAllocator(),
		ngapIDs:                  idgenerator.NewGenerator(1, MaxValueOfAmfUeNgapID),
		Name:                     "amf",
		RelativeCapacity:         0xff,
		TimeZone:                 nasConvert.GetTimeZone(time.Now()),
		T3502Value:               720 * time.Second,
		T3512Value:               3600 * time.Second,
		T3513Cfg:                 defaultTimerCfg,
		T3522Cfg:                 defaultTimerCfg,
		T3550Cfg:                 defaultTimerCfg,
		T3555Cfg:                 defaultTimerCfg,
		T3560Cfg:                 defaultTimerCfg,
		T3565Cfg:                 defaultTimerCfg,
		NetworkFeatureSupport5GS: &NetworkFeatureSupport5GS{Enable: true},
	}

	return a
}

var defaultTimerCfg = TimerValue{
	Enable:        true,
	ExpireTime:    6 * time.Second,
	MaxRetryTimes: 4,
}

// NewRanUe allocates a new RAN UE context on the given radio.
func (a *AMF) NewRanUe(radio *Radio, ranUeNgapID int64) (*RanUe, error) {
	amfUeNgapID, err := a.allocateAmfUeNgapID()
	if err != nil {
		return nil, fmt.Errorf("error allocating amf ue ngap id: %+v", err)
	}

	ranUe := &RanUe{
		AmfUeNgapID: amfUeNgapID,
		RanUeNgapID: ranUeNgapID,
		Radio:       radio,
		Log:         radio.Log.With(logger.AmfUeNgapID(amfUeNgapID)),
		freeNgapID:  a.ngapIDs.FreeID,
	}

	radio.RanUEs[ranUeNgapID] = ranUe

	return ranUe, nil
}

// ReAllocateGuti allocates a new 5G-GUTI for the UE and preserves the old one.
func (a *AMF) ReAllocateGuti(ctx context.Context, ue *AmfUe, supportedGuami *models.Guami) error {
	ue.OldTmsi = ue.Tmsi

	tmsi, err := a.allocateTMSI(ctx)
	if err != nil {
		return fmt.Errorf("failed to allocate TMSI: %v", err)
	}

	ue.Tmsi = tmsi
	ue.OldGuti = ue.Guti
	ue.Guti, err = etsi.NewGUTI(
		supportedGuami.PlmnID.Mcc,
		supportedGuami.PlmnID.Mnc,
		supportedGuami.AmfID,
		tmsi,
	)

	return err
}

// FreeOldGuti releases the previous TMSI/GUTI for the UE.
func (a *AMF) FreeOldGuti(ue *AmfUe) {
	a.tmsi.Free(ue.OldTmsi)
	ue.OldGuti = etsi.InvalidGUTI
	ue.OldTmsi = etsi.InvalidTMSI
}

func (amf *AMF) StmsiToGuti(ctx context.Context, buf [7]byte) (etsi.GUTI, error) {
	operatorInfo, err := amf.GetOperatorInfo(ctx)
	if err != nil {
		return etsi.InvalidGUTI, fmt.Errorf("could not get operator info: %v", err)
	}

	tmpReginID := operatorInfo.Guami.AmfID[:2]
	amfID := hex.EncodeToString(buf[1:3])

	tmsi5G, err := etsi.NewTMSI(binary.BigEndian.Uint32(buf[3:]))
	if err != nil {
		return etsi.InvalidGUTI, err
	}

	guti, err := etsi.NewGUTI(operatorInfo.Guami.PlmnID.Mcc, operatorInfo.Guami.PlmnID.Mnc, tmpReginID+amfID, tmsi5G)
	if err != nil {
		return etsi.InvalidGUTI, err
	}

	return guti, nil
}
