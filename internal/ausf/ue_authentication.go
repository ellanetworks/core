// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package ausf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/ueauth"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var tracer = otel.Tracer("ella-core/ausf")

func UeAuthPostRequestProcedure(ctx context.Context, updateAuthenticationInfo models.AuthenticationInfo) (*models.UeAuthenticationCtx, error) {
	ctx, span := tracer.Start(ctx, "AUSF UEAuthentication PostRequest")
	defer span.End()
	span.SetAttributes(
		attribute.String("ue.suci", updateAuthenticationInfo.Suci),
	)

	var responseBody models.UeAuthenticationCtx
	var authInfoReq models.AuthenticationInfoRequest

	suci := updateAuthenticationInfo.Suci

	snName := updateAuthenticationInfo.ServingNetworkName
	servingNetworkAuthorized := IsServingNetworkAuthorized(snName)
	if !servingNetworkAuthorized {
		return nil, fmt.Errorf("serving network not authorized: %s", snName)
	}

	authInfoReq.ServingNetworkName = snName
	if updateAuthenticationInfo.ResynchronizationInfo != nil {
		ausfCurrentContext := GetAusfUeContext(suci)
		if ausfCurrentContext == nil {
			return nil, fmt.Errorf("ue context not found for suci: %v", suci)
		}
		updateAuthenticationInfo.ResynchronizationInfo.Rand = ausfCurrentContext.Rand
		authInfoReq.ResynchronizationInfo = updateAuthenticationInfo.ResynchronizationInfo
	}

	authInfoResult, err := CreateAuthData(ctx, authInfoReq, suci)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth data: %s", err)
	}

	supi := authInfoResult.Supi
	ausfUeContext := NewAusfUeContext(supi)

	AddAusfUeContextToPool(suci, ausfUeContext)

	// Derive HXRES* from XRES*
	concat := authInfoResult.AuthenticationVector.Rand + authInfoResult.AuthenticationVector.XresStar

	hxresStarBytes, err := hex.DecodeString(concat)
	if err != nil {
		return nil, fmt.Errorf("decode error: %s", err)
	}

	hxresStarAll := sha256.Sum256(hxresStarBytes)
	hxresStar := hex.EncodeToString(hxresStarAll[16:]) // last 128 bits

	// Derive Kseaf from Kausf
	Kausf := authInfoResult.AuthenticationVector.Kausf

	ausfDecode, err := hex.DecodeString(Kausf)
	if err != nil {
		return nil, fmt.Errorf("AUSF decode failed: %s", err)
	}

	P0 := []byte(snName)

	Kseaf, err := ueauth.GetKDFValue(ausfDecode, ueauth.FCForKseafDerivation, P0, ueauth.KDFLen(P0))
	if err != nil {
		return nil, fmt.Errorf("failed to get KDF value: %s", err)
	}

	ausfUeContext.XresStar = authInfoResult.AuthenticationVector.XresStar
	ausfUeContext.Kseaf = hex.EncodeToString(Kseaf)
	ausfUeContext.Rand = authInfoResult.AuthenticationVector.Rand

	var av5gAka models.Av5gAka
	av5gAka.Rand = authInfoResult.AuthenticationVector.Rand
	av5gAka.Autn = authInfoResult.AuthenticationVector.Autn
	av5gAka.HxresStar = hxresStar

	responseBody.Var5gAuthData = av5gAka

	return &responseBody, nil
}

func Auth5gAkaComfirmRequestProcedure(ctx context.Context, resStar string, suci string) (*models.ConfirmationDataResponse, error) {
	_, span := tracer.Start(ctx, "AUSF UEAuthentication ConfirmRequest")
	defer span.End()
	span.SetAttributes(
		attribute.String("ue.suci", suci),
		attribute.String("auth.Method", "5G AKA"),
	)

	var responseBody models.ConfirmationDataResponse
	responseBody.AuthResult = models.AuthResultFailure

	// currentSupi := GetSupiFromSuciSupiMap(suci)
	// if currentSupi == "" {
	// 	return nil, fmt.Errorf("supi not found for suci: %s", suci)
	// }

	ausfCurrentContext := GetAusfUeContext(suci)
	if ausfCurrentContext == nil {
		return nil, fmt.Errorf("ausf ue context is nil for suci: %s", suci)
	}

	// Compare the received RES* with the stored XRES*
	if strings.Compare(resStar, ausfCurrentContext.XresStar) == 0 {
		responseBody.AuthResult = models.AuthResultSuccess
		responseBody.Kseaf = ausfCurrentContext.Kseaf
	} else {
		responseBody.AuthResult = models.AuthResultFailure
	}

	responseBody.Supi = ausfCurrentContext.Supi
	return &responseBody, nil
}
