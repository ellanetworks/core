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
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type RelAction int

const (
	UeContextN2NormalRelease RelAction = iota
	UeContextReleaseHandover
	UeContextReleaseUeContext
	UeContextReleaseDueToNwInitiatedDeregistraion
	UeContextReleaseAbortRegistration
	// UeContextReleaseToEPS releases the RAN context of a UE that has moved to EPS. It
	// only ever removes the connection: the AMF UE context outlives it on purpose, so a
	// return to 5GS reuses the retained 5G security context (TS 23.501 §5.17.2.1).
	UeContextReleaseToEPS
)

// releaseGuardTimeout bounds the wait for a UE Context Release Complete after a UE
// Context Release Command is sent; on expiry the action-keyed local cleanup runs, so a
// lost Complete cannot leak the UeConn + AMF-UE-NGAP-ID. TS 38.413 §8.3 defines no CN-
// side supervision timer; this is a robustness guard.
const releaseGuardTimeout = 5 * time.Second

// UeConn represents one UE's radio-level state on a single Radio. Apart from locMu,
// which guards Tai and Location, it has no mutex of its own: it is protected either by
// the owning Radio's single SCTP goroutine, or by UeContext.Mutex when accessed via
// UeContext.Conn(). Callers of UeContext.Conn() must
// capture the returned pointer in a local and reuse it — the pointer may change between
// calls.
type UeConn struct {
	ranUeNgapID  atomic.Int64
	AmfUeNgapID  models.AmfUeNgapID
	HandOverType ngap.HandoverType
	locMu        sync.Mutex
	Tai          models.Tai
	Location     models.UserLocation
	// Written only under amf.mu but read all over the NGAP dispatch path without
	// it, so atomic — as UeContext.active is for the reverse pointer.
	ue atomic.Pointer[UeContext]
	// conn is the NGAP association this UE sends through and the key into the AMF's
	// radios index for node metadata (looked up via amf.radioFor(conn)).
	conn atomic.Pointer[NGAPWriter]
	// radio is the serving node's ID and name, for hot-path last-seen tagging without a
	// registry lookup. Atomic: written under amf.mu, read off it on the dispatch path.
	radio atomic.Pointer[radioRef]
	// amf is the owning registry; connection-lifecycle methods reach amf.mu through it.
	// Always set at creation.
	amf              *AMF
	ReleaseAction    RelAction
	UeContextRequest bool
	// ics is the Initial Context Setup progress (an ICSState). It is read and written
	// from the NGAP dispatch goroutine, the SMF N1N2 path, and the NAS-guard timer
	// callback, so it is atomic; mutate it only through ICS()/ClaimICS()/MarkICS*/ResetICS.
	ics        atomic.Int32
	n2Setups   n2SetupTxns
	n2Sessions n2Sessions
	n2Releases n2Releases
	inboundNAS atomic.Uint32

	deferredCause atomic.Pointer[ngap.Cause]
	deferGuard    guard.Guard
	logFields     atomic.Pointer[[]zap.Field]
	baseLogFields atomic.Pointer[[]zap.Field]
	supi          atomic.Pointer[string]
	// releasing gates a UE Context Release Command so a second one is not sent for the
	// same RAN UE. Guarded by AMF.mu, like the conns registry it lives in.
	releasing bool
	// releaseGuard supervises a sent UE Context Release Command; if the Release Complete
	// is lost it fires once (releaseGuardTimeout) and runs the action-keyed cleanup.
	releaseGuard guard.Guard
	icsGuard     guard.Guard

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

	cipheringStarted atomic.Bool

	AuthenticationCtx *ausf.AuthResult
	AuthNgKsi         models.NgKsi

	RegisteredBeforeRegistration bool
	// resyncTried records whether an SQN re-synchronisation (AUTS) has been attempted
	// this authentication exchange: the first synch failure resyncs, a second rejects
	// (TS 24.501 §5.4.1.3.7 f)/NOTE 4).
	resyncTried bool

	RegistrationRequest *fgs.RegistrationRequest
	// RegistrationRequestPlain is compared byte-for-byte for duplicate detection
	// (TS 24.501 §5.5.1.2.8), immune to the decoder dropping IEs it does not model.
	RegistrationRequestPlain []byte
	// Plain arrival means the UE held no security context, so it must repeat the
	// request in the SECURITY MODE COMPLETE (TS 24.501 §4.4.6 case a).
	RegistrationRequestReplayRequired bool
	RegistrationType5GS               fgs.RegistrationType
	IdentityTypeUsedForRegistration   uint8
	RetransmissionOfInitialNASMsg     bool

	EPSArrival *EPSArrival

	RegistrationAcceptPlain []byte
}

type EPSArrival struct {
	Sessions *interworking.ArrivingSessions

	NeedsSecurityModeControl bool
}

func (ueConn *UeConn) ArrivedFromEPS() bool {
	return ueConn != nil && ueConn.EPSArrival != nil
}

func (ueConn *UeConn) ArrivalNeedsSecurityModeControl() bool {
	return ueConn.ArrivedFromEPS() && ueConn.EPSArrival.NeedsSecurityModeControl
}

func (a *EPSArrival) ArrivingSessions() *interworking.ArrivingSessions {
	if a == nil {
		return nil
	}

	return a.Sessions
}

type radioRef struct {
	id   string
	name string
}

func (ueConn *UeConn) setRadio(id, name string) {
	ueConn.radio.Store(&radioRef{id: id, name: name})
}

func (ueConn *UeConn) radioIDName() (string, string) {
	if r := ueConn.radio.Load(); r != nil {
		return r.id, r.name
	}

	return "", ""
}

func (ueConn *UeConn) radioName() string {
	_, name := ueConn.radioIDName()

	return name
}

func (ueConn *UeConn) LogFields() []zap.Field {
	if ueConn == nil {
		return nil
	}

	if f := ueConn.logFields.Load(); f != nil {
		return *f
	}

	return nil
}

func (ueConn *UeConn) Log(ctx context.Context) *zap.Logger {
	return logger.From(ctx, logger.AmfLog, ueConn.LogFields()...)
}

func (ueConn *UeConn) setLogFields(fields []zap.Field) {
	ueConn.logFields.Store(&fields)
}

func (ueConn *UeConn) bindLogFields(base []zap.Field) {
	ueConn.baseLogFields.Store(&base)
	ueConn.refreshLog()
}

func (ueConn *UeConn) bindSupi(supi etsi.SUPI) {
	if ueConn == nil || !supi.IsValid() {
		return
	}

	s := supi.String()
	ueConn.supi.Store(&s)
	ueConn.refreshLog()
}

func (ueConn *UeConn) refreshLog() {
	base := ueConn.baseLogFields.Load()
	if base == nil {
		return
	}

	fields := make([]zap.Field, 0, len(*base)+3)
	fields = append(fields, *base...)

	if supi := ueConn.supi.Load(); supi != nil {
		fields = append(fields, logger.SUPI(*supi))
	}

	fields = append(fields, logger.AmfUeNgapID(ueConn.AmfUeNgapID))

	if ranUeNgapID := ueConn.RanUeNgapID(); ranUeNgapID != models.RanUeNgapIDUnspecified {
		fields = append(fields, logger.RanUeNgapID(ranUeNgapID))
	}

	ueConn.setLogFields(fields)
}

// Parent returns the UeContext this connection is bound to, or nil when bare.
func (ueConn *UeConn) Parent() *UeContext {
	return ueConn.ue.Load()
}

func (ueConn *UeConn) RanUeNgapID() models.RanUeNgapID {
	return models.RanUeNgapID(ueConn.ranUeNgapID.Load())
}

func (ueConn *UeConn) setRanUeNgapID(ranUeNgapID models.RanUeNgapID) {
	ueConn.ranUeNgapID.Store(int64(ranUeNgapID))
}

// Release stops the NAS guard and clears this connection from its UeContext. Clearing
// ue.active is done under the registry lock (amf.mu), like bind, so it cannot race an
// AttachUeConn. A key-changing procedure still in flight is left to its supervision
// deadline (TS 38.413 handover guard), which runs its cleanup.
func (ueConn *UeConn) Release(ctx context.Context) {
	ueConn.cancelDeferredRelease()
	ueConn.stopTimers(ctx)

	if ue := ueConn.ue.Load(); ue != nil && ue.PagingState() == PagingDelivering {
		ue.PagingFailed(ctx, models.N1N2UENotResponding)
	}

	a := ueConn.amf
	if a == nil {
		return
	}

	a.mu.Lock()
	if ue := ueConn.ue.Load(); ue != nil {
		ue.active.CompareAndSwap(ueConn, nil)
	}
	a.mu.Unlock()
}

// stopTimers stops the connection's NAS and Initial Context Setup guards.
func (ueConn *UeConn) stopTimers(ctx context.Context) {
	ueConn.StopNASGuard(ctx)
	ueConn.icsGuard.Stop()
}

// armNASGuardWith arms the connection's NAS common-procedure guard (a no-op when cfg is
// disabled). The procedures are mutually exclusive, so arming supersedes any prior one.
func (ueConn *UeConn) armNASGuardWith(ctx context.Context, cfg guard.TimerValue, name string, onRetransmit func(context.Context, int32), onAbort func(context.Context)) {
	if !cfg.Enable {
		return
	}

	link := trace.SpanContextFromContext(ctx)

	ueConn.nasGuardName.Store(&name)
	ueConn.nasGuard.Arm(cfg.ExpireTime, cfg.MaxRetryTimes,
		func(attempt int32) {
			guardCtx, span := guardSpan(link, "amf/nas_guard_retransmit", name, attempt)
			defer span.End()

			onRetransmit(guardCtx, attempt)
		},
		func() {
			guardCtx, span := guardSpan(link, "amf/nas_guard_expire", name, 0)
			defer span.End()

			onAbort(guardCtx)
		},
	)
}

func guardSpan(link trace.SpanContext, spanName string, timer string, attempt int32) (context.Context, trace.Span) {
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attribute.String("nas.guard.timer", timer)),
	}

	if attempt > 0 {
		opts = append(opts, trace.WithAttributes(attribute.Int("nas.guard.attempt", int(attempt))))
	}

	if link.IsValid() {
		opts = append(opts, trace.WithLinks(trace.Link{SpanContext: link}))
	}

	return tracer.Start(context.Background(), spanName, opts...)
}

func (ueConn *UeConn) StopNASGuard(ctx context.Context) {
	ueConn.nasGuardName.Store(nil)
	ueConn.nasGuard.Stop()
	ueConn.ResumeDeferredReleaseIfSettled(ctx)
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

	amf.reg.Track(conn, r)
}

// CountUeConnsForTest reports the number of UE-associated NGAP connections.
func (amf *AMF) CountUeConnsForTest() int {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	return len(amf.conns)
}

// ClearRadiosForTest empties the radio index, for tests that assert on no radios.
func (amf *AMF) ClearRadiosForTest() {
	amf.mu.Lock()
	defer amf.mu.Unlock()

	clear(amf.reg.ByConn)
}

// RadioForTest returns the radio registered for conn, for tests that assert on the
// radio index membership.
func (amf *AMF) RadioForTest(conn NGAPWriter) (*Radio, bool) {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	return amf.reg.Radio(conn)
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
	ueConn.icsGuard.Stop()
}

func (ueConn *UeConn) NoteInboundNAS() {
	ueConn.inboundNAS.Add(1)
}

func (ueConn *UeConn) SentFrom5GMMIdle() bool {
	return ueConn.inboundNAS.Load() <= 1
}

// ResetICS returns the connection to ICSNotStarted, rolling back a claim whose send
// failed so a retry re-attempts the context setup.
func (ueConn *UeConn) ResetICS() {
	ueConn.ics.Store(int32(ICSNotStarted))
	ueConn.icsGuard.Stop()
}

func (ueConn *UeConn) superviseICS(ctx context.Context) {
	a := ueConn.amf
	if a == nil || !a.ICSGuardCfg.Enable {
		return
	}

	link := trace.SpanContextFromContext(ctx)

	ueConn.icsGuard.ArmOnce(a.ICSGuardCfg.ExpireTime, func() {
		ue := ueConn.UeContext()
		if ue == nil || ue.Conn() != ueConn || ueConn.ICS() != ICSPending {
			return
		}

		guardCtx, span := guardSpan(link, "amf/ics_guard_expire", "Initial Context Setup", 0)
		defer span.End()

		ueConn.Log(guardCtx).Warn("no answer to the Initial Context Setup Request; releasing the NG connection")

		if ue.State() == RegistrationInitiated {
			a.abortCommonProcedure(guardCtx, ue)
			return
		}

		a.ReleaseOnRANRequest(guardCtx, ueConn, ngap.Cause{Group: ngap.CauseGroupNAS, Value: ngap.CauseNASUnspecified}, nil)
	})
}

func (ueConn *UeConn) AbortICS(ctx context.Context) {
	ueConn.ResetICS()
	ueConn.EndN2Setup(ctx, N2SetupInitialContext)
}

// The registration/auth status fields (RegistrationType5GS, IdentityTypeUsedForRegistration,
// RetransmissionOfInitialNASMsg, resyncTried) are written on the NAS goroutine and read by
// the status export from another goroutine. The setters below publish them under the parent
// UeContext lock — the lock the export holds — so the export never observes a torn write. The
// same-goroutine reads (registration/auth logic) stay plain: they race neither the export nor
// each other.

func (ueConn *UeConn) SetRegistrationType5GS(v uint8) {
	if ue := ueConn.ue.Load(); ue != nil {
		ue.mu.Lock()
		defer ue.mu.Unlock()
	}

	ueConn.RegistrationType5GS = fgs.RegistrationType(v)
}

func (ueConn *UeConn) SetIdentityTypeUsedForRegistration(v uint8) {
	if ue := ueConn.ue.Load(); ue != nil {
		ue.mu.Lock()
		defer ue.mu.Unlock()
	}

	ueConn.IdentityTypeUsedForRegistration = v
}

func (ueConn *UeConn) SetRetransmissionOfInitialNASMsg(v bool) {
	if ue := ueConn.ue.Load(); ue != nil {
		ue.mu.Lock()
		defer ue.mu.Unlock()
	}

	ueConn.RetransmissionOfInitialNASMsg = v
}

func (ueConn *UeConn) SetResyncTried(v bool) {
	if ue := ueConn.ue.Load(); ue != nil {
		ue.mu.Lock()
		defer ue.mu.Unlock()
	}

	ueConn.resyncTried = v
}

// ResyncTried reports whether an AUTS re-synchronisation has already been attempted
// this authentication exchange.
func (ueConn *UeConn) ResyncTried() bool {
	if ue := ueConn.ue.Load(); ue != nil {
		ue.mu.Lock()
		defer ue.mu.Unlock()
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

func (ueConn *UeConn) CipheringStarted() bool {
	if ueConn == nil {
		return false
	}

	return ueConn.cipheringStarted.Load()
}

func (ueConn *UeConn) MarkCipheringStarted() {
	if ueConn != nil {
		ueConn.cipheringStarted.Store(true)
	}
}

// Radio returns the Radio this UeConn is associated with, or nil.
func (ueConn *UeConn) Radio() *Radio {
	if ueConn == nil {
		return nil
	}

	return ueConn.amf.radioFor(ueConn.Conn())
}

// UeContext returns the currently attached UeContext, or nil.
func (ueConn *UeConn) UeContext() *UeContext {
	if ueConn == nil {
		return nil
	}

	return ueConn.ue.Load()
}

// TouchLastSeen propagates a last-seen timestamp to the associated UeContext.
// Safe to call on nil receivers or when UeContext/Radio is nil.
func (ueConn *UeConn) TouchLastSeen() {
	if ueConn == nil {
		return
	}

	ue := ueConn.ue.Load()
	if ue == nil {
		return
	}

	ue.TouchLastSeen()
}

// sendTarget resolves the AMF and radio this RAN UE sends through.
func (ueConn *UeConn) sendTarget() (*AMF, NGAPWriter, error) {
	if ueConn == nil {
		return nil, nil, fmt.Errorf("ran ue is nil")
	}

	conn := ueConn.Conn()
	if conn == nil {
		return nil, nil, fmt.Errorf("conn is nil")
	}

	if ueConn.amf == nil {
		return nil, nil, fmt.Errorf("amf is nil")
	}

	return ueConn.amf, conn, nil
}

func (ueConn *UeConn) Conn() NGAPWriter {
	if ueConn == nil {
		return nil
	}

	w := ueConn.conn.Load()
	if w == nil {
		return nil
	}

	return *w
}

func (ueConn *UeConn) setConn(w NGAPWriter) {
	ueConn.conn.Store(&w)
}

// StopReleaseGuard cancels the Release-Complete supervision timer.
func (ueConn *UeConn) StopReleaseGuard() {
	ueConn.releaseGuard.Stop()
}

func (a *AMF) ReleaseOnRANRequest(ctx context.Context, ueConn *UeConn, cause ngap.Cause, reported []uint8) {
	amfUe := ueConn.UeContext()

	if amfUe != nil && amfUe.State() != Registered {
		ueConn.Log(ctx).Info("Ue Context in Non GMM-Registered")

		ueConn.ReleaseAction = UeContextReleaseUeContext

		ueConn.SendUEContextReleaseCommand(ctx, cause)

		for _, sr := range amfUe.SmContextRefs() {
			if err := a.Session.ReleaseSmContext(ctx, sr.Ref); err != nil {
				ueConn.Log(ctx).Error("error sending release sm context request", zap.Error(err), logger.PDUSessionID(sr.PduSessionID))
			}
		}

		return
	}

	if amfUe != nil {
		ueConn.Log(ctx).Debug("Ue Context in GMM-Registered")

		a.deactivateReleasedSessions(ctx, ueConn, amfUe, reported)
	}

	ueConn.ReleaseAction = UeContextN2NormalRelease

	ueConn.SendUEContextReleaseCommand(ctx, cause)
}

func (a *AMF) deactivateReleasedSessions(ctx context.Context, ueConn *UeConn, amfUe *UeContext, reported []uint8) {
	if reported == nil {
		ueConn.Log(ctx).Info("Pdu Session IDs not received from gNB, Releasing the UE Context with SMF using local context")

		for _, sr := range amfUe.SmContextRefs() {
			if ueConn.N2SessionInactive(sr.PduSessionID) {
				ueConn.Log(ctx).Info("Pdu Session is inactive so not sending deactivate to SMF", logger.PDUSessionID(sr.PduSessionID))

				continue
			}

			if err := a.Session.DeactivateSmContext(ctx, sr.Ref); err != nil {
				ueConn.Log(ctx).Warn("Send Update SmContextDeactivate UpCnxState Error", zap.Error(err), logger.PDUSessionID(sr.PduSessionID))
			}
		}

		return
	}

	for _, pduSessionID := range reported {
		smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
		if !ok {
			ueConn.Log(ctx).Warn("no SM context for a PDU session the NG-RAN node reported as established",
				logger.PDUSessionID(pduSessionID))

			continue
		}

		if err := a.Session.DeactivateSmContext(ctx, smContext.Ref); err != nil {
			ueConn.Log(ctx).Error("Send Update SmContextDeactivate UpCnxState Error", zap.Error(err), logger.PDUSessionID(pduSessionID))
		}
	}
}

func (a *AMF) ReleaseUeConn(ctx context.Context, ueConn *UeConn) {
	a.ReleaseUeConnServedBy(ctx, ueConn, nil)
}

func (a *AMF) ReleaseUeConnServedBy(ctx context.Context, ueConn *UeConn, served []uint8) (wentIdle bool) {
	amfUe := ueConn.UeContext()
	if amfUe == nil {
		if err := a.RemoveUeConn(ctx, ueConn); err != nil {
			ueConn.Log(ctx).Error("failed to remove RAN UE connection", zap.Error(err))
		}

		return false
	}

	registered := amfUe.State() == Registered
	if registered {
		for _, sr := range amfUe.SmContextRefs() {
			if len(served) > 0 && !slices.Contains(served, sr.PduSessionID) && ueConn.N2SessionInactive(sr.PduSessionID) {
				continue
			}

			if err := a.Session.DeactivateSmContext(ctx, sr.Ref); err != nil {
				ueConn.Log(ctx).Warn("Send Update SmContextDeactivate UpCnxState Error", zap.Error(err), logger.PDUSessionID(sr.PduSessionID))
			}
		}

		a.StartMobileReachable(amfUe)
	}

	switch ueConn.ReleaseAction {
	case UeContextN2NormalRelease:
		if err := a.RemoveUeConn(ctx, ueConn); err != nil {
			ueConn.Log(ctx).Error("failed to remove RAN UE connection", zap.Error(err))
		}

		return registered
	case UeContextReleaseUeContext:
		if err := a.RemoveUeConn(ctx, ueConn); err != nil {
			ueConn.Log(ctx).Error("failed to remove RAN UE connection", zap.Error(err))
		}

		// A UE without a valid security context (never fully registered) has its AMF UE
		// context deleted; a registered UE is kept.
		if !amfUe.Secured() {
			a.DeregisterAndRemoveUeContext(ctx, amfUe)

			return false
		}

		return registered
	case UeContextReleaseDueToNwInitiatedDeregistraion:
		if err := a.RemoveUeConn(ctx, ueConn); err != nil {
			ueConn.Log(ctx).Error("failed to remove RAN UE connection", zap.Error(err))
		}

		a.DeregisterAndRemoveUeContext(ctx, amfUe)
	case UeContextReleaseAbortRegistration:
		// A mid-registration UE is never "keep-context, go idle": delete unconditionally,
		// even if it is Secured (a post-SMC registration failure).
		if err := a.RemoveUeConn(ctx, ueConn); err != nil {
			ueConn.Log(ctx).Error("failed to remove RAN UE connection", zap.Error(err))
		}

		a.DeregisterAndRemoveUeContext(ctx, amfUe)
	case UeContextReleaseHandover:
		a.ClearHandover(amfUe)

		if err := a.RemoveUeConn(ctx, ueConn); err != nil {
			ueConn.Log(ctx).Error("failed to remove RAN UE connection", zap.Error(err))
		}
	case UeContextReleaseToEPS:
		if err := a.RemoveUeConn(ctx, ueConn); err != nil {
			ueConn.Log(ctx).Error("failed to remove RAN UE connection", zap.Error(err))
		}
	default:
		ueConn.Log(ctx).Error("Invalid Release Action", zap.Any("release_action", ueConn.ReleaseAction))
	}

	return false
}

// abortHandoverOnRemoval ends an in-flight N2 handover when this UeConn — being removed —
// is its prepared target or its source. On source removal the prepared target is released
// explicitly (radio-connection-with-UE-lost): the guard that would reap it is stopped when
// RemoveUeConn ends the key-chain procedure. On target removal the source is left in place,
// aborted on the radio by its own TNGRELOCprep/Overall timers (TS 38.413).
func (ueConn *UeConn) abortHandoverOnRemoval(ctx context.Context) {
	ue := ueConn.ue.Load()
	if ue == nil {
		return
	}

	a := ueConn.amf
	source := a.HandoverSource(ue)
	target := a.HandoverTarget(ue)

	fromEPS := a.HandoverFromEPS(ue)

	switch ueConn {
	case target:
		a.ClearHandover(ue)
		a.UnbindHandoverTarget(ctx, ue)

		if fromEPS {
			a.dropRelocationFromEPS(ctx, ue)
		}

		ueConn.Log(ctx).Info("aborted in-flight N2 handover: target association removed")
	case source:
		a.ClearHandover(ue)
		a.UnbindHandoverTarget(ctx, ue)

		if target != nil {
			target.ReleaseAction = UeContextReleaseHandover

			target.SendUEContextReleaseCommand(ctx,
				ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkRadioConnectionWithUELost})
		}

		ueConn.Log(ctx).Info("released prepared N2 handover target: source association removed")
	}
}

// DropStaleUe removes any connection on radio still bound to ranUeNgapID, clearing the way
// for a new InitialUEMessage to reuse that RAN-UE-NGAP-ID. A gNB may reuse the ID before its
// prior UEContextRelease completes; dropping the stale conn keeps a deferred
// UEContextReleaseComplete from removing the freshly created context. Torn down after
// releasing a.mu, since RemoveUeConn re-acquires it and may call the SMF.
func (a *AMF) DropStaleUe(ctx context.Context, radio *Radio, ranUeNgapID models.RanUeNgapID) {
	a.mu.Lock()

	var stale []*UeConn

	for _, ueConn := range a.conns {
		if ueConn.Conn() == radio.Conn && ueConn.RanUeNgapID() == ranUeNgapID {
			stale = append(stale, ueConn)
		}
	}

	a.mu.Unlock()

	for _, ueConn := range stale {
		ueConn.Log(ctx).Debug("RAN UE NGAP ID reused in InitialUEMessage, removing stale UeConn")

		if err := a.RemoveUeConn(ctx, ueConn); err != nil {
			ueConn.Log(ctx).Error("failed to remove RAN UE connection", zap.Error(err))
		}
	}
}

func (a *AMF) RemoveUeConn(ctx context.Context, ueConn *UeConn) error {
	if ueConn == nil {
		return fmt.Errorf("ran ue is nil")
	}

	ueConn.abortHandoverOnRemoval(ctx)

	if ue := ueConn.ue.Load(); ue != nil {
		a.ReleaseNasConnection(ctx, ue, ueConn)
	}

	a.mu.Lock()
	delete(a.conns, int64(ueConn.AmfUeNgapID))
	a.mu.Unlock()

	a.connIDs.FreeID(int64(ueConn.AmfUeNgapID))

	ueConn.Log(ctx).Debug("ran ue removed")

	return nil
}

// CommitPathSwitch re-points the UE at the target radio and commits the advanced
// {NH, NCC} chain atomically under the registry lock (TS 33.501 §6.9.2.1.1). It
// returns false if the UE was released during the user-plane switch, leaving the
// chain unadvanced so the source context stays consistent. The global conns
// index is keyed by the unchanged AMF UE NGAP ID, so the switch only re-points
// the UE at its new radio and RAN UE NGAP ID.
func (a *AMF) CommitPathSwitch(ctx context.Context, ue *UeContext, ueConn *UeConn, ran *Radio, ranUeNgapID models.RanUeNgapID, nh [32]uint8, ncc uint8) bool {
	a.mu.Lock()

	if ueConn == nil || ran == nil || a.conns[int64(ueConn.AmfUeNgapID)] != ueConn {
		a.mu.Unlock()

		return false
	}

	ueConn.setConn(ran.Conn)
	ueConn.setRadio(radioIDOf(ran), ran.name)
	ueConn.setRanUeNgapID(ranUeNgapID)

	if supi := ue.Supi(); supi.IsIMSI() {
		a.lastSeen.refresh(supi.IMSI(), radioIDOf(ran), ran.name, ue.lastSeenTime())
	}

	ue.mu.Lock()
	ue.nh = nh
	ue.ncc = ncc
	ue.mu.Unlock()

	ueConn.bindLogFields(ran.LogFields())

	a.mu.Unlock()

	ueConn.Log(ctx).Info("ran ue switched to new Ran")

	return true
}

// NewUeConnForTest creates a UeConn and registers it in the AMF's conns index.
// It is intended for use in external test packages only. If the radio is not yet
// bound to an AMF, a throwaway one is created so a handler invoked with this same
// radio resolves the UE; tests that share a specific AMF must BindAMFForTest first.
func NewUeConnForTest(radio *Radio, ranUeNgapID models.RanUeNgapID, amfUeNgapID models.AmfUeNgapID) *UeConn {
	if radio.amf == nil {
		radio.amf = New(nil, nil, nil)
	}

	ueConn := &UeConn{
		AmfUeNgapID: amfUeNgapID,
		amf:         radio.amf,
	}
	ueConn.setConn(radio.Conn)
	ueConn.setRanUeNgapID(ranUeNgapID)

	radio.amf.mu.Lock()

	ueConn.setRadio(radioIDOf(radio), radio.name)

	radio.amf.conns[int64(amfUeNgapID)] = ueConn
	if radio.Conn != nil {
		radio.amf.reg.Track(radio.Conn, radio)
	}
	radio.amf.mu.Unlock()

	return ueConn
}
