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
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/nas/nasType"
)

func SendUEAuthenticationAuthenticateRequest(ue *context.AmfUe, resynchronizationInfo *models.ResynchronizationInfo) (*models.UeAuthenticationCtx, error) {
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
		return nil, fmt.Errorf("couldn't convert MNC to integer: %s", err)
	} else {
		authInfo.ServingNetworkName = fmt.Sprintf("5G:mnc%03d.mcc%s.3gppnetwork.org", mnc, plmnId.Mcc)
	}
	if resynchronizationInfo != nil {
		authInfo.ResynchronizationInfo = resynchronizationInfo
	}

	ueAuthenticationCtx, err := ausf.UeAuthPostRequestProcedure(authInfo)
	if err != nil {
		return nil, fmt.Errorf("UE Authentication Authenticate Request failed: %s", err)
	}
	return ueAuthenticationCtx, nil
}

func SendAuth5gAkaConfirmRequest(ueSuci string, resStar string) (*models.ConfirmationDataResponse, error) {
	confirmResult, err := ausf.Auth5gAkaComfirmRequestProcedure(resStar, ueSuci)
	if err != nil {
		return nil, fmt.Errorf("couldn't receive AKA authentication confirmation from ausf: %s", err)
	}
	return confirmResult, nil
}

func SendEapAuthConfirmRequest(ueSuci string, eapMsg nasType.EAPMessage) (*models.EapSession, error) {
	eapPayload := base64.StdEncoding.EncodeToString(eapMsg.GetEAPMessage())
	response, err := ausf.EapAuthComfirmRequestProcedure(eapPayload, ueSuci)
	if err != nil {
		return nil, fmt.Errorf("couldn't receive EAP authentication confirmation from ausf: %s", err)
	}
	return response, nil
}
