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

// expectedAttach describes the signaled default-bearer fields a successful EPS
// attach must carry. It is the 4G counterpart of the 5G
// validate.ExpectedPDUSessionEstablishmentAccept, limited to the fields Ella
// Core actually signals over S1AP/EPS NAS.
//
// It mirrors the 5G validator's coverage: IP-in-subnet, APN/DNN, PDN type, QCI,
// and the per-APN Session-AMBR (signaled via the APN-AMBR IE, TS 24.301
// §9.9.4.2 — the 4G analogue of the 5G Session-AMBR).
type expectedAttach struct {
	UEIPv4Subnet        netip.Prefix // UE IPv4 must fall inside this subnet (zero value => skip)
	APN                 string       // Access Point Name in the Activate Default EPS Bearer Context Request (empty => skip)
	PDNType             uint8        // eps.PDNTypeIPv4 / IPv6 / IPv4v6 (0 => skip)
	QCI                 byte         // default bearer QCI, equals the policy 5QI (0 => skip)
	SessAmbrDownlinkBps uint64       // per-APN Session-AMBR downlink, bits/s (0 => skip)
	SessAmbrUplinkBps   uint64       // per-APN Session-AMBR uplink, bits/s (0 => skip)
	RequireGUTI         bool         // the MME must have assigned a GUTI
}

// mbpsToBps converts whole megabits/s to bits/s.
const mbpsToBps = 1_000_000

// defaultExpectedAttach is the baseline expectation for a UE attaching on the
// default profile/policy/data-network: an IPv4 lease from the default pool, the
// default APN, IPv4 PDN type, the default policy's QCI (9), and the default
// policy's Session-AMBR (100/100 Mbps).
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

// assertAttach checks an EPS attach's AttachResult against exp, returning a
// descriptive error on the first mismatch. Zero-valued expectation fields are
// skipped so callers assert only what their scenario provisions.
func assertAttach(res *s1enb.AttachResult, exp expectedAttach) error {
	if exp.RequireGUTI && res.GUTI == nil {
		return fmt.Errorf("attach: MME assigned no GUTI")
	}

	return assertBearer(bearerFields{
		pdnType: res.PDNType, qci: res.QCI, apn: res.APN, ueIPv4: res.UEIPv4,
		sessAmbrDLBps: res.SessAmbrDownlinkBps, sessAmbrULBps: res.SessAmbrUplinkBps,
	}, exp)
}

// assertPDN checks an additional PDN connection's PDNResult against exp. A
// secondary PDN carries no GUTI, so exp.RequireGUTI is ignored here.
func assertPDN(pdn *s1enb.PDNResult, exp expectedAttach) error {
	return assertBearer(bearerFields{
		pdnType: pdn.PDNType, qci: pdn.QCI, apn: pdn.APN, ueIPv4: pdn.UEIPv4,
		sessAmbrDLBps: pdn.SessAmbrDownlinkBps, sessAmbrULBps: pdn.SessAmbrUplinkBps,
	}, exp)
}

// bearerFields are the signaled default-bearer fields common to an attach and an
// additional PDN connection.
type bearerFields struct {
	pdnType                      uint8
	qci                          byte
	apn                          string
	ueIPv4                       string
	sessAmbrDLBps, sessAmbrULBps uint64
}

// assertBearer checks bearer fields against exp; zero-valued expectations skip.
func assertBearer(b bearerFields, exp expectedAttach) error {
	if exp.PDNType != 0 && b.pdnType != exp.PDNType {
		return fmt.Errorf("bearer: PDN type = %d, want %d", b.pdnType, exp.PDNType)
	}

	if exp.QCI != 0 && b.qci != exp.QCI {
		return fmt.Errorf("bearer: QCI = %d, want %d", b.qci, exp.QCI)
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
