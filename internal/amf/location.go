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
			nr.AgeOfLocationInformation = ageOfLocation(*uli.TimeStamp, curTime)
		}

		return models.UserLocation{NrLocation: nr}, tai, true
	case ngap.UserLocationEUTRA:
		eutra := &models.EutraLocation{
			Tai:                 tai,
			Ecgi:                &models.Ecgi{PlmnID: &cellPlmnID, EutraCellID: fmt.Sprintf("%07x", uli.CellIdentity)},
			UeLocationTimestamp: &curTime,
		}
		if uli.TimeStamp != nil {
			eutra.AgeOfLocationInformation = ageOfLocation(*uli.TimeStamp, curTime)
		}

		return models.UserLocation{EutraLocation: eutra}, tai, true
	case ngap.UserLocationN3IWF:
		// Untrusted non-3GPP access reports the UE's transport address instead of
		// a cell, so this alternative carries no TAI of its own. TS 23.501
		// §5.3.2.3 has the N3IWF supply one — dedicated to non-3GPP access — over
		// N2 setup or in the optional id-TAI extension of the with-PortNumber
		// alternative, neither of which is modelled, so the operator's own TAI
		// stands in and the location is recorded rather than dropped.
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

const (
	// Seconds between the RFC 5905 era that starts 1900-01-01 and the Unix epoch.
	ntpEpochOffset = 2208988800

	// TS 29.571 bounds ageOfLocationInformation at 0..32767 minutes.
	maxAgeOfLocationMinutes = 32767
)

// ageOfLocation converts the Time Stamp IE, which carries UTC seconds in the
// format of the first four octets of an RFC 5905 timestamp (TS 38.413
// §9.3.1.75), into the elapsed minutes TS 29.571 defines for
// ageOfLocationInformation. A stamp ahead of the AMF's clock reads as 0, the
// value TS 29.571 gives to a location obtained by a just-completed reporting
// procedure.
func ageOfLocation(ts ngap.TimeStamp, now time.Time) int32 {
	generated := time.Unix(int64(binary.BigEndian.Uint32(ts[:]))-ntpEpochOffset, 0).UTC()

	minutes := int64(now.Sub(generated) / time.Minute)

	switch {
	case minutes < 0:
		return 0
	case minutes > maxAgeOfLocationMinutes:
		return maxAgeOfLocationMinutes
	default:
		return int32(minutes)
	}
}

// decodePLMN mirrors the MME's helper of the same name.
func decodePLMN(p ngap.PLMNIdentity) models.PlmnID {
	return util.PLMNToModels(p)
}
