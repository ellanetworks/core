// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"fmt"
	"strconv"
	"strings"
)

// A bitrate reaches the radio over a different IE depending on whether it caps
// a session or a UE, and each IE has its own range. Values outside it are
// rejected on the way in, where the operator can act on the reason: the EPS
// encoders clamp Session-AMBR and fail outright on UE-AMBR, neither of which
// surfaces at session establishment.
const (
	// Session-AMBR travels in NAS. EPS scales it in kbps and runs out of units
	// above 10 Gbps (TS 24.008 §10.5.6.5B); 5G carries a unit octet and a
	// 2-octet value (TS 24.501 §9.11.4.14), so the value itself must fit.
	MaxSessionAmbrBpsFor4G   = 10_000_000_000
	MaxSessionAmbrValueFor5G = 65535

	// UE-AMBR never travels in NAS: it is an ASN.1 integer of bits/s in S1AP
	// (BitRate ::= INTEGER (0..10000000000), TS 36.413) and NGAP
	// (BitRate ::= INTEGER (0..4000000000000, ...), TS 38.413). Exceeding the
	// S1AP range is fatal to the whole InitialContextSetupRequest, so the UE
	// fails to attach. TS 36.413 carries higher rates in the
	// UEAggregate-MaximumBitrates-ExtIEs extension, which Ella Core does not
	// implement.
	MaxUeAmbrBpsFor4G = 10_000_000_000
	MaxUeAmbrBpsFor5G = 4_000_000_000_000
)

// bitrateToBps resolves a "<value> <unit>" bitrate to bits per second, or 0 if
// it is not one isValidBitrate would accept.
func bitrateToBps(bitrate string) uint64 {
	s := strings.Split(bitrate, " ")
	if len(s) != 2 {
		return 0
	}

	n, err := strconv.ParseUint(s[0], 10, 64)
	if err != nil {
		return 0
	}

	switch s[1] {
	case "Kbps":
		return n * 1_000
	case "Mbps":
		return n * 1_000_000
	case "Gbps":
		return n * 1_000_000_000
	default:
		return 0
	}
}

// checkSessionAmbrEncodable reports whether every RAT the profile permits can
// carry the Session-AMBR.
func checkSessionAmbrEncodable(allow4G, allow5G bool, label, bitrate string) error {
	if allow4G {
		if bps := bitrateToBps(bitrate); bps > MaxSessionAmbrBpsFor4G {
			return fmt.Errorf("%s of %s exceeds the 10 Gbps ceiling of a profile that allows 4G (TS 24.008 §10.5.6.5B)", label, bitrate)
		}
	}

	if allow5G {
		s := strings.Split(bitrate, " ")
		if len(s) == 2 {
			if n, err := strconv.ParseUint(s[0], 10, 64); err == nil && n > MaxSessionAmbrValueFor5G {
				return fmt.Errorf("%s of %s exceeds the largest value 5G can encode (%d %s); use a larger unit", label, bitrate, MaxSessionAmbrValueFor5G, s[1])
			}
		}
	}

	return nil
}

// checkUeAmbrEncodable reports whether every RAT the profile permits can carry
// the UE-AMBR.
func checkUeAmbrEncodable(allow4G, allow5G bool, label, bitrate string) error {
	bps := bitrateToBps(bitrate)

	if allow4G && bps > MaxUeAmbrBpsFor4G {
		return fmt.Errorf("%s of %s exceeds the 10 Gbps ceiling of a profile that allows 4G (TS 36.413 BitRate)", label, bitrate)
	}

	if allow5G && bps > MaxUeAmbrBpsFor5G {
		return fmt.Errorf("%s of %s exceeds the 4 Tbps ceiling of 5G (TS 38.413 BitRate)", label, bitrate)
	}

	return nil
}
