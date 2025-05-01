// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package ausf

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/internal/util/ueauth"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// Generates a random int between 0 and 255
func GenerateRandomNumber() (uint8, error) {
	maxN := big.NewInt(256)
	randomNumber, err := rand.Int(rand.Reader, maxN)
	if err != nil {
		return 0, err
	}
	return uint8(randomNumber.Int64()), nil
}

func UeAuthPostRequestProcedure(updateAuthenticationInfo models.AuthenticationInfo, ctx context.Context) (*models.UeAuthenticationCtx, error) {
	var responseBody models.UeAuthenticationCtx
	var authInfoReq models.AuthenticationInfoRequest

	supiOrSuci := updateAuthenticationInfo.SupiOrSuci

	snName := updateAuthenticationInfo.ServingNetworkName
	servingNetworkAuthorized := IsServingNetworkAuthorized(snName)
	if !servingNetworkAuthorized {
		return nil, fmt.Errorf("serving network NOT AUTHORIZED")
	}

	responseBody.ServingNetworkName = snName
	authInfoReq.ServingNetworkName = snName
	if updateAuthenticationInfo.ResynchronizationInfo != nil {
		ausfCurrentSupi := GetSupiFromSuciSupiMap(supiOrSuci)
		ausfCurrentContext := GetAusfUeContext(ausfCurrentSupi)
		updateAuthenticationInfo.ResynchronizationInfo.Rand = ausfCurrentContext.Rand
		authInfoReq.ResynchronizationInfo = updateAuthenticationInfo.ResynchronizationInfo
	}

	authInfoResult, err := udm.CreateAuthData(authInfoReq, supiOrSuci, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth data: %s", err)
	}

	ueid := authInfoResult.Supi
	ausfUeContext := NewAusfUeContext(ueid)
	ausfUeContext.ServingNetworkName = snName
	ausfUeContext.AuthStatus = models.AuthResultOngoing
	AddAusfUeContextToPool(ausfUeContext)

	AddSuciSupiPairToMap(supiOrSuci, ueid)

	if authInfoResult.AuthType == models.AuthType5GAka {
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
		ausfUeContext.Kausf = Kausf
		ausfUeContext.Kseaf = hex.EncodeToString(Kseaf)
		ausfUeContext.Rand = authInfoResult.AuthenticationVector.Rand

		var av5gAka models.Av5gAka
		av5gAka.Rand = authInfoResult.AuthenticationVector.Rand
		av5gAka.Autn = authInfoResult.AuthenticationVector.Autn
		av5gAka.HxresStar = hxresStar

		responseBody.Var5gAuthData = av5gAka
	} else if authInfoResult.AuthType == models.AuthTypeEAPAkaPrime {
		identity := ueid
		ikPrime := authInfoResult.AuthenticationVector.IkPrime
		ckPrime := authInfoResult.AuthenticationVector.CkPrime
		RAND := authInfoResult.AuthenticationVector.Rand
		AUTN := authInfoResult.AuthenticationVector.Autn
		XRES := authInfoResult.AuthenticationVector.Xres
		ausfUeContext.XRES = XRES

		ausfUeContext.Rand = authInfoResult.AuthenticationVector.Rand

		kEncr, kAut, kRe, MSK, EMSK, err := eapAkaPrimePrf(ikPrime, ckPrime, identity)
		if err != nil {
			return nil, fmt.Errorf("EAP-AKA' PRF failed: %s", err)
		}
		_, _, _, _, _ = kEncr, kAut, kRe, MSK, EMSK
		ausfUeContext.kAut = kAut
		Kausf := EMSK[0:32]
		ausfUeContext.Kausf = Kausf
		KausfDecode, err := hex.DecodeString(Kausf)
		if err != nil {
			return nil, fmt.Errorf("AUSF decode failed: %s", err)
		}
		P0 := []byte(snName)
		Kseaf, err := ueauth.GetKDFValue(KausfDecode, ueauth.FCForKseafDerivation, P0, ueauth.KDFLen(P0))
		if err != nil {
			return nil, fmt.Errorf("failed to get KDF value: %s", err)
		}
		ausfUeContext.Kseaf = hex.EncodeToString(Kseaf)

		randIdentifier, err := GenerateRandomNumber()
		if err != nil {
			return nil, fmt.Errorf("failed to generate random number: %s", err)
		}
		var eapPkt EapPacket
		eapPkt.Identifier = randIdentifier
		eapPkt.Code = EapCode(1)
		eapPkt.Type = EapType(50) // according to RFC5448 6.1
		var atRand, atAutn, atKdf, atKdfInput, atMAC string
		if atRandTmp, err := EapEncodeAttribute("AT_RAND", RAND); err != nil {
			return nil, fmt.Errorf("EAP encode RAND failed: %s", err)
		} else {
			atRand = atRandTmp
		}
		if atAutnTmp, err := EapEncodeAttribute("AT_AUTN", AUTN); err != nil {
			return nil, fmt.Errorf("EAP encode AUTN failed: %s", err)
		} else {
			atAutn = atAutnTmp
		}
		if atKdfTmp, err := EapEncodeAttribute("AT_KDF", snName); err != nil {
			return nil, fmt.Errorf("EAP encode KDF failed: %s", err)
		} else {
			atKdf = atKdfTmp
		}
		if atKdfInputTmp, err := EapEncodeAttribute("AT_KDF_INPUT", snName); err != nil {
			return nil, fmt.Errorf("EAP encode KDF failed: %s", err)
		} else {
			atKdfInput = atKdfInputTmp
		}
		if atMACTmp, err := EapEncodeAttribute("AT_MAC", ""); err != nil {
			return nil, fmt.Errorf("EAP encode MAC failed: %s", err)
		} else {
			atMAC = atMACTmp
		}

		dataArrayBeforeMAC := atRand + atAutn + atMAC + atKdf + atKdfInput
		eapPkt.Data = []byte(dataArrayBeforeMAC)
		encodedPktBeforeMAC := eapPkt.Encode()

		MACvalue, err := CalculateAtMAC([]byte(kAut), encodedPktBeforeMAC)
		if err != nil {
			return nil, fmt.Errorf("calculate MAC failed: %s", err)
		}
		atMacNum := fmt.Sprintf("%02x", AtMacAttribute)
		var atMACfirstRow []byte
		if atMACfirstRowTmp, err := hex.DecodeString(atMacNum + "05" + "0000"); err != nil {
			return nil, fmt.Errorf("MAC decode failed: %s", err)
		} else {
			atMACfirstRow = atMACfirstRowTmp
		}
		wholeAtMAC := append(atMACfirstRow, MACvalue...)

		atMAC = string(wholeAtMAC)
		dataArrayAfterMAC := atRand + atAutn + atMAC + atKdf + atKdfInput

		eapPkt.Data = []byte(dataArrayAfterMAC)
		encodedPktAfterMAC := eapPkt.Encode()
		responseBody.Var5gAuthData = base64.StdEncoding.EncodeToString(encodedPktAfterMAC)
	}

	responseBody.AuthType = authInfoResult.AuthType

	return &responseBody, nil
}

func Auth5gAkaComfirmRequestProcedure(resStar string, confirmationDataResponseID string) (*models.ConfirmationDataResponse, error) {
	var responseBody models.ConfirmationDataResponse
	responseBody.AuthResult = models.AuthResultFailure

	if !CheckIfSuciSupiPairExists(confirmationDataResponseID) {
		return nil, fmt.Errorf("supiSuciPair does not exist, confirmation failed (queried by %s)", confirmationDataResponseID)
	}

	currentSupi := GetSupiFromSuciSupiMap(confirmationDataResponseID)
	if !CheckIfAusfUeContextExists(currentSupi) {
		return nil, fmt.Errorf("SUPI does not exist, confirmation failed (queried by %s)", currentSupi)
	}

	ausfCurrentContext := GetAusfUeContext(currentSupi)

	// Compare the received RES* with the stored XRES*
	if strings.Compare(resStar, ausfCurrentContext.XresStar) == 0 {
		ausfCurrentContext.AuthStatus = models.AuthResultSuccess
		responseBody.AuthResult = models.AuthResultSuccess
		responseBody.Kseaf = ausfCurrentContext.Kseaf
	} else {
		ausfCurrentContext.AuthStatus = models.AuthResultFailure
		responseBody.AuthResult = models.AuthResultFailure
	}

	responseBody.Supi = currentSupi
	return &responseBody, nil
}

func EapAuthComfirmRequestProcedure(eapPayload string, eapSessionID string) (*models.EapSession, error) {
	if !CheckIfSuciSupiPairExists(eapSessionID) {
		return nil, fmt.Errorf("supi-suci pair does not exist")
	}

	currentSupi := GetSupiFromSuciSupiMap(eapSessionID)
	if !CheckIfAusfUeContextExists(currentSupi) {
		return nil, fmt.Errorf("supi does not exist")
	}

	ausfCurrentContext := GetAusfUeContext(currentSupi)
	eapPayloadBytes, err := base64.StdEncoding.DecodeString(eapPayload)
	if err != nil {
		return nil, fmt.Errorf("EAP Payload decode failed: %s", err)
	}

	eapGoPkt := gopacket.NewPacket(eapPayloadBytes, layers.LayerTypeEAP, gopacket.Default)
	eapLayer := eapGoPkt.Layer(layers.LayerTypeEAP)
	eapContent, _ := eapLayer.(*layers.EAP)
	var responseBody models.EapSession
	if eapContent.Code != layers.EAPCodeResponse {
		ausfCurrentContext.AuthStatus = models.AuthResultFailure
		responseBody.AuthResult = models.AuthResultOngoing
		failEapAkaNoti, err := ConstructFailEapAkaNotification(eapContent.Id)
		if err != nil {
			return nil, fmt.Errorf("construct EAP-AKA' failed: %s", err)
		}
		responseBody.EapPayload = failEapAkaNoti
		return &responseBody, nil
	}
	switch ausfCurrentContext.AuthStatus {
	case models.AuthResultOngoing:
		responseBody.KSeaf = ausfCurrentContext.Kseaf
		responseBody.Supi = currentSupi
		Kautn := ausfCurrentContext.kAut
		XRES := ausfCurrentContext.XRES
		RES, decodeOK, err := decodeResMac(eapContent.TypeData, eapContent.Contents, Kautn)
		if err != nil {
			return nil, fmt.Errorf("decode RES MAC failed: %s", err)
		}
		if !decodeOK {
			ausfCurrentContext.AuthStatus = models.AuthResultFailure
			responseBody.AuthResult = models.AuthResultOngoing
			failEapAkaNoti, err := ConstructFailEapAkaNotification(eapContent.Id)
			if err != nil {
				return nil, fmt.Errorf("construct EAP-AKA' failed: %s", err)
			}
			responseBody.EapPayload = failEapAkaNoti
		} else if XRES == string(RES) { // decodeOK && XRES == res, auth success
			responseBody.AuthResult = models.AuthResultSuccess
			eapSuccPkt := ConstructEapNoTypePkt(EapCodeSuccess, eapContent.Id)
			responseBody.EapPayload = eapSuccPkt
			ausfCurrentContext.AuthStatus = models.AuthResultSuccess
		} else {
			ausfCurrentContext.AuthStatus = models.AuthResultFailure
			responseBody.AuthResult = models.AuthResultOngoing
			failEapAkaNoti, err := ConstructFailEapAkaNotification(eapContent.Id)
			if err != nil {
				return nil, fmt.Errorf("construct EAP-AKA' failed: %s", err)
			}
			responseBody.EapPayload = failEapAkaNoti
		}

	case models.AuthResultFailure:
		eapFailPkt := ConstructEapNoTypePkt(EapCodeFailure, eapPayload[1])
		responseBody.EapPayload = eapFailPkt
		responseBody.AuthResult = models.AuthResultFailure
	}

	return &responseBody, nil
}
