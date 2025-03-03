// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"fmt"
	"net/http"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/pcf"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/util"
)

// SendSMPolicyAssociationCreate creates the SM Policy Decision
func SendSMPolicyAssociationCreate(smContext *context.SMContext) (*models.SmPolicyDecision, int, error) {
	httpRspStatusCode := http.StatusInternalServerError
	smPolicyData := models.SmPolicyContextData{}
	smPolicyData.Supi = smContext.Supi
	smPolicyData.PduSessionId = smContext.PDUSessionID
	smPolicyData.NotificationUri = fmt.Sprintf("nsmf-callback/sm-policies/%s",
		smContext.Ref,
	)
	smPolicyData.Dnn = smContext.Dnn
	smPolicyData.PduSessionType = util.PDUSessionTypeToModels(smContext.SelectedPDUSessionType)
	smPolicyData.AccessType = smContext.AnType
	smPolicyData.RatType = smContext.RatType
	smPolicyData.Ipv4Address = smContext.PDUAddress.Ip.To4().String()
	smPolicyData.SubsSessAmbr = &models.Ambr{
		Uplink:   smContext.DnnConfiguration.SessionAmbr.Uplink,
		Downlink: smContext.DnnConfiguration.SessionAmbr.Downlink,
	}
	smPolicyData.SubsDefQos = &models.SubscribedDefaultQos{
		Arp: &models.Arp{
			PriorityLevel: smContext.DnnConfiguration.Var5gQosProfile.Arp.PriorityLevel,
			PreemptCap:    smContext.DnnConfiguration.Var5gQosProfile.Arp.PreemptCap,
			PreemptVuln:   smContext.DnnConfiguration.Var5gQosProfile.Arp.PreemptVuln,
		},
	}
	smPolicyData.SliceInfo = &models.Snssai{
		Sst: smContext.Snssai.Sst,
		Sd:  smContext.Snssai.Sd,
	}
	smPolicyData.ServingNetwork = &models.PlmnId{
		Mcc: smContext.ServingNetwork.Mcc,
		Mnc: smContext.ServingNetwork.Mnc,
	}
	smPolicyData.SuppFeat = "F"

	smPolicyDecision, err := pcf.CreateSMPolicy(smPolicyData)
	if err != nil {
		return nil, httpRspStatusCode, fmt.Errorf("setup sm policy association failed: %s", err.Error())
	}
	err = validateSmPolicyDecision(smPolicyDecision)
	if err != nil {
		return nil, httpRspStatusCode, fmt.Errorf("setup sm policy association failed: %s", err.Error())
	}
	return smPolicyDecision, http.StatusCreated, nil
}

func SendSMPolicyAssociationDelete(smContext *context.SMContext, smDelReq *models.ReleaseSmContextRequest) (int, error) {
	smPolicyID := fmt.Sprintf("%s-%d", smContext.Supi, smContext.PDUSessionID)
	err := pcf.DeleteSMPolicy(smPolicyID)
	if err != nil {
		logger.SmfLog.Warnf("smf policy delete failed, [%v] ", err.Error())
		return http.StatusInternalServerError, err
	}
	return http.StatusAccepted, nil
}

func validateSmPolicyDecision(smPolicy *models.SmPolicyDecision) error {
	// Validate just presence of important IEs as of now
	// Sess Rules
	for name, rule := range smPolicy.SessRules {
		if rule.AuthSessAmbr == nil {
			logger.SmfLog.Errorf("SM policy decision rule [%s] validation failure, authorised session ambr missing", name)
			return fmt.Errorf("authorised session ambr missing")
		}

		if rule.AuthDefQos == nil {
			logger.SmfLog.Errorf("SM policy decision rule [%s] validation failure, authorised default qos missing", name)
			return fmt.Errorf("authorised default qos missing")
		}
	}
	return nil
}
