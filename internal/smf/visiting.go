// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import "github.com/ellanetworks/core/internal/models"

func (smContext *SMContext) visiting() bool {
	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	return smContext.roamingHome != models.PlmnID{}
}

func (smContext *SMContext) homeSnssaiLocked() *models.Snssai {
	if h := smContext.homeSession(); h != nil {
		return new(h.Rules().HomeSnssai)
	}

	if smContext.roamingHome == (models.PlmnID{}) || smContext.Snssai == nil {
		return nil
	}

	return &models.Snssai{Sst: smContext.Snssai.Sst, Sd: models.NormalizeSD(smContext.Snssai.Sd)}
}
