// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/internal/tester/s1enb"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/nas/eps"
)

// expectedAttach is the set of signaled default-bearer fields a successful EPS
// attach must carry, limited to what Ella Core signals over S1AP/EPS NAS.
type expectedAttach struct {
	UEIPv4Subnet        netip.Prefix // UE IPv4 must fall inside this subnet (zero value => skip)
	APN                 string       // Access Point Name in the Activate Default EPS Bearer Context Request (empty => skip)
	PDNType             uint8        // eps.PDNTypeIPv4 / IPv6 / IPv4v6 (0 => skip)
	QCI                 byte         // default bearer QCI, equals the policy 5QI (0 => skip)
	ARP                 byte         // default bearer ARP priority level, 1-15 (0 => skip)
	SessAmbrDownlinkBps uint64       // per-APN Session-AMBR downlink, bits/s (0 => skip)
	SessAmbrUplinkBps   uint64       // per-APN Session-AMBR uplink, bits/s (0 => skip)
	UEAmbrDownlinkBps   uint64       // UE-AMBR downlink, bits/s (0 => skip); attach only
	UEAmbrUplinkBps     uint64       // UE-AMBR uplink, bits/s (0 => skip); attach only
	RequireGUTI         bool         // the MME must have assigned a GUTI
	RequireUEIPv6       bool         // the attach must carry an IPv6 interface identifier
}

// familyExpect is the baseline attach expectation for env's IP family. The global
// IPv6 prefix is not asserted — EPS signals only the IID, the prefix arrives via
// SLAAC. EPS PDN type and 5G PDU-session type share values (IPv4=1/IPv6=2/
// IPv4v6=3), so env.PDUSessionType() selects the family for both.
func familyExpect(env scenarios.Env, apn, ipv4Pool string) expectedAttach {
	exp := expectedAttach{
		APN:                 apn,
		PDNType:             env.PDUSessionType(),
		QCI:                 9,
		SessAmbrDownlinkBps: 100 * mbpsToBps,
		SessAmbrUplinkBps:   100 * mbpsToBps,
		RequireGUTI:         true,
	}

	if env.IPFamily() != scenarios.IPv6Only {
		exp.UEIPv4Subnet = netip.MustParsePrefix(ipv4Pool)
	}

	if env.HasIPv6() {
		exp.RequireUEIPv6 = true
	}

	return exp
}

const mbpsToBps = 1_000_000

func defaultExpectedAttach() expectedAttach {
	return expectedAttach{
		UEIPv4Subnet:        netip.MustParsePrefix(scenarios.DefaultUEIPv4Pool),
		APN:                 scenarios.DefaultDNN,
		PDNType:             eps.PDNTypeIPv4,
		QCI:                 9,
		SessAmbrDownlinkBps: 100 * mbpsToBps,
		SessAmbrUplinkBps:   100 * mbpsToBps,
		RequireGUTI:         true,
	}
}

// Zero-valued expectation fields are skipped so callers assert only what their
// scenario provisions.
func assertAttach(res *s1enb.AttachResult, exp expectedAttach) error {
	if exp.RequireGUTI && res.GUTI == nil {
		return fmt.Errorf("attach: MME assigned no GUTI")
	}

	if exp.UEAmbrDownlinkBps != 0 && res.UEAmbrDownlinkBps != exp.UEAmbrDownlinkBps {
		return fmt.Errorf("attach: UE-AMBR downlink = %d bps, want %d", res.UEAmbrDownlinkBps, exp.UEAmbrDownlinkBps)
	}

	if exp.UEAmbrUplinkBps != 0 && res.UEAmbrUplinkBps != exp.UEAmbrUplinkBps {
		return fmt.Errorf("attach: UE-AMBR uplink = %d bps, want %d", res.UEAmbrUplinkBps, exp.UEAmbrUplinkBps)
	}

	return assertBearer(bearerFields{
		pdnType: res.PDNType, qci: res.QCI, arp: res.ARP, apn: res.APN, ueIPv4: res.UEIPv4, ueIPv6: res.UEIPv6,
		sessAmbrDLBps: res.SessAmbrDownlinkBps, sessAmbrULBps: res.SessAmbrUplinkBps,
	}, exp)
}

// A secondary PDN carries no GUTI, so exp.RequireGUTI is ignored here.
func assertPDN(pdn *s1enb.PDNResult, exp expectedAttach) error {
	return assertBearer(bearerFields{
		pdnType: pdn.PDNType, qci: pdn.QCI, arp: pdn.ARP, apn: pdn.APN, ueIPv4: pdn.UEIPv4,
		sessAmbrDLBps: pdn.SessAmbrDownlinkBps, sessAmbrULBps: pdn.SessAmbrUplinkBps,
	}, exp)
}

type bearerFields struct {
	pdnType                      uint8
	qci                          byte
	arp                          byte
	apn                          string
	ueIPv4                       string
	ueIPv6                       string
	sessAmbrDLBps, sessAmbrULBps uint64
}

func assertBearer(b bearerFields, exp expectedAttach) error {
	if exp.PDNType != 0 && b.pdnType != exp.PDNType {
		return fmt.Errorf("bearer: PDN type = %d, want %d", b.pdnType, exp.PDNType)
	}

	if exp.QCI != 0 && b.qci != exp.QCI {
		return fmt.Errorf("bearer: QCI = %d, want %d", b.qci, exp.QCI)
	}

	if exp.ARP != 0 && b.arp != exp.ARP {
		return fmt.Errorf("bearer: ARP = %d, want %d", b.arp, exp.ARP)
	}

	if exp.APN != "" && b.apn != exp.APN {
		return fmt.Errorf("bearer: APN = %q, want %q", b.apn, exp.APN)
	}

	if exp.SessAmbrDownlinkBps != 0 && b.sessAmbrDLBps != exp.SessAmbrDownlinkBps {
		return fmt.Errorf("bearer: session-AMBR downlink = %d bps, want %d", b.sessAmbrDLBps, exp.SessAmbrDownlinkBps)
	}

	if exp.SessAmbrUplinkBps != 0 && b.sessAmbrULBps != exp.SessAmbrUplinkBps {
		return fmt.Errorf("bearer: session-AMBR uplink = %d bps, want %d", b.sessAmbrULBps, exp.SessAmbrUplinkBps)
	}

	if exp.RequireUEIPv6 && b.ueIPv6 == "" {
		return fmt.Errorf("bearer: no IPv6 interface identifier assigned")
	}

	if exp.UEIPv4Subnet.IsValid() {
		if b.ueIPv4 == "" {
			return fmt.Errorf("bearer: no IPv4 assigned (want one in %s)", exp.UEIPv4Subnet)
		}

		addr, err := netip.ParseAddr(b.ueIPv4)
		if err != nil {
			return fmt.Errorf("bearer: parse UE IPv4 %q: %w", b.ueIPv4, err)
		}

		if !exp.UEIPv4Subnet.Contains(addr) {
			return fmt.Errorf("bearer: UE IPv4 %s not in %s", b.ueIPv4, exp.UEIPv4Subnet)
		}
	}

	return nil
}
