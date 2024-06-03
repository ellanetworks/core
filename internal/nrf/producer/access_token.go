// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"net/http"

	jwt "github.com/golang-jwt/jwt"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/canard/internal/nrf/logger"
)

func HandleAccessTokenRequest(request *httpwrapper.Request) *httpwrapper.Response {
	// Param of AccessTokenRsp
	logger.AccessTokenLog.Infoln("Handle AccessTokenRequest")

	accessTokenReq := request.Body.(models.AccessTokenReq)

	response, errResponse := AccessTokenProcedure(accessTokenReq)

	if response != nil {
		// status code is based on SPEC, and option headers
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if errResponse != nil {
		return httpwrapper.NewResponse(http.StatusBadRequest, nil, errResponse)
	}
	problemDetails := &models.ProblemDetails{
		Status: http.StatusForbidden,
		Cause:  "UNSPECIFIED",
	}
	return httpwrapper.NewResponse(http.StatusForbidden, nil, problemDetails)
}

func AccessTokenProcedure(request models.AccessTokenReq) (response *models.AccessTokenRsp,
	errResponse *models.AccessTokenErr) {
	logger.AccessTokenLog.Infoln("In AccessTokenProcedure")

	var expiration int32 = 1000
	scope := request.Scope
	tokenType := "Bearer"

	// Create AccessToken
	accessTokenClaims := models.AccessTokenClaims{
		Iss:            "1234567",                  // TODO: NF instance id of the NRF
		Sub:            request.NfInstanceId,       // nfInstanceId of service consumer
		Aud:            request.TargetNfInstanceId, // nfInstanceId of service producer
		Scope:          request.Scope,              // TODO: the name of the NF services for which the
		Exp:            expiration,                 //       access_token is authorized for use
		StandardClaims: jwt.StandardClaims{},
	}

	mySigningKey := []byte("NRF") // AllYourBase
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	accessToken, err := token.SignedString(mySigningKey)
	if err != nil {
		logger.AccessTokenLog.Warnln("Signed string error: ", err)
		errResponse = &models.AccessTokenErr{
			Error: "invalid_request",
		}

		return nil, errResponse
	}

	response = &models.AccessTokenRsp{
		AccessToken: accessToken,
		TokenType:   tokenType,
		ExpiresIn:   expiration,
		Scope:       scope,
	}

	return response, nil
}
