// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// UESecurityCapability is the UE security capability information element
// (TS 24.501 §9.11.3.54): the 5GS algorithms the UE supports and, when it also
// supports S1 mode, the EPS algorithms it would use there. The network replays
// it in the SECURITY MODE COMMAND so the UE can detect bidding-down
// (TS 33.501 §6.7.2).
//
// It is the 5GS counterpart of eps.UESecurityCapability, which carries the UMTS
// and GERAN algorithms in the same optional position.
type UESecurityCapability struct {
	EA nas.AlgorithmSet // octet 3: 5G-EA0 in bit 8 down to 5G-EA7 in bit 1
	IA nas.AlgorithmSet // octet 4: 5G-IA0 in bit 8 down to 5G-IA7 in bit 1

	// HasEPS reports whether the UE advertised the EPS algorithms of octets 5-6,
	// which it does when it supports S1 mode.
	HasEPS bool
	EEA    nas.AlgorithmSet // octet 5: EEA0 in bit 8 down to EEA7 in bit 1
	EIA    nas.AlgorithmSet // octet 6: EIA0 in bit 8 down to EIA7 in bit 1

	// Spare holds octets 7 onwards verbatim. They are spare in this release, so
	// they carry no meaning yet, but keeping them lets the element re-encode
	// byte-for-byte — which the bidding-down comparison depends on.
	Spare []byte
}

// maxUESecurityCapabilityLen is the element's longest value: TS 24.501
// §9.11.3.54 caps the whole element at 10 octets, two of which are its IEI and
// length.
const maxUESecurityCapabilityLen = 8

// ParseUESecurityCapability decodes a UE security capability IE value.
func ParseUESecurityCapability(b []byte) (UESecurityCapability, error) {
	// TS 24.501 table 9.11.3.54.1 pairs the algorithm octets, so an odd count
	// below the spare region cannot be a well-formed element.
	if len(b) < 2 || len(b) == 3 || len(b) > maxUESecurityCapabilityLen {
		return UESecurityCapability{}, fmt.Errorf(
			"nas/fgs: UE security capability is %d octets, want 2, or 4 to %d", len(b), maxUESecurityCapabilityLen)
	}

	c := UESecurityCapability{EA: nas.AlgorithmSet(b[0]), IA: nas.AlgorithmSet(b[1])}

	if len(b) >= 4 {
		c.HasEPS = true
		c.EEA = nas.AlgorithmSet(b[2])
		c.EIA = nas.AlgorithmSet(b[3])

		if len(b) > 4 {
			c.Spare = bytes.Clone(b[4:])
		}
	}

	return c, nil
}

// AppendBinary encodes the UE security capability IE value onto b.
func (c UESecurityCapability) AppendBinary(b []byte) ([]byte, error) {
	if len(c.Spare) > 0 && !c.HasEPS {
		return b, fmt.Errorf("nas/fgs: UE security capability: the spare octets require the EPS algorithm octets")
	}

	if n := 4 + len(c.Spare); c.HasEPS && n > maxUESecurityCapabilityLen {
		return b, fmt.Errorf(
			"nas/fgs: UE security capability is %d octets, want at most %d", n, maxUESecurityCapabilityLen)
	}

	w := nas.NewWriter(b)

	w.U8(uint8(c.EA))
	w.U8(uint8(c.IA))

	if c.HasEPS {
		w.U8(uint8(c.EEA))
		w.U8(uint8(c.EIA))
		w.Raw(c.Spare)
	}

	return w.Result(b)
}

// MarshalBinary encodes the UE security capability IE value.
func (c UESecurityCapability) MarshalBinary() ([]byte, error) { return c.AppendBinary(nil) }

// SupportsEA reports whether the UE supports 128-5G-EAn (n = 0..7).
func (c UESecurityCapability) SupportsEA(n uint8) bool { return c.EA.Supports(n) }

// SupportsIA reports whether the UE supports 128-5G-IAn (n = 0..7).
func (c UESecurityCapability) SupportsIA(n uint8) bool { return c.IA.Supports(n) }

// SupportsEEA reports whether the UE supports EEAn in S1 mode (n = 0..7). It is
// false when the UE advertised no EPS algorithms.
func (c UESecurityCapability) SupportsEEA(n uint8) bool {
	return c.HasEPS && c.EEA.Supports(n)
}

// SupportsEIA reports whether the UE supports EIAn in S1 mode (n = 0..7). It is
// false when the UE advertised no EPS algorithms.
func (c UESecurityCapability) SupportsEIA(n uint8) bool {
	return c.HasEPS && c.EIA.Supports(n)
}

// Equal reports whether two capabilities are the same down to their spare
// octets. Bidding-down detection turns on exactly this comparison, so it is a
// method rather than a == the spare tail would silently make illegal
// (TS 33.501 §6.7.2).
func (c UESecurityCapability) Equal(o UESecurityCapability) bool {
	return c.EA == o.EA && c.IA == o.IA && c.HasEPS == o.HasEPS &&
		c.EEA == o.EEA && c.EIA == o.EIA && bytes.Equal(c.Spare, o.Spare)
}

func (c UESecurityCapability) String() string {
	if !c.HasEPS {
		return fmt.Sprintf("5G-EA %08b, 5G-IA %08b", c.EA, c.IA)
	}

	return fmt.Sprintf("5G-EA %08b, 5G-IA %08b, EEA %08b, EIA %08b", c.EA, c.IA, c.EEA, c.EIA)
}

// GMMCapability is the 5GMM capability information element
// (TS 24.501 §9.11.3.1): what the UE supports beyond the security algorithms,
// which 5GS carries separately from UESecurityCapability. EPS has no counterpart
// — TS 24.301 folds the same ground into its UE network capability.
type GMMCapability struct {
	SGC      bool // service gap control
	HCCPCIoT bool // 5G header compression for control-plane CIoT
	// N3Data reports support for N3 data transfer. Its bit is inverted on the
	// wire — 0 means supported (TS 24.501 table 9.11.3.1.1) — so it is the one
	// field here that is not the octet's bit as sent.
	N3Data     bool
	CPCIoT     bool // control-plane CIoT 5GS optimisation
	RestrictEC bool // restriction on the use of enhanced coverage
	LPP        bool // LTE positioning protocol
	HOAttach   bool // handover attach from EPS with a PDN connection
	S1Mode     bool // S1 mode, so the UE can move to E-UTRAN

	// Rest holds octets 4 onwards verbatim, so later-release capability bits the
	// element carries survive decoding.
	Rest []byte
}

// maxGMMCapabilityLen is the element's longest value: TS 24.501 §9.11.3.1 caps
// the whole element at 15 octets, two of which are its IEI and length.
const maxGMMCapabilityLen = 13

// ParseGMMCapability decodes a 5GMM capability IE value.
func ParseGMMCapability(b []byte) (GMMCapability, error) {
	if len(b) < 1 {
		return GMMCapability{}, fmt.Errorf("nas/fgs: 5GMM capability is empty")
	}

	if len(b) > maxGMMCapabilityLen {
		return GMMCapability{}, fmt.Errorf(
			"nas/fgs: 5GMM capability is %d octets, want at most %d", len(b), maxGMMCapabilityLen)
	}

	c := GMMCapability{
		SGC:        b[0]&(1<<7) != 0,
		HCCPCIoT:   b[0]&(1<<6) != 0,
		N3Data:     b[0]&(1<<5) == 0,
		CPCIoT:     b[0]&(1<<4) != 0,
		RestrictEC: b[0]&(1<<3) != 0,
		LPP:        b[0]&(1<<2) != 0,
		HOAttach:   b[0]&(1<<1) != 0,
		S1Mode:     b[0]&1 != 0,
	}

	if len(b) > 1 {
		c.Rest = bytes.Clone(b[1:])
	}

	return c, nil
}

// AppendBinary encodes the 5GMM capability IE value onto b.
func (c GMMCapability) AppendBinary(b []byte) ([]byte, error) {
	if n := 1 + len(c.Rest); n > maxGMMCapabilityLen {
		return b, fmt.Errorf("nas/fgs: 5GMM capability is %d octets, want at most %d", n, maxGMMCapabilityLen)
	}

	octet := boolBit(c.SGC, 7) | boolBit(c.HCCPCIoT, 6) | boolBit(!c.N3Data, 5) | boolBit(c.CPCIoT, 4) |
		boolBit(c.RestrictEC, 3) | boolBit(c.LPP, 2) | boolBit(c.HOAttach, 1) | boolBit(c.S1Mode, 0)

	return append(append(b, octet), c.Rest...), nil
}

// MarshalBinary encodes the 5GMM capability IE value.
func (c GMMCapability) MarshalBinary() ([]byte, error) { return c.AppendBinary(nil) }
