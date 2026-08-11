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
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/util/idgenerator"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

// localTimeZone formats now's UTC offset as the TS 29.571 time-zone string
// "[+-]HH:MM[+][1-2]" carried in the Local time zone and Universal time IEs.
func localTimeZone(now time.Time) string {
	_, offset := now.Zone()
	if now.IsDST() {
		offset -= 3600
	}

	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}

	tz := fmt.Sprintf("%s%02d:%02d", sign, offset/3600, (offset%3600)/60)
	if now.IsDST() {
		tz += "+1"
	}

	return tz
}

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
	CreateSmContext(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, dnn string, snssai *models.Snssai, requestType fgs.RequestType, n1Msg []byte, epsBearerIdentity uint8) (string, []byte, error)
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
	UpdateSmContextN2HandoverCanceled(ctx context.Context, smContextRef string) error
	UpdateSmContextN2HandoverFailed(ctx context.Context, smContextRef string, n2Data []byte) error
	UpdateSmContextXnHandoverPathSwitchReq(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error)
	UpdateSmContextN2ModifyIndication(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error)
	UpdateSmContextXnHandoverFailed(ctx context.Context, smContextRef string, n2Data []byte) error
	ReconcileSmContext(ctx context.Context, req *models.SessionReconcileRequest) error
	GetSessionPolicy(ctx context.Context, supi etsi.SUPI, snssai *models.Snssai, dnn string) (*smf.Policy, error)
	HandlePagingFailure(ctx context.Context, supi etsi.SUPI, pduSessionID uint8) error
	ClearPagingSuppression(ctx context.Context, supi etsi.SUPI, pduSessionID uint8) error
}

type NetworkFeatureSupport5GS struct {
	Enable  bool
	ImsVoPS uint8
	Emc     uint8
	Emf     uint8
	Mpsi    uint8
	EmcN3   uint8
	Mcsi    uint8
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
	HandleNAS(ctx context.Context, ue *UeConn, nasPdu []byte)
	IsServiceRequest(nasPdu []byte) bool
	HandleServiceRequest(ctx context.Context, ue *UeConn, nasPdu []byte)
}

// LPPHandler is called by the AMF when an UL NAS Transport carries an LPP payload.
// The AMF looks up the UE by SUPI and forwards the LPP data to the handler (LMF).
type LPPHandler interface {
	ForwardLPP(ctx context.Context, supi etsi.SUPI, correlationID, lppData []byte) error
}

// Concurrency model — a registry lock, a per-UE lock, and atomics:
//
//   - AMF.mu guards the registry and connection lifecycle: the UEs/uesByTmsi maps,
//     the radios/radiosByID maps, the conns index (UE-associated NGAP connections by
//     AMF-UE-NGAP-ID, plus their identity fields RanUeNgapID/owning radio), the
//     handover FSM (ue.handover), and the UE's 5G-GUTI/5G-TMSI identity keys.
//   - UeContext.Mutex guards that UE's data: the security context (keys, NAS COUNTs,
//     the NH/NCC key chain), the mobility state, the SM contexts, and the UeConn
//     binding (ue.active).
//   - Hot non-security fields are atomics: last-seen and the active NAS connection
//     pointer.
//
// Shared invariant: security key material — the keys, NAS
// COUNTs, and the NH/NCC key chain — is derived, read, and committed only under
// UeContext.Mutex, never under the registry lock.
//
// Lock ordering (acquire in this order, never reverse):
//
//	AMF.mu  →  UeContext.Mutex
//
// Never hold UeContext.Mutex while acquiring AMF.mu. Never hold any lock across an
// external call (SMF, DB, NGAP send): snapshot, release, then call.
type AMF struct {
	mu sync.RWMutex

	tmsi    *etsi.TmsiAllocator
	connIDs *idgenerator.IDGenerator

	lcsCorrelationSeq atomic.Uint32

	DBInstance               DBer
	Ausf                     Authenticator
	UEs                      map[etsi.SUPI]*UeContext
	uesByTmsi                map[etsi.TMSI]*UeContext // 5G-TMSI (current and in-flight old) -> UE; the full GUTI is rebuilt from the constant GUAMI
	conns                    map[int64]*UeConn        // UE-associated NGAP connections keyed by AMF-UE-NGAP-ID
	radios                   map[NGAPWriter]*Radio
	radiosByID               map[string]*Radio // radios that have claimed a Global RAN Node ID
	RelativeCapacity         int64
	Name                     string
	NetworkFeatureSupport5GS *NetworkFeatureSupport5GS
	T3502Value               time.Duration
	T3512Value               time.Duration
	TimeZone                 string // "[+-]HH:MM[+][1-2]", Refer to TS 29.571 Simple Data Types
	T3513Cfg                 guard.TimerValue
	NASGuardCfg              guard.TimerValue
	handoverGuardTimeout     time.Duration
	Session                  SmfSbi
	NAS                      NASHandler
	LPPHandler               LPPHandler
	EPS                      interworking.EPSPeer
}

func (a *AMF) HandoverGuardTimeout() time.Duration {
	return a.handoverGuardTimeout
}

func (a *AMF) allocateTMSI(ctx context.Context) (etsi.TMSI, error) {
	val, err := a.tmsi.Allocate(ctx)
	if err != nil {
		return val, fmt.Errorf("could not allocate TMSI: %v", err)
	}

	return val, nil
}

func (a *AMF) allocateAmfUeNgapID() (models.AmfUeNgapID, error) {
	val, err := a.connIDs.Allocate()
	if err != nil {
		return -1, fmt.Errorf("could not allocate AmfUeNgapID: %v", err)
	}

	return models.AmfUeNgapID(val), nil
}

// CommitUEIdentity indexes the UE by SUPI and supersedes any prior context for the
// same subscriber, so a subscriber maps to exactly one context. The new context is
// indexed atomically, capturing any prior context under the registry lock; that prior
// context is then fully torn down — including external SMF session release — outside
// the lock, its guarded SUPI-delete leaving the new index intact. The AuthProof
// witnesses that the registration was authenticated first, so an unauthenticated
// registration citing a victim's identity can never index itself or tear down the
// victim's context (TS 24.501 §4.4.4.3).
func (amf *AMF) CommitUEIdentity(ctx context.Context, ue *UeContext, _ AuthProof) error {
	if !ue.supi.IsValid() {
		return fmt.Errorf("supi is empty")
	}

	amf.mu.Lock()
	old, superseded := amf.UEs[ue.supi]
	superseded = superseded && old != ue
	amf.UEs[ue.supi] = ue
	ue.smf = amf.Session
	amf.mu.Unlock()

	if superseded {
		amf.DeregisterAndRemoveUeContext(ctx, old)
	}

	return nil
}

// claimRelease atomically marks the RAN UE as having a UE Context Release Command in
// flight, returning false when one already is, so a duplicate is suppressed. The flag
// is not cleared: a completed release removes the RAN UE, and a failed send leaves it
// claimed.
func (amf *AMF) claimRelease(ueConn *UeConn) bool {
	amf.mu.Lock()
	defer amf.mu.Unlock()

	if ueConn.releasing {
		return false
	}

	ueConn.releasing = true

	return true
}

func (amf *AMF) DeregisterAndRemoveUeContext(ctx context.Context, ue *UeContext) {
	// Defuse idle-mode supervision so a mobile-reachable/implicit-dereg callback
	// cannot fire against a UE being torn down (e.g. network-initiated
	// deregistration of an idle UE).
	amf.stopIdleTimers(ue)

	// Capture the connection before Deregister releases it (Release clears ue.active but
	// leaves conn.ue intact).
	ueConn := ue.active.Load()

	ue.Deregister(ctx)

	// Only remove the UeConn if it still belongs to this context: a fresh re-registration
	// transfers the shared radio connection to a new context before this superseded husk is
	// torn down, and removing it then would kill the live registration.
	if ueConn != nil && ueConn.ue.Load() == ue {
		err := amf.RemoveUeConn(ctx, ueConn)
		if err != nil {
			logger.AmfLog.Error("failed to remove RAN UE", zap.Error(err))
		}
	}

	amf.mu.Lock()
	amf.releaseTmsisLocked(ue)

	// Only delete the SUPI index if it still points to this context: an authenticated
	// re-registration indexes the new context under the same SUPI before this superseded
	// context is torn down, and deleting unconditionally would drop the live registration.
	if ue.supi.IsValid() && amf.UEs[ue.supi] == ue {
		delete(amf.UEs, ue.supi)
	}

	amf.mu.Unlock()
}

func (amf *AMF) DeregisterSubscriber(ctx context.Context, supi etsi.SUPI) {
	ue, ok := amf.LookupUeBySupi(supi)
	if !ok {
		logger.AmfLog.Debug("UE with SUPI not found", logger.SUPI(supi.String()))
		return
	}

	// A connected UE with a security context is told to deregister over the air,
	// guarded by T3522; the accept — or T3522 exhaustion — then removes the
	// context. An idle or unsecured UE cannot be signalled, so it is removed
	// locally.
	if ue.Conn() != nil && ue.secured {
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

func (amf *AMF) LookupUeBySupi(supi etsi.SUPI) (*UeContext, bool) {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	value, ok := amf.UEs[supi]
	if !ok {
		return nil, false
	}

	return value, true
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
		amf:           amf,
		supportedTAIs: make([]SupportedTAI, 0),
		Conn:          conn,
		connectedAt:   now,
		Log:           logger.AmfLog.With(logger.RanAddr(remoteAddr.String())),
	}

	radio.SetLastSeenAt(now)

	amf.mu.Lock()
	defer amf.mu.Unlock()

	amf.radios[conn] = &radio

	return &radio, nil
}

func (amf *AMF) FindRadioByConn(conn *sctp.SCTPConn) (*Radio, bool) {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	ran, ok := amf.radios[conn]
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
//
// UE contexts are released on both the evicted radio and radio itself: NG Setup
// re-initialises the NGAP UE-related contexts unless the two nodes agree to
// retain them (TS 38.413 §8.7.1.1), and Ella Core never offers UE retention. A
// gNB repeating NG Setup on its existing association — what an SCTP restart
// produces — would otherwise keep UEs the gNB has already forgotten.
func (amf *AMF) ClaimRanID(radio *Radio, ranNodeID ngap.GlobalRANNodeID) *Radio {
	newID := util.RANNodeIDToModels(ranNodeID)
	present := ranPresentFor(ranNodeID.Kind)

	key, _ := radioIDKey(&newID)

	amf.mu.Lock()

	evicted := amf.radiosByID[key]
	if evicted == radio {
		evicted = nil
	}

	if evicted != nil {
		delete(amf.radios, evicted.Conn)
	}

	if oldKey, ok := radioIDKey(radio.RanID); ok && oldKey != key {
		delete(amf.radiosByID, oldKey)
	}

	radio.RanPresent = present
	radio.RanID = &newID
	amf.radiosByID[key] = radio
	amf.mu.Unlock()

	amf.RemoveAllUeInRan(context.Background(), radio)

	if evicted != nil {
		amf.RemoveAllUeInRan(context.Background(), evicted)

		if evicted.Conn != nil {
			// Aborted, not shut down: the incumbent has been superseded and a
			// graceful close would stall this NG Setup until it times out.
			_ = evicted.Abort()
		}
	}

	return evicted
}

// RebindRanID re-keys a connected radio onto the Global RAN Node ID a RAN
// CONFIGURATION UPDATE carries, so the TNLA stays associated with the right
// NG-C interface instance (TS 38.413 §8.7.2.2).
//
// Unlike ClaimRanID it leaves every UE context alone, because §8.7.2.1 states
// the procedure "does not affect existing UE-related contexts, if any". For the
// same reason it never evicts an incumbent holding the same identity: it reports
// false and leaves the registry untouched, so a conflicting update costs nobody
// their sessions.
func (amf *AMF) RebindRanID(radio *Radio, ranNodeID ngap.GlobalRANNodeID) bool {
	newID := util.RANNodeIDToModels(ranNodeID)

	key, ok := radioIDKey(&newID)
	if !ok {
		return false
	}

	amf.mu.Lock()
	defer amf.mu.Unlock()

	if holder, taken := amf.radiosByID[key]; taken && holder != radio {
		return false
	}

	if oldKey, ok := radioIDKey(radio.RanID); ok && oldKey == key {
		return true
	} else if ok {
		delete(amf.radiosByID, oldKey)
	}

	radio.RanPresent = ranPresentFor(ranNodeID.Kind)
	radio.RanID = &newID
	amf.radiosByID[key] = radio

	return true
}

// ranPresentFor maps a Global RAN Node ID alternative onto the node kind the
// Radio records. The three ng-eNB macro variants are one kind here: they differ
// only in identifier width, which RanID already carries.
func ranPresentFor(kind ngap.RANNodeIDKind) int {
	switch kind {
	case ngap.RANNodeIDGNB:
		return RanPresentGNbID
	case ngap.RANNodeIDMacroNgENB, ngap.RANNodeIDShortMacroNgENB, ngap.RANNodeIDLongMacroNgENB:
		return RanPresentNgeNbID
	case ngap.RANNodeIDN3IWF:
		return RanPresentN3IwfID
	}

	return 0
}

// ListRadios returns an immutable snapshot of every connected radio for status/API,
// so the live *Radio never leaves the AMF.
func (amf *AMF) ListRadios() []RadioInfo {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	out := make([]RadioInfo, 0, len(amf.radios))
	for _, ran := range amf.radios {
		out = append(out, ran.info())
	}

	return out
}

// HasRadio reports whether a radio with the given RAN node name is connected.
func (amf *AMF) HasRadio(name string) bool {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	for _, ran := range amf.radios {
		if ran.name == name {
			return true
		}
	}

	return false
}

// ConnectedRadios returns the live radios, for internal send paths (paging, drain)
// that must reach the connection. Never hand these to the API — use ListRadios.
func (amf *AMF) ConnectedRadios() []*Radio {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	out := make([]*Radio, 0, len(amf.radios))
	for _, ran := range amf.radios {
		out = append(out, ran)
	}

	return out
}

func (amf *AMF) CountRadios() int {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	return len(amf.radios)
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
	amf.RemoveAllUeInRan(ctx, ran)

	amf.mu.Lock()
	defer amf.mu.Unlock()

	delete(amf.radios, ran.Conn)

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

	radio.amf = amf

	if radio.Conn == nil {
		radio.Conn = conn
	}

	amf.radios[radio.Conn] = radio

	if key, ok := radioIDKey(radio.RanID); ok {
		amf.radiosByID[key] = radio
	}
}

func (amf *AMF) LookupUeConn(amfUeNgapID models.AmfUeNgapID) *UeConn {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	return amf.conns[int64(amfUeNgapID)]
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
		uesByTmsi:                make(map[etsi.TMSI]*UeContext),
		conns:                    make(map[int64]*UeConn),
		radios:                   make(map[NGAPWriter]*Radio),
		radiosByID:               make(map[string]*Radio),
		DBInstance:               db,
		Ausf:                     ausf,
		Session:                  smf,
		tmsi:                     etsi.NewTMSIAllocator(),
		connIDs:                  idgenerator.NewGenerator(1, MaxValueOfAmfUeNgapID),
		Name:                     "amf",
		RelativeCapacity:         0xff,
		TimeZone:                 localTimeZone(time.Now()),
		T3502Value:               720 * time.Second,
		T3512Value:               3600 * time.Second,
		T3513Cfg:                 defaultTimerCfg,
		NASGuardCfg:              defaultTimerCfg,
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

var defaultTimerCfg = guard.TimerValue{
	Enable:        true,
	ExpireTime:    6 * time.Second,
	MaxRetryTimes: 4,
}

// NewUeConn allocates a new RAN UE context on the given radio.
func (a *AMF) NewUeConn(radio *Radio, ranUeNgapID models.RanUeNgapID) (*UeConn, error) {
	amfUeNgapID, err := a.allocateAmfUeNgapID()
	if err != nil {
		return nil, fmt.Errorf("error allocating amf ue ngap id: %+v", err)
	}

	ueConn := &UeConn{
		AmfUeNgapID: amfUeNgapID,
		RanUeNgapID: ranUeNgapID,
		conn:        radio.Conn,
		radioName:   radio.name,
		amf:         a,
		Log:         radio.Log.With(logger.AmfUeNgapID(amfUeNgapID)),
	}

	a.mu.Lock()
	a.conns[int64(amfUeNgapID)] = ueConn
	a.mu.Unlock()

	return ueConn, nil
}

// Leaving the mobile-reachable or implicit-deregistration timers armed lets a
// deregistration reach the SMF, and through it the UPF, after both have closed.
// The three families each need their own lock.
func (amf *AMF) StopAllTimers() {
	amf.mu.Lock()

	ues := make([]*UeContext, 0, len(amf.UEs))
	for _, ue := range amf.UEs {
		ues = append(ues, ue)
	}

	conns := make([]*UeConn, 0, len(amf.conns))
	for _, c := range amf.conns {
		conns = append(conns, c)
	}

	// Bumping idleGen defuses a callback already in flight.
	for _, ue := range ues {
		amf.stopIdleTimersLocked(ue)
	}

	amf.mu.Unlock()

	for _, ue := range ues {
		ue.mu.Lock()
		ue.stopUeMuTimersLocked()
		ue.mu.Unlock()
	}

	// A UE in CM-IDLE has no connection and a bare connection has no UE, so
	// neither list alone covers both.
	for _, c := range conns {
		c.StopNASGuard()
		c.releaseGuard.Stop()
	}
}

// GetUELocation returns the UserLocation for a registered UE, or false if the UE
// is not found in the AMF's UE pool.
func (amf *AMF) GetUELocation(supi etsi.SUPI) (models.UserLocation, bool) {
	ue, ok := amf.LookupUeBySupi(supi)
	if !ok {
		return models.UserLocation{}, false
	}

	return ue.GetUserLocation(), true
}

func (amf *AMF) IsUERegistered(supi etsi.SUPI) bool {
	ue, ok := amf.LookupUeBySupi(supi)
	if !ok {
		return false
	}

	return ue.State() == Registered
}

// RefreshLocation triggers an active location refresh by sending a
// LocationReportingControl(Direct) NGAP message to the RAN for the given UE.
func (amf *AMF) RefreshLocation(ctx context.Context, supi etsi.SUPI) error {
	ue, ok := amf.LookupUeBySupi(supi)
	if !ok {
		return fmt.Errorf("UE not found")
	}

	ueConn := ue.Conn()
	if ueConn == nil {
		return fmt.Errorf("UE has no active RAN connection")
	}

	if err := ueConn.SendLocationReportingControl(ctx, ngap.EventTypeDirect); err != nil {
		return err
	}

	logger.AmfLog.Info("location refresh triggered via LocationReportingControl(Direct)",
		logger.SUPI(supi.String()),
		zap.Uint64("amf-ue-id", uint64(ueConn.AmfUeNgapID)),
		zap.Uint32("ran-ue-id", uint32(ueConn.RanUeNgapID)),
	)

	return nil
}
