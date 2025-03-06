// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-FileCopyrightText: 2024 Canonical Ltd.
// SPDX-License-Identifier: Apache-2.0

package udm

import (
	"sync"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/marshtojsonstring"
)

var udmContext UDMContext

type UDMContext struct {
	DbInstance *db.Database
	UdmUePool  sync.Map // map[supi]*UdmUeContext
}

func SetDbInstance(dbInstance *db.Database) {
	udmContext.DbInstance = dbInstance
}

func (context *UDMContext) ManageSmData(smDatafromUDR []models.SessionManagementSubscriptionData) (mp map[string]models.SessionManagementSubscriptionData) {
	smDataMap := make(map[string]models.SessionManagementSubscriptionData)
	AllDnns := make([]map[string]models.DnnConfiguration, len(smDatafromUDR))

	for idx, smSubscriptionData := range smDatafromUDR {
		singleNssaiStr := marshtojsonstring.MarshToJsonString(smSubscriptionData.SingleNssai)[0]
		smDataMap[singleNssaiStr] = smSubscriptionData
		AllDnns[idx] = smSubscriptionData.DnnConfigurations
	}

	return smDataMap
}

// functions related UecontextInSmfData
func (context *UDMContext) CreateUeContextInSmfDataforUe(supi string, body models.UeContextInSmfData) {
	ue, ok := context.UdmUeFindBySupi(supi)
	if !ok {
		ue = context.NewUdmUe(supi)
	}
	ue.UeCtxtInSmfData = &body
}

// functions for SmfSelectionSubscriptionData
func (context *UDMContext) CreateSmfSelectionSubsDataforUe(supi string, body models.SmfSelectionSubscriptionData) {
	ue, ok := context.UdmUeFindBySupi(supi)
	if !ok {
		ue = context.NewUdmUe(supi)
	}
	ue.SmfSelSubsData = &body
}

func (context *UDMContext) NewUdmUe(supi string) *UdmUeContext {
	ue := new(UdmUeContext)
	context.UdmUePool.Store(supi, ue)
	return ue
}

func (context *UDMContext) UdmUeFindBySupi(supi string) (*UdmUeContext, bool) {
	if value, ok := context.UdmUePool.Load(supi); ok {
		return value.(*UdmUeContext), ok
	} else {
		return nil, false
	}
}
