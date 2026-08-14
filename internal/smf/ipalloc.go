// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func netipToIP(addr netip.Addr) net.IP {
	if addr.Is4() {
		b := addr.As4()
		return net.IP(b[:])
	}

	b := addr.As16()

	return net.IP(b[:])
}

func normalisePDUSessionType(requested uint8) uint8 {
	switch requested {
	case 0, 6:
		return uint8(fgs.PDUSessionTypeIPv4v6)
	default:
		return requested
	}
}

func (s *SMF) negotiatePDUSessionType(_ context.Context, requested uint8, policy *Policy) (uint8, error) {
	hasIPv4 := policy.IPv4Pool != ""
	hasIPv6 := policy.IPv6Pool != ""

	switch requested {
	case uint8(fgs.PDUSessionTypeIPv4):
		if hasIPv4 {
			return uint8(fgs.PDUSessionTypeIPv4), nil
		}

		return 0, fmt.Errorf("no IPv4 pool available for DNN")

	case uint8(fgs.PDUSessionTypeIPv6):
		if hasIPv6 {
			return uint8(fgs.PDUSessionTypeIPv6), nil
		}

		return 0, fmt.Errorf("no IPv6 pool available for DNN")

	case uint8(fgs.PDUSessionTypeIPv4v6):
		if hasIPv4 && hasIPv6 {
			return uint8(fgs.PDUSessionTypeIPv4v6), nil
		}

		if hasIPv4 {
			return uint8(fgs.PDUSessionTypeIPv4), nil
		}

		if hasIPv6 {
			return uint8(fgs.PDUSessionTypeIPv6), nil
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
func pduSessionTypeRejectCause(requested uint8, policy *Policy) fgs.GSMCause {
	hasIPv4 := policy.IPv4Pool != ""
	hasIPv6 := policy.IPv6Pool != ""

	switch requested {
	case uint8(fgs.PDUSessionTypeIPv6):
		if hasIPv4 && !hasIPv6 {
			return fgs.GSMCausePDUSessionTypeIPv4OnlyAllowed
		}
	case uint8(fgs.PDUSessionTypeIPv4):
		if !hasIPv4 && hasIPv6 {
			return fgs.GSMCausePDUSessionTypeIPv6OnlyAllowed
		}
	}

	return fgs.GSMCauseUnknownPDUSessionType
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
	if fgs.PDUSessionType(requested) != fgs.PDUSessionTypeIPv4v6 || fgs.PDUSessionType(negotiated) == fgs.PDUSessionTypeIPv4v6 {
		return narrowNone
	}

	if fgs.PDUSessionType(negotiated) == fgs.PDUSessionTypeIPv4 {
		return narrowIPv4Only
	}

	return narrowIPv6Only
}

// allocateUEAddresses allocates the UE address(es) for sc.PDUSessionType and
// stores them on sc. On failure it releases whatever it had allocated. The
// caller holds sc.Mutex.
func (s *SMF) allocateUEAddresses(ctx context.Context, dn DNNStore, sc *SMContext) (ueAddresses, error) {
	imsi := sc.Supi.IMSI()

	var addrs ueAddresses

	if fgs.PDUSessionType(sc.PDUSessionType) == fgs.PDUSessionTypeIPv4 || fgs.PDUSessionType(sc.PDUSessionType) == fgs.PDUSessionTypeIPv4v6 {
		ipv4, err := dn.AllocateIP(ctx, imsi, sc.sessionKey())
		if err != nil {
			return ueAddresses{}, fmt.Errorf("allocate UE IPv4: %w", err)
		}

		sc.PDUIPV4Address = netipToIP(ipv4)
		addrs.IPv4 = ipv4
	}

	if fgs.PDUSessionType(sc.PDUSessionType) == fgs.PDUSessionTypeIPv6 || fgs.PDUSessionType(sc.PDUSessionType) == fgs.PDUSessionTypeIPv4v6 {
		ipv6Prefix, err := dn.AllocateIPv6(ctx, imsi, sc.sessionKey())
		if err != nil {
			s.releaseAllocatedAddresses(ctx, dn, sc)
			return ueAddresses{}, fmt.Errorf("allocate UE IPv6 prefix: %w", err)
		}

		sc.PDUIPV6Prefix = netipToIP(ipv6Prefix)
		addrs.IPv6Prefix = ipv6Prefix

		iid, err := GenerateIID()
		if err != nil {
			s.releaseAllocatedAddresses(ctx, dn, sc)
			return ueAddresses{}, fmt.Errorf("assign IPv6 IID: %w", err)
		}

		sc.IPv6IID = iid
		addrs.IPv6IID = iid
	}

	return addrs, nil
}

// releaseAllocatedAddresses releases the UE IP leases recorded on smContext and
// clears them, so a later rollback does not double-release.
func (s *SMF) releaseAllocatedAddresses(ctx context.Context, dn DNNStore, smContext *SMContext) {
	if smContext.PDUIPV4Address != nil {
		if _, err := dn.ReleaseIP(ctx, smContext.Supi.IMSI(), smContext.sessionKey()); err != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to release IPv4 address", zap.Error(err))
		}

		smContext.PDUIPV4Address = nil
	}

	if smContext.PDUIPV6Prefix != nil {
		if _, err := dn.ReleaseIPv6(ctx, smContext.Supi.IMSI(), smContext.sessionKey()); err != nil {
			logger.WithTrace(ctx, logger.SmfLog).Error("failed to release IPv6 address", zap.Error(err))
		}

		smContext.PDUIPV6Prefix = nil
	}
}
