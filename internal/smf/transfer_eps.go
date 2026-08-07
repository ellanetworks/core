// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// transferToEPS moves an existing PDU session onto EPS in answer to a PDN
// CONNECTIVITY REQUEST with request type "handover" (TS 24.301 §6.5.1.2,
// TS 23.502 §4.11.2.2). The UE keeps its address because the anchor keeps the
// session: nothing is established, the existing one changes access.
//
// The answer is built out of what a move preserves — the UE address, the
// negotiated PDN type, the DNS and the uplink F-TEID — so the UE sees the
// connection it already had. The downlink stays on 5GS until the eNB's S1-U
// endpoint arrives (ModifyEPSSession), which is where the move commits.
func (s *SMF) transferToEPS(ctx context.Context, supi etsi.SUPI, req models.EPSBearerRequest, policy *Policy) (models.EPSBearer, error) {
	sc, err := s.findTransferable(supi, req.PDUSessionID, transferRequest{
		Access: Access4G,
		EBI:    req.EPSBearerIdentity,
		Dnn:    req.APN,
		Policy: policy,
	})
	if err != nil {
		return models.EPSBearer{}, err
	}

	if err := s.prepareTransfer(sc, transferRequest{
		Access: Access4G,
		EBI:    req.EPSBearerIdentity,
		Dnn:    req.APN,
		Policy: policy,
	}); err != nil {
		return models.EPSBearer{}, err
	}

	bearer, err := epsBearerForSession(sc, req)
	if err != nil {
		sc.abandonTransfer()

		return models.EPSBearer{}, err
	}

	logger.WithTrace(ctx, logger.SmfLog).Info("moving a PDU session to EPS",
		logger.SUPI(supi.String()), logger.PDUSessionID(req.PDUSessionID),
		zap.Uint8("ebi", req.EPSBearerIdentity), zap.String("apn", req.APN))

	return bearer, nil
}

// epsBearerForSession renders the anchored session as the PDN connection the
// MME will describe to the UE. Caller must not hold sc.Mutex.
func epsBearerForSession(sc *SMContext, req models.EPSBearerRequest) (models.EPSBearer, error) {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if sc.Tunnel == nil {
		return models.EPSBearer{}, fmt.Errorf("%w: PDU session %d has no tunnel to report", ErrSessionNotMovable, req.PDUSessionID)
	}

	// The PDU session type is the session's, negotiated when it was established;
	// a move does not renegotiate it. The enumerations diverge above IPv4v6, so
	// the translation refuses what it cannot carry rather than mistranslating.
	pdnType, err := pdnTypeFor(sc.PDUSessionType)
	if err != nil {
		return models.EPSBearer{}, fmt.Errorf("%w: %v", ErrSessionNotMovable, err)
	}

	bearer := models.EPSBearer{
		Ref:          sc.Ref,
		PDNType:      pdnType,
		PDUSessionID: sc.PDUSessionID,
		Snssai:       sc.Snssai,
		SGW:          models.FTEID{TEID: sc.Tunnel.N3TEID, Addr: sc.Tunnel.N3IPv4},
		SGWN3IPv6:    sc.Tunnel.N3IPv6,
		IPv6IID:      sc.IPv6IID,
	}

	if sc.PDUIPV4Address != nil {
		if addr, ok := netip.AddrFromSlice(sc.PDUIPV4Address); ok {
			bearer.IPv4 = addr.Unmap()
		}
	}

	if sc.PDUIPV6Prefix != nil {
		if addr, ok := netip.AddrFromSlice(sc.PDUIPV6Prefix); ok {
			bearer.IPv6Prefix = addr
		}
	}

	// The DNS the UE is told is the one the target access's policy carries, since
	// the data network is the same and its resolver may have been reconfigured
	// since the session was established.
	if dns := net.ParseIP(req.DNS); dns != nil {
		if addr, ok := netip.AddrFromSlice(dns); ok {
			bearer.DNS = addr.Unmap()
		}
	}

	return bearer, nil
}

// AbandonEPSTransfer drops a move to EPS the MME could not carry through — it
// failed to build or protect the ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST, so
// the UE will never bind the bearer. The session stays on the access serving it.
func (s *SMF) AbandonEPSTransfer(_ context.Context, ref string) {
	sc := s.GetSession(ref)
	if sc == nil {
		return
	}

	sc.abandonTransfer()
}

// pdnTypeFor maps a negotiated 5GS PDU session type to the EPS PDN type. The
// enumerations agree on IPv4/IPv6/IPv4v6 and diverge above them (TS 24.301
// §9.9.4.10 vs TS 24.501 §9.11.4.11), so anything else is refused rather than
// cast: an Ethernet PDU session silently becoming "non IP" would hand the UE a
// connection it cannot use.
func pdnTypeFor(pduSessionType uint8) (eps.PDNType, error) {
	switch pduSessionType {
	case 1:
		return eps.PDNTypeIPv4, nil
	case 2:
		return eps.PDNTypeIPv6, nil
	case 3:
		return eps.PDNTypeIPv4v6, nil
	default:
		return 0, fmt.Errorf("PDU session type %d has no EPS PDN type", pduSessionType)
	}
}
