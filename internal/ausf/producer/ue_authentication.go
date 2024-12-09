package producer

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/ueauth"
	"github.com/yeastengine/ella/internal/ausf/context"
	"github.com/yeastengine/ella/internal/ausf/logger"
	"github.com/yeastengine/ella/internal/udm/producer"
)

const (
	UPSTREAM_SERVER_ERROR                = "UPSTREAM_SERVER_ERROR"
	USER_NOT_FOUND_ERROR                 = "USER_NOT_FOUND"
	SERVING_NETWORK_NOT_AUTHORIZED_ERROR = "SERVING_NETWORK_NOT_AUTHORIZED"
	AV_GENERATION_PROBLEM_ERROR          = "AV_GENERATION_PROBLEM"
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
	servingNetworkAuthorized := context.IsServingNetworkAuthorized(snName)
	if !servingNetworkAuthorized {
		return nil, fmt.Errorf("serving network NOT AUTHORIZED")
	}
	logger.UeAuthPostLog.Infoln("serving network authorized: ", snName)

	responseBody.ServingNetworkName = snName
	authInfoReq.ServingNetworkName = snName
	self := context.GetSelf()
	authInfoReq.AusfInstanceId = self.GetSelfID()

	if updateAuthenticationInfo.ResynchronizationInfo != nil {
		logger.UeAuthPostLog.Warnln("Auts: ", updateAuthenticationInfo.ResynchronizationInfo.Auts)
		ausfCurrentSupi := context.GetSupiFromSuciSupiMap(supiOrSuci)
		logger.UeAuthPostLog.Warnln(ausfCurrentSupi)
		ausfCurrentContext := context.GetAusfUeContext(ausfCurrentSupi)
		logger.UeAuthPostLog.Warnln(ausfCurrentContext.Rand)
		updateAuthenticationInfo.ResynchronizationInfo.Rand = ausfCurrentContext.Rand
		logger.UeAuthPostLog.Warnln("Rand: ", updateAuthenticationInfo.ResynchronizationInfo.Rand)
		authInfoReq.ResynchronizationInfo = updateAuthenticationInfo.ResynchronizationInfo
	}

	authInfoResult, err := producer.CreateAuthData(authInfoReq, supiOrSuci)
	if err != nil {
		return nil, fmt.Errorf("CreateAuthData failed: %s", err.Error())
	}

	ueid := authInfoResult.Supi
	ausfUeContext := context.NewAusfUeContext(ueid)
	ausfUeContext.ServingNetworkName = snName
	ausfUeContext.AuthStatus = models.AuthResult_ONGOING
	context.AddAusfUeContextToPool(ausfUeContext)

	logger.UeAuthPostLog.Infof("Add SuciSupiPair (%s, %s) to map.\n", supiOrSuci, ueid)
	context.AddSuciSupiPairToMap(supiOrSuci, ueid)

	if authInfoResult.AuthType == models.AuthType__5_G_AKA {
		logger.UeAuthPostLog.Infoln("Use 5G AKA auth method")

		// Derive HXRES* from XRES*
		concat := authInfoResult.AuthenticationVector.Rand + authInfoResult.AuthenticationVector.XresStar
		var hxresStarBytes []byte
		if bytes, err := hex.DecodeString(concat); err != nil {
			logger.Auth5gAkaComfirmLog.Warnf("decode error: %+v", err)
		} else {
			hxresStarBytes = bytes
		}
		hxresStarAll := sha256.Sum256(hxresStarBytes)
		hxresStar := hex.EncodeToString(hxresStarAll[16:]) // last 128 bits

		// Derive Kseaf from Kausf
		Kausf := authInfoResult.AuthenticationVector.Kausf
		var KausfDecode []byte
		if ausfDecode, err := hex.DecodeString(Kausf); err != nil {
			logger.Auth5gAkaComfirmLog.Warnf("AUSF decode failed: %+v", err)
		} else {
			KausfDecode = ausfDecode
		}
		P0 := []byte(snName)
		Kseaf, err := ueauth.GetKDFValue(KausfDecode, ueauth.FC_FOR_KSEAF_DERIVATION, P0, ueauth.KDFLen(P0))
		if err != nil {
			logger.Auth5gAkaComfirmLog.Error(err)
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
	} else if authInfoResult.AuthType == models.AuthType_EAP_AKA_PRIME {
		logger.UeAuthPostLog.Infoln("Use EAP-AKA' auth method")

		identity := ueid
		ikPrime := authInfoResult.AuthenticationVector.IkPrime
		ckPrime := authInfoResult.AuthenticationVector.CkPrime
		RAND := authInfoResult.AuthenticationVector.Rand
		AUTN := authInfoResult.AuthenticationVector.Autn
		XRES := authInfoResult.AuthenticationVector.Xres
		ausfUeContext.XRES = XRES

		ausfUeContext.Rand = authInfoResult.AuthenticationVector.Rand

		K_encr, K_aut, K_re, MSK, EMSK := eapAkaPrimePrf(ikPrime, ckPrime, identity)
		_, _, _, _, _ = K_encr, K_aut, K_re, MSK, EMSK
		ausfUeContext.K_aut = K_aut
		Kausf := EMSK[0:32]
		ausfUeContext.Kausf = Kausf
		var KausfDecode []byte
		if ausfDecode, err := hex.DecodeString(Kausf); err != nil {
			logger.Auth5gAkaComfirmLog.Warnf("AUSF decode failed: %+v", err)
		} else {
			KausfDecode = ausfDecode
		}
		P0 := []byte(snName)
		Kseaf, err := ueauth.GetKDFValue(KausfDecode, ueauth.FC_FOR_KSEAF_DERIVATION, P0, ueauth.KDFLen(P0))
		if err != nil {
			logger.Auth5gAkaComfirmLog.Error(err)
		}
		ausfUeContext.Kseaf = hex.EncodeToString(Kseaf)

		var eapPkt EapPacket
		randIdentifier, err := GenerateRandomNumber()
		if err != nil {
			logger.Auth5gAkaComfirmLog.Warnf("Generate random number failed: %+v", err)
		}
		eapPkt.Identifier = randIdentifier
		eapPkt.Code = EapCode(1)
		eapPkt.Type = EapType(50) // according to RFC5448 6.1
		var atRand, atAutn, atKdf, atKdfInput, atMAC string
		if atRandTmp, err := EapEncodeAttribute("AT_RAND", RAND); err != nil {
			logger.Auth5gAkaComfirmLog.Warnf("EAP encode RAND failed: %+v", err)
		} else {
			atRand = atRandTmp
		}
		if atAutnTmp, err := EapEncodeAttribute("AT_AUTN", AUTN); err != nil {
			logger.Auth5gAkaComfirmLog.Warnf("EAP encode AUTN failed: %+v", err)
		} else {
			atAutn = atAutnTmp
		}
		if atKdfTmp, err := EapEncodeAttribute("AT_KDF", snName); err != nil {
			logger.Auth5gAkaComfirmLog.Warnf("EAP encode KDF failed: %+v", err)
		} else {
			atKdf = atKdfTmp
		}
		if atKdfInputTmp, err := EapEncodeAttribute("AT_KDF_INPUT", snName); err != nil {
			logger.Auth5gAkaComfirmLog.Warnf("EAP encode KDF failed: %+v", err)
		} else {
			atKdfInput = atKdfInputTmp
		}
		if atMACTmp, err := EapEncodeAttribute("AT_MAC", ""); err != nil {
			logger.Auth5gAkaComfirmLog.Warnf("EAP encode MAC failed: %+v", err)
		} else {
			atMAC = atMACTmp
		}

		dataArrayBeforeMAC := atRand + atAutn + atMAC + atKdf + atKdfInput
		eapPkt.Data = []byte(dataArrayBeforeMAC)
		encodedPktBeforeMAC := eapPkt.Encode()

		MACvalue := CalculateAtMAC([]byte(K_aut), encodedPktBeforeMAC)
		atMacNum := fmt.Sprintf("%02x", context.AT_MAC_ATTRIBUTE)
		var atMACfirstRow []byte
		if atMACfirstRowTmp, err := hex.DecodeString(atMacNum + "05" + "0000"); err != nil {
			logger.Auth5gAkaComfirmLog.Warnf("MAC decode failed: %+v", err)
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

	responseBody.Links = make(map[string]models.LinksValueSchema)
	responseBody.AuthType = authInfoResult.AuthType

	return &responseBody, nil
}

func Auth5gAkaComfirmRequestProcedure(updateConfirmationData models.ConfirmationData, ConfirmationDataResponseID string,
) (*models.ConfirmationDataResponse, error) {
	var responseBody models.ConfirmationDataResponse
	success := false
	responseBody.AuthResult = models.AuthResult_FAILURE

	if !context.CheckIfSuciSupiPairExists(ConfirmationDataResponseID) {
		logger.Auth5gAkaComfirmLog.Infof("supiSuciPair does not exist, confirmation failed (queried by %s)\n",
			ConfirmationDataResponseID)
		return nil, fmt.Errorf("supiSuciPair does not exist")
	}

	currentSupi := context.GetSupiFromSuciSupiMap(ConfirmationDataResponseID)
	if !context.CheckIfAusfUeContextExists(currentSupi) {
		logger.Auth5gAkaComfirmLog.Infof("SUPI does not exist, confirmation failed (queried by %s)\n", currentSupi)
		return nil, fmt.Errorf("SUPI does not exist")
	}

	ausfCurrentContext := context.GetAusfUeContext(currentSupi)
	servingNetworkName := ausfCurrentContext.ServingNetworkName

	// Compare the received RES* with the stored XRES*
	if strings.Compare(updateConfirmationData.ResStar, ausfCurrentContext.XresStar) == 0 {
		ausfCurrentContext.AuthStatus = models.AuthResult_SUCCESS
		responseBody.AuthResult = models.AuthResult_SUCCESS
		success = true
		logger.Auth5gAkaComfirmLog.Infoln("5G AKA confirmation succeeded")
		responseBody.Kseaf = ausfCurrentContext.Kseaf
	} else {
		ausfCurrentContext.AuthStatus = models.AuthResult_FAILURE
		responseBody.AuthResult = models.AuthResult_FAILURE
		logConfirmFailureAndInformUDM(ConfirmationDataResponseID, models.AuthType__5_G_AKA,
			"5G AKA confirmation failed")
	}

	if sendErr := sendAuthResultToUDM(currentSupi, models.AuthType__5_G_AKA, success, servingNetworkName); sendErr != nil {
		logger.Auth5gAkaComfirmLog.Infoln(sendErr.Error())
		return nil, fmt.Errorf("sendAuthResultToUDM failed")
	}

	responseBody.Supi = currentSupi
	return &responseBody, nil
}

// return response, problemDetails
func EapAuthComfirmRequestProcedure(updateEapSession models.EapSession, eapSessionID string) (*models.EapSession,
	error,
) {
	var responseBody models.EapSession

	if !context.CheckIfSuciSupiPairExists(eapSessionID) {
		logger.Auth5gAkaComfirmLog.Infoln("supiSuciPair does not exist, confirmation failed")
		return nil, fmt.Errorf("supiSuciPair does not exist")
	}

	currentSupi := context.GetSupiFromSuciSupiMap(eapSessionID)
	if !context.CheckIfAusfUeContextExists(currentSupi) {
		logger.Auth5gAkaComfirmLog.Infoln("SUPI does not exist, confirmation failed")
		return nil, fmt.Errorf("SUPI does not exist")
	}

	ausfCurrentContext := context.GetAusfUeContext(currentSupi)
	servingNetworkName := ausfCurrentContext.ServingNetworkName
	var eapPayload []byte
	if eapPayloadTmp, err := base64.StdEncoding.DecodeString(updateEapSession.EapPayload); err != nil {
		logger.Auth5gAkaComfirmLog.Warnf("EAP Payload decode failed: %+v", err)
	} else {
		eapPayload = eapPayloadTmp
	}

	eapGoPkt := gopacket.NewPacket(eapPayload, layers.LayerTypeEAP, gopacket.Default)
	eapLayer := eapGoPkt.Layer(layers.LayerTypeEAP)
	eapContent, _ := eapLayer.(*layers.EAP)

	if eapContent.Code != layers.EAPCodeResponse {
		logConfirmFailureAndInformUDM(eapSessionID, models.AuthType_EAP_AKA_PRIME,
			"eap packet code error")
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
		Kautn := ausfCurrentContext.K_aut
		XRES := ausfCurrentContext.XRES
		RES, decodeOK := decodeResMac(eapContent.TypeData, eapContent.Contents, Kautn)
		if !decodeOK {
			ausfCurrentContext.AuthStatus = models.AuthResult_FAILURE
			responseBody.AuthResult = models.AuthResult_ONGOING
			logConfirmFailureAndInformUDM(eapSessionID, models.AuthType_EAP_AKA_PRIME,
				"eap packet decode error")
			failEapAkaNoti := ConstructFailEapAkaNotification(eapContent.Id)
			responseBody.EapPayload = failEapAkaNoti
		} else if XRES == string(RES) { // decodeOK && XRES == res, auth success
			logger.EapAuthComfirmLog.Infoln("Correct RES value, EAP-AKA' auth succeed")
			responseBody.AuthResult = models.AuthResult_SUCCESS
			eapSuccPkt := ConstructEapNoTypePkt(EapCodeSuccess, eapContent.Id)
			responseBody.EapPayload = eapSuccPkt
			if sendErr := sendAuthResultToUDM(eapSessionID, models.AuthType_EAP_AKA_PRIME, true, servingNetworkName); sendErr != nil {
				logger.EapAuthComfirmLog.Infoln(sendErr.Error())
				return nil, fmt.Errorf("sendAuthResultToUDM failed")
			}
			ausfCurrentContext.AuthStatus = models.AuthResult_SUCCESS
		} else {
			ausfCurrentContext.AuthStatus = models.AuthResult_FAILURE
			responseBody.AuthResult = models.AuthResult_ONGOING
			logConfirmFailureAndInformUDM(eapSessionID, models.AuthType_EAP_AKA_PRIME,
				"Wrong RES value, EAP-AKA' auth failed")
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
