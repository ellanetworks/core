// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-FileCopyrightText: 2024 Canonical Ltd.
// SPDX-License-Identifier: Apache-2.0

package udm

import (
	"sync"

	"github.com/ellanetworks/core/internal/models"
)

type UdmUeContext struct {
	Gpsi                      string
	Nssai                     *models.Nssai
	SmfSelSubsData            *models.SmfSelectionSubscriptionData
	UeCtxtInSmfData           *models.UeContextInSmfData
	SessionManagementSubsData map[string]models.SessionManagementSubscriptionData
	SubscribeToNotifChange    map[string]*models.SdmSubscription
	smfSelSubsDataLock        sync.Mutex
	SmSubsDataLock            sync.RWMutex
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
