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
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/s1ap"
	"go.opentelemetry.io/otel"
)

// DefaultS1MMEPort is the standard S1-MME SCTP port (TS 36.412).
const DefaultS1MMEPort = 36412

// NASHandler is the EMM/ESM NAS layer's entry surface, implemented in
// internal/mme/nas and injected so the S1AP layer dispatches uplink NAS without
// the kernel importing its layers (kernel ⊅ nas).
type NASHandler interface {
	HandleNAS(ctx context.Context, ue *UeContext, nas []byte)
	HandleServiceRequest(ctx context.Context, conn NasWriter, msg *s1ap.InitialUEMessage)
	DispatchEMM(ctx context.Context, ue *UeContext, plain []byte, integrityVerified bool)
}

// MME is Ella Core's 4G Mobility Management Entity control-plane network function.
// epsSessionManager is the converged session anchor (SMF acting as PGW-C) the
// MME delegates EPS default-bearer establishment to: it allocates the UE IP and
// owns the session. *smf.SMF satisfies it. Defined here (consumer side) so there
// is no mme → smf import.
type epsSessionManager interface {
	// CreateEPSSession negotiates the PDN type, allocates the UE address(es), and
	// programs the default bearer, returning the negotiated type, the addresses,
	// and the S-GW S1-U F-TEID for the eNB to send uplink to.
	CreateEPSSession(ctx context.Context, req models.EPSBearerRequest) (models.EPSBearer, error)
	// ModifyEPSSession sets the downlink endpoint to the eNB S1-U F-TEID once it
	// is known from the Initial Context Setup Response. ebi identifies the PDN
	// connection's default bearer.
	ModifyEPSSession(ctx context.Context, imsi string, ebi uint8, enb models.FTEID) error
	// UpdateEPSSessionAMBR updates the Session-AMBR enforced by the UPF QER for a
	// PDN connection's default bearer, in the "<n> <unit>" form. Used when a policy
	// edit changes the per-APN Session-AMBR mid-session.
	UpdateEPSSessionAMBR(ctx context.Context, imsi string, ebi uint8, ambrUplink, ambrDownlink string) error
	// DeactivateEPSSession buffers the downlink bearer when the UE goes ECM-IDLE
	// so downlink data triggers paging.
	DeactivateEPSSession(ctx context.Context, imsi string, ebi uint8) error
	ReleaseEPSSession(ctx context.Context, imsi string, ebi uint8) error
}

// Concurrency model. A UE's state is touched by several goroutines: the eNB
// dispatch loop (serial per SCTP association), the data-network reconcile
// backstop, the status and detach API, and timer callbacks. Two locks, with a
// fixed ordering, plus two atomics:
//
//   - MME.mu guards the registry and lifecycle: the ues/byMTMSI/enbs maps,
//     nextMMEUEID, the M-TMSI allocator, each UE's S1 identity (conn,
//     MME/ENB-UE-S1AP-IDs), the idle/paging/NAS-guard timers and their generation
//     counters, and the releasing flag.
//   - UeContext.mu guards that UE's data: the EPS NAS security context (keys and
//     NAS COUNTs), the PDN/bearer state (the pdns map, defaultEBI, and each
//     connection's in-flight modification flags), and imsi. The security context
//     is reached only through chokepoint methods (installNASSecurityContext,
//     protectDownlink, tryUnprotectUplink, deriveInitialKeNB, markSecured,
//     securitySnapshot) so the keys never leave the kernel and the COUNT invariant
//     is auditable in one place.
//   - UeContext.emmState is atomic — an enum read on the hot path, kept lock-free.
//     The ECM state is derived from whether the UE holds an S1-connection (ue.s1).
//
// Lock ordering (acquire in this order, never reverse):
//
//	MME.mu  →  UeContext.mu
//
// Never hold a lock across an external call (SMF, DB, SCTP send): snapshot the
// state, release, then send. Reads of a UE's data from another goroutine that
// first observe emmState == EMM-REGISTERED (status, reconcile) are safe without
// UeContext.mu — the atomic store at registration carries the happens-before.
type MME struct {
	Cred    *udm.Service
	Bearer  bearerStore
	Session epsSessionManager
	NAS     NASHandler

	mu          sync.RWMutex
	enbs        map[*sctp.SCTPConn]*enbState
	enbByID     map[string]NasWriter  // S1-setup-complete eNBs keyed by Global eNB ID, for S1-handover target resolution
	conns       map[uint32]*S1Conn    // UE-associated S1-connections keyed by MME-UE-S1AP-ID; conn.ue is nil until a UE context is bound
	ues         map[string]*UeContext // persistent UE contexts keyed by IMSI; survives the connection across ECM-IDLE
	byMTMSI     map[uint32]*UeContext // keyed by M-TMSI, for S-TMSI lookup
	nextMMEUEID uint32
	// mtmsi allocates an unpredictable M-TMSI (TS 23.401 privacy): random MSBs
	// with allocate/free.
	mtmsi *etsi.TmsiAllocator

	// Idle-mode reachability supervision (TS 24.301). Fields so tests can
	// shorten them.
	mobileReachableTime time.Duration
	implicitDetachTime  time.Duration

	// NAS common-procedure guard (TS 24.301: T3450/T3460/T3470). Fields so
	// tests can shorten them.
	nasGuardTimeout       time.Duration
	nasGuardMaxRetransmit int

	// ESM bearer-procedure guard (TS 24.301: T3486 modify, T3495 deactivate),
	// which the spec sets longer than the common-procedure timers. Field so tests
	// can shorten it.
	esmGuardTimeout time.Duration

	// Paging supervision (T3413, TS 24.301 §5.6.2). Fields so tests can shorten
	// them.
	pagingTimeout       time.Duration
	pagingMaxRetransmit int

	// S1-handover supervision bounds the whole procedure (HANDOVER REQUIRED →
	// NOTIFY) so a target that goes silent does not pin the UE's handover slot.
	// Field so tests can shorten it.
	handoverGuardTimeout time.Duration
}

// T3412PeriodicTAU is the periodic tracking-area-update timer the MME advertises
// to UEs (TS 24.301). It is the single source for both the value encoded into the
// Attach Accept and the mobile reachable timer below, so the two cannot drift if
// it ever becomes configurable.
const T3412PeriodicTAU = 54 * time.Minute

// mobileReachableTime supervises the UE's periodic tracking area updating: it is
// the periodic-TAU timer + 4 minutes (TS 24.301 §5.3.5), derived from
// T3412PeriodicTAU exactly as the AMF derives T3512 + 4 minutes. implicitDetachTime
// is the grace period the MME waits after the mobile reachable timer before it
// implicitly detaches an unreachable UE (the value is network-dependent).
const (
	defaultMobileReachableTime = T3412PeriodicTAU + 4*time.Minute
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
func New(cred *udm.Service, bearer bearerStore, session epsSessionManager) *MME {
	return &MME{
		Cred:        cred,
		Bearer:      bearer,
		Session:     session,
		enbs:        make(map[*sctp.SCTPConn]*enbState),
		enbByID:     make(map[string]NasWriter),
		conns:       make(map[uint32]*S1Conn),
		ues:         make(map[string]*UeContext),
		byMTMSI:     make(map[uint32]*UeContext),
		nextMMEUEID: 1,
		mtmsi:       etsi.NewTMSIAllocator(),

		mobileReachableTime: defaultMobileReachableTime,
		implicitDetachTime:  defaultImplicitDetachTime,

		nasGuardTimeout:       defaultNASGuardTimeout,
		nasGuardMaxRetransmit: defaultNASGuardMaxRetransmit,
		esmGuardTimeout:       defaultESMGuardTimeout,

		pagingTimeout:       defaultPagingTimeout,
		pagingMaxRetransmit: defaultPagingMaxRetransmit,

		handoverGuardTimeout: defaultHandoverGuardTimeout,
	}
}

// tracer instruments the MME's S1AP/EMM control plane.
var Tracer = otel.Tracer("ella-core/mme")
