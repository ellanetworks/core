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
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/guard"
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
	UpdateSmContextN2ModifyIndication(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error)
	UpdateSmContextHandoverFailed(ctx context.Context, smContextRef string, n2Data []byte) error
	ReconcileSmContext(ctx context.Context, req *models.SessionReconcileRequest) error
	GetSessionPolicy(ctx context.Context, supi etsi.SUPI, snssai *models.Snssai, dnn string) (*smf.Policy, error)
	HandlePagingFailure(ctx context.Context, supi etsi.SUPI, pduSessionID uint8) error
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
	// IsServiceRequest reports whether an initial NAS PDU is a SERVICE REQUEST, so the NGAP
	// layer routes it to HandleServiceRequest before minting a context.
	IsServiceRequest(nasPdu []byte) bool
	// HandleServiceRequest resolves-or-rejects an initial SERVICE REQUEST without minting a
	// context (TS 24.501 §5.6.1.5, §4.4.4.3).
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

	// lcsCorrelationSeq issues the LCS correlation identifiers the AMF assigns
	// to LPP transfers (TS 24.501 §5.4.5.3.2 case c, NOTE 2).
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
	// NASGuardCfg configures the single NAS common-procedure supervision timer
	// (T3550/T3555/T3560/T3565/T3570/T3522 — all 6 s ×4 in TS 24.501 §10.2).
	NASGuardCfg guard.TimerValue
	// handoverGuardTimeout bounds an N2 handover (HANDOVER REQUIRED → NOTIFY); see
	// defaultHandoverGuardTimeout.
	handoverGuardTimeout time.Duration
	Session              SmfSbi
	NAS                  NASHandler
	LPPHandler           LPPHandler
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
	if ueConn != nil && ueConn.ue == ue {
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
		delete(amf.radios, evicted.Conn)
	}

	if oldKey, ok := radioIDKey(radio.RanID); ok && oldKey != key {
		delete(amf.radiosByID, oldKey)
	}

	radio.RanPresent = present
	radio.RanID = &newID
	amf.radiosByID[key] = radio
	amf.mu.Unlock()

	if evicted != nil {
		amf.RemoveAllUeInRan(context.Background(), evicted)

		if evicted.Conn != nil {
			_ = evicted.Close()
		}
	}

	return evicted
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
		UEs:              make(map[etsi.SUPI]*UeContext),
		uesByTmsi:        make(map[etsi.TMSI]*UeContext),
		conns:            make(map[int64]*UeConn),
		radios:           make(map[NGAPWriter]*Radio),
		radiosByID:       make(map[string]*Radio),
		DBInstance:       db,
		Ausf:             ausf,
		Session:          smf,
		tmsi:             etsi.NewTMSIAllocator(),
		connIDs:          idgenerator.NewGenerator(1, MaxValueOfAmfUeNgapID),
		Name:             "amf",
		RelativeCapacity: 0xff,
		TimeZone:         nasConvert.GetTimeZone(time.Now()),
		T3502Value:       720 * time.Second,
		// Periodic-registration timer. The spec default of 54 min (TS 24.501 §10.2)
		// is not representable in the GPRS Timer 3 IE — above 31 min it steps in
		// 10-min units (TS 24.008 §10.5.7.4a) — so 54 min encodes down to 50 min and
		// the signalled value diverges from the T3512+4 min mobile-reachable timer.
		// One hour encodes exactly, keeping the two consistent.
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

// SendPaging pages an idle UE and arms its paging-supervision timer. The timer is
// per-UE and persistent (T3513, TS 24.501 §5.4.3): paging targets a UE with no NAS
// connection, so the timer cannot live on the connection object.
func (amf *AMF) SendPaging(ctx context.Context, ue *UeContext, ngapBuf []byte) error {
	if ue == nil {
		return fmt.Errorf("amf ue is nil")
	}

	tmsi := ue.Tmsi()
	logger.From(ctx, logger.AmfLog).Info("Paging", logger.SUPI(ue.Supi().String()), zap.Uint32("5g-tmsi", tmsi.Uint32()))

	amf.pageRadios(ctx, ue, ngapBuf)
	amf.armPaging(ue, ngapBuf)

	return nil
}

// armPaging starts the paging-supervision guard for a UE just paged: retransmit Paging on
// each interval up to a bound, then abandon (T3513, TS 24.501 §5.4.3). Check-and-arm under
// the UE lock so a second downlink trigger cannot reset an in-flight supervision. No-op when
// T3513 is disabled.
func (amf *AMF) armPaging(ue *UeContext, ngapBuf []byte) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.pagingTimer.Active() {
		return
	}

	ue.pagingTimer.ArmWith(amf.T3513Cfg,
		func(attempt int32) { amf.retransmitPaging(ue, ngapBuf, attempt) },
		func() { amf.abandonPaging(ue) })
}

// retransmitPaging resends the Paging each guard interval (T3513, TS 24.501 §5.4.3), or
// stops the guard once the UE has answered by re-establishing its connection.
func (amf *AMF) retransmitPaging(ue *UeContext, ngapBuf []byte, attempt int32) {
	if ue.Conn() != nil {
		ue.pagingTimer.Stop()
		return
	}

	logger.AmfLog.Info("paging unanswered, retransmitting", logger.SUPI(ue.Supi().String()), zap.Int32("attempt", attempt))
	amf.pageRadios(context.Background(), ue, ngapBuf)
}

// abandonPaging runs when the retransmission budget is exhausted (TS 24.501 §5.4.3).
// The anchor is notified so later downlink data re-pages the UE (TS 23.502 §4.2.3.3).
func (amf *AMF) abandonPaging(ue *UeContext) {
	logger.AmfLog.Info("paging unanswered, abandoning procedure", logger.SUPI(ue.Supi().String()))

	msg := ue.N1N2Message()
	if msg == nil {
		return
	}

	if err := amf.Session.HandlePagingFailure(context.Background(), ue.Supi(), msg.PduSessionID); err != nil {
		logger.AmfLog.Warn("failed to re-arm paging after failure",
			logger.SUPI(ue.Supi().String()), zap.Error(err))
	}
}

// pageRadios sends the paging PDU to every radio whose supported TAIs intersect
// the UE's registration area.
func (amf *AMF) pageRadios(ctx context.Context, ue *UeContext, ngapBuf []byte) {
	taiList := ue.RegistrationArea

	for _, ran := range amf.ConnectedRadios() {
		for _, item := range ran.SupportedTAIList() {
			if InTaiList(item.Tai, taiList) {
				if err := amf.SendToRadio(ctx, ran.Conn, send.NGAPProcedurePaging, ngapBuf); err != nil {
					// The send failure is logged at the chokepoint.
					continue
				}

				break
			}
		}
	}
}

// StopAllTimers stops every timer on every UE, so no timer-driven activity fires while
// the system is tearing down.
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
