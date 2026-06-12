// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/util/idgenerator"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// Authenticator is the interface the AMF requires from the AUSF.
type Authenticator interface {
	Authenticate(ctx context.Context, suci string, plmn models.PlmnID, resync *ausf.ResyncInfo) (*ausf.AuthResult, error)
	Confirm(ctx context.Context, resStar, suci string) (etsi.SUPI, []byte, error)
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
	UpdateSmContextN2HandoverComplete(ctx context.Context, smContextRef string) error
	UpdateSmContextXnHandoverPathSwitchReq(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error)
	UpdateSmContextHandoverFailed(ctx context.Context, smContextRef string, n2Data []byte) error
	ReconcileSmContext(ctx context.Context, req *models.SessionReconcileRequest) error
	GetSessionPolicy(ctx context.Context, supi etsi.SUPI, snssai *models.Snssai, dnn string) (*smf.Policy, error)
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
	GetDataNetworkByID(ctx context.Context, id string) (*db.DataNetwork, error)
	GetNetworkSliceByID(ctx context.Context, id string) (*db.NetworkSlice, error)
	ListNetworkSlicesByIDs(ctx context.Context, ids []string) ([]db.NetworkSlice, error)
	GetProfileByID(ctx context.Context, id string) (*db.Profile, error)
	GetPolicyByProfileAndSlice(ctx context.Context, profileID, sliceID string) (*db.Policy, error)
	ListAllNetworkSlices(ctx context.Context) ([]db.NetworkSlice, error)
	ListPoliciesByProfile(ctx context.Context, profileID string) ([]db.Policy, error)
	NodeID() int
}

type NASHandler interface {
	HandleNAS(ctx context.Context, ue *RanUe, nasPdu []byte) error
}

// LPPHandler is called by the AMF when an UL NAS Transport carries an LPP payload.
// The AMF looks up the UE by SUPI and forwards the LPP data to the handler (LMF).
type LPPHandler interface {
	ForwardLPP(ctx context.Context, supi etsi.SUPI, lppData []byte) error
}

// Lock ordering (acquire in this order, never reverse):
//
//	AMF.mu  →  UeContext.Mutex
//
// Never hold UeContext.Mutex while acquiring AMF.mu.
// Never hold any lock while making external calls (SMF, DB, NGAP send).
type AMF struct {
	mu sync.RWMutex

	tmsi    *etsi.TmsiAllocator
	ngapIDs *idgenerator.IDGenerator

	DBInstance               DBer
	Ausf                     Authenticator
	UEs                      map[etsi.SUPI]*UeContext
	Radios                   map[*sctp.SCTPConn]*Radio
	radiosByID               map[string]*Radio // radios that have claimed a Global RAN Node ID, for O(1) target resolution
	RelativeCapacity         int64
	Name                     string
	NetworkFeatureSupport5GS *NetworkFeatureSupport5GS
	T3502Value               time.Duration
	T3512Value               time.Duration
	TimeZone                 string // "[+-]HH:MM[+][1-2]", Refer to TS 29.571 Simple Data Types
	T3513Cfg                 TimerValue
	T3522Cfg                 TimerValue
	T3550Cfg                 TimerValue
	T3555Cfg                 TimerValue
	T3560Cfg                 TimerValue
	T3565Cfg                 TimerValue
	// handoverGuardTimeout bounds an N2 handover (HANDOVER REQUIRED → NOTIFY); see
	// defaultHandoverGuardTimeout.
	handoverGuardTimeout time.Duration
	Smf                  SmfSbi
	NAS                  NASHandler
	LPPHandler           LPPHandler
}

// HandoverGuardTimeout returns the N2 handover supervision timeout.
func (a *AMF) HandoverGuardTimeout() time.Duration {
	return a.handoverGuardTimeout
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

func (amf *AMF) AddUeContextToPool(ue *UeContext) error {
	if !ue.supi.IsValid() {
		return fmt.Errorf("supi is empty")
	}

	amf.mu.Lock()
	defer amf.mu.Unlock()

	amf.UEs[ue.supi] = ue
	ue.smf = amf.Smf

	return nil
}

func (amf *AMF) DeregisterAndRemoveUeContext(ctx context.Context, ue *UeContext) {
	ue.Deregister(ctx)

	ue.mu.Lock()
	ranUe := ue.ranUe
	ue.mu.Unlock()

	if ranUe != nil {
		err := ranUe.Remove(ctx)
		if err != nil {
			logger.AmfLog.Error("failed to remove RAN UE", zap.Error(err))
		}
	}

	amf.tmsi.Free(ue.Tmsi)

	if !ue.supi.IsValid() {
		return
	}

	amf.mu.Lock()
	delete(amf.UEs, ue.supi)
	amf.mu.Unlock()
}

func (amf *AMF) DeregisterSubscriber(ctx context.Context, supi etsi.SUPI) {
	ue, ok := amf.FindUeContextBySupi(supi)
	if !ok {
		logger.AmfLog.Debug("UE with SUPI not found", logger.SUPI(supi.String()))
		return
	}

	// A connected UE with a security context is told to deregister over the air,
	// guarded by T3522; the accept — or T3522 exhaustion — then removes the
	// context. An idle or unsecured UE cannot be signalled, so it is removed
	// locally.
	if ue.RanUe() != nil && ue.secured {
		if err := amf.sendNetworkInitiatedDeregistration(ctx, ue); err != nil {
			logger.AmfLog.Warn("failed to send network-initiated deregistration; removing UE context locally",
				zap.Error(err), logger.SUPI(supi.String()))
			amf.DeregisterAndRemoveUeContext(ctx, ue)
		}

		return
	}

	amf.DeregisterAndRemoveUeContext(ctx, ue)
	logger.AmfLog.Info("removed ue context", logger.SUPI(supi.String()))
}

func (amf *AMF) FindUeContextBySupi(supi etsi.SUPI) (*UeContext, bool) {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	value, ok := amf.UEs[supi]
	if !ok {
		return nil, false
	}

	return value, true
}

// UESnapshot atomically looks up the UE by SUPI and returns a
// point-in-time snapshot of its connection state.
func (amf *AMF) UESnapshot(supi etsi.SUPI) (UESnapshot, bool) {
	ue, ok := amf.FindUeContextBySupi(supi)
	if !ok {
		return UESnapshot{}, false
	}

	return ue.Snapshot(), true
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
		Log:           logger.AmfLog.With(logger.RanAddr(remoteAddr.String())),
	}

	radio.SetLastSeenAt(now)

	amf.mu.Lock()
	defer amf.mu.Unlock()

	amf.Radios[conn] = &radio

	return &radio, nil
}

func (amf *AMF) FindRadioByConn(conn *sctp.SCTPConn) (*Radio, bool) {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	ran, ok := amf.Radios[conn]
	if !ok {
		return nil, false
	}

	return ran, true
}

// radioIDKey is the radiosByID index key for a Global RAN Node ID, prefixed by
// node type so the gNB/ng-eNB/N3IWF identifier spaces cannot collide. Returns
// false when no identifier is set.
func radioIDKey(id *models.GlobalRanNodeID) (string, bool) {
	switch {
	case id == nil:
		return "", false
	case id.GNbID != nil:
		return "gnb:" + id.GNbID.GNBValue, true
	case id.NgeNbID != "":
		return "ngenb:" + id.NgeNbID, true
	case id.N3IwfID != "":
		return "n3iwf:" + id.N3IwfID, true
	}

	return "", false
}

func (amf *AMF) FindRadioByRanID(ranNodeID models.GlobalRanNodeID) (*Radio, bool) {
	key, ok := radioIDKey(&ranNodeID)
	if !ok {
		return nil, false
	}

	amf.mu.RLock()
	defer amf.mu.RUnlock()

	radio, ok := amf.radiosByID[key]

	return radio, ok
}

// ClaimRanID assigns ranNodeID to radio, evicting any other radio holding the
// same Global RAN Node ID. Returns the evicted radio, or nil.
func (amf *AMF) ClaimRanID(radio *Radio, ranNodeID *ngapType.GlobalRANNodeID) *Radio {
	newID := util.RanIDToModels(*ranNodeID)
	present := ranNodeID.Present

	key, _ := radioIDKey(&newID)

	amf.mu.Lock()

	evicted := amf.radiosByID[key]
	if evicted == radio {
		evicted = nil
	}

	if evicted != nil {
		delete(amf.Radios, evicted.Conn)
	}

	// Drop the radio's stale index entry when it already holds a different ID.
	if oldKey, ok := radioIDKey(radio.RanID); ok && oldKey != key {
		delete(amf.radiosByID, oldKey)
	}

	radio.RanPresent = present
	radio.RanID = &newID
	amf.radiosByID[key] = radio
	amf.mu.Unlock()

	if evicted != nil {
		evicted.RemoveAllUeInRan(context.Background())

		if evicted.Conn != nil {
			_ = evicted.Conn.Close()
		}
	}

	return evicted
}

func (amf *AMF) ListRadios() []*Radio {
	ranList := make([]*Radio, 0)

	amf.mu.RLock()
	defer amf.mu.RUnlock()

	for _, ran := range amf.Radios {
		ranList = append(ranList, ran)
	}

	return ranList
}

func (amf *AMF) CountRadios() int {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	return len(amf.Radios)
}

func (amf *AMF) CountRegisteredSubscribers() int {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	count := 0

	for _, ue := range amf.UEs {
		if ue.State() == Registered {
			count++
		}
	}

	return count
}

// RemoveRadio removes a radio and all UEs bound to it.
func (amf *AMF) RemoveRadio(ctx context.Context, ran *Radio) {
	ran.RemoveAllUeInRan(ctx)

	amf.mu.Lock()
	defer amf.mu.Unlock()

	delete(amf.Radios, ran.Conn)

	if key, ok := radioIDKey(ran.RanID); ok && amf.radiosByID[key] == ran {
		delete(amf.radiosByID, key)
	}
}

// IndexRadioForTest registers a directly-constructed radio in both the
// by-connection and by-RAN-ID maps, mirroring NewRadio followed by ClaimRanID.
// For tests that build a Radio with its RanID already set.
func (amf *AMF) IndexRadioForTest(conn *sctp.SCTPConn, radio *Radio) {
	amf.mu.Lock()
	defer amf.mu.Unlock()

	radio.Conn = conn
	amf.Radios[conn] = radio

	if key, ok := radioIDKey(radio.RanID); ok {
		amf.radiosByID[key] = radio
	}
}

func (amf *AMF) FindUeContextByGuti(guti etsi.GUTI) (*UeContext, bool) {
	if guti == etsi.InvalidGUTI {
		return nil, false
	}

	amf.mu.RLock()
	defer amf.mu.RUnlock()

	for _, ue := range amf.UEs {
		if ue.guti == guti || ue.OldGuti == guti {
			return ue, true
		}
	}

	return nil, false
}

func (amf *AMF) FindRanUeByAmfUeNgapID(amfUeNgapID int64) *RanUe {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	for _, ran := range amf.Radios {
		if ranUe := ran.FindUEByAmfUeNgapID(amfUeNgapID); ranUe != nil {
			return ranUe
		}
	}

	return nil
}

// NetworkFeatureSupport returns the 5GS network feature support config.
// If not configured, returns a zero-value struct with Enable set to true (the default).
func (amf *AMF) NetworkFeatureSupport() NetworkFeatureSupport5GS {
	if amf.NetworkFeatureSupport5GS != nil {
		return *amf.NetworkFeatureSupport5GS
	}

	return NetworkFeatureSupport5GS{Enable: true}
}

// New creates a fully initialized AMF. Call Start to open the N2 listener.
func New(db DBer, ausf Authenticator, smf SmfSbi) *AMF {
	a := &AMF{
		UEs:                      make(map[etsi.SUPI]*UeContext),
		Radios:                   make(map[*sctp.SCTPConn]*Radio),
		radiosByID:               make(map[string]*Radio),
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
		handoverGuardTimeout:     defaultHandoverGuardTimeout,
		NetworkFeatureSupport5GS: &NetworkFeatureSupport5GS{Enable: true, ImsVoPS: 1},
	}

	return a
}

// defaultHandoverGuardTimeout bounds an N2 handover from HANDOVER REQUIRED to
// HANDOVER NOTIFY. It is generous relative to the source gNB's
// TNGRELOCprep/TNGRELOCOverall so a normal handover completes first; it fires
// only when the target gNB never answers (TS 38.413), abandoning the
// half-prepared handover so a silent target cannot pin the UE's N2Handover
// procedure.
const defaultHandoverGuardTimeout = 10 * time.Second

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
		radio:       radio,
		Log:         radio.Log.With(logger.AmfUeNgapID(amfUeNgapID)),
		freeNgapID:  a.ngapIDs.FreeID,
	}

	radio.mu.Lock()
	radio.RanUEs[amfUeNgapID] = ranUe
	radio.mu.Unlock()

	return ranUe, nil
}

// ReAllocateGuti allocates a new 5G-GUTI for the UE and preserves the old one.
func (a *AMF) ReAllocateGuti(ctx context.Context, ue *UeContext, supportedGuami *models.Guami) error {
	ue.OldTmsi = ue.Tmsi

	tmsi, err := a.allocateTMSI(ctx)
	if err != nil {
		return fmt.Errorf("failed to allocate TMSI: %v", err)
	}

	ue.Tmsi = tmsi
	ue.OldGuti = ue.guti
	ue.guti, err = etsi.NewGUTI(
		supportedGuami.PlmnID.Mcc,
		supportedGuami.PlmnID.Mnc,
		supportedGuami.AmfID,
		tmsi,
	)

	return err
}

// FreeOldGuti releases the previous TMSI/GUTI for the UE.
func (a *AMF) FreeOldGuti(ue *UeContext) {
	a.tmsi.Free(ue.OldTmsi)
	ue.OldGuti = etsi.InvalidGUTI
	ue.OldTmsi = etsi.InvalidTMSI
}

func (amf *AMF) StmsiToGuti(ctx context.Context, buf [7]byte) (etsi.GUTI, error) {
	operatorInfo, err := amf.OperatorInfo(ctx)
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

// SendPaging sends a paging message to all radios whose TAIs match the UE's
// registration area. If T3513 is enabled, a retransmission timer is started.
func (amf *AMF) SendPaging(ctx context.Context, ue *UeContext, ngapBuf []byte) error {
	if ue == nil {
		return fmt.Errorf("amf ue is nil")
	}

	amf.mu.RLock()
	defer amf.mu.RUnlock()

	taiList := ue.RegistrationArea

	for _, ran := range amf.Radios {
		for _, item := range ran.SupportedTAIs {
			if InTaiList(item.Tai, taiList) {
				err := ran.NGAPSender.SendToRan(ctx, ngapBuf, send.NGAPProcedurePaging)
				if err != nil {
					ue.Log.Error("failed to send paging", zap.Error(err))
					continue
				}

				ue.Log.Info("sent paging to TAI", zap.Any("tai", item.Tai), zap.Any("tac", item.Tai.Tac))

				break
			}
		}
	}

	if amf.T3513Cfg.Enable {
		cfg := amf.T3513Cfg
		conn := ue.NasConn()
		conn.T3513.Arm(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			ue.Log.Info("t3513 expires, retransmit paging", zap.Int32("retry", expireTimes))

			for _, ran := range amf.ListRadios() {
				for _, item := range ran.SupportedTAIs {
					if InTaiList(item.Tai, taiList) {
						err := ran.NGAPSender.SendToRan(context.Background(), ngapBuf, send.NGAPProcedurePaging)
						if err != nil {
							ue.Log.Error("failed to send paging", zap.Error(err))
							continue
						}

						ue.Log.Info("sent paging to TAI", zap.Any("tai", item.Tai), zap.Any("tac", item.Tai.Tac))

						break
					}
				}
			}
		}, func() {
			ue.Log.Warn("T3513 expires, abort paging procedure", zap.Int32("retry", cfg.MaxRetryTimes))
		})
	}

	return nil
}

// StopAllTimers stops every timer on every UE. Call this during shutdown
// to prevent paging retransmissions and other timer-driven activity from
// firing while the system is tearing down.
func (amf *AMF) StopAllTimers() {
	amf.mu.RLock()

	ues := make([]*UeContext, 0, len(amf.UEs))
	for _, ue := range amf.UEs {
		ues = append(ues, ue)
	}

	amf.mu.RUnlock()

	for _, ue := range ues {
		ue.mu.Lock()
		ue.stopAllTimersLocked()
		ue.mu.Unlock()
	}
}

func (amf *AMF) RemoveUEBySupi(supi etsi.SUPI) {
	amf.mu.Lock()
	defer amf.mu.Unlock()

	delete(amf.UEs, supi)
}

// GetUELocation returns the UserLocation for a registered UE, or false if the UE
// is not found in the AMF's UE pool.
func (amf *AMF) GetUELocation(supi etsi.SUPI) (models.UserLocation, bool) {
	ue, ok := amf.FindUeContextBySupi(supi)
	if !ok {
		return models.UserLocation{}, false
	}

	return ue.GetUserLocation(), true
}

// IsUERegistered returns true if the UE exists in the AMF's UE pool and is in
// the Registered state.
func (amf *AMF) IsUERegistered(supi etsi.SUPI) bool {
	ue, ok := amf.FindUeContextBySupi(supi)
	if !ok {
		return false
	}

	return ue.State() == Registered
}
