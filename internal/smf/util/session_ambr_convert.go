package util

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/nas/nasType"
)

func ModelsToSessionAMBR(ambr *models.Ambr) (nasType.SessionAMBR, error) {
	var sessAmbr nasType.SessionAMBR
	uplink := strings.Split(ambr.Uplink, " ")
	if bitRate, err := strconv.ParseUint(uplink[0], 10, 16); err != nil {
		return sessAmbr, fmt.Errorf("failed to parse uplink bitrate: %v", err)
	} else {
		var bitRateBytes [2]byte
		binary.BigEndian.PutUint16(bitRateBytes[:], uint16(bitRate))
		sessAmbr.SetSessionAMBRForUplink(bitRateBytes)
	}
	sessAmbr.SetUnitForSessionAMBRForUplink(strToAMBRUnit(uplink[1]))

	downlink := strings.Split(ambr.Downlink, " ")
	if bitRate, err := strconv.ParseUint(downlink[0], 10, 16); err != nil {
		return sessAmbr, fmt.Errorf("failed to parse downlink bitrate: %v", err)
	} else {
		var bitRateBytes [2]byte
		binary.BigEndian.PutUint16(bitRateBytes[:], uint16(bitRate))
		sessAmbr.SetSessionAMBRForDownlink(bitRateBytes)
	}
	sessAmbr.SetUnitForSessionAMBRForDownlink(strToAMBRUnit(downlink[1]))
	return sessAmbr, nil
}

func strToAMBRUnit(unit string) uint8 {
	switch unit {
	case "bps":
		return nasMessage.SessionAMBRUnitNotUsed
	case "Kbps":
		return nasMessage.SessionAMBRUnit1Kbps
	case "Mbps":
		return nasMessage.SessionAMBRUnit1Mbps
	case "Gbps":
		return nasMessage.SessionAMBRUnit1Gbps
	case "Tbps":
		return nasMessage.SessionAMBRUnit1Tbps
	case "Pbps":
		return nasMessage.SessionAMBRUnit1Pbps
	}
	return nasMessage.SessionAMBRUnitNotUsed
}
