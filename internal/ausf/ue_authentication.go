// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package ausf

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ellanetworks/core/internal/logger"
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

func UeAuthPostRequestProcedure(updateAuthenticationInfo models.AuthenticationInfo) (*models.UeAuthenticationCtx, error) {
	var responseBody models.UeAuthenticationCtx
	var authInfoReq models.AuthenticationInfoRequest

	supiOrSuci := updateAuthenticationInfo.SupiOrSuci

	snName := updateAuthenticationInfo.ServingNetworkName
	servingNetworkAuthorized := IsServingNetworkAuthorized(snName)
	if !servingNetworkAuthorized {
		return nil, fmt.Errorf("serving network NOT AUTHORIZED")
	}
	logger.AusfLog.Infoln("serving network authorized: ", snName)

	responseBody.ServingNetworkName = snName
	authInfoReq.ServingNetworkName = snName
	if updateAuthenticationInfo.ResynchronizationInfo != nil {
		ausfCurrentSupi := GetSupiFromSuciSupiMap(supiOrSuci)
		ausfCurrentContext := GetAusfUeContext(ausfCurrentSupi)
		updateAuthenticationInfo.ResynchronizationInfo.Rand = ausfCurrentContext.Rand
		authInfoReq.ResynchronizationInfo = updateAuthenticationInfo.ResynchronizationInfo
	}

	authInfoResult, err := udm.CreateAuthData(authInfoReq, supiOrSuci)
	if err != nil {
		return nil, fmt.Errorf("CreateAuthData failed: %s", err.Error())
	}

	ueid := authInfoResult.Supi
	ausfUeContext := NewAusfUeContext(ueid)
	ausfUeContext.ServingNetworkName = snName
	ausfUeContext.AuthStatus = models.AuthResult_ONGOING
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
		Kseaf, err := ueauth.GetKDFValue(ausfDecode, ueauth.FcForKseafDerivation, P0, ueauth.KDFLen(P0))
		if err != nil {
			logger.AusfLog.Error(err)
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
	} else if authInfoResult.AuthType == models.AuthTypeEapAkaPrime {
		identity := ueid
		ikPrime := authInfoResult.AuthenticationVector.IkPrime
		ckPrime := authInfoResult.AuthenticationVector.CkPrime
		RAND := authInfoResult.AuthenticationVector.Rand
		AUTN := authInfoResult.AuthenticationVector.Autn
		XRES := authInfoResult.AuthenticationVector.Xres
		ausfUeContext.XRES = XRES

		ausfUeContext.Rand = authInfoResult.AuthenticationVector.Rand

		KEncr, KAut, KRe, MSK, EMSK := eapAkaPrimePrf(ikPrime, ckPrime, identity)
		_, _, _, _, _ = KEncr, KAut, KRe, MSK, EMSK
		ausfUeContext.KAut = KAut
		Kausf := EMSK[0:32]
		ausfUeContext.Kausf = Kausf
		KausfDecode, err := hex.DecodeString(Kausf)
		if err != nil {
			return nil, fmt.Errorf("AUSF decode failed: %s", err)
		}
		P0 := []byte(snName)
		Kseaf, err := ueauth.GetKDFValue(KausfDecode, ueauth.FcForKseafDerivation, P0, ueauth.KDFLen(P0))
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
			logger.AusfLog.Warnf("EAP encode RAND failed: %+v", err)
		} else {
			atRand = atRandTmp
		}
		if atAutnTmp, err := EapEncodeAttribute("AT_AUTN", AUTN); err != nil {
			logger.AusfLog.Warnf("EAP encode AUTN failed: %+v", err)
		} else {
			atAutn = atAutnTmp
		}
		if atKdfTmp, err := EapEncodeAttribute("AT_KDF", snName); err != nil {
			logger.AusfLog.Warnf("EAP encode KDF failed: %+v", err)
		} else {
			atKdf = atKdfTmp
		}
		if atKdfInputTmp, err := EapEncodeAttribute("AT_KDF_INPUT", snName); err != nil {
			logger.AusfLog.Warnf("EAP encode KDF failed: %+v", err)
		} else {
			atKdfInput = atKdfInputTmp
		}
		if atMACTmp, err := EapEncodeAttribute("AT_MAC", ""); err != nil {
			logger.AusfLog.Warnf("EAP encode MAC failed: %+v", err)
		} else {
			atMAC = atMACTmp
		}

		dataArrayBeforeMAC := atRand + atAutn + atMAC + atKdf + atKdfInput
		eapPkt.Data = []byte(dataArrayBeforeMAC)
		encodedPktBeforeMAC := eapPkt.Encode()

		MACvalue := CalculateAtMAC([]byte(KAut), encodedPktBeforeMAC)
		atMacNum := fmt.Sprintf("%02x", AtMacAttribute)
		var atMACfirstRow []byte
		if atMACfirstRowTmp, err := hex.DecodeString(atMacNum + "05" + "0000"); err != nil {
			logger.AusfLog.Warnf("MAC decode failed: %+v", err)
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
	responseBody.AuthResult = models.AuthResult_FAILURE

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
		ausfCurrentContext.AuthStatus = models.AuthResult_SUCCESS
		responseBody.AuthResult = models.AuthResult_SUCCESS
		logger.AusfLog.Infoln("5G AKA confirmation succeeded")
		responseBody.Kseaf = ausfCurrentContext.Kseaf
	} else {
		ausfCurrentContext.AuthStatus = models.AuthResult_FAILURE
		responseBody.AuthResult = models.AuthResult_FAILURE
		logger.AusfLog.Infoln("5G AKA confirmation failed")
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
		logger.AusfLog.Infoln("eap packet code error")
		ausfCurrentContext.AuthStatus = models.AuthResult_FAILURE
		responseBody.AuthResult = models.AuthResult_ONGOING
		failEapAkaNoti := ConstructFailEapAkaNotification(eapContent.Id)
		responseBody.EapPayload = failEapAkaNoti
		return &responseBody, nil
	}
	switch ausfCurrentContext.AuthStatus {
	case models.AuthResult_ONGOING:
		responseBody.KSeaf = ausfCurrentContext.Kseaf
		responseBody.Supi = currentSupi
		Kautn := ausfCurrentContext.KAut
		XRES := ausfCurrentContext.XRES
		RES, decodeOK := decodeResMac(eapContent.TypeData, eapContent.Contents, Kautn)
		if !decodeOK {
			ausfCurrentContext.AuthStatus = models.AuthResult_FAILURE
			responseBody.AuthResult = models.AuthResult_ONGOING
			logger.AusfLog.Infoln("eap packet decode error")
			failEapAkaNoti := ConstructFailEapAkaNotification(eapContent.Id)
			responseBody.EapPayload = failEapAkaNoti
		} else if XRES == string(RES) { // decodeOK && XRES == res, auth success
			logger.AusfLog.Infoln("Correct RES value, EAP-AKA' auth succeed")
			responseBody.AuthResult = models.AuthResult_SUCCESS
			eapSuccPkt := ConstructEapNoTypePkt(EapCodeSuccess, eapContent.Id)
			responseBody.EapPayload = eapSuccPkt
			ausfCurrentContext.AuthStatus = models.AuthResult_SUCCESS
		} else {
			ausfCurrentContext.AuthStatus = models.AuthResult_FAILURE
			responseBody.AuthResult = models.AuthResult_ONGOING
			logger.AusfLog.Infoln("Wrong RES value, EAP-AKA' auth failed")
			failEapAkaNoti := ConstructFailEapAkaNotification(eapContent.Id)
			responseBody.EapPayload = failEapAkaNoti
		}

	case models.AuthResult_FAILURE:
		eapFailPkt := ConstructEapNoTypePkt(EapCodeFailure, eapPayload[1])
		responseBody.EapPayload = eapFailPkt
		responseBody.AuthResult = models.AuthResult_FAILURE
	}

	return &responseBody, nil
}
