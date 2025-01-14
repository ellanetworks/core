// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pcf

import (
	"fmt"
	"strings"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/udr"
	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
)

func DeleteAMPolicy(polAssoID string) error {
	const prefix = "policies/"
	polAssoID = strings.TrimPrefix(polAssoID, prefix)
	ue := pcfCtx.PCFUeFindByPolicyID(polAssoID)
	if ue == nil {
		return fmt.Errorf("polAssoID[%s] not found in UePool", polAssoID)
	}
	_, exists := ue.AMPolicyData[polAssoID]
	if !exists {
		return fmt.Errorf("polAssoID[%s] not found in AMPolicyData", polAssoID)
	}
	delete(ue.AMPolicyData, polAssoID)
	return nil
}

func UpdateAMPolicy(polAssoID string, policyAssociationUpdateRequest models.PolicyAssociationUpdateRequest) (*models.PolicyUpdate, error) {
	ue := pcfCtx.PCFUeFindByPolicyID(polAssoID)
	if ue == nil || ue.AMPolicyData[polAssoID] == nil {
		return nil, fmt.Errorf("polAssoID not found  in PCF")
	}

	amPolicyData := ue.AMPolicyData[polAssoID]
	var response models.PolicyUpdate
	if policyAssociationUpdateRequest.NotificationUri != "" {
		amPolicyData.NotificationURI = policyAssociationUpdateRequest.NotificationUri
	}
	if policyAssociationUpdateRequest.AltNotifIpv4Addrs != nil {
		amPolicyData.AltNotifIpv4Addrs = policyAssociationUpdateRequest.AltNotifIpv4Addrs
	}
	if policyAssociationUpdateRequest.AltNotifIpv6Addrs != nil {
		amPolicyData.AltNotifIpv6Addrs = policyAssociationUpdateRequest.AltNotifIpv6Addrs
	}
	for _, trigger := range policyAssociationUpdateRequest.Triggers {
		switch trigger {
		case models.RequestTrigger_LOC_CH:
			if policyAssociationUpdateRequest.UserLoc == nil {
				return nil, fmt.Errorf("UserLoc doesn't exist in Policy Association Requset Update while Triggers include LOC_CH")
			}
			amPolicyData.UserLoc = policyAssociationUpdateRequest.UserLoc
			logger.PcfLog.Infof("Ue[%s] UserLocation %+v", ue.Supi, amPolicyData.UserLoc)
		case models.RequestTrigger_PRA_CH:
			if policyAssociationUpdateRequest.PraStatuses == nil {
				return nil, fmt.Errorf("PraStatuses doesn't exist in Policy Association")
			}
			for praID, praInfo := range policyAssociationUpdateRequest.PraStatuses {
				logger.PcfLog.Infof("Policy Association Presence Id[%s] change state to %s", praID, praInfo.PresenceState)
			}
		case models.RequestTrigger_SERV_AREA_CH:
			if policyAssociationUpdateRequest.ServAreaRes == nil {
				return nil, fmt.Errorf("ServAreaRes doesn't exist in Policy Association Requset Update while Triggers include SERV_AREA_CH")
			} else {
				amPolicyData.ServAreaRes = policyAssociationUpdateRequest.ServAreaRes
				response.ServAreaRes = policyAssociationUpdateRequest.ServAreaRes
			}
		case models.RequestTrigger_RFSP_CH:
			if policyAssociationUpdateRequest.Rfsp == 0 {
				return nil, fmt.Errorf("rfsp doesn't exist in Policy Association Requset Update while Triggers include RFSP_CH")
			} else {
				amPolicyData.Rfsp = policyAssociationUpdateRequest.Rfsp
				response.Rfsp = policyAssociationUpdateRequest.Rfsp
			}
		}
	}

	response.Triggers = amPolicyData.Triggers

	return &response, nil
}

func CreateAMPolicy(policyAssociationRequest models.PolicyAssociationRequest) (*models.PolicyAssociation, string, error) {
	var response models.PolicyAssociation
	var ue *UeContext
	if val, ok := pcfCtx.UePool.Load(policyAssociationRequest.Supi); ok {
		ue = val.(*UeContext)
	}
	if ue == nil {
		if newUe, err := pcfCtx.NewPCFUe(policyAssociationRequest.Supi); err != nil {
			return nil, "", fmt.Errorf("supi Format Error: %s", err.Error())
		} else {
			ue = newUe
		}
	}
	response.Request = &policyAssociationRequest
	assolID := fmt.Sprintf("%s-%d", ue.Supi, ue.PolAssociationIDGenerator)
	amPolicy := ue.AMPolicyData[assolID]

	if amPolicy == nil || amPolicy.AmPolicyData == nil {
		amData, err := udr.GetAmPolicyData(ue.Supi)
		if err != nil {
			return nil, "", fmt.Errorf("can't find UE[%s] AM Policy Data in UDR", ue.Supi)
		}
		if amPolicy == nil {
			amPolicy = ue.NewUeAMPolicyData(assolID, policyAssociationRequest)
		}
		amPolicy.AmPolicyData = amData
	}

	var requestSuppFeat openapi.SupportedFeature
	if suppFeat, err := openapi.NewSupportedFeature(policyAssociationRequest.SuppFeat); err != nil {
		logger.PcfLog.Warnln(err)
	} else {
		requestSuppFeat = suppFeat
	}
	amPolicy.SuppFeat = pcfCtx.PcfSuppFeats[models.
		ServiceName_NPCF_AM_POLICY_CONTROL].NegotiateWith(
		requestSuppFeat).String()
	if amPolicy.Rfsp != 0 {
		response.Rfsp = amPolicy.Rfsp
	}
	response.SuppFeat = amPolicy.SuppFeat
	ue.PolAssociationIDGenerator++
	// Create location header for update, delete, get
	locationHeader := GetResourceURI(models.ServiceName_NPCF_AM_POLICY_CONTROL, assolID)
	logger.PcfLog.Debugf("AMPolicy association Id[%s] Create", assolID)
	return &response, locationHeader, nil
}
