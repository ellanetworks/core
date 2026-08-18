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
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/fivegskeys"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/interworking"
	lmfmodels "github.com/ellanetworks/core/internal/lmf/models"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/ueauth"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

type SmContext struct {
	Ref                string
	Snssai             *models.Snssai
	Dnn                string
	EBI                uint8
	PduSessionInactive bool
}

type UeContext struct {
	mu sync.Mutex

	state   StateType
	regStep RegStep

	arrivedFromEPSHandover bool
	retainsEPSRegistration bool
	exportableToEPSUntil   time.Time

	PlmnID  models.PlmnID
	Suci    string
	supi    etsi.SUPI
	Imei    etsi.IMEI // PEI carrying the IMEI/IMEISV (TS 23.501 §5.9.3)
	tmsi    etsi.TMSI
	oldTmsi etsi.TMSI

	Location models.UserLocation
	Tai      models.Tai
	lastSeen atomic.Int64 // Unix nanoseconds

	handover *handoverContext

	smf SmfSbi

	active atomic.Pointer[UeConn]

	procedures *procedure.Registry

	secured              bool
	ueSecurityCapability *fgs.UESecurityCapability // TS 24.501 §9.11.3.54

	gmmCapability         *fgs.GMMCapability
	s1UENetworkCapability []byte
	s1ModeAttested        bool

	epsNASAlgorithmsOffered *interworking.EPSNASAlgorithms
	epsNASAlgorithms        *interworking.EPSNASAlgorithms
	epsSecurityCapability   *eps.UESecurityCapability

	ngKsi        models.NgKsi
	knasInt      [16]uint8
	knasEnc      [16]uint8
	kgnb         []uint8
	nh           [32]uint8 // AS key-chain Next Hop (TS 33.501)
	ncc          uint8
	ulCount      nas.UplinkCounter
	dl           *nas.DownlinkSender
	sc           *nas.SecurityContext
	cipheringAlg nas.CipheringAlgorithm
	integrityAlg nas.IntegrityAlgorithm
	kamf         []uint8
	abba         []uint8

	Ambr                     *models.Ambr
	AllowedNssai             []models.Snssai
	RegistrationArea         []models.Tai
	RadioCapability          []byte
	RadioCapabilityForPaging *models.UERadioCapabilityForPaging
	DRXParameter             fgs.DRXValue // 5GS DRX cycle (TS 24.501 §9.11.3.2A)
	SmContextList            map[uint8]*SmContext

	allow4G bool

	mobileReachableTimer        guard.Guard
	implicitDeregistrationTimer guard.Guard
	idleGen                     uint64

	radioMu           sync.RWMutex
	radioMeasurements *lmfmodels.RadioMeasurements

	nrppaMu       sync.RWMutex
	nrppaMessages []NRPPaMessage

	nrppaRoutingIDs map[int64]struct{}

	pagingTimer guard.Guard

	n1n2Message atomic.Pointer[models.N1N2MessageTransferRequest]
}

func NewUeContext() *UeContext {
	ue := &UeContext{
		state:            Deregistered,
		SmContextList:    make(map[uint8]*SmContext),
		RegistrationArea: make([]models.Tai, 0),
		procedures:       procedure.NewRegistry(logger.AmfLog),
		tmsi:             etsi.InvalidTMSI,
		oldTmsi:          etsi.InvalidTMSI,
		dl:               nas.NewDownlinkSender(protectDownlink),
	}

	return ue
}

func (ue *UeContext) Tmsi() etsi.TMSI {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.tmsi
}

func (ue *UeContext) OldTmsi() etsi.TMSI {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.oldTmsi
}

func (ue *UeContext) Procedures() *procedure.Registry {
	return ue.procedures
}

func (ue *UeContext) BeginKeyChainProc(t procedure.Type) bool {
	if ue.procedures == nil {
		return true
	}

	return ue.procedures.Begin(t) == nil
}

func (ue *UeContext) EndKeyChainProc(t procedure.Type) {
	if ue.procedures != nil {
		ue.procedures.End(t)
	}
}

func (ue *UeContext) endKeyChainProcs() {
	ue.EndKeyChainProc(procedure.SecurityMode)
	ue.EndKeyChainProc(procedure.N2Handover)
	ue.EndKeyChainProc(procedure.PathSwitch)
}

func (ue *UeContext) SuperviseKeyChainProc(t procedure.Type, deadline time.Time, cancel procedure.CancelFunc) {
	if ue.procedures != nil {
		_ = ue.procedures.Supervise(t, deadline, cancel)
	}
}

func (ue *UeContext) Conn() *UeConn {
	if ue == nil {
		return nil
	}

	return ue.active.Load()
}

func (ue *UeContext) N1N2Message() *models.N1N2MessageTransferRequest {
	return ue.n1n2Message.Load()
}

func (ue *UeContext) SetN1N2Message(m *models.N1N2MessageTransferRequest) {
	ue.n1n2Message.Store(m)
}

func (ue *UeContext) ClearN1N2Message() {
	ue.n1n2Message.Store(nil)
}

func (a *AMF) attachUeConnLocked(ue *UeContext, ueConn *UeConn) *UeConn {
	oldUeConn := ue.active.Load()

	ueConn.ue.Store(ue)

	var displaced *UeConn

	if oldUeConn != nil && oldUeConn != ueConn {
		if oldUeConn.ue.Load() == ue {
			oldUeConn.Log.Info("Detached UeContext from previous UeConn")
			oldUeConn.ue.Store(nil)
			displaced = oldUeConn
		}
	}

	ue.lastSeen.Store(time.Now().UnixNano())

	ue.active.Store(ueConn)

	a.stopIdleTimersLocked(ue)

	return displaced
}

func (a *AMF) AttachUeConn(ue *UeContext, ueConn *UeConn) {
	if ueConn == nil {
		return
	}

	a.mu.Lock()
	displaced := a.attachUeConnLocked(ue, ueConn)
	a.mu.Unlock()

	if displaced != nil {
		displaced.SendUEContextReleaseCommand(context.Background(),
			ngap.Cause{Group: ngap.CauseGroupNAS, Value: ngap.CauseNASNormalRelease})
	}

	a.clearPagingSuppression(context.Background(), ue)
}

func (a *AMF) clearPagingSuppression(ctx context.Context, ue *UeContext) {
	if a.Session == nil {
		return
	}

	supi := ue.Supi()

	for id := range ue.SmContextSnapshot() {
		if err := a.Session.ClearPagingSuppression(ctx, supi, id); err != nil {
			logger.AmfLog.Warn("failed to clear paging suppression on reconnect",
				logger.SUPI(supi.String()), zap.Error(err))
		}
	}
}

func (ue *UeContext) AllocateRegistrationArea(supportedTais []models.Tai) {
	ue.RegistrationArea = append([]models.Tai(nil), supportedTais...)
}

func (ue *UeContext) IsAllowedNssai(targetSNssai *models.Snssai) bool {
	for _, s := range ue.AllowedNssai {
		if s.Equal(*targetSNssai) {
			return true
		}
	}

	return false
}

func (ue *UeContext) SecurityContextIsValid() bool {
	return ue.secured && ue.ngKsi.Ksi != int32(nas.NoKeyAvailable)
}

func (ue *UeContext) TouchLastSeen() {
	ue.lastSeen.Store(time.Now().UnixNano())
}

func (ue *UeContext) lastSeenTime() time.Time {
	ns := ue.lastSeen.Load()
	if ns == 0 {
		return time.Time{}
	}

	return time.Unix(0, ns)
}

func (ue *UeContext) SetLastSeenForTest(t time.Time) {
	ue.lastSeen.Store(t.UnixNano())
}

type UESnapshot struct {
	Imei               string
	LastSeenAt         time.Time
	CipheringAlgorithm string
	IntegrityAlgorithm string
}

func (ue *UeContext) Snapshot() UESnapshot {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	snap := UESnapshot{
		Imei:               ue.Imei.IMEI(),
		LastSeenAt:         ue.lastSeenTime(),
		CipheringAlgorithm: cipheringAlgName(ue.cipheringAlg),
		IntegrityAlgorithm: integrityAlgName(ue.integrityAlg),
	}

	return snap
}

func (ue *UeContext) DeriveKamf(kseaf []byte) error {
	if !ue.supi.IsValid() || !ue.supi.IsIMSI() {
		return fmt.Errorf("supi is not a valid IMSI")
	}

	P0 := []byte(ue.supi.IMSI())
	L0 := ueauth.KDFLen(P0)
	P1 := ue.abba
	L1 := ueauth.KDFLen(P1)

	kAmfBytes, err := ueauth.GetKDFValue(kseaf, ueauth.FCForKamfDerivation, P0, L0, P1, L1)
	if err != nil {
		return fmt.Errorf("could not get kdf value: %v", err)
	}

	ue.kamf = kAmfBytes

	return nil
}

func (ue *UeContext) InstallNASSecurityContext(nea nas.CipheringAlgorithm, nia nas.IntegrityAlgorithm, _ AuthProof) error {
	ue.mu.Lock()
	ue.cipheringAlg, ue.integrityAlg = nea, nia
	err := ue.deriveAlgKeyLocked()
	sc := ue.sc

	if err == nil {
		ue.ulCount.Reset()
	}
	ue.mu.Unlock()

	if err != nil {
		ue.dl.Clear()

		return err
	}

	ue.dl.Install(sc, nas.DownlinkCounter{})

	return nil
}

func (ue *UeContext) deriveAlgKeyLocked() error {
	kenc, err := fivegskeys.DeriveKNASEnc(ue.kamf, ue.cipheringAlg)
	if err != nil {
		return err
	}

	kint, err := fivegskeys.DeriveKNASInt(ue.kamf, ue.integrityAlg)
	if err != nil {
		return err
	}

	ue.knasEnc, ue.knasInt = kenc, kint

	return ue.installSecurityContextLocked()
}

func (ue *UeContext) installSecurityContextLocked() error {
	sc, err := nas.NewSecurityContext(nas.SecurityContextOptions{
		Integrity:    ue.integrityAlg,
		Ciphering:    ue.cipheringAlg,
		IntegrityKey: ue.knasInt,
		CipherKey:    ue.knasEnc,
		// NIA0 is deliberately selectable by the operator.
		AllowNullIntegrity: ue.integrityAlg == nas.IntegrityNull,
	})
	if err != nil {
		ue.sc = nil

		return err
	}

	ue.sc = sc

	return nil
}

func (ue *UeContext) deriveAnKeyLocked() error {
	key, err := fivegskeys.DeriveKgNB(ue.kamf, ue.ulCount.LastAccepted().Value())
	if err != nil {
		return err
	}

	ue.kgnb = key[:]

	return nil
}

func (ue *UeContext) DeriveNH(syncInput []byte) error {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.deriveNHLocked(syncInput)
}

func (ue *UeContext) deriveNHLocked(syncInput []byte) error {
	nh, err := fivegskeys.DeriveNH(ue.kamf, syncInput)
	if err != nil {
		return err
	}

	ue.nh = nh

	return nil
}

func (ue *UeContext) UpdateSecurityContext() error {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if err := ue.deriveAnKeyLocked(); err != nil {
		return fmt.Errorf("error deriving AnKey: %v", err)
	}

	if err := ue.deriveNHLocked(ue.kgnb); err != nil {
		return fmt.Errorf("error deriving NH: %v", err)
	}

	ue.ncc = 1

	return nil
}

func (ue *UeContext) AdvancePathSwitchNH() (nh [32]uint8, ncc uint8, err error) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.deriveNextNHLocked()
}

func (ue *UeContext) deriveNextNHLocked() ([32]uint8, uint8, error) {
	nh, err := fivegskeys.DeriveNH(ue.kamf, ue.nh[:])
	if err != nil {
		return [32]uint8{}, 0, err
	}

	return nh, (ue.ncc + 1) % 8, nil
}

func (ue *UeContext) ClearRegistrationRequestData() {
	conn := ue.Conn()

	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.arrivedFromEPSHandover = false
	ue.retainsEPSRegistration = false

	if conn == nil {
		return
	}

	conn.RegistrationRequest = nil
	conn.RegistrationRequestPlain = nil
	conn.RegistrationType5GS = 0
	conn.IdentityTypeUsedForRegistration = 0
	conn.resyncTried = false
	conn.RetransmissionOfInitialNASMsg = false
	conn.RegistrationAcceptPlain = nil
	conn.EPSArrival = nil

	if r := ue.active.Load(); r != nil {
		r.UeContextRequest = false
	}

	for _, t := range ue.procedures.ActiveTypes() {
		ue.procedures.End(procedure.Type(t))
	}
}

func (ue *UeContext) ClearRegistrationData(ctx context.Context) {
	ue.releaseSmContexts(ctx)
}

func (ue *UeContext) CreateSmContext(pduSessionID uint8, ref string, snssai *models.Snssai, dnn string) error {
	if pduSessionID < 1 || pduSessionID > 15 {
		return fmt.Errorf("invalid PDU session ID %d: must be in range 1-15 per TS 24.501", pduSessionID)
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.SmContextList[pduSessionID] = &SmContext{
		Ref:    ref,
		Snssai: snssai,
		Dnn:    dnn,
	}

	return nil
}

func (ue *UeContext) takeSmContexts() map[uint8]*SmContext {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	taken := ue.SmContextList
	ue.SmContextList = make(map[uint8]*SmContext)

	return taken
}

func (ue *UeContext) adoptSmContexts(carried map[uint8]*SmContext) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	for pduSessionID, smContext := range carried {
		if _, held := ue.SmContextList[pduSessionID]; held {
			continue
		}

		ue.SmContextList[pduSessionID] = smContext
	}
}

func (ue *UeContext) DeleteSmContext(pduSessionID uint8) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	delete(ue.SmContextList, pduSessionID)
}

func (ue *UeContext) SmContextFindByPDUSessionID(pduSessionID uint8) (*SmContext, bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	smContext, ok := ue.SmContextList[pduSessionID]

	return smContext, ok
}

func (ue *UeContext) SetSmContextInactive(pduSessionID uint8) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if sc, ok := ue.SmContextList[pduSessionID]; ok {
		sc.PduSessionInactive = true
	}
}

func (ue *UeContext) SetSmContextActive(pduSessionID uint8) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if sc, ok := ue.SmContextList[pduSessionID]; ok {
		sc.PduSessionInactive = false
	}
}

func (ue *UeContext) HasActivePduSessions() bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	for _, smContext := range ue.SmContextList {
		if !smContext.PduSessionInactive {
			return true
		}
	}

	return false
}

func protectDownlink(plain []byte, sht uint8, count nas.Count, sc *nas.SecurityContext) ([]byte, error) {
	headerType := fgs.SecurityHeaderType(sht)

	switch headerType {
	case fgs.SHTIntegrityProtected, fgs.SHTIntegrityProtectedCiphered, fgs.SHTIntegrityProtectedNewContext:
	default:
		return nil, fmt.Errorf("wrong security header type: 0x%0x", sht)
	}

	return fgs.Protect(plain, headerType, count, nas.DirectionDownlink, sc)
}

func (ue *UeContext) SendDownlinkNAS(plain []byte, sht uint8, write nas.WriteFunc) error {
	if ue == nil {
		return fmt.Errorf("amf ue is nil")
	}

	if fgs.SecurityHeaderType(sht) == fgs.SHTPlain || !ue.Secured() {
		return write(plain)
	}

	if err := ue.dl.Send(plain, sht, write); err != nil {
		return err
	}

	if fgs.SecurityHeaderType(sht).Ciphered() {
		ue.Conn().MarkCipheringStarted()
	}

	return nil
}

func (ue *UeContext) StopProcedureTimers() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	conn := ue.Conn()
	if conn == nil {
		return
	}

	conn.StopNASGuard()
}

func (a *AMF) ReleaseNasConnection(ue *UeContext, target *UeConn) {
	if ue == nil {
		return
	}

	conn := ue.Conn()
	if conn == nil {
		return
	}

	a.mu.Lock()
	detached := a.detachUeConnLocked(ue, target)
	a.mu.Unlock()

	if detached == nil {
		return
	}

	ue.endKeyChainProcs()
	ue.StopProcedureTimers()

	detached.Release()
}

func (a *AMF) detachUeConnLocked(ue *UeContext, target *UeConn) *UeConn {
	cur := ue.active.Load()
	if cur == nil {
		return nil
	}

	if target != nil && cur != target {
		return nil
	}

	if cur.ue.Load() == ue {
		cur.ue.Store(nil)
	}

	ue.active.Store(nil)

	return cur
}

func (ue *UeContext) stopUeMuTimersLocked() {
	ue.pagingTimer.Stop()
}

func (ue *UeContext) StopPaging() {
	if ue == nil {
		return
	}

	ue.pagingTimer.Stop()
}

func (ue *UeContext) PagingActive() bool {
	if ue == nil {
		return false
	}

	return ue.pagingTimer.Active()
}

func (ue *UeContext) Deregister(ctx context.Context) {
	if conn := ue.Conn(); conn != nil {
		conn.Release()
	}

	ue.endKeyChainProcs()

	ue.mu.Lock()

	ue.stopUeMuTimersLocked()

	ue.transitionToLocked(Deregistered)

	smContextRefs := make([]string, 0, len(ue.SmContextList))
	for _, smContext := range ue.SmContextList {
		smContextRefs = append(smContextRefs, smContext.Ref)
	}

	ue.SmContextList = make(map[uint8]*SmContext)
	ue.mu.Unlock()

	if ue.smf != nil {
		for _, smContextRef := range smContextRefs {
			err := ue.smf.ReleaseSmContext(ctx, smContextRef)
			if err != nil {
				logger.From(ctx, logger.AmfLog).Error("Release SmContext Error", zap.Error(err))
			}
		}
	}

	logger.From(ctx, logger.AmfLog).Debug("ue deregistered", logger.SUPI(ue.supi.String()))
}

func (ue *UeContext) deactivateSmContexts(ctx context.Context) {
	if ue == nil || ue.smf == nil {
		return
	}

	for _, ref := range ue.SmContextRefs() {
		if err := ue.smf.DeactivateSmContext(ctx, ref.Ref); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("failed to deactivate SM context for paging", zap.Error(err), zap.Uint8("PduSessionID", ref.PduSessionID))
		}
	}
}

func (ue *UeContext) releaseSmContexts(ctx context.Context) {
	ue.mu.Lock()

	smContextRefs := make([]string, 0, len(ue.SmContextList))
	for _, smContext := range ue.SmContextList {
		smContextRefs = append(smContextRefs, smContext.Ref)
	}

	ue.SmContextList = make(map[uint8]*SmContext)
	ue.mu.Unlock()

	if ue.smf == nil {
		return
	}

	for _, smContextRef := range smContextRefs {
		err := ue.smf.ReleaseSmContext(ctx, smContextRef)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Error("Release SmContext Error", zap.Error(err))
		}
	}
}

func (ue *UeContext) GetUserLocation() models.UserLocation {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.Location
}

func (ue *UeContext) IsUserLocationEmpty() bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	loc := ue.Location

	return loc.EutraLocation == nil &&
		loc.NrLocation == nil &&
		loc.N3gaLocation == nil
}

func (ue *UeContext) DeleteSmContextRef(pduSessionID uint8, ref string) bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	sc, ok := ue.SmContextList[pduSessionID]
	if !ok || sc.Ref != ref {
		return false
	}

	delete(ue.SmContextList, pduSessionID)

	return true
}
