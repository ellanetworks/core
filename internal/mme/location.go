// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// UpdateLocation records the UE's serving cell (E-UTRAN CGI + TAI) from an S1AP
// User Location. The 16-bit S1AP TAC renders as the two least-significant octets
// of the 6-hex-digit TAC (TS 23.003, matching the gNB TAI rendering).
func (c *UeConn) UpdateLocation(ctx context.Context, cgi s1ap.EUTRANCGI, tai s1ap.TAI) {
	curTime := time.Now().UTC()

	plmnID, err := decodePLMN(tai.PLMNIdentity)
	if err != nil {
		logger.MmeLog.Warn("could not decode the TAI PLMN of a User Location", zap.Error(err))
		return
	}

	ePlmnID, err := decodePLMN(cgi.PLMNIdentity)
	if err != nil {
		logger.MmeLog.Warn("could not decode the cell PLMN of a User Location", zap.Error(err))
		return
	}

	// A fresh EutraLocation is built every call rather than mutated in place: the
	// snapshot published under ue.mu is aliased by concurrent LMF/API readers, so
	// the object it points at must never change after publication.
	eutra := &models.EutraLocation{
		Tai: &models.Tai{
			PlmnID: &plmnID,
			Tac:    fmt.Sprintf("%06x", uint16(tai.TAC)),
		},
		Ecgi: &models.Ecgi{
			PlmnID:      &ePlmnID,
			EutraCellID: fmt.Sprintf("%07x", cgi.CellID),
		},
		UeLocationTimestamp: &curTime,
	}

	c.locMu.Lock()
	c.Location.EutraLocation = eutra
	loc := c.Location

	ue := c.ue.Load()
	moved := false

	if ue != nil {
		ue.mu.Lock()
		moved = !ue.Location.SameCell(loc)
		ue.Location = loc
		ue.mu.Unlock()
	}
	c.locMu.Unlock()

	if moved {
		c.m.reportLocationChange(ctx, ue, loc)
	}
}

// GetUserLocation returns a copy of the UE's user location.
func (ue *UeContext) GetUserLocation() models.UserLocation {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.Location
}

// IsUserLocationEmpty returns true if the UE has no location information.
func (ue *UeContext) IsUserLocationEmpty() bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	loc := ue.Location

	return loc.EutraLocation == nil &&
		loc.NrLocation == nil &&
		loc.N3gaLocation == nil
}

// GetUELocation returns the user location of the UE identified by supi.
func (m *MME) GetUELocation(supi etsi.SUPI) (models.UserLocation, bool) {
	ue, ok := m.LookupUeBySupi(supi)
	if !ok {
		return models.UserLocation{}, false
	}

	return ue.GetUserLocation(), true
}

func (m *MME) IsUERegistered(supi etsi.SUPI) bool {
	ue, ok := m.LookupUeBySupi(supi)
	if !ok {
		return false
	}

	return ue.EMMState() == EMMRegistered
}
