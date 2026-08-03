// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
)

// UpdateLocation records the UE's serving cell from an NGAP User Location
// Information. NGAP carries one CHOICE where S1AP carries a separate CGI and
// TAI, and its NR alternative has no S1AP counterpart (TS 38.413 §9.3.1.16).
func (ueConn *UeConn) UpdateLocation(uli ngap.UserLocationInformation) {
	curTime := time.Now().UTC()
	plmnID := decodePLMN(uli.TAI.PLMNIdentity)
	cellPlmnID := decodePLMN(uli.PLMNIdentity)

	tai := &models.Tai{
		PlmnID: &plmnID,
		Tac:    fmt.Sprintf("%06x", uint32(uli.TAI.TAC)),
	}

	// A fresh location is built every call rather than mutated in place: the
	// snapshot published under ue.mu is aliased by concurrent LMF/API readers,
	// so the object it points at must never change after publication.
	switch uli.Kind {
	case ngap.UserLocationNR:
		nr := &models.NrLocation{
			Tai:                 tai,
			Ncgi:                &models.Ncgi{PlmnID: &cellPlmnID, NrCellID: fmt.Sprintf("%09x", uli.CellIdentity)},
			UeLocationTimestamp: &curTime,
		}
		if uli.TimeStamp != nil {
			nr.AgeOfLocationInformation = ageOfLocation(*uli.TimeStamp)
		}

		ueConn.Location.NrLocation = nr
	case ngap.UserLocationEUTRA:
		eutra := &models.EutraLocation{
			Tai:                 tai,
			Ecgi:                &models.Ecgi{PlmnID: &cellPlmnID, EutraCellID: fmt.Sprintf("%07x", uli.CellIdentity)},
			UeLocationTimestamp: &curTime,
		}
		if uli.TimeStamp != nil {
			eutra.AgeOfLocationInformation = ageOfLocation(*uli.TimeStamp)
		}

		ueConn.Location.EutraLocation = eutra
	default:
		return
	}

	ueConn.Tai = *tai

	if ueConn.ue != nil {
		ueConn.ue.mu.Lock()
		ueConn.ue.Location = ueConn.Location
		ueConn.ue.Tai = *tai
		ueConn.ue.mu.Unlock()
	}
}

// ageOfLocation converts the NGAP TimeStamp, an NTP-era seconds count, into the
// minutes since the location was determined that models.*Location carries.
func ageOfLocation(ts ngap.TimeStamp) int32 {
	seconds := binary.BigEndian.Uint32(ts[:])
	// NTP epoch (1900-01-01) to Unix epoch (1970-01-01).
	const ntpToUnix = 2208988800

	if seconds < ntpToUnix {
		return 0
	}

	age := time.Since(time.Unix(int64(seconds-ntpToUnix), 0)).Minutes()
	if age < 0 {
		return 0
	}

	return int32(age)
}

// decodePLMN mirrors the MME's helper of the same name.
func decodePLMN(p ngap.PLMNIdentity) models.PlmnID {
	return util.PLMNToModels(p)
}
