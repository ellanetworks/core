// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// UESecurityCapability is the UE security capability information element
// (TS 24.301 §9.9.3.36): the EPS algorithms the UE supports and, optionally, the
// UMTS and GERAN algorithms it would use on those accesses. The MME replays it
// in the SECURITY MODE COMMAND so the UE can detect bidding-down
// (TS 33.401 §7.2.4.4).
//
// It is the EPS counterpart of fgs.UESecurityCapability, which carries the EPS
// algorithms in the same optional position. Octets 5-6 (UMTS) are present
// whenever octet 7 (GERAN) is, so the optional tails are two flags rather than
// one opaque slice.
type UESecurityCapability struct {
	EEA nas.AlgorithmSet // octet 3: EEA0 in bit 8 down to EEA7 in bit 1
	EIA nas.AlgorithmSet // octet 4: EIA0 in bit 8 down to EIA6 in bit 2; bit 1 is EPS-UPIP

	HasUMTS bool
	UEA     nas.AlgorithmSet // octet 5: UEA0 in bit 8 down to UEA7 in bit 1
	UIA     nas.AlgorithmSet // octet 6: bit 8 spare, UIA1 in bit 7 down to UIA7 in bit 1

	HasGERAN bool
	GEA      nas.AlgorithmSet // octet 7: bit 8 spare, GEA1 in bit 7 down to GEA7 in bit 1
}

// ParseUESecurityCapability decodes a UE security capability IE value.
func ParseUESecurityCapability(b []byte) (UESecurityCapability, error) {
	switch len(b) {
	case 2, 4, 5:
	default:
		return UESecurityCapability{}, fmt.Errorf("nas/eps: UE security capability is %d octets, want 2, 4 or 5", len(b))
	}

	c := UESecurityCapability{EEA: nas.AlgorithmSet(b[0]), EIA: nas.AlgorithmSet(b[1])}

	// The spare bits of octets 6 and 7 are kept as received rather than masked:
	// the bidding-down comparison is byte-for-byte, so normalising them here
	// would make two distinct replays compare equal.
	if len(b) >= 4 {
		c.HasUMTS = true
		c.UEA = nas.AlgorithmSet(b[2])
		c.UIA = nas.AlgorithmSet(b[3])
	}

	if len(b) == 5 {
		c.HasGERAN = true
		c.GEA = nas.AlgorithmSet(b[4])
	}

	return c, nil
}

// AppendBinary encodes the UE security capability IE value onto b.
func (c UESecurityCapability) AppendBinary(b []byte) ([]byte, error) {
	if c.HasGERAN && !c.HasUMTS {
		return b, fmt.Errorf("nas/eps: UE security capability: the GERAN octet requires the UMTS octets")
	}

	w := nas.NewWriter(b)

	w.U8(uint8(c.EEA))
	w.U8(uint8(c.EIA))

	if c.HasUMTS {
		w.U8(uint8(c.UEA))
		w.U8(uint8(c.UIA))
	}

	if c.HasGERAN {
		w.U8(uint8(c.GEA))
	}

	return w.Result(b)
}

// MarshalBinary encodes the UE security capability IE value.
func (c UESecurityCapability) MarshalBinary() ([]byte, error) { return c.AppendBinary(nil) }

// SupportsEEA reports whether the UE supports EEAn (n = 0..7).
func (c UESecurityCapability) SupportsEEA(n uint8) bool { return c.EEA.Supports(n) }

// SupportsEIA reports whether the UE supports EIAn (n = 0..6). TS 24.301
// §9.9.3.36 gives octet 4 bit 1 to EPS-UPIP rather than to EIA7, so there is no
// EIA7 to report and n = 7 is always false; EPSUPIP reads that bit.
func (c UESecurityCapability) SupportsEIA(n uint8) bool {
	return n <= 6 && c.EIA.Supports(n)
}

// EPSUPIP reports whether the UE supports user-plane integrity protection in EPS
// (TS 24.301 §9.9.3.36, octet 4 bit 1).
func (c UESecurityCapability) EPSUPIP() bool { return c.EIA&0x01 != 0 }

// SupportsUEA reports whether the UE supports UEAn (n = 0..7). It is false when
// the UE advertised no UMTS algorithms.
func (c UESecurityCapability) SupportsUEA(n uint8) bool {
	return c.HasUMTS && c.UEA.Supports(n)
}

// SupportsUIA reports whether the UE supports UIAn (n = 1..7). It is false when
// the UE advertised no UMTS algorithms; bit 8 of the octet is spare, so there is
// no UIA0.
func (c UESecurityCapability) SupportsUIA(n uint8) bool {
	return c.HasUMTS && n >= 1 && c.UIA.Supports(n)
}

// SupportsGEA reports whether the UE supports GEAn (n = 1..7). It is false when
// the UE advertised no GERAN algorithms; bit 8 of the octet is spare, so there
// is no GEA0.
func (c UESecurityCapability) SupportsGEA(n uint8) bool {
	return c.HasGERAN && n >= 1 && c.GEA.Supports(n)
}

// Equal reports whether two capabilities are the same down to their spare bits.
// Bidding-down detection turns on exactly this comparison (TS 33.401 §7.2.4.4);
// it is a method so it stays symmetric with the 5GS type, whose spare tail rules
// out ==.
func (c UESecurityCapability) Equal(o UESecurityCapability) bool { return c == o }

func (c UESecurityCapability) String() string {
	out := fmt.Sprintf("EEA %08b, EIA %08b", c.EEA, c.EIA)
	if c.HasUMTS {
		out += fmt.Sprintf(", UEA %08b, UIA %08b", c.UEA, c.UIA)
	}

	if c.HasGERAN {
		out += fmt.Sprintf(", GEA %08b", c.GEA)
	}

	return out
}

// UENetworkCapability is the UE network capability information element
// (TS 24.301 §9.9.3.34): the EPS algorithms, optionally the UMTS algorithms, and
// then per-release feature bits. It is what a UE advertises in ATTACH REQUEST;
// UESecurityCapability is the subset the MME replays.
//
// EPS folds the algorithms and the feature bits into one element; 5GS split them
// into fgs.UESecurityCapability and fgs.GMMCapability.
type UENetworkCapability struct {
	EEA nas.AlgorithmSet // octet 3: EEA0 in bit 8 down to EEA7 in bit 1
	EIA nas.AlgorithmSet // octet 4: EIA0 in bit 8 down to EIA6 in bit 2; bit 1 is EPS-UPIP

	HasUMTS bool
	UEA     nas.AlgorithmSet // octet 5: UEA0 in bit 8 down to UEA7 in bit 1
	UCS2    bool             // octet 6 bit 8: the UE prefers the default alphabet over UCS2
	UIA     nas.AlgorithmSet // octet 6: UIA1 in bit 7 down to UIA7 in bit 1, bit 8 masked off

	// Rest holds octets 7 onwards verbatim: the per-release feature bits, which
	// the network does not act on but must not lose.
	Rest []byte
}

// ParseUENetworkCapability decodes a UE network capability IE value.
func ParseUENetworkCapability(b []byte) (UENetworkCapability, error) {
	// TS 24.301 table 9.9.3.34.1 pairs the algorithm octets, so an odd count
	// below the feature region cannot be a well-formed element.
	if len(b) < 2 || len(b) == 3 || len(b) > maxUENetworkCapabilityLen {
		return UENetworkCapability{}, fmt.Errorf(
			"nas/eps: UE network capability is %d octets, want 2, or 4 to %d", len(b), maxUENetworkCapabilityLen)
	}

	c := UENetworkCapability{EEA: nas.AlgorithmSet(b[0]), EIA: nas.AlgorithmSet(b[1])}

	if len(b) >= 4 {
		c.HasUMTS = true
		c.UEA = nas.AlgorithmSet(b[2])
		c.UCS2 = b[3]&(1<<7) != 0
		c.UIA = nas.AlgorithmSet(b[3] & 0x7F)

		if len(b) > 4 {
			c.Rest = bytes.Clone(b[4:])
		}
	}

	return c, nil
}

// maxUENetworkCapabilityLen is the element's longest value: TS 24.301 §9.9.3.34
// caps the whole element at 15 octets, two of which are its IEI and length.
const maxUENetworkCapabilityLen = 13

// AppendBinary encodes the UE network capability IE value onto b.
func (c UENetworkCapability) AppendBinary(b []byte) ([]byte, error) {
	if len(c.Rest) > 0 && !c.HasUMTS {
		return b, fmt.Errorf("nas/eps: UE network capability: the feature octets require the UMTS algorithm octets")
	}

	if n := 4 + len(c.Rest); c.HasUMTS && n > maxUENetworkCapabilityLen {
		return b, fmt.Errorf(
			"nas/eps: UE network capability is %d octets, want at most %d", n, maxUENetworkCapabilityLen)
	}

	w := nas.NewWriter(b)

	w.U8(uint8(c.EEA))
	w.U8(uint8(c.EIA))

	if c.HasUMTS {
		w.U8(uint8(c.UEA))
		w.U8(boolBit(c.UCS2, 7) | uint8(c.UIA)&0x7F)
		w.Raw(c.Rest)
	}

	return w.Result(b)
}

// MarshalBinary encodes the UE network capability IE value.
func (c UENetworkCapability) MarshalBinary() ([]byte, error) { return c.AppendBinary(nil) }

func (c UENetworkCapability) String() string {
	out := fmt.Sprintf("EEA %08b, EIA %08b", c.EEA, c.EIA)
	if c.HasUMTS {
		out += fmt.Sprintf(", UEA %08b, UIA %08b", c.UEA, c.UIA)
	}

	return out
}

func (c MSNetworkCapability) String() string { return fmt.Sprintf("%x", c.Rest) }

// Equal reports whether two capabilities are the same down to their feature
// octets.
func (c UENetworkCapability) Equal(o UENetworkCapability) bool {
	return c.EEA == o.EEA && c.EIA == o.EIA && c.HasUMTS == o.HasUMTS &&
		c.UEA == o.UEA && c.UCS2 == o.UCS2 && c.UIA == o.UIA && bytes.Equal(c.Rest, o.Rest)
}

// SupportsEEA reports whether the UE supports EEAn (n = 0..7).
func (c UENetworkCapability) SupportsEEA(n uint8) bool { return c.EEA.Supports(n) }

// SupportsEIA reports whether the UE supports EIAn (n = 0..6). TS 24.301
// §9.9.3.34 gives octet 4 bit 1 to EPS-UPIP rather than to EIA7.
func (c UENetworkCapability) SupportsEIA(n uint8) bool {
	return n <= 6 && c.EIA.Supports(n)
}

// EPSUPIP reports whether the UE supports user-plane integrity protection in EPS
// (TS 24.301 §9.9.3.34, octet 4 bit 1).
func (c UENetworkCapability) EPSUPIP() bool { return c.EIA&0x01 != 0 }

// SupportsUEA reports whether the UE supports UEAn (n = 0..7). It is false when
// the UE advertised no UMTS algorithms.
func (c UENetworkCapability) SupportsUEA(n uint8) bool {
	return c.HasUMTS && c.UEA.Supports(n)
}

// SupportsUIA reports whether the UE supports UIAn (n = 1..7). It is false when
// the UE advertised no UMTS algorithms; bit 8 of the octet is UCS2, so there is
// no UIA0.
func (c UENetworkCapability) SupportsUIA(n uint8) bool {
	return c.HasUMTS && n >= 1 && c.UIA.Supports(n)
}

// MSNetworkCapability is the MS network capability information element
// (TS 24.008 §10.5.5.12), which a UE includes in ATTACH REQUEST when it also
// supports GERAN or UTRAN. Only the GERAN ciphering algorithms are modelled;
// they are what the MME replays in the UE security capability.
type MSNetworkCapability struct {
	// GEA1 is octet 1 bit 8; GEA2 to GEA7 are octet 2 bits 7 down to 2.
	GEA1 bool
	// GEAExtended holds GEA2 in bit 6 down to GEA7 in bit 1, the layout octet 2
	// uses once shifted into place.
	GEAExtended nas.AlgorithmSet

	// Rest holds the element verbatim, so it re-encodes unchanged whatever the
	// release. The modelled fields are a read-only view onto it.
	Rest []byte
}

// ParseMSNetworkCapability decodes an MS network capability IE value.
// maxMSNetworkCapabilityLen is the element's longest value: TS 24.008 §10.5.5.12
// caps the whole element at 10 octets, two of which are its IEI and length.
const maxMSNetworkCapabilityLen = 8

// ParseMSNetworkCapability decodes an MS network capability IE value.
func ParseMSNetworkCapability(b []byte) (MSNetworkCapability, error) {
	if len(b) < 1 {
		return MSNetworkCapability{}, fmt.Errorf("nas/eps: MS network capability is empty")
	}

	if len(b) > maxMSNetworkCapabilityLen {
		return MSNetworkCapability{}, fmt.Errorf(
			"nas/eps: MS network capability is %d octets, want at most %d", len(b), maxMSNetworkCapabilityLen)
	}

	c := MSNetworkCapability{GEA1: b[0]&(1<<7) != 0, Rest: bytes.Clone(b)}
	if len(b) >= 2 {
		c.GEAExtended = nas.AlgorithmSet(b[1] >> 1 & 0x3F)
	}

	return c, nil
}

// AppendBinary encodes the MS network capability IE value onto b.
func (c MSNetworkCapability) AppendBinary(b []byte) ([]byte, error) {
	if len(c.Rest) == 0 {
		return b, fmt.Errorf("nas/eps: MS network capability is empty")
	}

	if len(c.Rest) > maxMSNetworkCapabilityLen {
		return b, fmt.Errorf(
			"nas/eps: MS network capability is %d octets, want at most %d", len(c.Rest), maxMSNetworkCapabilityLen)
	}

	return append(b, c.Rest...), nil
}

// MarshalBinary encodes the MS network capability IE value.
func (c MSNetworkCapability) MarshalBinary() ([]byte, error) { return c.AppendBinary(nil) }

// GERANCiphering maps the GERAN ciphering algorithms onto the layout the UE
// security capability GERAN octet uses (TS 24.301 §9.9.3.36: GEA1 in bit 7 down
// to GEA7 in bit 1, bit 8 spare). It is 0 when the UE advertises no GERAN
// ciphering.
func (c MSNetworkCapability) GERANCiphering() nas.AlgorithmSet {
	return nas.AlgorithmSet(boolBit(c.GEA1, 6)) | c.GEAExtended
}

// SupportsGEA reports whether the UE supports GEAn (n = 1..7).
func (c MSNetworkCapability) SupportsGEA(n uint8) bool {
	if n == 1 {
		return c.GEA1
	}

	return n >= 2 && n <= 7 && c.GEAExtended.Supports(n)
}

// ReplayedUESecurityCapability builds the UE security capability the MME echoes
// in the SECURITY MODE COMMAND (TS 24.301 §9.9.3.36) so the UE can detect
// bidding-down; the UE rejects the command with cause #23 if the replay differs
// from the capabilities it sent. It carries the EPS and UMTS algorithms from the
// UE network capability and the GERAN algorithms from the MS network capability,
// which may be absent.
func ReplayedUESecurityCapability(ueNetCap UENetworkCapability, msNetCap *MSNetworkCapability) UESecurityCapability {
	c := UESecurityCapability{EEA: ueNetCap.EEA, EIA: ueNetCap.EIA}

	var gea nas.AlgorithmSet
	if msNetCap != nil {
		gea = msNetCap.GERANCiphering()
	}

	// Bit 8 of octet 6 is UCS2 support in the UE network capability but spare in
	// the UE security capability, so it does not carry over.
	uea, uia := ueNetCap.UEA, ueNetCap.UIA&0x7F

	// Which octets appear turns on the algorithms the UE advertised, not on how
	// long its own element happened to be (TS 24.301 §9.9.3.36). A UE that sent
	// octets 5-6 with every bit clear indicated no Iu-mode algorithm, so replaying
	// those octets would not match the capability it derives for itself and would
	// read as bidding-down.
	switch {
	case uea == 0 && uia == 0 && gea == 0:
		// No Iu-mode and no Gb-mode algorithm: octets 5, 6 and 7 are omitted.
		return c
	case uea == 0 && uia == 0:
		// Gb mode only: octets 5 and 6 are included zero-filled so 7 can follow.
		c.HasUMTS = true
		c.HasGERAN = true
		c.GEA = gea
	default:
		c.HasUMTS = true
		c.UEA, c.UIA = uea, uia

		if gea != 0 {
			c.HasGERAN = true
			c.GEA = gea
		}
	}

	return c
}

// boolBit is 1<<pos when set, 0 otherwise.
func boolBit(set bool, pos uint8) uint8 {
	if !set {
		return 0
	}

	return 1 << pos
}

// n1ModeOctet is octet 9 of the UE network capability, counting from octet 3;
// Rest starts at octet 7 (TS 24.301 figure 9.9.3.34.1).
const (
	n1ModeOctet = 2
	n1ModeBit   = 1 << 5
)

// SupportsN1Mode reports whether the UE indicated support for N1 mode for 3GPP
// access (octet 9, bit 6). A UE that does can move between EPS and 5GS, which is
// what gates the IWK N26 indication (TS 24.301 §5.5.1.2.4, §5.5.3.2.4). A UE
// that sent a capability too short to carry the bit indicated nothing, which is
// read as "not supported" — TS 24.301 §9.9.3.34 has the receiver treat absent
// octets as all-zero.
func (c UENetworkCapability) SupportsN1Mode() bool {
	if len(c.Rest) <= n1ModeOctet {
		return false
	}

	return c.Rest[n1ModeOctet]&n1ModeBit != 0
}
