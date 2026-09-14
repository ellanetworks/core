// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import "github.com/ellanetworks/core/internal/models"

func FarDestinationsForTest(fars []models.FAR) map[uint32]models.Interface {
	return farDestinations(fars)
}
