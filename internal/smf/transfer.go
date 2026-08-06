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
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

var (
	// ErrSessionNotTransferable reports that no session answers the identity the
	// UE named. Each access maps it to #54.
	ErrSessionNotTransferable = errors.New("session does not exist on the other access")

	// ErrSessionOnOtherDNN reports a session that exists on a different data
	// network from the one the UE named.
	ErrSessionOnOtherDNN = errors.New("session is on another data network")

	// ErrSessionNotMovable reports a session that exists and matches, but cannot
	// move as the request describes.
	ErrSessionNotMovable = errors.New("session cannot move as described")
)

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

	// A context evicted from the pool can still hold a tunnel when its teardown
	// failed, so membership is checked rather than inferred from its rules.
	if s.GetSession(sc.Ref) != sc {
		return nil, fmt.Errorf("%w: PDU session %d is not in the pool", ErrSessionNotMovable, pduSessionID)
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
	// Telling the UE #54 for a session the network holds invites it to discard
	// local state for a live session (TS 24.501 annex B), so the cases where the
	// session exists carry their own errors.
	if sc.Access == req.Access {
		return fmt.Errorf("%w: PDU session %d is already on %s", ErrSessionNotMovable, sc.PDUSessionID, req.Access)
	}

	if sc.Dnn != req.Dnn {
		return fmt.Errorf("%w: PDU session %d is on data network %q, not %q", ErrSessionOnOtherDNN, sc.PDUSessionID, sc.Dnn, req.Dnn)
	}

	// The only slice boundary on the transfer path, so an absent S-NSSAI on either
	// side fails closed.
	if (sc.Snssai == nil) != (req.Snssai == nil) {
		return fmt.Errorf("%w: PDU session %d cannot have its slice compared", ErrSessionNotMovable, sc.PDUSessionID)
	}

	if sc.Snssai != nil && !sc.Snssai.Equal(*req.Snssai) {
		return fmt.Errorf("%w: PDU session %d is on slice %+v, not %+v", ErrSessionNotMovable, sc.PDUSessionID, sc.Snssai, req.Snssai)
	}

	if sc.releasing {
		return fmt.Errorf("%w: PDU session %d is being released", ErrSessionNotMovable, sc.PDUSessionID)
	}

	if sc.Tunnel == nil || sc.Tunnel.DataPath == nil || sc.Tunnel.DataPath.DownLinkTunnel == nil ||
		sc.Tunnel.DataPath.DownLinkTunnel.PDR == nil || sc.PFCPContext == nil {
		return fmt.Errorf("%w: PDU session %d has no user plane to move", ErrSessionNotMovable, sc.PDUSessionID)
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

	// The source access keeps routing until the target binds its downlink
	// (TS 23.502 §4.11.2.2 step 14 follows step 13; §4.11.2.3 step 10 follows the
	// user plane switch of step 9). A target that never binds releases the
	// session, and that release drains this too.
	sc.Mutex.Lock()
	sc.pendingSourceRelease = &sourceRelease{from: from, id: sourceID}
	sc.Mutex.Unlock()

	return nil
}

// sourceRelease is the access a transfer moved a session off, and the identity
// it held there.
type sourceRelease struct {
	from AccessType
	id   SessionIdentity
}

// takeSourceRelease claims the outstanding source release, if any, so it runs
// once however the transfer concludes.
func (sc *SMContext) takeSourceRelease() *sourceRelease {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	pending := sc.pendingSourceRelease
	sc.pendingSourceRelease = nil

	return pending
}

// releaseTransferSource drops the routing the session left behind. Caller must
// not hold sc.Mutex.
func (s *SMF) releaseTransferSource(ctx context.Context, sc *SMContext) {
	pending := sc.takeSourceRelease()
	if pending == nil {
		return
	}

	s.dropSourceRouting(ctx, sc.Supi, sc.Ref, pending.from, pending.id)
}

// transferESMCause maps a refused transfer to the ESM cause the MME rejects with.
func transferESMCause(err error) eps.ESMCause {
	switch {
	case errors.Is(err, ErrSessionOnOtherDNN):
		return eps.ESMCauseMissingOrUnknownAPN
	case errors.Is(err, ErrSessionNotMovable):
		return eps.ESMCauseRequestRejectedUnspecified
	default:
		return eps.ESMCausePDNConnectionDoesNotExist
	}
}

// transfer5GSMCause maps a refused transfer to the 5GSM cause the AMF rejects with.
func transfer5GSMCause(err error) fgs.GSMCause {
	switch {
	case errors.Is(err, ErrSessionOnOtherDNN):
		return fgs.GSMCauseMissingOrUnknownDNN
	case errors.Is(err, ErrSessionNotMovable):
		return fgs.GSMCauseRequestRejectedUnspecified
	default:
		return fgs.GSMCausePDUSessionDoesNotExist
	}
}

// The session and the UE address survive (TS 23.502 §4.11.2.2 step 14,
// §4.11.2.3 step 10).
func (s *SMF) dropSourceRouting(ctx context.Context, supi etsi.SUPI, ref string, from AccessType, id SessionIdentity) {
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

		s.amf.SessionTransferred(ctx, supi, id.PDUSessionID, ref, n2Release)
	case Access4G:
		if s.mme != nil {
			s.mme.SessionTransferred(ctx, supi.IMSI(), id.EBI, ref)
		}
	}
}

// A failure leaves the session's EPS bearer identity out of step with the access
// it is on. Caller holds sc.Mutex.
func (s *SMF) restoreEPSBearerIdentity(ctx context.Context, sc *SMContext, ebi uint8) {
	if err := s.setEPSBearerIdentity(sc, ebi); err != nil {
		logger.WithTrace(ctx, logger.SmfLog).Error("failed to set the EPS bearer identity of a moved session",
			zap.Error(err), logger.SUPI(sc.Supi.String()),
			logger.PDUSessionID(sc.PDUSessionID), zap.Uint8("ebi", ebi))
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

	// Claiming the target access's bearer identity can fail, so it precedes the
	// UPF change and is undone on abort. Giving up the source one cannot be undone
	// reliably — another session may claim it meanwhile — so it follows.
	if req.EBI != 0 {
		if err := s.setEPSBearerIdentity(sc, req.EBI); err != nil {
			return from, sourceID, err
		}
	}

	policy := req.Policy
	if len(policy.NetworkRules) == 0 && sc.PolicyData != nil {
		// Network rules are subscriber and policy scoped, not access scoped.
		policy.NetworkRules = sc.PolicyData.NetworkRules
	}

	// The QoS change and the downlink suspend travel in one PFCP modification, so
	// a rejected transfer cannot leave the UPF holding half of it.
	qerList, restoreQERs, err := stageSessionQERs(sc, policy.QosData.QFI, policy.Ambr.Uplink, policy.Ambr.Downlink)
	if err != nil {
		if req.EBI != 0 {
			s.restoreEPSBearerIdentity(ctx, sc, sourceID.EBI)
		}

		return from, sourceID, fmt.Errorf("stage target access QoS: %w", err)
	}

	dl := sc.Tunnel.DataPath.DownLinkTunnel.PDR
	restoreDownlink := downlinkSnapshot(dl)

	farList, err := handleUpCnxStateDeactivate(sc)
	if err != nil {
		restoreDownlink()
		restoreQERs()

		if req.EBI != 0 {
			s.restoreEPSBearerIdentity(ctx, sc, sourceID.EBI)
		}

		return from, sourceID, fmt.Errorf("suspend downlink: %w", err)
	}

	// The UE is not idle, so paging the access it left reaches nobody.
	dl.FAR.ApplyAction.Nocp = false

	if err := s.upf.ModifySession(ctx, BuildModifyRequest(sc.PFCPContext.RemoteSEID, policy.PolicyID, nil, farList, qerList)); err != nil {
		restoreDownlink()
		restoreQERs()

		if req.EBI != 0 {
			s.restoreEPSBearerIdentity(ctx, sc, sourceID.EBI)
		}

		return from, sourceID, fmt.Errorf("move session in the UPF: %w", err)
	}

	if req.EBI == 0 {
		s.restoreEPSBearerIdentity(ctx, sc, 0)
	}

	sc.Access = req.Access
	sc.PolicyData = policy

	if req.Snssai != nil {
		sc.Snssai = req.Snssai
	}

	// A T3591 expiry would apply a modification to a session the UE now holds on
	// the other access (TS 24.501 §6.3.2.5 a).
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
