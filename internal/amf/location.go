// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

// UpdateLocation records the UE's serving cell from an NGAP User Location
// Information. NGAP carries one CHOICE where S1AP carries a separate CGI and
// TAI, and only its E-UTRA alternative has an S1AP counterpart
// (TS 38.413 §9.3.1.16).
func (ueConn *UeConn) UpdateLocation(ctx context.Context, uli ngap.UserLocationInformation) {
	// The whole location is replaced rather than one alternative written into
	// it: a UE that moves between accesses must not keep the location it had on
	// the one it left. The value is also built fresh every call because the
	// snapshot published under ue.mu is aliased by concurrent LMF/API readers,
	// so what it points at must never change after publication.
	loc, tai, ok := ueConn.buildLocation(ctx, uli)
	if !ok {
		return
	}

	ueConn.Location = loc
	ueConn.Tai = *tai

	if ueConn.ue != nil {
		ueConn.ue.mu.Lock()
		ueConn.ue.Location = loc
		ueConn.ue.Tai = *tai
		ueConn.ue.mu.Unlock()
	}
}

func (ueConn *UeConn) buildLocation(ctx context.Context, uli ngap.UserLocationInformation) (models.UserLocation, *models.Tai, bool) {
	curTime := time.Now().UTC()
	cellPlmnID := decodePLMN(uli.PLMNIdentity)
	plmnID := decodePLMN(uli.TAI.PLMNIdentity)

	tai := &models.Tai{
		PlmnID: &plmnID,
		Tac:    fmt.Sprintf("%06x", uint32(uli.TAI.TAC)),
	}

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

		return models.UserLocation{NrLocation: nr}, tai, true
	case ngap.UserLocationEUTRA:
		eutra := &models.EutraLocation{
			Tai:                 tai,
			Ecgi:                &models.Ecgi{PlmnID: &cellPlmnID, EutraCellID: fmt.Sprintf("%07x", uli.CellIdentity)},
			UeLocationTimestamp: &curTime,
		}
		if uli.TimeStamp != nil {
			eutra.AgeOfLocationInformation = ageOfLocation(*uli.TimeStamp)
		}

		return models.UserLocation{EutraLocation: eutra}, tai, true
	case ngap.UserLocationN3IWF:
		// Untrusted non-3GPP access reports the UE's transport address instead
		// of a cell, so the serving TAI is this AMF's own rather than one the
		// N3IWF supplies (TS 23.501 §5.3.2.3).
		n3gppTai, err := ueConn.operatorTai(ctx)
		if err != nil {
			logger.AmfLog.Error("cannot record an N3IWF location without an operator TAI", zap.Error(err))
			return models.UserLocation{}, nil, false
		}

		ipv4, ipv6 := uli.IPAddress.IPs()
		n3ga := &models.N3gaLocation{
			N3gppTai:   n3gppTai,
			PortNumber: int32(uli.PortNumber),
		}

		if ipv4.IsValid() {
			n3ga.UeIpv4Addr = ipv4.String()
		}

		if ipv6.IsValid() {
			n3ga.UeIpv6Addr = ipv6.String()
		}

		return models.UserLocation{N3gaLocation: n3ga}, n3gppTai, true
	default:
		return models.UserLocation{}, nil, false
	}
}

func (ueConn *UeConn) operatorTai(ctx context.Context) (*models.Tai, error) {
	if ueConn.amf == nil {
		return nil, fmt.Errorf("connection has no owning AMF")
	}

	operatorInfo, err := ueConn.amf.OperatorInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get operator info: %w", err)
	}

	if len(operatorInfo.Tais) == 0 {
		return nil, fmt.Errorf("operator has no tracking area")
	}

	tai := operatorInfo.Tais[0]

	return &tai, nil
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
