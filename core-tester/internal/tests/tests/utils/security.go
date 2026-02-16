package utils

import (
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
)

type IntegrityAlgorithms struct {
	Nia0 bool
	Nia1 bool
	Nia2 bool
	Nia3 bool
}

type CipheringAlgorithms struct {
	Nea0 bool
	Nea1 bool
	Nea2 bool
	Nea3 bool
}

type UeSecurityCapability struct {
	Integrity IntegrityAlgorithms
	Ciphering CipheringAlgorithms
}

func GetUESecurityCapability(secCap *UeSecurityCapability) *nasType.UESecurityCapability {
	UESecurityCapability := &nasType.UESecurityCapability{
		Iei:    nasMessage.RegistrationRequestUESecurityCapabilityType,
		Len:    2,
		Buffer: []uint8{0x00, 0x00},
	}

	// Ciphering algorithms
	UESecurityCapability.SetEA0_5G(boolToUint8(secCap.Ciphering.Nea0))
	UESecurityCapability.SetEA1_128_5G(boolToUint8(secCap.Ciphering.Nea1))
	UESecurityCapability.SetEA2_128_5G(boolToUint8(secCap.Ciphering.Nea2))
	UESecurityCapability.SetEA3_128_5G(boolToUint8(secCap.Ciphering.Nea3))

	// Integrity algorithms
	UESecurityCapability.SetIA0_5G(boolToUint8(secCap.Integrity.Nia0))
	UESecurityCapability.SetIA1_128_5G(boolToUint8(secCap.Integrity.Nia1))
	UESecurityCapability.SetIA2_128_5G(boolToUint8(secCap.Integrity.Nia2))
	UESecurityCapability.SetIA3_128_5G(boolToUint8(secCap.Integrity.Nia3))

	return UESecurityCapability
}

func boolToUint8(boolean bool) uint8 {
	if boolean {
		return 1
	} else {
		return 0
	}
}
