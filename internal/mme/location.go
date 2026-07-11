// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

// UpdateLocation records the UE's serving cell (E-UTRAN CGI + TAI) from an S1AP
// User Location. The 16-bit S1AP TAC renders as the two least-significant octets
// of the 6-hex-digit TAC (TS 23.003, matching the gNB TAI rendering).
func (c *UeConn) UpdateLocation(cgi s1ap.EUTRANCGI, tai s1ap.TAI) {
	curTime := time.Now().UTC()
	plmnID := decodePLMN(tai.PLMNIdentity)
	ePlmnID := decodePLMN(cgi.PLMNIdentity)

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

	c.Location.EutraLocation = eutra

	if c.ue != nil {
		c.ue.mu.Lock()
		c.ue.Location = c.Location
		c.ue.mu.Unlock()
	}
}

func (ue *UeContext) GetUserLocation() models.UserLocation {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.Location
}

func (ue *UeContext) IsUserLocationEmpty() bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	loc := ue.Location

	return loc.EutraLocation == nil &&
		loc.NrLocation == nil &&
		loc.N3gaLocation == nil
}

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
