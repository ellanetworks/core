// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-FileCopyrightText: 2024 Canonical Ltd.
// SPDX-License-Identifier: Apache-2.0

package udm

import (
	"sync"

	"github.com/omec-project/openapi/models"
)

type UdmUeContext struct {
	Supi                      string
	Nssai                     *models.Nssai
	Amf3GppAccessRegistration *models.Amf3GppAccessRegistration
	SmfSelSubsData            *models.SmfSelectionSubscriptionData
	UeCtxtInSmfData           *models.UeContextInSmfData
	SessionManagementSubsData map[string]models.SessionManagementSubscriptionData
	UdmSubsToNotify           map[string]*models.SubscriptionDataSubscriptions
	smfSelSubsDataLock        sync.Mutex
	SmSubsDataLock            sync.RWMutex
}

func (ue *UdmUeContext) init() {
	ue.UdmSubsToNotify = make(map[string]*models.SubscriptionDataSubscriptions)
}

// SetSmfSelectionSubsData ... functions to set SmfSelectionSubscriptionData
func (udmUeContext *UdmUeContext) SetSmfSelectionSubsData(smfSelSubsData *models.SmfSelectionSubscriptionData) {
	udmUeContext.smfSelSubsDataLock.Lock()
	defer udmUeContext.smfSelSubsDataLock.Unlock()
	udmUeContext.SmfSelSubsData = smfSelSubsData
}

// SetSMSubsData ... functions to set SessionManagementSubsData
func (udmUeContext *UdmUeContext) SetSMSubsData(smSubsData map[string]models.SessionManagementSubscriptionData) {
	udmUeContext.SmSubsDataLock.Lock()
	defer udmUeContext.SmSubsDataLock.Unlock()
	udmUeContext.SessionManagementSubsData = smSubsData
}
