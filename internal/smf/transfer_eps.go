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

	if held := s.currentEPSSession(supi, req.EPSBearerIdentity); held != nil && held != sc {
		s.handlePduSessionContextReplacement(ctx, held, Access4G)
	}

	if err := s.prepareTransfer(sc, move); err != nil {
		return models.EPSBearer{}, err
	}

	bearer, err := epsBearerForSession(sc, req.DNS)
	if err != nil {
		sc.abandonTransferTo(Access4G)

		return models.EPSBearer{}, err
	}

	logger.WithTrace(ctx, logger.SmfLog).Info("moving a PDU session to EPS",
		logger.SUPI(supi.String()), logger.PDUSessionID(req.PDUSessionID),
		zap.Uint8("ebi", req.EPSBearerIdentity), zap.String("apn", req.APN))

	return bearer, nil
}

func sessionDNS(sc *SMContext) string {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if sc.PolicyData == nil || sc.PolicyData.DNS == nil {
		return ""
	}

	return sc.PolicyData.DNS.String()
}

func epsBearerForSession(sc *SMContext, dns string) (models.EPSBearer, error) {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if sc.Tunnel == nil {
		return models.EPSBearer{}, fmt.Errorf("%w: PDU session %d has no tunnel to report", ErrSessionNotMovable, sc.PDUSessionID)
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

	if dns := net.ParseIP(dns); dns != nil {
		if addr, ok := netip.AddrFromSlice(dns); ok {
			bearer.DNS = addr.Unmap()
		}
	}

	return bearer, nil
}

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

func allowedPDNTypeCause(policy *Policy) eps.ESMCause {
	hasIPv4 := policy.IPv4Pool != ""
	hasIPv6 := policy.IPv6Pool != ""

	switch {
	case hasIPv4 && !hasIPv6:
		return eps.ESMCausePDNTypeIPv4OnlyAllowed
	case !hasIPv4 && hasIPv6:
		return eps.ESMCausePDNTypeIPv6OnlyAllowed
	case !hasIPv4 && !hasIPv6:
		return eps.ESMCauseInsufficientResources
	default:
		return eps.ESMCausePDNTypeIPv4v6OnlyAllowed
	}
}

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
