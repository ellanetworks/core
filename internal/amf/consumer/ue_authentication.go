// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	ctxt "context"
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/models"
)

func SendUEAuthenticationAuthenticateRequest(ctx ctxt.Context, ue *context.AmfUe, resynchronizationInfo *models.ResynchronizationInfo) (*models.UeAuthenticationCtx, error) {
	if ue.Tai.PlmnID == nil {
		return nil, fmt.Errorf("tai is not available in UE context")
	}

	mnc, err := strconv.Atoi(ue.Tai.PlmnID.Mnc)
	if err != nil {
		return nil, fmt.Errorf("could not convert mnc to int: %v", err)
	}

	authInfo := models.AuthenticationInfo{
		Suci:                  ue.Suci,
		ServingNetworkName:    fmt.Sprintf("5G:mnc%03d.mcc%s.3gppnetwork.org", mnc, ue.Tai.PlmnID.Mcc),
		ResynchronizationInfo: resynchronizationInfo,
	}

	ueAuthenticationCtx, err := ausf.UeAuthPostRequestProcedure(ctx, authInfo)
	if err != nil {
		return nil, fmt.Errorf("ausf UE Authentication Authenticate Request failed: %s", err.Error())
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
