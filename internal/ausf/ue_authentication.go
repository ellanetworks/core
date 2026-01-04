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
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("ella-core/ausf")

func UeAuthPostRequestProcedure(ctx context.Context, updateAuthenticationInfo models.AuthenticationInfo) (*models.Av5gAka, error) {
	ctx, span := tracer.Start(
		ctx,
		"AUSF UEAuthentication PostRequest",
		trace.WithAttributes(
			attribute.String("ue.suci", updateAuthenticationInfo.Suci),
		),
	)
	defer span.End()

	suci := updateAuthenticationInfo.Suci

	snName := updateAuthenticationInfo.ServingNetworkName
	servingNetworkAuthorized := ausf.isServingNetworkAuthorized(snName)

	if !servingNetworkAuthorized {
		return nil, fmt.Errorf("serving network not authorized: %s", snName)
	}

	authInfoReq := models.AuthenticationInfoRequest{
		ServingNetworkName: snName,
	}

	if updateAuthenticationInfo.ResynchronizationInfo != nil {
		ausfCurrentContext := ausf.getUeContext(suci)
		if ausfCurrentContext == nil {
			return nil, fmt.Errorf("ue context not found for suci: %v", suci)
		}

		updateAuthenticationInfo.ResynchronizationInfo.Rand = ausfCurrentContext.Rand
		authInfoReq.ResynchronizationInfo = updateAuthenticationInfo.ResynchronizationInfo
	}

	authInfoResult, err := ausf.CreateAuthData(ctx, authInfoReq, suci)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth data: %s", err)
	}

	// Derive HXRES* from XRES*
	concat := authInfoResult.AuthenticationVector.Rand + authInfoResult.AuthenticationVector.XresStar

	hxresStarBytes, err := hex.DecodeString(concat)
	if err != nil {
		return nil, fmt.Errorf("decode error: %s", err)
	}

	hxresStarAll := sha256.Sum256(hxresStarBytes)
	hxresStar := hex.EncodeToString(hxresStarAll[16:]) // last 128 bits

	// Derive Kseaf from Kausf
	ausfDecode, err := hex.DecodeString(authInfoResult.AuthenticationVector.Kausf)
	if err != nil {
		return nil, fmt.Errorf("AUSF decode failed: %s", err)
	}

	P0 := []byte(snName)

	kSeaf, err := ueauth.GetKDFValue(ausfDecode, ueauth.FCForKseafDerivation, P0, ueauth.KDFLen(P0))
	if err != nil {
		return nil, fmt.Errorf("failed to get KDF value: %s", err)
	}

	ausfUeContext := &AusfUeContext{
		Supi:     authInfoResult.Supi,
		XresStar: authInfoResult.AuthenticationVector.XresStar,
		Kseaf:    hex.EncodeToString(kSeaf),
		Rand:     authInfoResult.AuthenticationVector.Rand,
	}

	ausf.addUeContextToPool(suci, ausfUeContext)

	return &models.Av5gAka{
		Rand:      authInfoResult.AuthenticationVector.Rand,
		Autn:      authInfoResult.AuthenticationVector.Autn,
		HxresStar: hxresStar,
	}, nil
}

func Auth5gAkaComfirmRequestProcedure(resStar string, suci string) (string, string, error) {
	ausfCurrentContext := ausf.getUeContext(suci)
	if ausfCurrentContext == nil {
		return "", "", fmt.Errorf("ausf ue context is nil for suci: %s", suci)
	}

	if strings.Compare(resStar, ausfCurrentContext.XresStar) != 0 {
		return "", "", fmt.Errorf("RES* mismatch for suci: %s", suci)
	}

	return ausfCurrentContext.Supi, ausfCurrentContext.Kseaf, nil
}
