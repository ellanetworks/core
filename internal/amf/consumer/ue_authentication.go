// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	ctx "context"
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/nas/nasType"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella/consumer")

func SendUEAuthenticationAuthenticateRequest(ue *context.AmfUe, resynchronizationInfo *models.ResynchronizationInfo, ctext ctx.Context) (*models.UeAuthenticationCtx, error) {
	guamiList := context.GetServedGuamiList(ctext)
	servedGuami := guamiList[0]
	var plmnID *models.PlmnID
	if ue.Tai.PlmnID != nil {
		plmnID = ue.Tai.PlmnID
	} else {
		ue.GmmLog.Warn("Tai is not received from Serving Network", zap.String("mcc", servedGuami.PlmnID.Mcc), zap.String("mnc", servedGuami.PlmnID.Mnc))
		plmnID = servedGuami.PlmnID
	}

	var authInfo models.AuthenticationInfo
	authInfo.SupiOrSuci = ue.Suci
	if mnc, err := strconv.Atoi(plmnID.Mnc); err != nil {
		return nil, err
	} else {
		authInfo.ServingNetworkName = fmt.Sprintf("5G:mnc%03d.mcc%s.3gppnetwork.org", mnc, plmnID.Mcc)
	}
	if resynchronizationInfo != nil {
		authInfo.ResynchronizationInfo = resynchronizationInfo
	}

	ueAuthenticationCtx, err := ausf.UeAuthPostRequestProcedure(authInfo, ctext)
	if err != nil {
		logger.AmfLog.Error("UE Authentication Authenticate Request failed", zap.Error(err))
		return nil, err
	}
	return ueAuthenticationCtx, nil
}

func SendAuth5gAkaConfirmRequest(ue *context.AmfUe, resStar string, ctext ctx.Context) (*models.ConfirmationDataResponse, error) {
	_, span := tracer.Start(ctext, "SendAuth5gAkaConfirmRequest")
	defer span.End()

	span.SetAttributes(
		attribute.String("ue.suci", ue.Suci),
	)
	confirmResult, err := ausf.Auth5gAkaComfirmRequestProcedure(resStar, ue.Suci)
	if err != nil {
		return nil, fmt.Errorf("ausf 5G-AKA Confirm Request failed: %s", err.Error())
	}
	return confirmResult, nil
}

func SendEapAuthConfirmRequest(suci string, eapMsg nasType.EAPMessage) (*models.EapSession, error) {
	eapPayload := base64.StdEncoding.EncodeToString(eapMsg.GetEAPMessage())
	response, err := ausf.EapAuthComfirmRequestProcedure(eapPayload, suci)
	if err != nil {
		return nil, fmt.Errorf("ausf EAP Confirm Request failed: %s", err.Error())
	}
	return response, nil
}
