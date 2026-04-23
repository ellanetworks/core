package utils

import "github.com/free5gc/nas/nasMessage"

func NasToGPRSTimer3(timerValueNas uint8) int {
	unit := (timerValueNas & 0b11100000) >> 5
	value := int(timerValueNas & 0b00011111)

	switch unit {
	case nasMessage.GPRSTimer3UnitMultiplesOf2Seconds:
		return value * 2
	case nasMessage.GPRSTimer3UnitMultiplesOf30Seconds:
		return value * 30
	case nasMessage.GPRSTimer3UnitMultiplesOf1Minute:
		return value * 60
	case nasMessage.GPRSTimer3UnitMultiplesOf10Minutes:
		return value * 600
	case nasMessage.GPRSTimer3UnitMultiplesOf1Hour:
		return value * 3600
	case nasMessage.GPRSTimer3UnitMultiplesOf10Hours:
		return value * 36000
	default:
		// Undefined unit value per spec — treat as 0
		return 0
	}
}
