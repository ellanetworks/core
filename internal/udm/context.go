// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-FileCopyrightText: 2024 Canonical Ltd.
// SPDX-License-Identifier: Apache-2.0

package udm

import (
	"sync"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/util/idgenerator"
	"github.com/ellanetworks/core/internal/util/suci"
	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
)

var udmContext UDMContext

type UDMContext struct {
	DbInstance                *db.Database
	UriScheme                 models.UriScheme
	UdmUePool                 sync.Map // map[supi]*UdmUeContext
	GpsiSupiList              models.IdentityData
	SuciProfiles              []suci.SuciProfile
	EeSubscriptionIDGenerator *idgenerator.IDGenerator
}

func (context *UDMContext) ManageSmData(smDatafromUDR []models.SessionManagementSubscriptionData, snssaiFromReq string,
	dnnFromReq string) (mp map[string]models.SessionManagementSubscriptionData,
) {
	smDataMap := make(map[string]models.SessionManagementSubscriptionData)
	AllDnns := make([]map[string]models.DnnConfiguration, len(smDatafromUDR))

	for idx, smSubscriptionData := range smDatafromUDR {
		singleNssaiStr := openapi.MarshToJsonString(smSubscriptionData.SingleNssai)[0]
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
	ue.init()
	ue.Supi = supi
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

func (context *UDMContext) CreateAmf3gppRegContext(supi string, body models.Amf3GppAccessRegistration) {
	ue, ok := context.UdmUeFindBySupi(supi)
	if !ok {
		ue = context.NewUdmUe(supi)
	}
	ue.Amf3GppAccessRegistration = &body
}
