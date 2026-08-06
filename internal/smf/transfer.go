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
// described. Each access maps it to ESM #54 "PDN connection does not exist"
// (TS 24.301 §6.5.1.6 b) or 5GSM #54 "PDU session does not exist"
// (TS 24.501 §6.4.1.7 d).
var ErrSessionNotTransferable = errors.New("session does not exist on the other access")

// transferRequest is what the target access brings to a transfer. The PDU
// session identity is absent: it is the correlator that found the session, and
// it does not change.
type transferRequest struct {
	Access AccessType
	// EBI is the default bearer's EPS bearer identity on EPS, and 0 on 5GS.
	EBI    uint8
	Dnn    string
	Snssai *models.Snssai
	Policy *Policy
}

// findTransferable resolves the session a UE asked to move to the other access,
// correlated by PDU session identity (TS 23.502 §4.11.2.2 step 13,
// §4.11.2.3 step 9).
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

// transferable reports whether the session can move as req describes. Caller
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
// the UPF session, its SEID and its uplink F-TEID. The downlink is left
// buffering until the target access binds its own RAN endpoint.
func (s *SMF) transferSession(ctx context.Context, sc *SMContext, req transferRequest) error {
	// One transfer at a time per session: the move spans several blocking UPF
	// calls.
	if err := sc.procedures.Begin(procedure.Transfer); err != nil {
		return fmt.Errorf("%w: %v", ErrSessionNotTransferable, err)
	}

	defer sc.procedures.End(procedure.Transfer)

	from, sourceID, err := s.moveSession(ctx, sc, req)
	if err != nil {
		return err
	}

	s.dropSourceRouting(ctx, sc.Supi, from, sourceID)

	return nil
}

// dropSourceRouting tells the access a session left to forget it and to release
// what its radio still holds for it, without releasing the session or the UE
// address (TS 23.502 §4.11.2.2 step 14, §4.11.2.3 step 10).
func (s *SMF) dropSourceRouting(ctx context.Context, supi etsi.SUPI, from AccessType, id SessionIdentity) {
	switch from {
	case Access5G:
		if s.amf == nil {
			return
		}

		// A UE in dual-registration mode stays on NG-RAN while it moves sessions
		// one at a time, so the moved session's resources are stranded unless the
		// RAN is told to release them.
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

	// A release, a replacement or another transfer can land between the request
	// and this lock.
	if err := sc.transferable(req); err != nil {
		return from, sourceID, err
	}

	if err := s.setEPSBearerIdentity(sc, req.EBI); err != nil {
		return from, sourceID, err
	}

	policy := req.Policy
	if len(policy.NetworkRules) == 0 && sc.PolicyData != nil {
		// Network rules are subscriber and policy scoped, not access scoped.
		policy.NetworkRules = sc.PolicyData.NetworkRules
	}

	if err := s.applySessionQERs(ctx, sc, policy.PolicyID, policy.QosData.QFI, policy.Ambr.Uplink, policy.Ambr.Downlink); err != nil {
		s.setEPSBearerIdentity(sc, sourceID.EBI) //nolint:errcheck // restoring a key this session just held
		return from, sourceID, fmt.Errorf("apply target access QoS: %w", err)
	}

	// The snapshot keeps memory and the UPF in step if the UPF rejects the change.
	dl := sc.Tunnel.DataPath.DownLinkTunnel.PDR
	restore := downlinkSnapshot(dl)

	farList, err := handleUpCnxStateDeactivate(sc)
	if err != nil {
		restore()
		s.setEPSBearerIdentity(sc, sourceID.EBI) //nolint:errcheck // restoring a key this session just held

		return from, sourceID, fmt.Errorf("suspend downlink: %w", err)
	}

	// The UE is not idle, so paging the access it left reaches nobody.
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

	// The move ends whatever the source access had outstanding: a T3592 expiry
	// aborts a session the UE still holds (TS 24.501 §6.3.2.5, §6.3.3).
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
// address round-trips to the family it was allocated in.
func ipToNetip(ip net.IP) netip.Addr {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return netip.Addr{}
	}

	return addr.Unmap()
}
