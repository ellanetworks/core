// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package mme implements Ella Core's 4G Mobility Management Entity control
// plane (the S1-MME interface), built on the github.com/ellanetworks/core/s1ap
// codec. It handles eNB S1 Setup, the EPS NAS procedures (attach,
// authentication, security mode, identity, tracking area update, service
// request, detach), UE contexts, and default-bearer activation via the
// SMF/PGW-C anchor.
package mme

import (
	"context"
	"sync"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/internal/util/idgenerator"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.opentelemetry.io/otel"
)

// DefaultS1MMEPort is the standard S1-MME SCTP port (TS 36.412).
const DefaultS1MMEPort = 36412

// maxMMEUES1APID is the largest MME-UE-S1AP-ID, INTEGER (0..2^32-1) (TS 36.413).
const maxMMEUES1APID int64 = 4294967295

// NASHandler is the EMM/ESM NAS layer's entry surface, implemented in
// internal/mme/nas and injected so the S1AP layer dispatches uplink NAS without
// the kernel importing its layers (kernel ⊅ nas).
type NASHandler interface {
	HandleNAS(ctx context.Context, conn *UeConn, nas []byte)
	HandleServiceRequest(ctx context.Context, conn S1APWriter, msg *s1ap.InitialUEMessage)
}

// epsSessionManager is the converged session anchor (SMF acting as PGW-C) the
// MME delegates EPS default-bearer establishment to: it allocates the UE IP and
// owns the session. *smf.SMF satisfies it. Defined here (consumer side) so there
// is no mme → smf import.
type epsSessionManager interface {
	// CreateEPSSession negotiates the PDN type, allocates the UE address(es), and
	// programs the default bearer, returning the negotiated type, the addresses,
	// and the S-GW S1-U F-TEID for the eNB to send uplink to.
	CreateEPSSession(ctx context.Context, req models.EPSBearerRequest) (models.EPSBearer, error)
	// ModifyEPSSession sets the downlink endpoint to the eNB S1-U F-TEID. ebi
	// identifies the PDN connection's default bearer.
	ModifyEPSSession(ctx context.Context, imsi string, ebi uint8, enb models.FTEID) error
	// UpdateEPSSessionAMBR updates the Session-AMBR enforced by the UPF QER for a
	// PDN connection's default bearer, in the "<n> <unit>" form.
	UpdateEPSSessionAMBR(ctx context.Context, imsi string, ebi uint8, ambrUplink, ambrDownlink string) error
	// DeactivateEPSSession buffers the downlink bearer when the UE goes ECM-IDLE
	// so downlink data triggers paging.
	DeactivateEPSSession(ctx context.Context, imsi string, ebi uint8) error
	// ReleaseEPSSession releases the anchor session identified by its unique ref
	// (as returned in models.EPSBearer.Ref and stored on the PDN connection), so a
	// superseded context releases its own session and never a newer one that reused
	// the same (IMSI, EBI).
	ReleaseEPSSession(ctx context.Context, ref string) error
}

// credentialProvider is the UDM surface the MME requires for EPS authentication:
// generating an EPS-AKA authentication vector for a subscriber (TS 33.401). *udm.Service
// satisfies it. Held as an interface so the dependency is decoupled and mockable.
type credentialProvider interface {
	GenerateEPSVector(ctx context.Context, imsi string, plmnID []byte, resyncAuts, resyncRand string) (*udm.EPSAV, error)
}

// Concurrency model. A UE's state is touched by several goroutines: the eNB
// dispatch loop (serial per SCTP association), the data-network reconcile
// backstop, the status and detach API, and timer callbacks. Two locks, with a
// fixed ordering, plus two atomics:
//
//   - MME.mu guards the registry and lifecycle: the UEs/uesByTmsi/radios maps,
//     the MME-UE-S1AP-ID allocator, the M-TMSI allocator, each UE's S1-connection
//     fields (conn, MME/ENB-UE-S1AP-IDs, the releasing flag), and the
//     idle/paging/NAS-guard timers and their generation counters. The UE's
//     S1-connection *pointer* itself (ue.active) is swapped under MME.mu on bind/release
//     but is an atomic.Pointer so the hot path reads it lock-free via Conn().
//   - UeContext.mu guards that UE's data: the EMM registration state (emmState),
//     the EPS NAS security context (keys, NAS COUNTs, and the NH/NCC key chain),
//     the PDN/bearer state (the pdns map, defaultEBI, and each connection's
//     in-flight modification flags), and imsi. The security context is reached only
//     through chokepoint methods (installNASSecurityContext, protectDownlink,
//     tryUnprotectUplink, deriveInitialKeNB, markSecured, Snapshot) so the keys
//     never leave the kernel and the COUNT invariant is auditable in one place. The
//     ECM state is derived from whether the UE holds an S1-connection (ue.active).
//
// Shared invariant: a UE's registration state and security
// key material — the keys, NAS COUNTs, and the NH/NCC key chain — are read and
// written only under UeContext.mu, never under the registry lock.
//
// Lock ordering (acquire in this order, never reverse):
//
//	MME.mu  →  UeContext.mu
//
// Never hold a lock across an external call (SMF, DB, SCTP send): snapshot the
// state, release, then send. A reader that observes emmState == EMM-REGISTERED
// under UeContext.mu (status, reconcile) may then read the UE's other registered
// data — the mutex is the publication barrier that carries the happens-before from
// the TransitionTo at registration.
type MME struct {
	Cred    credentialProvider
	Bearer  bearerStore
	Session epsSessionManager
	NAS     NASHandler

	// EPSNetworkFeatureSupport is advertised in Attach/TAU Accept (TS 24.301
	// §9.9.3.12A); nil falls back to the default.
	EPSNetworkFeatureSupport *eps.EPSNetworkFeatureSupport

	mu         sync.RWMutex
	radios     map[S1APWriter]*Radio
	radiosByID map[string]*Radio        // S1-setup-complete eNBs keyed by Global eNB ID, for S1-handover target resolution
	conns      map[uint32]*UeConn       // UE-associated S1-connections keyed by MME-UE-S1AP-ID; conn.ue is nil until a UE context is bound
	UEs        map[etsi.SUPI]*UeContext // persistent UE contexts keyed by SUPI; survives the connection across ECM-IDLE
	uesByTmsi  map[etsi.TMSI]*UeContext // keyed by M-TMSI, for S-TMSI lookup
	connIDs    *idgenerator.IDGenerator // recycling MME-UE-S1AP-ID allocator (TS 36.413 no-immediate-reuse)
	// tmsi allocates an unpredictable M-TMSI (TS 23.401 privacy): random MSBs
	// with allocate/free.
	tmsi *etsi.TmsiAllocator

	// Supervision timers are fields, not constants, so tests can shorten them.
	mobileReachableTime time.Duration // idle-mode reachability (TS 24.301)
	implicitDetachTime  time.Duration

	// Retransmitting supervision guards, held as guard.TimerValue.
	nasGuardCfg guard.TimerValue // NAS common-procedure guard (TS 24.301: T3450/T3460/T3470)
	esmGuardCfg guard.TimerValue // ESM bearer-procedure guard (TS 24.301: T3486/T3495), 4G-only (no AMF peer)
	pagingCfg   guard.TimerValue // paging supervision (T3413, TS 24.301 §5.6.2)

	// handoverGuardTimeout bounds the whole S1 handover (HANDOVER REQUIRED → NOTIFY)
	// so a silent target does not pin the UE's handover slot.
	handoverGuardTimeout time.Duration
}

// T3412PeriodicTAU is the periodic tracking-area-update timer the MME advertises
// to UEs (TS 24.301). It is the single source for both the value encoded into the
// Attach Accept and the mobile reachable timer below, so the two cannot drift if
// it ever becomes configurable.
const T3412PeriodicTAU = 54 * time.Minute

// T3402Backoff is the value advertised in the T3402 IE of an ATTACH REJECT — the
// back-off before the UE retries the attach. Both specs default it to 12 min
// (TS 24.301 §10.2 “T3402 Default 12” / TS 24.501 §10.2 “T3502 Default 12”).
const T3402Backoff = 12 * time.Minute

// defaultMobileReachableTime supervises the UE's periodic tracking area updating
// (TS 24.301 §5.3.5). defaultImplicitDetachTime is the grace period after the mobile
// reachable timer before the MME implicitly detaches an unreachable UE
// (network-dependent).
const (
	// mobileReachableMargin is added to the periodic timer (T3412) to form the mobile
	// reachable timer: TS 24.301 §5.3.5 — "4 minutes greater than T3412".
	mobileReachableMargin = 4 * time.Minute

	defaultMobileReachableTime = T3412PeriodicTAU + mobileReachableMargin
	defaultImplicitDetachTime  = 2 * time.Minute
)

// The NAS common-procedure guard timers T3450/T3460/T3470 default to 6 s with up
// to 4 retransmissions (TS 24.301). S1AP runs over reliable SCTP, so the
// retransmissions rarely fire; the guard bounds a procedure the UE stops
// answering and releases the UE.
const (
	defaultNASGuardTimeout       = 6 * time.Second
	defaultNASGuardMaxRetransmit = 4
)

// defaultESMGuardTimeout is the retransmission interval for the ESM bearer
// procedures (T3486 modify, T3495 deactivate, TS 24.301 §10.2.1): 8 s, longer
// than the 6 s common-procedure guard.
const defaultESMGuardTimeout = 8 * time.Second

// The paging supervision timer T3413 bounds how long the MME waits for a paged
// UE to respond before retransmitting and, after a bounded number of attempts,
// giving up (TS 24.301 §5.6.2; the value is network-dependent).
const (
	defaultPagingTimeout       = 6 * time.Second
	defaultPagingMaxRetransmit = 4
)

// defaultHandoverGuardTimeout bounds an S1 handover from HANDOVER REQUIRED to
// NOTIFY. It is generous relative to the source eNB's TS1RELOCprep/TS1RELOCOverall
// (a few seconds) so a normal handover always completes first; it only fires when
// the target eNB never answers (TS 36.413 §8.4).
const defaultHandoverGuardTimeout = 10 * time.Second

// New returns an MME network function. cred is the shared credential authority
// (HSS+UDM/ARPF) for EPS-AKA vectors; bearer is the subscription-data store used
// to resolve a subscriber's default-bearer QoS; session is the SMF+PGW-C anchor
// that allocates the UE IP. The MME never holds subscriber keys or the SQN.
func New(cred credentialProvider, bearer bearerStore, session epsSessionManager) *MME {
	return &MME{
		Cred:                     cred,
		Bearer:                   bearer,
		Session:                  session,
		EPSNetworkFeatureSupport: &eps.EPSNetworkFeatureSupport{IMSVoPS: true},
		radios:                   make(map[S1APWriter]*Radio),
		radiosByID:               make(map[string]*Radio),
		conns:                    make(map[uint32]*UeConn),
		UEs:                      make(map[etsi.SUPI]*UeContext),
		uesByTmsi:                make(map[etsi.TMSI]*UeContext),
		connIDs:                  idgenerator.NewGenerator(1, maxMMEUES1APID),
		tmsi:                     etsi.NewTMSIAllocator(),

		mobileReachableTime: defaultMobileReachableTime,
		implicitDetachTime:  defaultImplicitDetachTime,

		nasGuardCfg: guard.TimerValue{Enable: true, ExpireTime: defaultNASGuardTimeout, MaxRetryTimes: int32(defaultNASGuardMaxRetransmit)},
		esmGuardCfg: guard.TimerValue{Enable: true, ExpireTime: defaultESMGuardTimeout, MaxRetryTimes: int32(defaultNASGuardMaxRetransmit)},
		pagingCfg:   guard.TimerValue{Enable: true, ExpireTime: defaultPagingTimeout, MaxRetryTimes: int32(defaultPagingMaxRetransmit)},

		handoverGuardTimeout: defaultHandoverGuardTimeout,
	}
}

// NetworkFeatureSupport returns the EPS network feature support advertised to UEs
// (TS 24.301 §9.9.3.12A), or the default when unset.
func (m *MME) NetworkFeatureSupport() *eps.EPSNetworkFeatureSupport {
	if m.EPSNetworkFeatureSupport != nil {
		nfs := *m.EPSNetworkFeatureSupport
		return &nfs
	}

	return &eps.EPSNetworkFeatureSupport{IMSVoPS: true}
}

// Tracer instruments the MME's S1AP/EMM control plane.
var Tracer = otel.Tracer("ella-core/mme")
