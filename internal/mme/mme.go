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
	"fmt"
	"sync"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/s1ap"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// DefaultS1MMEPort is the standard S1-MME SCTP port (TS 36.412).
const DefaultS1MMEPort = 36412

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
//     is reached only through chokepoint methods (downlinkSecCtx, nextDownlinkCount,
//     setEPSSecurityContext, markSecured, securitySnapshot) so the COUNT invariant
//     is auditable in one place.
//   - UeContext.emmState/ecmState are atomic — independent enums read on the hot
//     path, kept lock-free.
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
	srv     *sctp.Server
	cred    *udm.Service
	bearer  bearerStore
	session epsSessionManager

	mu          sync.RWMutex
	enbs        map[*sctp.SCTPConn]*enbState
	ues         map[uint32]*UeContext // keyed by MME-UE-S1AP-ID
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

	// Paging supervision (T3413, TS 24.301 §5.6.2). Fields so tests can shorten
	// them.
	pagingTimeout       time.Duration
	pagingMaxRetransmit int
}

// mobileReachableTime supervises the UE's periodic tracking area updating; its
// default is T3412 + 4 minutes (TS 24.301). implicitDetachTime is the
// grace period the MME waits after the mobile reachable timer before it
// implicitly detaches an unreachable UE (the value is network-dependent).
const (
	defaultMobileReachableTime = 58 * time.Minute
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

// The paging supervision timer T3413 bounds how long the MME waits for a paged
// UE to respond before retransmitting and, after a bounded number of attempts,
// giving up (TS 24.301 §5.6.2; the value is network-dependent).
const (
	defaultPagingTimeout       = 6 * time.Second
	defaultPagingMaxRetransmit = 4
)

// New returns an MME network function. cred is the shared credential authority
// (HSS+UDM/ARPF) for EPS-AKA vectors; bearer is the subscription-data store used
// to resolve a subscriber's default-bearer QoS; session is the SMF+PGW-C anchor
// that allocates the UE IP. The MME never holds subscriber keys or the SQN.
func New(cred *udm.Service, bearer bearerStore, session epsSessionManager) *MME {
	return &MME{
		cred:        cred,
		bearer:      bearer,
		session:     session,
		enbs:        make(map[*sctp.SCTPConn]*enbState),
		ues:         make(map[uint32]*UeContext),
		byMTMSI:     make(map[uint32]*UeContext),
		nextMMEUEID: 1,
		mtmsi:       etsi.NewTMSIAllocator(),

		mobileReachableTime: defaultMobileReachableTime,
		implicitDetachTime:  defaultImplicitDetachTime,

		nasGuardTimeout:       defaultNASGuardTimeout,
		nasGuardMaxRetransmit: defaultNASGuardMaxRetransmit,

		pagingTimeout:       defaultPagingTimeout,
		pagingMaxRetransmit: defaultPagingMaxRetransmit,
	}
}

// Start binds the S1-MME SCTP listener and begins accepting eNB associations.
func (m *MME) Start(ctx context.Context, address string, port int) error {
	m.srv = sctp.NewServer(sctp.Config{
		PPID:   s1apPPID,
		Name:   "S1-MME",
		Logger: logger.MmeLog,
	}, sctp.Callbacks{
		Dispatch:     m.dispatch,
		OnDisconnect: m.removeENB,
	})

	return m.srv.ListenAndServe(ctx, address, port, "")
}

// Shutdown stops the S1-MME server, closing the listener and all associations.
func (m *MME) Shutdown(ctx context.Context) {
	if m.srv != nil {
		m.srv.Shutdown(ctx)
	}
}

// tracer instruments the MME's S1AP/EMM control plane.
var tracer = otel.Tracer("ella-core/mme")

// dispatch decodes an S1AP PDU and routes it to the matching procedure handler.
func (m *MME) dispatch(ctx context.Context, conn *sctp.SCTPConn, msg []byte) {
	// Inbound S1AP carries no propagated trace context, so this is a fresh root
	// span.
	ctx, span := tracer.Start(ctx, "s1ap/receive",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.Int("s1ap.message_size", len(msg)),
			attribute.String("network.protocol.name", "s1ap"),
			attribute.String("network.transport", "sctp"),
		),
	)
	defer span.End()

	if conn != nil {
		span.SetAttributes(
			attribute.String("network.peer.address", addrString(conn.RemoteAddr())),
			attribute.String("network.local.address", addrString(conn.LocalAddr())),
		)
	}

	pdu, err := s1ap.Unmarshal(msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to decode S1AP PDU")
		logger.MmeLog.Warn("failed to decode S1AP PDU", zap.Error(err))

		return
	}

	messageType := s1apMessageType(pdu)
	span.SetAttributes(attribute.String("s1ap.message_type", string(messageType)))

	// Track the eNB from an S1 Setup Request before logging, so the inbound
	// event is attributed to the radio ahead of the outbound S1 Setup Response.
	isSetup := false
	if im, ok := pdu.(*s1ap.InitiatingMessage); ok && im.ProcedureCode == s1ap.ProcS1Setup {
		isSetup = true

		m.trackENBFromSetup(conn, im.Value)
	}

	m.touchENB(conn)
	m.logNetworkEvent(ctx, conn, messageType, logger.DirectionInbound, msg)

	// TS 36.413: S1 Setup is the first S1AP procedure on a TNL
	// association. Until it completes, drop every other message — including UE
	// signalling from an eNB whose S1 Setup was rejected.
	if !isSetup && !m.enbSetupComplete(conn) {
		logger.MmeLog.Warn("S1AP message before S1 Setup, dropping",
			zap.String("message-type", string(messageType)))

		return
	}

	switch p := pdu.(type) {
	case *s1ap.InitiatingMessage:
		switch p.ProcedureCode {
		case s1ap.ProcS1Setup:
			m.handleS1Setup(ctx, conn, p.Value)
		case s1ap.ProcInitialUEMessage:
			m.handleInitialUEMessage(ctx, conn, p.Value)
		case s1ap.ProcUplinkNASTransport:
			m.handleUplinkNASTransport(ctx, conn, p.Value)
		case s1ap.ProcUEContextReleaseRequest:
			m.handleUEContextReleaseRequest(ctx, conn, p.Value)
		case s1ap.ProcUECapabilityInfoIndication:
			m.handleUECapabilityInfoIndication(conn, p.Value)
		case s1ap.ProcPathSwitchRequest:
			m.handlePathSwitchRequest(ctx, conn, p.Value)
		case s1ap.ProcErrorIndication:
			m.handleErrorIndication(ctx, conn, p.Value)
		case s1ap.ProcReset:
			m.handleReset(conn, p.Value)
		case s1ap.ProcENBConfigurationUpdate:
			m.handleENBConfigurationUpdate(ctx, conn, p.Value)
		default:
			logger.MmeLog.Debug("ignoring S1AP initiating message", zap.Int("procedure-code", int(p.ProcedureCode)))
		}
	case *s1ap.SuccessfulOutcome:
		switch p.ProcedureCode {
		case s1ap.ProcInitialContextSetup:
			m.handleInitialContextSetupResponse(ctx, conn, p.Value)
		case s1ap.ProcUEContextRelease:
			m.handleUEContextReleaseComplete(conn, p.Value)
		case s1ap.ProcERABSetup:
			m.handleERABSetupResponse(conn, p.Value)
		case s1ap.ProcERABModify:
			m.handleERABModifyResponse(p.Value)
		case s1ap.ProcERABRelease:
			m.handleERABReleaseResponse(conn, p.Value)
		default:
			logger.MmeLog.Debug("ignoring S1AP successful outcome", zap.Int("procedure-code", int(p.ProcedureCode)))
		}
	default:
		logger.MmeLog.Debug("ignoring S1AP PDU")
	}
}

// causeUnknownPLMN is S1AP Cause Misc "unknown-PLMN" (TS 36.413, the
// sixth Misc root value), returned in S1 Setup Failure when the eNB broadcasts no
// PLMN this MME serves.
var causeUnknownPLMN = s1ap.Cause{Group: s1ap.CauseGroupMisc, Value: 5}

// handleS1Setup answers an eNB's S1 Setup Request: an S1 Setup Response when the
// eNB broadcasts a PLMN this MME serves, otherwise an S1 Setup Failure with
// cause "Unknown PLMN" (TS 36.413).
func (m *MME) handleS1Setup(ctx context.Context, conn *sctp.SCTPConn, value []byte) {
	plmn, err := m.operatorPLMN(ctx)
	if err != nil {
		logger.MmeLog.Error("failed to get operator PLMN for S1 Setup", zap.Error(err))
		return
	}

	mmeGroupID, mmeCode := m.mmeIdentity()

	req, outBytes, accepted, err := s1SetupOutcomeFor(value, plmn, mmeGroupID, mmeCode)
	if err != nil {
		logger.MmeLog.Error("failed to handle S1 Setup Request", zap.Error(err))
		return
	}

	logger.MmeLog.Info("S1 Setup Request",
		zap.String("enb-name", req.ENBName),
		zap.Uint32("enb-id", req.GlobalENBID.ENBID.Value),
	)

	if !accepted {
		if _, err := conn.WriteMsg(outBytes, &sctp.SndRcvInfo{PPID: s1apWirePPID, Stream: s1apStreamNonUE}); err != nil {
			logger.MmeLog.Error("failed to send S1 Setup Failure", zap.Error(err))
			return
		}

		m.logNetworkEvent(ctx, conn, S1APProcedureS1SetupFailure, logger.DirectionOutbound, outBytes)

		logger.MmeLog.Warn("S1 Setup rejected: eNB broadcasts no PLMN served by this MME (Unknown PLMN)",
			zap.String("enb-name", req.ENBName),
			zap.String("served-plmn", plmn.Mcc+"/"+plmn.Mnc))

		return
	}

	// A UE re-registers (tracking area updating) whenever the cell's broadcast TAC
	// is absent from its registered TAI list (TS 24.301). Surfacing an
	// eNB/operator TAC mismatch here explains otherwise-unexpected TAU churn.
	if opTAC, err := m.operatorTAC(ctx); err == nil {
		for _, ta := range req.SupportedTAs {
			if uint16(ta.TAC) != opTAC {
				logger.MmeLog.Warn("eNB TAC differs from operator TAC",
					zap.Uint16("enb-tac", uint16(ta.TAC)),
					zap.Uint16("operator-tac", opTAC))
			}
		}
	}

	if _, err := conn.WriteMsg(outBytes, &sctp.SndRcvInfo{PPID: s1apWirePPID, Stream: s1apStreamNonUE}); err != nil {
		logger.MmeLog.Error("failed to send S1 Setup Response", zap.Error(err))
		return
	}

	m.logNetworkEvent(ctx, conn, S1APProcedureS1SetupResponse, logger.DirectionOutbound, outBytes)

	// S1 Setup has completed: allow the eNB's UE-associated signalling through the
	// dispatcher's setup-first gate (TS 36.413).
	m.markENBSetupComplete(conn)

	logger.MmeLog.Info("S1 Setup Response sent", zap.String("enb-name", req.ENBName))
}

// handleENBConfigurationUpdate answers an eNB's ENB CONFIGURATION UPDATE: it
// validates that any updated supported TAs still broadcast a PLMN this MME serves
// (otherwise an ENB CONFIGURATION UPDATE FAILURE with cause "Unknown PLMN"),
// stores an updated eNB name, and acknowledges (TS 36.413 §8.7.4). The eNB blocks
// on this response, so an unhandled update would stall its reconfiguration.
func (m *MME) handleENBConfigurationUpdate(ctx context.Context, conn *sctp.SCTPConn, value []byte) {
	req, err := s1ap.ParseENBConfigurationUpdate(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode ENB Configuration Update", zap.Error(err))
		return
	}

	plmn, err := m.operatorPLMN(ctx)
	if err != nil {
		logger.MmeLog.Error("failed to get operator PLMN for ENB Configuration Update", zap.Error(err))
		return
	}

	out, accepted, err := enbConfigUpdateOutcomeFor(req, plmn)
	if err != nil {
		logger.MmeLog.Error("failed to handle ENB Configuration Update", zap.Error(err))
		return
	}

	msgType := S1APProcedureENBConfigUpdateAck
	if !accepted {
		msgType = S1APProcedureENBConfigUpdateFailure
	}

	if _, err := conn.WriteMsg(out, &sctp.SndRcvInfo{PPID: s1apWirePPID, Stream: s1apStreamNonUE}); err != nil {
		logger.MmeLog.Error("failed to send ENB Configuration Update response", zap.Error(err))
		return
	}

	m.logNetworkEvent(ctx, conn, msgType, logger.DirectionOutbound, out)

	if !accepted {
		logger.MmeLog.Warn("ENB Configuration Update rejected: eNB broadcasts no served PLMN (Unknown PLMN)")
		return
	}

	if req.ENBName != "" {
		m.updateENBName(conn, req.ENBName)
	}

	logger.MmeLog.Info("ENB Configuration Update acknowledged", zap.String("enb-name", req.ENBName))
}

// enbConfigUpdateOutcomeFor produces the S1AP response to an ENB CONFIGURATION
// UPDATE: an Acknowledge when any updated supported TAs still broadcast a PLMN
// this MME serves, otherwise an ENB CONFIGURATION UPDATE FAILURE with cause
// "Unknown PLMN" (TS 36.413 §8.7.4). accepted reports which was produced. An
// update with no supported TAs (a name- or DRX-only change) is always accepted.
func enbConfigUpdateOutcomeFor(req *s1ap.ENBConfigurationUpdate, plmn models.PlmnID) (out []byte, accepted bool, err error) {
	if len(req.SupportedTAs) > 0 {
		served, err := encodePLMN(plmn)
		if err != nil {
			return nil, false, fmt.Errorf("mme: encode served PLMN: %w", err)
		}

		if !enbBroadcastsPLMN(req.SupportedTAs, served) {
			out, err = (&s1ap.ENBConfigurationUpdateFailure{Cause: causeUnknownPLMN}).Marshal()
			if err != nil {
				return nil, false, fmt.Errorf("mme: marshal ENB Configuration Update Failure: %w", err)
			}

			return out, false, nil
		}
	}

	out, err = (&s1ap.ENBConfigurationUpdateAcknowledge{}).Marshal()
	if err != nil {
		return nil, false, fmt.Errorf("mme: marshal ENB Configuration Update Acknowledge: %w", err)
	}

	return out, true, nil
}

// s1SetupOutcomeFor decodes an S1 Setup Request and produces the S1AP message to
// send back: an S1 Setup Response when the eNB broadcasts a PLMN this MME serves,
// otherwise an S1 Setup Failure with cause "Unknown PLMN" (TS 36.413).
// accepted reports which outcome was produced.
func s1SetupOutcomeFor(reqValue []byte, plmn models.PlmnID, mmeGroupID uint16, mmeCode uint8) (req *s1ap.S1SetupRequest, out []byte, accepted bool, err error) {
	req, err = s1ap.ParseS1SetupRequest(reqValue)
	if err != nil {
		return nil, nil, false, fmt.Errorf("mme: parse S1 Setup Request: %w", err)
	}

	served, err := encodePLMN(plmn)
	if err != nil {
		return req, nil, false, fmt.Errorf("mme: encode served PLMN: %w", err)
	}

	if !enbBroadcastsPLMN(req.SupportedTAs, served) {
		out, err = (&s1ap.S1SetupFailure{Cause: causeUnknownPLMN}).Marshal()
		if err != nil {
			return req, nil, false, fmt.Errorf("mme: marshal S1 Setup Failure: %w", err)
		}

		return req, out, false, nil
	}

	resp, err := buildS1SetupResponse(plmn, mmeGroupID, mmeCode)
	if err != nil {
		return req, nil, false, err
	}

	out, err = resp.Marshal()
	if err != nil {
		return req, nil, false, fmt.Errorf("mme: marshal S1 Setup Response: %w", err)
	}

	return req, out, true, nil
}

// enbBroadcastsPLMN reports whether any PLMN the eNB broadcasts across its
// supported TAs equals plmn (TS 36.413).
func enbBroadcastsPLMN(tas s1ap.SupportedTAs, plmn s1ap.PLMNIdentity) bool {
	for _, ta := range tas {
		for _, b := range ta.BroadcastPLMNs {
			if b == plmn {
				return true
			}
		}
	}

	return false
}
