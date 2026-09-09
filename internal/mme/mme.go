// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/radioreg"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/internal/util/idgenerator"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.opentelemetry.io/otel"
)

const DefaultS1MMEPort = 36412

const maxMMEUES1APID int64 = 4294967295

type NASHandler interface {
	HandleNAS(ctx context.Context, conn *UeConn, nas []byte)
	HandleServiceRequest(ctx context.Context, conn S1APWriter, msg *s1ap.InitialUEMessage)
}

type epsSessionManager interface {
	CreateEPSSession(ctx context.Context, req models.EPSBearerRequest) (models.EPSBearer, error)
	TransferIdleToEPS(ctx context.Context, supi etsi.SUPI, pduSessionID, epsBearerIdentity uint8, dnn string, snssai *models.Snssai) (models.EPSBearer, error)
	ModifyEPSSession(ctx context.Context, ref string, ebi uint8, enb models.FTEID) error
	UpdateEPSSessionAMBR(ctx context.Context, ref string, ambrUplink, ambrDownlink models.BitRate) error
	DeactivateEPSSession(ctx context.Context, ref string) error
	HandleEPSPagingFailure(ctx context.Context, imsi string, ebi uint8) error
	ClearEPSPagingSuppression(ctx context.Context, imsi string, ebi uint8) error
	ReleaseEPSSession(ctx context.Context, ref string) error
	EPSSubscriptionChanged(ctx context.Context, ref string) (models.SubscriptionDelta, error)
}

type credentialProvider interface {
	GenerateEPSVector(ctx context.Context, imsi string, plmnID []byte, resyncAuts, resyncRand string) (*udm.EPSAV, error)
}

// Concurrency model. A UE's state is touched by the eNB dispatch loop (serial per
// SCTP association), the reconcile backstop, the status/detach API, and timer
// callbacks. Three locks, acquired in this order and never reversed:
//
//		DownlinkSender  →  MME.mu  →  UeContext.mu
//
//	  - MME.mu guards the registry and lifecycle: the UEs/uesByTmsi/radios maps, the
//	    MME-UE-S1AP-ID and M-TMSI allocators, each UE's S1-connection fields (conn,
//	    MME/ENB-UE-S1AP-IDs, releasing), and the idle/paging/NAS-guard timers with
//	    their generation counters. ue.active is swapped under MME.mu on bind/release
//	    but is an atomic.Pointer, so the hot path reads it lock-free via Conn().
//	  - UeContext.mu guards that UE's data: emmState, imsi, the EPS NAS security
//	    context (keys, uplink NAS COUNT, NH/NCC chain), and the PDN/bearer state
//	    (pdns, defaultEBI, in-flight modification flags). The security context is
//	    reached only through chokepoint methods (installNASSecurityContext,
//	    tryUnprotectUplink, deriveInitialKeNB, markSecured, Snapshot) and the
//	    downlink sender. ECM state is derived from ue.active.
//	  - The nas.DownlinkSender lock (ue.downlink()) makes taking a downlink NAS COUNT
//	    and writing the message that carries it one step, so messages cannot be
//	    written out of COUNT order — the UE would fail their integrity check
//	    (TS 24.301 §4.4.3.1, §4.4.3.3).
//
// Never hold MME.mu or UeContext.mu across an external call (SMF, DB, SCTP send):
// snapshot, release, then send. The DownlinkSender's lock is the exception — it is
// held across the S1AP send, a non-blocking enqueue on the association's writer —
// so the write closure passed to SendProtected must only frame and write the
// protected PDU. It must not take UeContext.mu, call the SMF, DB or a peer RAT, or
// send a second protected message (the lock is not reentrant); the security context
// is built under UeContext.mu and handed to the sender afterwards.
//
// UeContext.mu is also the publication barrier: a reader that observes
// emmState == EMM-REGISTERED under it may read the UE's other registered data.
type MME struct {
	Cred    credentialProvider
	Bearer  bearerStore
	Session epsSessionManager
	NAS     NASHandler
	FiveGS  interworking.FiveGSPeer

	// EPSNetworkFeatureSupport is advertised in Attach/TAU Accept (TS 24.301
	// §9.9.3.12A); nil falls back to the default.
	EPSNetworkFeatureSupport *eps.NetworkFeatureSupport

	Name             string
	relativeCapacity atomic.Uint32

	mu        sync.RWMutex
	reg       *radioreg.Registry[S1APWriter, string, *Radio]
	conns     map[uint32]*UeConn       // UE-associated S1-connections keyed by MME-UE-S1AP-ID; conn.ue is nil until a UE context is bound
	UEs       map[etsi.SUPI]*UeContext // persistent UE contexts keyed by SUPI; survives the connection across ECM-IDLE
	uesByTmsi map[etsi.TMSI]*UeContext // keyed by M-TMSI, for S-TMSI lookup
	connIDs   *idgenerator.IDGenerator // recycling MME-UE-S1AP-ID allocator (TS 36.413 no-immediate-reuse)

	relocating map[etsi.SUPI]*relocation

	lastSeen lastSeenStore

	relocationIDs atomic.Uint64

	tmsi *etsi.TmsiAllocator

	// Supervision timers are fields, not constants, so tests can shorten them.
	mobileReachableTime time.Duration // idle-mode reachability (TS 24.301)
	implicitDetachTime  time.Duration

	// Retransmitting supervision guards, held as guard.TimerValue.
	nasGuardCfg guard.TimerValue // NAS common-procedure guard (TS 24.301: T3450/T3460/T3470)
	esmGuardCfg guard.TimerValue // ESM bearer-procedure guard (TS 24.301: T3486/T3495), 4G-only (no AMF peer)
	t3489Cfg    guard.TimerValue // ESM information request guard (TS 24.301: T3489)
	pagingCfg   guard.TimerValue // paging supervision (T3413, TS 24.301 §5.6.2)

	// handoverGuardTimeout bounds the whole S1 handover (HANDOVER REQUIRED → NOTIFY)
	// so a silent target does not pin the UE's handover slot.
	handoverGuardTimeout time.Duration
}

const T3412PeriodicTAU = 54 * time.Minute

const T3402Backoff = 12 * time.Minute

const (
	mobileReachableMargin      = 4 * time.Minute
	defaultMobileReachableTime = T3412PeriodicTAU + mobileReachableMargin
	defaultImplicitDetachTime  = 2 * time.Minute
)

const (
	defaultNASGuardTimeout       = 6 * time.Second
	defaultNASGuardMaxRetransmit = 4
)

// TS 24.301 §6.6.1.2.6 a) aborts on the third expiry.
const (
	defaultT3489Timeout       = 4 * time.Second
	defaultT3489MaxRetransmit = 2
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
	m := &MME{
		Cred:                     cred,
		Bearer:                   bearer,
		Session:                  session,
		EPSNetworkFeatureSupport: &eps.NetworkFeatureSupport{IMSVoPS: true},
		Name:                     "ella",
		reg:                      radioreg.New[S1APWriter, string, *Radio](DefaultRadioOfflineTTL, DefaultMaxOfflineRadios, time.Now),
		conns:                    make(map[uint32]*UeConn),
		UEs:                      make(map[etsi.SUPI]*UeContext),
		uesByTmsi:                make(map[etsi.TMSI]*UeContext),
		relocating:               make(map[etsi.SUPI]*relocation),
		connIDs:                  idgenerator.NewGenerator(1, maxMMEUES1APID),
		tmsi:                     etsi.NewTMSIAllocator(),

		mobileReachableTime: defaultMobileReachableTime,
		implicitDetachTime:  defaultImplicitDetachTime,

		nasGuardCfg: guard.TimerValue{Enable: true, ExpireTime: defaultNASGuardTimeout, MaxRetryTimes: int32(defaultNASGuardMaxRetransmit)},
		esmGuardCfg: guard.TimerValue{Enable: true, ExpireTime: defaultESMGuardTimeout, MaxRetryTimes: int32(defaultNASGuardMaxRetransmit)},
		t3489Cfg:    guard.TimerValue{Enable: true, ExpireTime: defaultT3489Timeout, MaxRetryTimes: int32(defaultT3489MaxRetransmit)},
		pagingCfg:   guard.TimerValue{Enable: true, ExpireTime: defaultPagingTimeout, MaxRetryTimes: int32(defaultPagingMaxRetransmit)},

		handoverGuardTimeout: defaultHandoverGuardTimeout,
	}

	m.relativeCapacity.Store(uint32(DefaultRelativeCapacity))

	return m
}

func (m *MME) NetworkFeatureSupport(ueCap eps.UENetworkCapability) *eps.NetworkFeatureSupport {
	nfs := eps.NetworkFeatureSupport{IMSVoPS: true}
	if m.EPSNetworkFeatureSupport != nil {
		nfs = *m.EPSNetworkFeatureSupport
	}

	nfs.EPCO = ueCap.SupportsEPCO()

	nfs.IWKN26 = false

	return &nfs
}

var Tracer = otel.Tracer("ella-core/mme")
