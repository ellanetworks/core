package util

import (
	"encoding/binary"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/nas/nasType"
)

func ModelsToSessionAMBR(ambr *models.Ambr) (sessAmbr nasType.SessionAMBR) {
	logger.SmfLog.Infof("%v", ambr)

	uplink := strings.Split(ambr.Uplink, " ")
	if bitRate, err := strconv.ParseUint(uplink[0], 10, 16); err != nil {
		logger.SmfLog.Warnf("uplink AMBR parse failed: %+v", err)
	} else {
		var bitRateBytes [2]byte
		binary.BigEndian.PutUint16(bitRateBytes[:], uint16(bitRate))
		sessAmbr.SetSessionAMBRForUplink(bitRateBytes)
	}
	sessAmbr.SetUnitForSessionAMBRForUplink(strToAMBRUnit(uplink[1]))

	downlink := strings.Split(ambr.Downlink, " ")
	if bitRate, err := strconv.ParseUint(downlink[0], 10, 16); err != nil {
		logger.SmfLog.Warnf("downlink AMBR parse failed: %+v", err)
	} else {
		var bitRateBytes [2]byte
		binary.BigEndian.PutUint16(bitRateBytes[:], uint16(bitRate))
		sessAmbr.SetSessionAMBRForDownlink(bitRateBytes)
	}
	sessAmbr.SetUnitForSessionAMBRForDownlink(strToAMBRUnit(downlink[1]))
	return
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
