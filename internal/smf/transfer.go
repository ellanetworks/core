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

var ErrSessionNotTransferable = errors.New("session does not exist on the other access")

type transferRequest struct {
	Access AccessType
	// EBI is 0 on 5GS.
	EBI    uint8
	Dnn    string
	Snssai *models.Snssai
	Policy *Policy
}

// The PDU session identity correlates the two accesses (TS 23.502 §4.11.2.2
// step 13, §4.11.2.3 step 9).
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

// Caller holds sc.Mutex.
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

// Caller holds sc.Mutex.
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

// Session continuity (TS 23.501 §5.17.2): the UE address, the UPF session, its
// SEID and its uplink F-TEID all survive the move. The downlink buffers until
// the target access binds its own RAN endpoint.
func (s *SMF) transferSession(ctx context.Context, sc *SMContext, req transferRequest) error {
	// The move spans several blocking UPF calls.
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

// The session and the UE address survive (TS 23.502 §4.11.2.2 step 14,
// §4.11.2.3 step 10).
func (s *SMF) dropSourceRouting(ctx context.Context, supi etsi.SUPI, from AccessType, id SessionIdentity) {
	switch from {
	case Access5G:
		if s.amf == nil {
			return
		}

		// A UE in dual-registration mode stays on NG-RAN while it moves sessions one
		// at a time, stranding the moved session's radio resources.
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

func (s *SMF) moveSession(ctx context.Context, sc *SMContext, req transferRequest) (AccessType, SessionIdentity, error) {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	from, sourceID := sc.Access, sc.SessionIdentity

	// A release, a replacement or another transfer can land before this lock.
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

	// A T3592 expiry would abort a session the UE still holds (TS 24.501
	// §6.3.2.5, §6.3.3).
	sc.stopProcedureTimer()
	sc.pendingPolicy = nil
	sc.establishmentPTI = 0
	sc.outstandingPTIs = nil

	logger.WithTrace(ctx, logger.SmfLog).Info("moved session to the other access",
		logger.SUPI(sc.Supi.String()), logger.PDUSessionID(sc.PDUSessionID),
		zap.Stringer("from", from), zap.Stringer("to", req.Access))

	return from, sourceID, nil
}

// Unmapping the IPv4-in-IPv6 form keeps the address in the family it was
// allocated in.
func ipToNetip(ip net.IP) netip.Addr {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return netip.Addr{}
	}

	return addr.Unmap()
}
