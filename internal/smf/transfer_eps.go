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
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func (s *SMF) transferToEPS(ctx context.Context, supi etsi.SUPI, req models.EPSBearerRequest, policy *Policy) (models.EPSBearer, error) {
	move := transferRequest{
		Access: Access4G,
		EBI:    req.EPSBearerIdentity,
		Dnn:    req.APN,
		Snssai: req.Snssai,
		Policy: policy,
	}

	sc, err := s.findTransferable(supi, req.PDUSessionID, move)
	if err != nil {
		return models.EPSBearer{}, err
	}

	// An attach from an already-attached UE deletes its existing EPS bearer
	// contexts (TS 24.301 §5.5.1.2.7 f). Left in place, the stale session holds
	// the bearer identity and every retry draws the same refusal.
	if held := s.currentEPSSession(supi, req.EPSBearerIdentity); held != nil && held != sc {
		s.handlePduSessionContextReplacement(ctx, held, Access4G)
	}

	if err := s.prepareTransfer(sc, move); err != nil {
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

func epsBearerForSession(sc *SMContext, req models.EPSBearerRequest) (models.EPSBearer, error) {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if sc.Tunnel == nil {
		return models.EPSBearer{}, fmt.Errorf("%w: PDU session %d has no tunnel to report", ErrSessionNotMovable, req.PDUSessionID)
	}

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

	if dns := net.ParseIP(req.DNS); dns != nil {
		if addr, ok := netip.AddrFromSlice(dns); ok {
			bearer.DNS = addr.Unmap()
		}
	}

	return bearer, nil
}

// The enumerations diverge above IPv4v6 (TS 24.301 §9.9.4.10 vs TS 24.501
// §9.11.4.11), so anything else is refused rather than cast.
// The mirror of pdnTypeFor, applied where an EPS PDN type enters the SMF so
// everything downstream speaks one enumeration.
func pduSessionTypeFor(pdnType uint8) (uint8, error) {
	switch eps.PDNType(pdnType) {
	case eps.PDNTypeIPv4:
		return uint8(fgs.PDUSessionTypeIPv4), nil
	case eps.PDNTypeIPv6:
		return uint8(fgs.PDUSessionTypeIPv6), nil
	case eps.PDNTypeIPv4v6:
		return uint8(fgs.PDUSessionTypeIPv4v6), nil
	case eps.PDNTypeUnusedIPv6:
		return uint8(fgs.PDUSessionTypeIPv6), nil
	default:
		return 0, fmt.Errorf("PDN type %d has no PDU session type", pdnType)
	}
}

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

// The EPS mirror of pduSessionTypeRejectCause: the anchor holds the pools, so it
// is what can tell an unusable type (#28) from one narrowed to a single family
// (#50, #51) (TS 24.301 §6.5.1.4.1). requested is in the 5GS enumeration.
func pdnTypeRejectCause(requested uint8, policy *Policy) eps.ESMCause {
	hasIPv4 := policy.IPv4Pool != ""
	hasIPv6 := policy.IPv6Pool != ""

	if !hasIPv4 && !hasIPv6 {
		return eps.ESMCauseInsufficientResources
	}

	switch requested {
	case uint8(fgs.PDUSessionTypeIPv6):
		if hasIPv4 && !hasIPv6 {
			return eps.ESMCausePDNTypeIPv4OnlyAllowed
		}
	case uint8(fgs.PDUSessionTypeIPv4):
		if !hasIPv4 && hasIPv6 {
			return eps.ESMCausePDNTypeIPv6OnlyAllowed
		}
	}

	return eps.ESMCauseUnknownPDNType
}
