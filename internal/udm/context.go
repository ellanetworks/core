// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-FileCopyrightText: 2024 Canonical Ltd.
// SPDX-License-Identifier: Apache-2.0

package udm

import (
	"fmt"
	"sync"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/marshtojsonstring"
)

var udmContext UDMContext

const (
	LocationUriAmf3GppAccessRegistration int = iota
	LocationUriAmfNon3GppAccessRegistration
	LocationUriSmfRegistration
	LocationUriSdmSubscription
	LocationUriSharedDataSubscription
)

type UDMContext struct {
	DbInstance *db.Database
	UdmUePool  sync.Map // map[supi]*UdmUeContext
}

func (context *UDMContext) ManageSmData(smDatafromUDR []models.SessionManagementSubscriptionData, snssaiFromReq string,
	dnnFromReq string) (mp map[string]models.SessionManagementSubscriptionData,
) {
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
func (context *UDMContext) CreateUeContextInSmfDataforUe(supi string, body models.UeContextInSmfData) error {
	ue, ok := context.UdmUeFindBySupi(supi)
	if !ok {
		return fmt.Errorf("ue not found")
	}
	ue.UeCtxtInSmfData = &body
	return nil
}

// functions for SmfSelectionSubscriptionData
func (context *UDMContext) CreateSmfSelectionSubsDataforUe(supi string, body models.SmfSelectionSubscriptionData) error {
	ue, ok := context.UdmUeFindBySupi(supi)
	if !ok {
		return fmt.Errorf("ue not found")
	}
	ue.SmfSelSubsData = &body
	return nil
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

func (context *UDMContext) CreateAmf3gppRegContext(supi string, body models.Amf3GppAccessRegistration) {
	_, ok := context.UdmUeFindBySupi(supi)
	if !ok {
		_ = context.NewUdmUe(supi)
	}
}
