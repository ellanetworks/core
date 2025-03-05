// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/nas/nasType"
)

func SendUEAuthenticationAuthenticateRequest(ue *context.AmfUe,
	resynchronizationInfo *models.ResynchronizationInfo,
) (*models.UeAuthenticationCtx, error) {
	guamiList := context.GetServedGuamiList()
	servedGuami := guamiList[0]
	var plmnId *models.PlmnId
	if ue.Tai.PlmnId != nil {
		plmnId = ue.Tai.PlmnId
	} else {
		ue.GmmLog.Warnf("Tai is not received from Serving Network, Serving Plmn [Mcc: %v Mnc: %v] is taken from Guami List", servedGuami.PlmnId.Mcc, servedGuami.PlmnId.Mnc)
		plmnId = servedGuami.PlmnId
	}

	var authInfo models.AuthenticationInfo
	authInfo.SupiOrSuci = ue.Suci
	if mnc, err := strconv.Atoi(plmnId.Mnc); err != nil {
		return nil, err
	} else {
		authInfo.ServingNetworkName = fmt.Sprintf("5G:mnc%03d.mcc%s.3gppnetwork.org", mnc, plmnId.Mcc)
	}
	if resynchronizationInfo != nil {
		authInfo.ResynchronizationInfo = resynchronizationInfo
	}

	ueAuthenticationCtx, err := ausf.UeAuthPostRequestProcedure(authInfo)
	if err != nil {
		logger.AmfLog.Errorf("UE Authentication Authenticate Request failed: %+v", err)
		return nil, err
	}
	return ueAuthenticationCtx, nil
}

func SendAuth5gAkaConfirmRequest(ue *context.AmfUe, resStar string) (
	*models.ConfirmationDataResponse, error,
) {
	confirmationData := models.ConfirmationData{
		ResStar: resStar,
	}
	confirmResult, err := ausf.Auth5gAkaComfirmRequestProcedure(confirmationData, ue.Suci)
	if err != nil {
		return nil, fmt.Errorf("ausf 5G-AKA Confirm Request failed: %s", err.Error())
	}
	return confirmResult, nil
}

func SendEapAuthConfirmRequest(ue *context.AmfUe, eapMsg nasType.EAPMessage) (
	*models.EapSession, error,
) {
	eapSession := models.EapSession{
		EapPayload: base64.StdEncoding.EncodeToString(eapMsg.GetEAPMessage()),
	}

	response, err := ausf.EapAuthComfirmRequestProcedure(eapSession, ue.Suci)
	if err != nil {
		return nil, fmt.Errorf("ausf EAP Confirm Request failed: %s", err.Error())
	}
	return response, nil
}
