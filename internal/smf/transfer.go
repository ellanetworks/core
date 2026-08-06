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
	"go.uber.org/zap"
)

// ErrSessionNotTransferable reports that the session a UE asked to move to the
// access it is now on does not exist, or exists but is not the one the UE
// described. Each access maps it to the cause its NAS defines for a transfer of
// something the network does not hold: ESM #54 "PDN connection does not exist"
// (TS 24.301 §6.5.1.4 b) and 5GSM #54 "PDU session does not exist"
// (TS 24.501 §6.4.1.4 d).
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
	EBI    uint8
	Dnn    string
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

	sc := s.currentPDUSession(supi, pduSessionID)
	if sc == nil {
		return nil, fmt.Errorf("%w: no session with PDU session id %d", ErrSessionNotTransferable, pduSessionID)
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if sc.Access == req.Access {
		return nil, fmt.Errorf("%w: PDU session %d is already on %s", ErrSessionNotTransferable, pduSessionID, req.Access)
	}

	if sc.Dnn != req.Dnn {
		return nil, fmt.Errorf("%w: PDU session %d is on data network %q, not %q", ErrSessionNotTransferable, pduSessionID, sc.Dnn, req.Dnn)
	}

	if sc.Tunnel == nil || sc.Tunnel.DataPath == nil || sc.PFCPContext == nil {
		return nil, fmt.Errorf("%w: PDU session %d has no user plane to move", ErrSessionNotTransferable, pduSessionID)
	}

	return sc, nil
}

// transferSession moves an established session to the other access with session
// continuity (TS 23.501 §5.17.2): the UE keeps its address, and the anchor keeps
// the UPF session, its SEID and its uplink F-TEID, so only the access-network
// end of the tunnel is rebuilt. The downlink is left buffering — the target
// access binds its own RAN endpoint afterwards (ModifyEPSSession on EPS, the N2
// resource setup response on 5GS), and that is what re-points the downlink FAR
// and sets the PDU Session Container behaviour for the new access.
func (s *SMF) transferSession(ctx context.Context, sc *SMContext, req transferRequest) error {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	from := sc.Access

	policy := req.Policy
	if len(policy.NetworkRules) == 0 && sc.PolicyData != nil {
		// Network rules are subscriber and policy scoped, not access scoped, and
		// the EPS resolution path does not produce them.
		policy.NetworkRules = sc.PolicyData.NetworkRules
	}

	// Session-AMBR and the QFI the user plane is marked with are resolved per
	// access, so the target's policy has to reach the UPF before its RAN does.
	if err := s.applySessionQERs(ctx, sc, policy.PolicyID, policy.QosData.QFI, policy.Ambr.Uplink, policy.Ambr.Downlink); err != nil {
		return fmt.Errorf("apply target access QoS: %w", err)
	}

	farList, err := handleUpCnxStateDeactivate(sc)
	if err != nil {
		return fmt.Errorf("suspend downlink: %w", err)
	}

	// Buffer without a downlink data notification: the UE is not idle, it is on
	// the other access, and paging the one it left would reach nobody.
	sc.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ApplyAction.Nocp = false

	if err := s.upf.ModifySession(ctx, BuildModifyRequest(sc.PFCPContext.RemoteSEID, policy.PolicyID, nil, farList, nil)); err != nil {
		return fmt.Errorf("suspend downlink in the UPF: %w", err)
	}

	sc.Access = req.Access
	sc.PolicyData = policy

	s.setEPSBearerIdentity(sc, req.EBI)

	logger.WithTrace(ctx, logger.SmfLog).Info("moved session to the other access",
		logger.SUPI(sc.Supi.String()), logger.PDUSessionID(sc.PDUSessionID),
		zap.Stringer("from", from), zap.Stringer("to", req.Access))

	return nil
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
