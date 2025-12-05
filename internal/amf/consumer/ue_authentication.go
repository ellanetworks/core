// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	ctxt "context"
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasType"
	"go.uber.org/zap"
)

func SendUEAuthenticationAuthenticateRequest(ctx ctxt.Context, ue *context.AmfUe, resynchronizationInfo *models.ResynchronizationInfo) (*models.UeAuthenticationCtx, error) {
	servedGuami, err := context.GetServedGuami(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get served guami: %v", err)
	}

	var plmnID *models.PlmnID
	if ue.Tai.PlmnID != nil {
		plmnID = ue.Tai.PlmnID
	} else {
		ue.GmmLog.Warn("Tai is not received from Serving Network", zap.String("mcc", servedGuami.PlmnID.Mcc), zap.String("mnc", servedGuami.PlmnID.Mnc))
		plmnID = servedGuami.PlmnID
	}

	var authInfo models.AuthenticationInfo
	authInfo.Suci = ue.Suci
	if mnc, err := strconv.Atoi(plmnID.Mnc); err != nil {
		return nil, err
	} else {
		authInfo.ServingNetworkName = fmt.Sprintf("5G:mnc%03d.mcc%s.3gppnetwork.org", mnc, plmnID.Mcc)
	}
	if resynchronizationInfo != nil {
		authInfo.ResynchronizationInfo = resynchronizationInfo
	}

	ueAuthenticationCtx, err := ausf.UeAuthPostRequestProcedure(ctx, authInfo)
	if err != nil {
		logger.AmfLog.Warn("UE Authentication Authenticate Request failed", zap.Error(err))
		return nil, err
	}

	return ueAuthenticationCtx, nil
}

func SendAuth5gAkaConfirmRequest(ctx ctxt.Context, ue *context.AmfUe, resStar string) (*models.ConfirmationDataResponse, error) {
	confirmResult, err := ausf.Auth5gAkaComfirmRequestProcedure(ctx, resStar, ue.Suci)
	if err != nil {
		return nil, fmt.Errorf("ausf 5G-AKA Confirm Request failed: %s", err.Error())
	}
	return confirmResult, nil
}

func SendEapAuthConfirmRequest(ctx ctxt.Context, suci string, eapMsg nasType.EAPMessage) (*models.EapSession, error) {
	eapPayload := base64.StdEncoding.EncodeToString(eapMsg.GetEAPMessage())
	response, err := ausf.EapAuthComfirmRequestProcedure(ctx, eapPayload, suci)
	if err != nil {
		return nil, fmt.Errorf("ausf EAP Confirm Request failed: %s", err.Error())
	}
	return response, nil
}
