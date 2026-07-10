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

// UpdateLocation records the UE's serving cell — E-UTRAN CGI and TAI — from an
// S1AP message's User Location on the connection, mirroring it to the persistent
// UE context when one is bound (TS 36.413). The 16-bit S1AP TAC renders as the
// two least-significant octets of the 6-hex-digit TAC, and the 28-bit E-UTRA
// Cell Identity as 7 hex digits (TS 23.003).
func (c *UeConn) UpdateLocation(cgi s1ap.EUTRANCGI, tai s1ap.TAI) {
	curTime := time.Now().UTC()

	if c.Location.EutraLocation == nil {
		c.Location.EutraLocation = new(models.EutraLocation)
	}

	plmnID := decodePLMN(tai.PLMNIdentity)
	tac := fmt.Sprintf("%06x", uint16(tai.TAC))

	if c.Location.EutraLocation.Tai == nil {
		c.Location.EutraLocation.Tai = new(models.Tai)
	}

	c.Location.EutraLocation.Tai.PlmnID = &plmnID
	c.Location.EutraLocation.Tai.Tac = tac

	ePlmnID := decodePLMN(cgi.PLMNIdentity)
	eutraCellID := fmt.Sprintf("%07x", cgi.CellID)

	if c.Location.EutraLocation.Ecgi == nil {
		c.Location.EutraLocation.Ecgi = new(models.Ecgi)
	}

	c.Location.EutraLocation.Ecgi.PlmnID = &ePlmnID
	c.Location.EutraLocation.Ecgi.EutraCellID = eutraCellID

	c.Location.EutraLocation.UeLocationTimestamp = &curTime

	if c.ue != nil {
		c.ue.Location = c.Location
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

// GetUELocation returns the UserLocation for a registered UE, or false if the UE
// is not found in the MME's UE pool.
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
