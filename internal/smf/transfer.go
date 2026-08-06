// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/ellanetworks/core/internal/smf/procedure"
	"go.uber.org/zap"
)

// ErrSessionNotTransferable reports that the session a UE asked to move to the
// access it is now on does not exist, or exists but is not the one the UE
// described. Each access maps it to the cause its NAS defines for a transfer of
// something the network does not hold: ESM #54 "PDN connection does not exist"
// (TS 24.301 §6.5.1.6 b) and 5GSM #54 "PDU session does not exist"
// (TS 24.501 §6.4.1.7 d).
var ErrSessionNotTransferable = errors.New("session does not exist on the other access")

// transferRequest is what the target access brings to a transfer: the access
// itself, the EPS bearer identity it names the session by there, the data
// network the UE named, and the policy the access resolved. The PDU session
// identity is absent because it is the correlator that found the session, and
// it does not change.
type transferRequest struct {
	Access AccessType
	// EBI is the default bearer's EPS bearer identity on EPS, and 0 on 5GS,
	// where the PDN connection's bearer identity is given up.
	EBI uint8
	Dnn string
	// Snssai is the slice the UE named, where its access signals one. A session
	// under a different slice is not the session the UE described, for the same
	// reason the data network has to match.
	Snssai *models.Snssai
	Policy *Policy
}

// findTransferable resolves the session a UE asked to move to the other access.
// The PDU session identity correlates the two (TS 23.502 §4.11.2.2 step 13,
// §4.11.2.3 step 9); the data network has to match too, because the UE names
// both and a session under a different one is not the session it described.
func (s *SMF) findTransferable(supi etsi.SUPI, pduSessionID uint8, req transferRequest) (*SMContext, error) {
	if pduSessionID == 0 {
		return nil, fmt.Errorf("%w: the UE named no PDU session identity", ErrSessionNotTransferable)
	}

	if pduSessionID > 15 {
		return nil, fmt.Errorf("%w: PDU session identity %d is outside the range a UE may allocate", ErrSessionNotTransferable, pduSessionID)
	}

	sc := s.currentPDUSession(supi, pduSessionID)
	if sc == nil {
		return nil, fmt.Errorf("%w: no session with PDU session id %d", ErrSessionNotTransferable, pduSessionID)
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if err := sc.transferable(req); err != nil {
		return nil, err
	}

	return sc, nil
}

// transferable reports whether the session can move as req describes. Both
// findTransferable and the move itself test it: the first to reject the request
// before any work, the second because the lock is dropped in between. Caller
// holds sc.Mutex.
func (sc *SMContext) transferable(req transferRequest) error {
	if sc.Access == req.Access {
		return fmt.Errorf("%w: PDU session %d is already on %s", ErrSessionNotTransferable, sc.PDUSessionID, req.Access)
	}

	if sc.Dnn != req.Dnn {
		return fmt.Errorf("%w: PDU session %d is on data network %q, not %q", ErrSessionNotTransferable, sc.PDUSessionID, sc.Dnn, req.Dnn)
	}

	if sc.Snssai != nil && req.Snssai != nil && !sc.Snssai.Equal(*req.Snssai) {
		return fmt.Errorf("%w: PDU session %d is on slice %+v, not %+v", ErrSessionNotTransferable, sc.PDUSessionID, sc.Snssai, req.Snssai)
	}

	if sc.releasing {
		return fmt.Errorf("%w: PDU session %d is being released", ErrSessionNotTransferable, sc.PDUSessionID)
	}

	if sc.Tunnel == nil || sc.Tunnel.DataPath == nil || sc.Tunnel.DataPath.DownLinkTunnel == nil ||
		sc.Tunnel.DataPath.DownLinkTunnel.PDR == nil || sc.PFCPContext == nil {
		return fmt.Errorf("%w: PDU session %d has no user plane to move", ErrSessionNotTransferable, sc.PDUSessionID)
	}

	return nil
}

// downlinkSnapshot captures the downlink rule state a suspend overwrites, and
// returns a closure that puts it back. Caller holds sc.Mutex.
func downlinkSnapshot(dl *PDR) func() {
	state, action := dl.State, dl.FAR.ApplyAction
	farState := dl.FAR.State

	var ohc *models.OuterHeaderCreation
	if dl.FAR.ForwardingParameters != nil {
		ohc = dl.FAR.ForwardingParameters.OuterHeaderCreation
	}

	return func() {
		dl.State, dl.FAR.ApplyAction, dl.FAR.State = state, action, farState
		if dl.FAR.ForwardingParameters != nil {
			dl.FAR.ForwardingParameters.OuterHeaderCreation = ohc
		}
	}
}

// transferSession moves an established session to the other access with session
// continuity (TS 23.501 §5.17.2): the UE keeps its address, and the anchor keeps
// the UPF session, its SEID and its uplink F-TEID, so only the access-network
// end of the tunnel is rebuilt. The downlink is left buffering — the target
// access binds its own RAN endpoint afterwards (ModifyEPSSession on EPS, the N2
// resource setup response on 5GS), and that is what re-points the downlink FAR
// and sets the PDU Session Container behaviour for the new access.
func (s *SMF) transferSession(ctx context.Context, sc *SMContext, req transferRequest) error {
	// One transfer at a time per session: the move spans several blocking UPF
	// calls, and two interleaved would each read the other's half-applied state —
	// the second would report the access the first had just moved to as the one
	// being left, and tell it to forget a session it had just been given.
	if err := sc.procedures.Begin(procedure.Transfer); err != nil {
		return fmt.Errorf("%w: %v", ErrSessionNotTransferable, err)
	}

	defer sc.procedures.End(procedure.Transfer)

	from, sourceID, err := s.moveSession(ctx, sc, req)
	if err != nil {
		return err
	}

	// The access the session left still routes to it. Left in place, that
	// context's own release path would tear down a session the UE is now using
	// on the other access, so the anchor tells it the session is gone — without
	// releasing the session or the UE address (TS 23.502 §4.11.2.2 step 14,
	// §4.11.2.3 step 10). Called outside the session lock, as every call out of
	// the SMF is.
	s.dropSourceRouting(ctx, sc.Supi, from, sourceID)

	return nil
}

// dropSourceRouting tells the access a session left to forget it and to release
// what its radio still holds for it.
func (s *SMF) dropSourceRouting(ctx context.Context, supi etsi.SUPI, from AccessType, id SessionIdentity) {
	switch from {
	case Access5G:
		if s.amf == nil {
			return
		}

		// A UE in dual-registration mode stays on NG-RAN while it moves sessions
		// one at a time, so the resources of the one that moved have to be
		// released or they are stranded there.
		n2Release, err := ngap.BuildPDUSessionResourceReleaseCommandTransfer()
		if err != nil {
			logger.WithTrace(ctx, logger.SmfLog).Warn("failed to build the N2 release for a moved session; dropping routing only",
				zap.Error(err), logger.SUPI(supi.String()), logger.PDUSessionID(id.PDUSessionID))

			n2Release = nil
		}

		s.amf.SessionTransferred(ctx, supi, id.PDUSessionID, n2Release)
	case Access4G:
		if s.mme != nil {
			s.mme.SessionTransferred(ctx, supi.IMSI(), id.EBI)
		}
	}
}

// moveSession performs the move itself, returning the access the session left
// and the identity it had there.
func (s *SMF) moveSession(ctx context.Context, sc *SMContext, req transferRequest) (AccessType, SessionIdentity, error) {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	from, sourceID := sc.Access, sc.SessionIdentity

	// findTransferable tested these and released the lock; re-test them under the
	// lock that performs the change, because a release, a replacement or another
	// transfer can land in between.
	if err := sc.transferable(req); err != nil {
		return from, sourceID, err
	}

	// The bearer identity is claimed before any UPF work, so a collision costs
	// nothing and the rollback below has one less thing to undo.
	if err := s.setEPSBearerIdentity(sc, req.EBI); err != nil {
		return from, sourceID, err
	}

	policy := req.Policy
	if len(policy.NetworkRules) == 0 && sc.PolicyData != nil {
		// Network rules are subscriber and policy scoped, not access scoped, and
		// the EPS resolution path does not produce them.
		policy.NetworkRules = sc.PolicyData.NetworkRules
	}

	// Session-AMBR and the QFI the user plane is marked with are resolved per
	// access, so the target's policy has to reach the UPF before its RAN does.
	if err := s.applySessionQERs(ctx, sc, policy.PolicyID, policy.QosData.QFI, policy.Ambr.Uplink, policy.Ambr.Downlink); err != nil {
		s.setEPSBearerIdentity(sc, sourceID.EBI) //nolint:errcheck // restoring a key this session just held
		return from, sourceID, fmt.Errorf("apply target access QoS: %w", err)
	}

	// The downlink FAR is about to be cleared in memory. If the UPF rejects the
	// change the two would diverge — worse, reconcile gates on the very flag being
	// cleared (upConnectionActive), so the session would never be reconciled
	// again — so snapshot enough to put it back.
	dl := sc.Tunnel.DataPath.DownLinkTunnel.PDR
	restore := downlinkSnapshot(dl)

	farList, err := handleUpCnxStateDeactivate(sc)
	if err != nil {
		restore()
		s.setEPSBearerIdentity(sc, sourceID.EBI) //nolint:errcheck // restoring a key this session just held

		return from, sourceID, fmt.Errorf("suspend downlink: %w", err)
	}

	// Buffer without a downlink data notification: the UE is not idle, it is on
	// the other access, and paging the one it left would reach nobody.
	dl.FAR.ApplyAction.Nocp = false

	if err := s.upf.ModifySession(ctx, BuildModifyRequest(sc.PFCPContext.RemoteSEID, policy.PolicyID, nil, farList, nil)); err != nil {
		restore()
		s.setEPSBearerIdentity(sc, sourceID.EBI) //nolint:errcheck // restoring a key this session just held

		return from, sourceID, fmt.Errorf("suspend downlink in the UPF: %w", err)
	}

	sc.Access = req.Access
	sc.PolicyData = policy

	if req.Snssai != nil {
		sc.Snssai = req.Snssai
	}

	// The move ends whatever the source access had outstanding: a retransmission
	// aimed at an access the UE has left reaches nobody, and a T3592 abort would
	// drop a live session (TS 24.501 §6.3.2.5, §6.3.3).
	sc.stopProcedureTimer()
	sc.pendingPolicy = nil
	sc.establishmentPTI = 0
	sc.outstandingPTIs = nil

	logger.WithTrace(ctx, logger.SmfLog).Info("moved session to the other access",
		logger.SUPI(sc.Supi.String()), logger.PDUSessionID(sc.PDUSessionID),
		zap.Stringer("from", from), zap.Stringer("to", req.Access))

	return from, sourceID, nil
}

// ipToNetip is the inverse of netipToIP, unmapping the IPv4-in-IPv6 form so an
// address round-trips to the family it was allocated in. An address the slice
// cannot represent reads as invalid, which the models treat as absent.
func ipToNetip(ip net.IP) netip.Addr {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return netip.Addr{}
	}

	return addr.Unmap()
}
