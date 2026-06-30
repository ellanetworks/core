// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

// netipToIP adapts a netip.Addr to the net.IP the NAS/NGAP code expects.
func netipToIP(addr netip.Addr) net.IP {
	if addr.Is4() {
		b := addr.As4()
		return net.IP(b[:])
	}

	b := addr.As16()

	return net.IP(b[:])
}

// negotiatePDUSessionType resolves the PDU session type against the available
// pools: a single-stack request is honored only if its pool exists, and an
// IPv4v6 request is downgraded to whichever single stack is available.
func (s *SMF) negotiatePDUSessionType(_ context.Context, requested uint8, policy *Policy) (uint8, error) {
	hasIPv4 := policy.IPv4Pool != ""
	hasIPv6 := policy.IPv6Pool != ""

	switch requested {
	case nasMessage.PDUSessionTypeIPv4:
		if hasIPv4 {
			return nasMessage.PDUSessionTypeIPv4, nil
		}

		return 0, fmt.Errorf("no IPv4 pool available for DNN")

	case nasMessage.PDUSessionTypeIPv6:
		if hasIPv6 {
			return nasMessage.PDUSessionTypeIPv6, nil
		}

		return 0, fmt.Errorf("no IPv6 pool available for DNN")

	case nasMessage.PDUSessionTypeIPv4IPv6:
		if hasIPv4 && hasIPv6 {
			return nasMessage.PDUSessionTypeIPv4IPv6, nil
		}

		if hasIPv4 {
			return nasMessage.PDUSessionTypeIPv4, nil
		}

		if hasIPv6 {
			return nasMessage.PDUSessionTypeIPv6, nil
		}

		return 0, fmt.Errorf("no IP pool available for DNN")

	default:
		return 0, fmt.Errorf("unsupported PDU session type: %d", requested)
	}
}

// pduSessionTypeRejectCause maps a failed PDU session type negotiation
// to the 5GSM cause prescribed by TS 24.501.
//
//   - IPv6 requested, only IPv4 supported           → #50 IPv4 only allowed
//   - IPv4 requested, only IPv6 supported           → #51 IPv6 only allowed
//   - IPv4/IPv6/IPv4v6 requested, neither supported → #28 unknown PDU session type
//   - Unstructured, Ethernet, reserved values       → #28 unknown PDU session type
func pduSessionTypeRejectCause(requested uint8, policy *Policy) uint8 {
	hasIPv4 := policy.IPv4Pool != ""
	hasIPv6 := policy.IPv6Pool != ""

	switch requested {
	case nasMessage.PDUSessionTypeIPv6:
		if hasIPv4 && !hasIPv6 {
			return nasMessage.Cause5GSMPDUSessionTypeIPv4OnlyAllowed
		}
	case nasMessage.PDUSessionTypeIPv4:
		if !hasIPv4 && hasIPv6 {
			return nasMessage.Cause5GSMPDUSessionTypeIPv6OnlyAllowed
		}
	}

	return nasMessage.Cause5GSMUnknownPDUSessionType
}

type pduTypeNarrowing uint8

const (
	narrowNone pduTypeNarrowing = iota
	narrowIPv4Only
	narrowIPv6Only
)

// narrowPDUType reports whether the negotiated type narrows the UE's IPv4v6
// request to a single family, so each access can signal the matching single-stack
// cause: 5GSM #50/#51 (TS 24.501) or ESM #50/#51 (TS 24.301).
func narrowPDUType(requested, negotiated uint8) pduTypeNarrowing {
	if requested != nasMessage.PDUSessionTypeIPv4IPv6 || negotiated == nasMessage.PDUSessionTypeIPv4IPv6 {
		return narrowNone
	}

	if negotiated == nasMessage.PDUSessionTypeIPv4 {
		return narrowIPv4Only
	}

	return narrowIPv6Only
}

// allocateUEAddresses allocates the UE address(es) for sc.PDUSessionType, stores
// them on sc, and returns the downlink-PDR key (the IPv4 address, or the /64
// prefix base for IPv6-only). On failure it releases whatever it had allocated.
// The caller holds sc.Mutex.
func (s *SMF) allocateUEAddresses(ctx context.Context, sc *SMContext) (netip.Addr, ueAddresses, error) {
	imsi := sc.Supi.IMSI()

	var (
		dlPdrIP netip.Addr
		addrs   ueAddresses
	)

	if sc.PDUSessionType == nasMessage.PDUSessionTypeIPv4 || sc.PDUSessionType == nasMessage.PDUSessionTypeIPv4IPv6 {
		ipv4, err := s.store.AllocateIP(ctx, imsi, sc.Dnn, sc.PDUSessionID)
		if err != nil {
			return netip.Addr{}, ueAddresses{}, fmt.Errorf("allocate UE IPv4: %w", err)
		}

		sc.PDUIPV4Address = netipToIP(ipv4)
		addrs.IPv4 = ipv4
		dlPdrIP = ipv4
	}

	if sc.PDUSessionType == nasMessage.PDUSessionTypeIPv6 || sc.PDUSessionType == nasMessage.PDUSessionTypeIPv4IPv6 {
		ipv6Prefix, err := s.store.AllocateIPv6(ctx, imsi, sc.Dnn, sc.PDUSessionID)
		if err != nil {
			s.releaseAllocatedAddresses(ctx, sc) // rolls back the IPv4 lease if dual-stack
			return netip.Addr{}, ueAddresses{}, fmt.Errorf("allocate UE IPv6 prefix: %w", err)
		}

		sc.PDUIPV6Prefix = netipToIP(ipv6Prefix)
		addrs.IPv6Prefix = ipv6Prefix

		iid, err := s.assignIID(sc.Dnn)
		if err != nil {
			s.releaseAllocatedAddresses(ctx, sc)
			return netip.Addr{}, ueAddresses{}, fmt.Errorf("assign IPv6 IID: %w", err)
		}

		sc.IPv6IID = iid
		addrs.IPv6IID = iid

		if sc.PDUSessionType == nasMessage.PDUSessionTypeIPv6 {
			dlPdrIP = ipv6Prefix
		}
	}

	return dlPdrIP, addrs, nil
}

// releaseAllocatedAddresses releases the UE IP leases recorded on smContext and
// clears them, so a later rollback does not double-release.
func (s *SMF) releaseAllocatedAddresses(ctx context.Context, smContext *SMContext) {
	if smContext.PDUIPV4Address != nil {
		if _, err := s.store.ReleaseIP(ctx, smContext.Supi.IMSI(), smContext.Dnn, smContext.PDUSessionID); err != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to release IPv4 address", zap.Error(err))
		}

		smContext.PDUIPV4Address = nil
	}

	if smContext.PDUIPV6Prefix != nil {
		if _, err := s.store.ReleaseIPv6(ctx, smContext.Supi.IMSI(), smContext.Dnn, smContext.PDUSessionID); err != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to release IPv6 address", zap.Error(err))
		}

		smContext.PDUIPV6Prefix = nil
	}
}
