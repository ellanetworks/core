// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "fmt"

// The single-bit type-1 elements an ATTACH REQUEST or TRACKING AREA UPDATE
// REQUEST carries are modelled as *bool: nil means the element is absent, and
// the value is the bit the element carries.
//
// TMSI status (TS 24.301 §9.9.3.31 → TS 24.008 §10.5.5.4) is true when the UE
// holds a valid TMSI; device properties (§9.9.2.0A → TS 24.008 §10.5.7.8) is
// true for a low-priority device; MS network feature support (§9.9.3.20A →
// TS 24.008 §10.5.1.15) is true when the UE supports the extended periodic
// timer. GUTIType is its own type, since its two values name the security
// context a GUTI came from rather than a capability.

// GUTIType is the type of the GUTI an ATTACH REQUEST or TRACKING AREA UPDATE
// REQUEST carries (TS 24.301 §9.9.3.45): one the EPS network allocated, or one
// mapped from a 5GS or GERAN/UTRAN identity.
type GUTIType uint8

// GUTI types (TS 24.301 §9.9.3.45, table 9.9.3.45.1).
const (
	GUTITypeNative GUTIType = 0
	GUTITypeMapped GUTIType = 1
)

func (t GUTIType) String() string {
	switch t {
	case GUTITypeNative:
		return "native GUTI"
	case GUTITypeMapped:
		return "mapped GUTI"
	default:
		return fmt.Sprintf("unknown GUTI type (%d)", uint8(t))
	}
}

// AdditionalUpdateType is the additional update type information element value
// (TS 24.301 §9.9.3.0B): what else the UE wants from an attach or tracking area
// update beyond the type in the message header.
type AdditionalUpdateType struct {
	// AUTV asks for SMS only rather than a combined procedure (bit 1).
	AUTV bool
	// SAF asks the network to keep the NAS signalling connection after the
	// procedure completes (bit 2).
	SAF bool
	// PNBCIoT is the preferred CIoT network behaviour (bits 3-4).
	PNBCIoT PreferredCIoTNetworkBehaviour
}

// ParseAdditionalUpdateType decodes an additional update type value nibble.
func ParseAdditionalUpdateType(v uint8) AdditionalUpdateType {
	return AdditionalUpdateType{
		AUTV:    v&0x01 != 0,
		SAF:     v&0x02 != 0,
		PNBCIoT: PreferredCIoTNetworkBehaviour(v >> 2 & 0x03),
	}
}

// Nibble is the value half-octet this element occupies.
func (a AdditionalUpdateType) Nibble() uint8 {
	return boolBit(a.AUTV, 0) | boolBit(a.SAF, 1) | uint8(a.PNBCIoT)&0x03<<2
}

func (a AdditionalUpdateType) String() string {
	return fmt.Sprintf("AUTV=%t SAF=%t PNB-CIoT=%s", a.AUTV, a.SAF, a.PNBCIoT)
}

// PreferredCIoTNetworkBehaviour is which CIoT optimization a UE prefers the
// network to use (TS 24.301 §9.9.3.0B, table 9.9.3.0B.1). It is the EPS
// counterpart of fgs.PreferredCIoTNetworkBehaviour, which codes the same values.
type PreferredCIoTNetworkBehaviour uint8

// Preferred CIoT network behaviours (TS 24.301 table 9.9.3.0B.1).
const (
	CIoTNoPreference  PreferredCIoTNetworkBehaviour = 0
	CIoTControlPlane  PreferredCIoTNetworkBehaviour = 1
	CIoTUserPlane     PreferredCIoTNetworkBehaviour = 2
	CIoTReservedValue PreferredCIoTNetworkBehaviour = 3
)

func (p PreferredCIoTNetworkBehaviour) String() string {
	return enumString(uint8(p), map[uint8]string{
		uint8(CIoTNoPreference):  "no additional information",
		uint8(CIoTControlPlane):  "control plane CIoT optimization",
		uint8(CIoTUserPlane):     "user plane CIoT optimization",
		uint8(CIoTReservedValue): "reserved",
	})
}
