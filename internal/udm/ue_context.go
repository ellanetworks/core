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
	Gpsi                      string
	Nssai                     *models.Nssai
	Amf3GppAccessRegistration *models.Amf3GppAccessRegistration
	SmfSelSubsData            *models.SmfSelectionSubscriptionData
	UeCtxtInSmfData           *models.UeContextInSmfData
	SessionManagementSubsData map[string]models.SessionManagementSubscriptionData
	SubscribeToNotifChange    map[string]*models.SdmSubscription
	smfSelSubsDataLock        sync.Mutex
	SmSubsDataLock            sync.RWMutex
}

func (udmUeContext *UdmUeContext) init() {
	udmUeContext.SubscribeToNotifChange = make(map[string]*models.SdmSubscription)
}

func (udmUeContext *UdmUeContext) CreateSubscriptiontoNotifChange(subscriptionID string, body *models.SdmSubscription) {
	if _, exist := udmUeContext.SubscribeToNotifChange[subscriptionID]; !exist {
		udmUeContext.SubscribeToNotifChange[subscriptionID] = body
	}
}

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
