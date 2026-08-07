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

func (s *SMF) transferToEPS(ctx context.Context, supi etsi.SUPI, req models.EPSBearerRequest, policy *Policy) (models.EPSBearer, error) {
	move := transferRequest{
		Access: Access4G,
		EBI:    req.EPSBearerIdentity,
		Dnn:    req.APN,
		Policy: policy,
	}

	sc, err := s.findTransferable(supi, req.PDUSessionID, move)
	if err != nil {
		return models.EPSBearer{}, err
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

func (s *SMF) AbandonEPSTransfer(_ context.Context, ref string) {
	sc := s.GetSession(ref)
	if sc == nil {
		return
	}

	sc.abandonTransfer()
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
