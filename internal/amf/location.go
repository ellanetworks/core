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

// ageOfLocation reinterprets the four TimeStamp octets as an int32, preserving
// what this AMF has always reported.
//
// TS 38.413 §9.3.1.16 makes TimeStamp an NTP-era seconds count while
// models.EutraLocation.AgeOfLocationInformation is minutes since last contact
// (TS 29.571), so the value is almost certainly wrong — but converting it is a
// behaviour change, not a codec swap, and belongs in its own change.
func ageOfLocation(ts ngap.TimeStamp) int32 {
	return int32(binary.BigEndian.Uint32(ts[:]))
}

// decodePLMN mirrors the MME's helper of the same name.
func decodePLMN(p ngap.PLMNIdentity) models.PlmnID {
	return util.PLMNToModels(p)
}
