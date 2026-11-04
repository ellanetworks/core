// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import "github.com/ellanetworks/core/internal/models"

func (smContext *SMContext) visiting() bool {
	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	return smContext.roamingHPLMN != models.PlmnID{}
}
