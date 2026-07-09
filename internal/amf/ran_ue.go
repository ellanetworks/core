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
	"encoding/hex"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

type RelAction int

const (
	UeContextN2NormalRelease RelAction = iota
	UeContextReleaseHandover
	UeContextReleaseUeContext
	UeContextReleaseDueToNwInitiatedDeregistraion
	// UeContextReleaseAbortRegistration releases the RAN context of a UE whose in-flight
	// registration failed. Cleanup deletes the UE context unconditionally, unlike
	// UeContextReleaseUeContext, which retains a Secured (registered) UE.
	UeContextReleaseAbortRegistration
)

// releaseGuardTimeout bounds the wait for a UE Context Release Complete after a UE
// Context Release Command is sent; on expiry the action-keyed local cleanup runs, so a
// lost Complete cannot leak the UeConn + AMF-UE-NGAP-ID. TS 38.413 §8.3 defines no CN-
// side supervision timer; this is a robustness guard.
const releaseGuardTimeout = 5 * time.Second

// UeConn represents one UE's radio-level state on a single Radio. It has no mutex of
// its own: it is protected either by the owning Radio's single SCTP goroutine, or by
// UeContext.Mutex when accessed via UeContext.Conn(). Callers of UeContext.Conn() must
// capture the returned pointer in a local and reuse it — the pointer may change between
// calls.
type UeConn struct {
	RanUeNgapID  int64
	AmfUeNgapID  int64
	HandOverType ngapType.HandoverType
	Tai          models.Tai
	Location     models.UserLocation
	ue           *UeContext
	// conn is the NGAP association this UE sends through and the key into the AMF's
	// radios index for node metadata (looked up via amf.radioFor(conn)).
	conn NGAPWriter
	// radioName is the node name captured at bind, for hot-path last-seen tagging without
	// a registry lookup. Immutable after bind.
	radioName string
	// amf is the owning registry; connection-lifecycle methods reach amf.mu through it.
	// Always set at creation.
	amf              *AMF
	ReleaseAction    RelAction
	UeContextRequest bool
	// ics is the Initial Context Setup progress (an ICSState). It is read and written
	// from the NGAP dispatch goroutine, the SMF N1N2 path, and the NAS-guard timer
	// callback, so it is atomic; mutate it only through ICS()/ClaimICS()/MarkICS*/ResetICS.
	ics atomic.Int32
	Log *zap.Logger
	// releasing gates a UE Context Release Command so a second one is not sent for the
	// same RAN UE. Guarded by AMF.mu, like the conns registry it lives in.
	releasing bool
	// releaseGuard supervises a sent UE Context Release Command; if the Release Complete
	// is lost it fires once (releaseGuardTimeout) and runs the action-keyed cleanup.
	releaseGuard guard.Guard

	// nasGuard is the single supervision timer for the 5GMM common procedures. They are
	// mutually exclusive, so one guard suffices.
	nasGuard guard.Guard
	// nasGuardName is the procedure the guard currently supervises. It is written on the
	// NAS-dispatch/network-initiated paths and read by the status export goroutine, so it
	// is atomic; use nasGuardProcName() to read.
	nasGuardName atomic.Pointer[string]

	// secureExchangeEstablished records that secure exchange of NAS messages has been
	// established on this connection (a NAS message has been successfully integrity
	// checked). Once set, TS 24.501 requires discarding any further message that is not
	// integrity protected or fails the check.
	secureExchangeEstablished bool

	AuthenticationCtx *ausf.AuthResult
	// resyncTried records whether an SQN re-synchronisation (AUTS) has been attempted
	// this authentication exchange: the first synch failure resyncs, a second rejects
	// (TS 24.501 §5.4.1.3.7 f)/NOTE 4).
	resyncTried bool

	RegistrationRequest             *nasMessage.RegistrationRequest
	RegistrationType5GS             uint8
	IdentityTypeUsedForRegistration uint8
	RetransmissionOfInitialNASMsg   bool

	// RegistrationAcceptPdu is the REGISTRATION ACCEPT last sent, kept to resend on a
	// duplicate REGISTRATION REQUEST with identical IEs while awaiting REGISTRATION
	// COMPLETE (TS 24.501 §5.5.1.2.8 case d).
	RegistrationAcceptPdu []byte
}

// Parent returns the UeContext this connection is bound to, or nil when bare.
func (ueConn *UeConn) Parent() *UeContext {
	return ueConn.ue
}

// Release stops the NAS guard and clears this connection from its UeContext. Clearing
// ue.active is done under the registry lock (amf.mu), like bind, so it cannot race an
// AttachUeConn. A key-changing procedure still in flight is left to its supervision
// deadline (TS 38.413 handover guard), which runs its cleanup.
func (ueConn *UeConn) Release() {
	ueConn.stopTimers()

	a := ueConn.amf
	if a == nil {
		return
	}

	a.mu.Lock()
	if ueConn.ue != nil {
		ueConn.ue.active.CompareAndSwap(ueConn, nil)
	}
	a.mu.Unlock()
}

// stopTimers stops the connection's NAS guard.
func (ueConn *UeConn) stopTimers() {
	ueConn.StopNASGuard()
}

// armNASGuardWith arms the connection's NAS common-procedure guard (a no-op when cfg is
// disabled). The procedures are mutually exclusive, so arming supersedes any prior one.
func (ueConn *UeConn) armNASGuardWith(cfg guard.TimerValue, name string, onRetransmit func(int32), onAbort func()) {
	if !cfg.Enable {
		return
	}

	ueConn.nasGuardName.Store(&name)
	ueConn.nasGuard.Arm(cfg.ExpireTime, cfg.MaxRetryTimes, onRetransmit, onAbort)
}

func (ueConn *UeConn) StopNASGuard() {
	ueConn.nasGuardName.Store(nil)
	ueConn.nasGuard.Stop()
}

// nasGuardProcName returns the procedure the NAS guard currently supervises, or ""
// when none. Safe for concurrent use.
func (ueConn *UeConn) nasGuardProcName() string {
	if p := ueConn.nasGuardName.Load(); p != nil {
		return *p
	}

	return ""
}

func (ueConn *UeConn) NASGuardActive() bool {
	return ueConn.nasGuard.Active()
}

// NASGuardForTest exposes the NAS common-procedure guard for external test packages.
func (ueConn *UeConn) NASGuardForTest() *guard.Guard {
	return &ueConn.nasGuard
}

// AMFForTest returns the owning AMF, for external test packages that reach the
// connection-lifecycle methods through the connection they created.
func (ueConn *UeConn) AMFForTest() *AMF {
	return ueConn.amf
}

// AMFForTest returns the owning AMF, for external test packages that reach the
// radio's registry methods through a radio they created.
func (r *Radio) AMFForTest() *AMF {
	return r.amf
}

// SetRadioForTest registers a radio in the connection index, for external test
// packages that seed a connected gNB without the SCTP setup path.
func (amf *AMF) SetRadioForTest(conn NGAPWriter, r *Radio) {
	amf.mu.Lock()
	defer amf.mu.Unlock()

	amf.radios[conn] = r
}

// ClearRadiosForTest empties the radio index, for tests that assert on no radios.
func (amf *AMF) ClearRadiosForTest() {
	amf.mu.Lock()
	defer amf.mu.Unlock()

	amf.radios = map[NGAPWriter]*Radio{}
}

// RadioForTest returns the radio registered for conn, for tests that assert on the
// radio index membership.
func (amf *AMF) RadioForTest(conn NGAPWriter) (*Radio, bool) {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	r, ok := amf.radios[conn]

	return r, ok
}

// ICSState tracks the AMF-side progress of the NGAP Initial Context Setup
// procedure for one UeConn.
type ICSState int

const (
	// ICSNotStarted: AMF has not sent InitialContextSetupRequest yet.
	ICSNotStarted ICSState = iota
	// ICSPending: InitialContextSetupRequest sent, awaiting response.
	ICSPending
	// ICSCompleted: InitialContextSetupResponse received.
	ICSCompleted
)

// ICS returns the connection's Initial Context Setup progress. Safe for concurrent
// use — it is raced across the NGAP dispatch, SMF N1N2, and NAS-guard timer goroutines.
func (ueConn *UeConn) ICS() ICSState {
	return ICSState(ueConn.ics.Load())
}

// ClaimICS atomically transitions the connection from ICSNotStarted to ICSPending,
// returning true to exactly one caller — the one responsible for sending the
// InitialContextSetupRequest. A false return means the context is already being set up
// (or is up), so the caller sends a standalone PDU Session Resource Setup instead.
func (ueConn *UeConn) ClaimICS() bool {
	return ueConn.ics.CompareAndSwap(int32(ICSNotStarted), int32(ICSPending))
}

// MarkICSPending records that an InitialContextSetupRequest has been sent.
func (ueConn *UeConn) MarkICSPending() {
	ueConn.ics.Store(int32(ICSPending))
}

// MarkICSCompleted records that the InitialContextSetupResponse has been received.
func (ueConn *UeConn) MarkICSCompleted() {
	ueConn.ics.Store(int32(ICSCompleted))
}

// ResetICS returns the connection to ICSNotStarted, rolling back a claim whose send
// failed so a retry re-attempts the context setup.
func (ueConn *UeConn) ResetICS() {
	ueConn.ics.Store(int32(ICSNotStarted))
}

// The registration/auth status fields (RegistrationType5GS, IdentityTypeUsedForRegistration,
// RetransmissionOfInitialNASMsg, resyncTried) are written on the NAS goroutine and read by
// the status export from another goroutine. The setters below publish them under the parent
// UeContext lock — the lock the export holds — so the export never observes a torn write. The
// same-goroutine reads (registration/auth logic) stay plain: they race neither the export nor
// each other.

func (ueConn *UeConn) SetRegistrationType5GS(v uint8) {
	if ueConn.ue != nil {
		ueConn.ue.mu.Lock()
		defer ueConn.ue.mu.Unlock()
	}

	ueConn.RegistrationType5GS = v
}

func (ueConn *UeConn) SetIdentityTypeUsedForRegistration(v uint8) {
	if ueConn.ue != nil {
		ueConn.ue.mu.Lock()
		defer ueConn.ue.mu.Unlock()
	}

	ueConn.IdentityTypeUsedForRegistration = v
}

func (ueConn *UeConn) SetRetransmissionOfInitialNASMsg(v bool) {
	if ueConn.ue != nil {
		ueConn.ue.mu.Lock()
		defer ueConn.ue.mu.Unlock()
	}

	ueConn.RetransmissionOfInitialNASMsg = v
}

func (ueConn *UeConn) SetResyncTried(v bool) {
	if ueConn.ue != nil {
		ueConn.ue.mu.Lock()
		defer ueConn.ue.mu.Unlock()
	}

	ueConn.resyncTried = v
}

// ResyncTried reports whether an AUTS re-synchronisation has already been attempted
// this authentication exchange.
func (ueConn *UeConn) ResyncTried() bool {
	if ueConn.ue != nil {
		ueConn.ue.mu.Lock()
		defer ueConn.ue.mu.Unlock()
	}

	return ueConn.resyncTried
}

// SecureExchangeEstablished reports whether secure exchange of NAS messages has been
// established on the connection (TS 24.501 §4.4.4.3). Dispatch-goroutine-confined.
func (ueConn *UeConn) SecureExchangeEstablished() bool {
	if ueConn == nil {
		return false
	}

	return ueConn.secureExchangeEstablished
}

// MarkSecureExchangeEstablished records that secure exchange of NAS messages has been
// established on the connection (TS 24.501 §4.4.4.3).
func (ueConn *UeConn) MarkSecureExchangeEstablished() {
	if ueConn != nil {
		ueConn.secureExchangeEstablished = true
	}
}

// Radio returns the Radio this UeConn is associated with, or nil.
func (ueConn *UeConn) Radio() *Radio {
	if ueConn == nil {
		return nil
	}

	return ueConn.amf.radioFor(ueConn.conn)
}

// UeContext returns the currently attached UeContext, or nil.
func (ueConn *UeConn) UeContext() *UeContext {
	if ueConn == nil {
		return nil
	}

	return ueConn.ue
}

// TouchLastSeen propagates a last-seen timestamp to the associated UeContext.
// Safe to call on nil receivers or when UeContext/Radio is nil.
func (ueConn *UeConn) TouchLastSeen() {
	if ueConn == nil || ueConn.ue == nil {
		return
	}

	ueConn.ue.TouchLastSeen()
}

// sendTarget resolves the AMF and radio this RAN UE sends through.
func (ueConn *UeConn) sendTarget() (*AMF, NGAPWriter, error) {
	if ueConn == nil {
		return nil, nil, fmt.Errorf("ran ue is nil")
	}

	if ueConn.conn == nil {
		return nil, nil, fmt.Errorf("conn is nil")
	}

	if ueConn.amf == nil {
		return nil, nil, fmt.Errorf("amf is nil")
	}

	return ueConn.amf, ueConn.conn, nil
}

// StopReleaseGuard cancels the Release-Complete supervision timer.
func (ueConn *UeConn) StopReleaseGuard() {
	ueConn.releaseGuard.Stop()
}

// ReleaseUeConn performs the ReleaseAction-keyed teardown of a UE's RAN connection after
// a UE Context Release. It runs both on a UE Context Release Complete and, if that
// Complete is lost, from the release guard's timeout, so a lost Complete cannot leak the
// UeConn.
func (a *AMF) ReleaseUeConn(ctx context.Context, ueConn *UeConn) {
	amfUe := ueConn.UeContext()
	if amfUe == nil {
		if err := a.RemoveUeConn(ctx, ueConn); err != nil {
			logger.From(ctx, ueConn.Log).Error(err.Error())
		}

		return
	}

	if amfUe.State() == Registered {
		for _, sr := range amfUe.SmContextRefs() {
			if err := a.Session.DeactivateSmContext(ctx, sr.Ref); err != nil {
				logger.From(ctx, ueConn.Log).Warn("Send Update SmContextDeactivate UpCnxState Error", zap.Error(err), zap.Uint8("PduSessionID", sr.PduSessionID))
			}
		}

		a.StartMobileReachable(amfUe)
	}

	switch ueConn.ReleaseAction {
	case UeContextN2NormalRelease:
		if err := a.RemoveUeConn(ctx, ueConn); err != nil {
			logger.From(ctx, ueConn.Log).Error(err.Error())
		}
	case UeContextReleaseUeContext:
		if err := a.RemoveUeConn(ctx, ueConn); err != nil {
			logger.From(ctx, ueConn.Log).Error(err.Error())
		}

		// A UE without a valid security context (never fully registered) has its AMF UE
		// context deleted; a registered UE is kept.
		if !amfUe.Secured() {
			a.DeregisterAndRemoveUeContext(ctx, amfUe)
		}
	case UeContextReleaseDueToNwInitiatedDeregistraion:
		if err := a.RemoveUeConn(ctx, ueConn); err != nil {
			logger.From(ctx, ueConn.Log).Error(err.Error())
		}

		a.DeregisterAndRemoveUeContext(ctx, amfUe)
	case UeContextReleaseAbortRegistration:
		// A mid-registration UE is never "keep-context, go idle": delete unconditionally,
		// even if it is Secured (a post-SMC registration failure).
		if err := a.RemoveUeConn(ctx, ueConn); err != nil {
			logger.From(ctx, ueConn.Log).Error(err.Error())
		}

		a.DeregisterAndRemoveUeContext(ctx, amfUe)
	case UeContextReleaseHandover:
		a.ClearHandover(amfUe)

		if err := a.RemoveUeConn(ctx, ueConn); err != nil {
			logger.From(ctx, ueConn.Log).Error(err.Error())
		}
	default:
		logger.From(ctx, ueConn.Log).Error("Invalid Release Action", zap.Any("ReleaseAction", ueConn.ReleaseAction))
	}
}

// abortHandoverIfPreparedTarget ends an in-flight N2 handover when this UeConn is its
// prepared target and is being removed. Ending it on the source stops the supervision
// guard and clears the FSM at once, so no stale handover lingers until the guard
// deadline. The source is left in place; its own handover timers abort it on the radio.
func (ueConn *UeConn) abortHandoverIfPreparedTarget(ctx context.Context) {
	ue := ueConn.ue
	if ue == nil || ueConn.amf.HandoverTarget(ue) != ueConn {
		return
	}

	if conn := ue.Conn(); conn != nil {
		conn.Parent().EndKeyChainProc(procedure.N2Handover)
	}

	ueConn.amf.ClearHandover(ue)

	logger.WithTrace(ctx, ueConn.Log).Info("aborted in-flight N2 handover: target association removed")
}

// DropStaleUe removes any connection on radio still bound to ranUeNgapID before a new
// InitialUEMessage reuses that RAN-UE-NGAP-ID. A gNB may reuse the ID before its prior
// UEContextRelease completes, so a deferred UEContextReleaseComplete carrying the old
// AMF-UE-NGAP-ID must not remove the freshly created context. Stale conns are found under
// a.mu, then torn down after the lock — RemoveUeConn re-acquires a.mu and may call the SMF.
func (a *AMF) DropStaleUe(ctx context.Context, radio *Radio, ranUeNgapID int64) {
	a.mu.Lock()

	var stale []*UeConn

	for _, ueConn := range a.conns {
		if ueConn.conn == radio.Conn && ueConn.RanUeNgapID == ranUeNgapID {
			stale = append(stale, ueConn)
		}
	}

	a.mu.Unlock()

	for _, ueConn := range stale {
		logger.WithTrace(ctx, ueConn.Log).Debug("RAN UE NGAP ID reused in InitialUEMessage, removing stale UeConn",
			zap.Int64("RanUeNgapID", ueConn.RanUeNgapID),
			zap.Int64("AmfUeNgapID", ueConn.AmfUeNgapID))

		if err := a.RemoveUeConn(ctx, ueConn); err != nil {
			logger.WithTrace(ctx, ueConn.Log).Error(err.Error())
		}
	}
}

func (a *AMF) RemoveUeConn(ctx context.Context, ueConn *UeConn) error {
	if ueConn == nil {
		return fmt.Errorf("ran ue is nil")
	}

	ueConn.abortHandoverIfPreparedTarget(ctx)

	if ueConn.ue != nil {
		a.ReleaseNasConnection(ueConn.ue, ueConn)
	}

	a.mu.Lock()
	delete(a.conns, ueConn.AmfUeNgapID)
	a.mu.Unlock()

	a.connIDs.FreeID(ueConn.AmfUeNgapID)

	logger.AmfLog.Info("ran ue removed",
		zap.Int64("amfUeNgapID", ueConn.AmfUeNgapID),
		zap.Int64("ranUeNgapID", ueConn.RanUeNgapID),
	)

	return nil
}

// CommitPathSwitch re-points the UE at the target radio and commits the advanced
// {NH, NCC} chain atomically under the registry lock (TS 33.501 §6.9.2.1.1). It
// returns false if the UE was released during the user-plane switch, leaving the
// chain unadvanced so the source context stays consistent. The global conns
// index is keyed by the unchanged AMF UE NGAP ID, so the switch only re-points
// the UE at its new radio and RAN UE NGAP ID.
func (a *AMF) CommitPathSwitch(ue *UeContext, ueConn *UeConn, ran *Radio, ranUeNgapID int64, nh [32]uint8, ncc uint8) bool {
	a.mu.Lock()

	if ueConn == nil || ran == nil || a.conns[ueConn.AmfUeNgapID] != ueConn {
		a.mu.Unlock()

		return false
	}

	ueConn.conn = ran.Conn
	ueConn.radioName = ran.name
	ueConn.RanUeNgapID = ranUeNgapID

	ue.mu.Lock()
	ue.nh = nh
	ue.ncc = ncc
	ue.mu.Unlock()

	a.mu.Unlock()

	ueConn.Log = ran.Log.With(logger.AmfUeNgapID(ueConn.AmfUeNgapID))
	ueConn.Log.Info("ran ue switched to new Ran", zap.Int64("RanUeNgapID", ueConn.RanUeNgapID))

	return true
}

func (ueConn *UeConn) UpdateLocation(ctx context.Context, amf *AMF, userLocationInformation *ngapType.UserLocationInformation) {
	if userLocationInformation == nil {
		return
	}

	curTime := time.Now().UTC()

	switch userLocationInformation.Present {
	case ngapType.UserLocationInformationPresentUserLocationInformationEUTRA:
		locationInfoEUTRA := userLocationInformation.UserLocationInformationEUTRA

		if ueConn.Location.EutraLocation == nil {
			ueConn.Location.EutraLocation = new(models.EutraLocation)
		}

		tAI := locationInfoEUTRA.TAI
		plmnID := util.PlmnIDToModels(tAI.PLMNIdentity)
		tac := hex.EncodeToString(tAI.TAC.Value)

		if ueConn.Location.EutraLocation.Tai == nil {
			ueConn.Location.EutraLocation.Tai = new(models.Tai)
		}

		ueConn.Location.EutraLocation.Tai.PlmnID = &plmnID
		ueConn.Location.EutraLocation.Tai.Tac = tac
		ueConn.Tai = *ueConn.Location.EutraLocation.Tai

		eUTRACGI := locationInfoEUTRA.EUTRACGI
		ePlmnID := util.PlmnIDToModels(eUTRACGI.PLMNIdentity)
		eutraCellID := ngapConvert.BitStringToHex(&eUTRACGI.EUTRACellIdentity.Value)

		if ueConn.Location.EutraLocation.Ecgi == nil {
			ueConn.Location.EutraLocation.Ecgi = new(models.Ecgi)
		}

		ueConn.Location.EutraLocation.Ecgi.PlmnID = &ePlmnID
		ueConn.Location.EutraLocation.Ecgi.EutraCellID = eutraCellID

		ueConn.Location.EutraLocation.UeLocationTimestamp = &curTime
		if locationInfoEUTRA.TimeStamp != nil {
			ueConn.Location.EutraLocation.AgeOfLocationInformation = ngapConvert.TimeStampToInt32(
				locationInfoEUTRA.TimeStamp.Value)
		}

		if ueConn.ue != nil {
			ueConn.ue.Location = ueConn.Location
			ueConn.ue.Tai = *ueConn.ue.Location.EutraLocation.Tai
		}
	case ngapType.UserLocationInformationPresentUserLocationInformationNR:
		locationInfoNR := userLocationInformation.UserLocationInformationNR

		if ueConn.Location.NrLocation == nil {
			ueConn.Location.NrLocation = new(models.NrLocation)
		}

		tAI := locationInfoNR.TAI
		plmnID := util.PlmnIDToModels(tAI.PLMNIdentity)
		tac := hex.EncodeToString(tAI.TAC.Value)

		if ueConn.Location.NrLocation.Tai == nil {
			ueConn.Location.NrLocation.Tai = new(models.Tai)
		}

		ueConn.Location.NrLocation.Tai.PlmnID = &plmnID
		ueConn.Location.NrLocation.Tai.Tac = tac
		ueConn.Tai = *ueConn.Location.NrLocation.Tai

		nRCGI := locationInfoNR.NRCGI
		nRPlmnID := util.PlmnIDToModels(nRCGI.PLMNIdentity)
		nRCellID := ngapConvert.BitStringToHex(&nRCGI.NRCellIdentity.Value)

		if ueConn.Location.NrLocation.Ncgi == nil {
			ueConn.Location.NrLocation.Ncgi = new(models.Ncgi)
		}

		ueConn.Location.NrLocation.Ncgi.PlmnID = &nRPlmnID
		ueConn.Location.NrLocation.Ncgi.NrCellID = nRCellID

		ueConn.Location.NrLocation.UeLocationTimestamp = &curTime
		if locationInfoNR.TimeStamp != nil {
			ueConn.Location.NrLocation.AgeOfLocationInformation = ngapConvert.TimeStampToInt32(locationInfoNR.TimeStamp.Value)
		}

		if ueConn.ue != nil {
			ueConn.ue.Location = ueConn.Location
			ueConn.ue.Tai = *ueConn.ue.Location.NrLocation.Tai
		}
	case ngapType.UserLocationInformationPresentUserLocationInformationN3IWF:
		locationInfoN3IWF := userLocationInformation.UserLocationInformationN3IWF

		if ueConn.Location.N3gaLocation == nil {
			ueConn.Location.N3gaLocation = new(models.N3gaLocation)
		}

		ip := locationInfoN3IWF.IPAddress
		port := locationInfoN3IWF.PortNumber

		ipv4Addr, ipv6Addr := ngapConvert.IPAddressToString(ip)

		ueConn.Location.N3gaLocation.UeIpv4Addr = ipv4Addr
		ueConn.Location.N3gaLocation.UeIpv6Addr = ipv6Addr
		ueConn.Location.N3gaLocation.PortNumber = ngapConvert.PortNumberToInt(port)

		operatorInfo, err := amf.OperatorInfo(ctx)
		if err != nil {
			logger.AmfLog.Error("Error getting supported TAI list", zap.Error(err))
			return
		}

		tmp, err := strconv.ParseUint(operatorInfo.Tais[0].Tac, 16, 32)
		if err != nil {
			logger.AmfLog.Error("Error parsing TAC", zap.String("Tac", operatorInfo.Tais[0].Tac), zap.Error(err))
		}

		ueConn.Location.N3gaLocation.N3gppTai = &models.Tai{
			PlmnID: operatorInfo.Tais[0].PlmnID,
			Tac:    fmt.Sprintf("%06x", tmp),
		}

		ueConn.Tai = *ueConn.Location.N3gaLocation.N3gppTai

		if ueConn.ue != nil {
			ueConn.ue.Location = ueConn.Location
			ueConn.ue.Tai = *ueConn.Location.N3gaLocation.N3gppTai
		}
	case ngapType.UserLocationInformationPresentNothing:
	}
}

// NewUeConnForTest creates a UeConn and registers it in the AMF's conns index.
// It is intended for use in external test packages only. If the radio is not yet
// bound to an AMF, a throwaway one is created so a handler invoked with this same
// radio resolves the UE; tests that share a specific AMF must BindAMFForTest first.
func NewUeConnForTest(radio *Radio, ranUeNgapID, amfUeNgapID int64, log *zap.Logger) *UeConn {
	if radio.amf == nil {
		radio.amf = New(nil, nil, nil)
	}

	ueConn := &UeConn{
		RanUeNgapID: ranUeNgapID,
		AmfUeNgapID: amfUeNgapID,
		conn:        radio.Conn,
		radioName:   radio.name,
		amf:         radio.amf,
		Log:         log,
	}

	radio.amf.mu.Lock()

	radio.amf.conns[amfUeNgapID] = ueConn
	if radio.Conn != nil {
		radio.amf.radios[radio.Conn] = radio
	}
	radio.amf.mu.Unlock()

	return ueConn
}
