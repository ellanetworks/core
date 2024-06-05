package producer

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"strings"

	"github.com/bronze1man/radius"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	"github.com/omec-project/util/ueauth"
	ausf_context "github.com/yeastengine/canard/internal/ausf/context"
	"github.com/yeastengine/canard/internal/ausf/logger"
)

const (
	UPSTREAM_SERVER_ERROR                = "UPSTREAM_SERVER_ERROR"
	USER_NOT_FOUND_ERROR                 = "USER_NOT_FOUND"
	SERVING_NETWORK_NOT_AUTHORIZED_ERROR = "SERVING_NETWORK_NOT_AUTHORIZED"
	AV_GENERATION_PROBLEM_ERROR          = "AV_GENERATION_PROBLEM"
)

// Generates a random int between 0 and 255
func GenerateRandomNumber() (uint8, error) {
	max := big.NewInt(256)
	randomNumber, err := rand.Int(rand.Reader, max)
	if err != nil {
		return 0, err
	}
	return uint8(randomNumber.Int64()), nil
}

func HandleEapAuthComfirmRequest(request *httpwrapper.Request) *httpwrapper.Response {
	logger.Auth5gAkaComfirmLog.Infof("EapAuthComfirmRequest")

	updateEapSession := request.Body.(models.EapSession)
	eapSessionID := request.Params["authCtxId"]

	response, problemDetails := EapAuthComfirmRequestProcedure(updateEapSession, eapSessionID)

	if response != nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	problemDetails = &models.ProblemDetails{
		Status: http.StatusForbidden,
		Cause:  "UNSPECIFIED",
	}
	return httpwrapper.NewResponse(http.StatusForbidden, nil, problemDetails)
}

func HandleAuth5gAkaComfirmRequest(request *httpwrapper.Request) *httpwrapper.Response {
	logger.Auth5gAkaComfirmLog.Infof("Auth5gAkaComfirmRequest")
	updateConfirmationData := request.Body.(models.ConfirmationData)
	ConfirmationDataResponseID := request.Params["authCtxId"]

	response, problemDetails := Auth5gAkaComfirmRequestProcedure(updateConfirmationData, ConfirmationDataResponseID)
	if response != nil {
		return httpwrapper.NewResponse(http.StatusOK, nil, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	problemDetails = &models.ProblemDetails{
		Status: http.StatusForbidden,
		Cause:  "UNSPECIFIED",
	}
	return httpwrapper.NewResponse(http.StatusForbidden, nil, problemDetails)
}

func HandleUeAuthPostRequest(request *httpwrapper.Request) *httpwrapper.Response {
	logger.UeAuthPostLog.Infof("HandleUeAuthPostRequest")
	updateAuthenticationInfo := request.Body.(models.AuthenticationInfo)

	response, locationURI, problemDetails := UeAuthPostRequestProcedure(updateAuthenticationInfo)
	respHeader := make(http.Header)
	respHeader.Set("Location", locationURI)

	if response != nil {
		return httpwrapper.NewResponse(http.StatusCreated, respHeader, response)
	} else if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	}
	problemDetails = &models.ProblemDetails{
		Status: http.StatusForbidden,
		Cause:  "UNSPECIFIED",
	}
	return httpwrapper.NewResponse(http.StatusForbidden, nil, problemDetails)
}

// func UeAuthPostRequestProcedure(updateAuthenticationInfo models.AuthenticationInfo) (
//
//	response *models.UeAuthenticationCtx, locationURI string, problemDetails *models.ProblemDetails) {
func UeAuthPostRequestProcedure(updateAuthenticationInfo models.AuthenticationInfo) (*models.UeAuthenticationCtx,
	string, *models.ProblemDetails,
) {
	var responseBody models.UeAuthenticationCtx
	var authInfoReq models.AuthenticationInfoRequest

	supiOrSuci := updateAuthenticationInfo.SupiOrSuci

	snName := updateAuthenticationInfo.ServingNetworkName
	servingNetworkAuthorized := ausf_context.IsServingNetworkAuthorized(snName)
	if !servingNetworkAuthorized {
		var problemDetails models.ProblemDetails
		problemDetails.Cause = SERVING_NETWORK_NOT_AUTHORIZED_ERROR
		problemDetails.Status = http.StatusForbidden
		logger.UeAuthPostLog.Infoln("403 forbidden: serving network NOT AUTHORIZED")
		return nil, "", &problemDetails
	}
	logger.UeAuthPostLog.Infoln("Serving network authorized")

	responseBody.ServingNetworkName = snName
	authInfoReq.ServingNetworkName = snName
	self := ausf_context.GetSelf()
	authInfoReq.AusfInstanceId = self.GetSelfID()

	if updateAuthenticationInfo.ResynchronizationInfo != nil {
		logger.UeAuthPostLog.Warningln("Auts: ", updateAuthenticationInfo.ResynchronizationInfo.Auts)
		ausfCurrentSupi := ausf_context.GetSupiFromSuciSupiMap(supiOrSuci)
		logger.UeAuthPostLog.Warningln(ausfCurrentSupi)
		ausfCurrentContext := ausf_context.GetAusfUeContext(ausfCurrentSupi)
		logger.UeAuthPostLog.Warningln(ausfCurrentContext.Rand)
		updateAuthenticationInfo.ResynchronizationInfo.Rand = ausfCurrentContext.Rand
		logger.UeAuthPostLog.Warningln("Rand: ", updateAuthenticationInfo.ResynchronizationInfo.Rand)
		authInfoReq.ResynchronizationInfo = updateAuthenticationInfo.ResynchronizationInfo
	}

	udmUrl := getUdmUrl(self.NrfUri)
	client := createClientToUdmUeau(udmUrl)
	authInfoResult, rsp, err := client.GenerateAuthDataApi.GenerateAuthData(context.Background(), supiOrSuci, authInfoReq)
	if err != nil {
		logger.UeAuthPostLog.Infoln(err.Error())
		var problemDetails models.ProblemDetails
		if authInfoResult.AuthenticationVector == nil {
			problemDetails.Cause = AV_GENERATION_PROBLEM_ERROR
		} else {
			problemDetails.Cause = UPSTREAM_SERVER_ERROR
		}
		problemDetails.Status = http.StatusInternalServerError
		return nil, "", &problemDetails
	}
	defer func() {
		if rspCloseErr := rsp.Body.Close(); rspCloseErr != nil {
			logger.UeAuthPostLog.Errorf("GenerateAuthDataApi response body cannot close: %+v", rspCloseErr)
		}
	}()

	ueid := authInfoResult.Supi
	ausfUeContext := ausf_context.NewAusfUeContext(ueid)
	ausfUeContext.ServingNetworkName = snName
	ausfUeContext.AuthStatus = models.AuthResult_ONGOING
	ausfUeContext.UdmUeauUrl = udmUrl
	ausf_context.AddAusfUeContextToPool(ausfUeContext)

	logger.UeAuthPostLog.Infof("Add SuciSupiPair (%s, %s) to map.\n", supiOrSuci, ueid)
	ausf_context.AddSuciSupiPairToMap(supiOrSuci, ueid)

	locationURI := self.Url + "/nausf-auth/v1/ue-authentications/" + supiOrSuci
	putLink := locationURI
	if authInfoResult.AuthType == models.AuthType__5_G_AKA {
		logger.UeAuthPostLog.Infoln("Use 5G AKA auth method")
		putLink += "/5g-aka-confirmation"

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
		logger.Auth5gAkaComfirmLog.Infof("XresStar = %x\n", authInfoResult.AuthenticationVector.XresStar)

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
		putLink += "/eap-session"

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

		var eapPkt radius.EapPacket
		randIdentifier, err := GenerateRandomNumber()
		if err != nil {
			logger.Auth5gAkaComfirmLog.Warnf("Generate random number failed: %+v", err)
		}
		eapPkt.Identifier = randIdentifier
		eapPkt.Code = radius.EapCode(1)
		eapPkt.Type = radius.EapType(50) // according to RFC5448 6.1
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
		atMacNum := fmt.Sprintf("%02x", ausf_context.AT_MAC_ATTRIBUTE)
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

	linksValue := models.LinksValueSchema{Href: putLink}
	responseBody.Links = make(map[string]models.LinksValueSchema)
	responseBody.Links["link"] = linksValue
	responseBody.AuthType = authInfoResult.AuthType

	return &responseBody, locationURI, nil
}

// func Auth5gAkaComfirmRequestProcedure(updateConfirmationData models.ConfirmationData,
//	ConfirmationDataResponseID string) (response *models.ConfirmationDataResponse,
//  problemDetails *models.ProblemDetails) {

func Auth5gAkaComfirmRequestProcedure(updateConfirmationData models.ConfirmationData,
	ConfirmationDataResponseID string,
) (*models.ConfirmationDataResponse, *models.ProblemDetails) {
	var responseBody models.ConfirmationDataResponse
	success := false
	responseBody.AuthResult = models.AuthResult_FAILURE

	if !ausf_context.CheckIfSuciSupiPairExists(ConfirmationDataResponseID) {
		logger.Auth5gAkaComfirmLog.Infof("supiSuciPair does not exist, confirmation failed (queried by %s)\n",
			ConfirmationDataResponseID)
		var problemDetails models.ProblemDetails
		problemDetails.Cause = USER_NOT_FOUND_ERROR
		problemDetails.Status = http.StatusBadRequest
		return nil, &problemDetails
	}

	currentSupi := ausf_context.GetSupiFromSuciSupiMap(ConfirmationDataResponseID)
	if !ausf_context.CheckIfAusfUeContextExists(currentSupi) {
		logger.Auth5gAkaComfirmLog.Infof("SUPI does not exist, confirmation failed (queried by %s)\n", currentSupi)
		var problemDetails models.ProblemDetails
		problemDetails.Cause = USER_NOT_FOUND_ERROR
		problemDetails.Status = http.StatusBadRequest
		return nil, &problemDetails
	}

	ausfCurrentContext := ausf_context.GetAusfUeContext(currentSupi)
	servingNetworkName := ausfCurrentContext.ServingNetworkName

	// Compare the received RES* with the stored XRES*
	logger.Auth5gAkaComfirmLog.Infof("res*: %x\nXres*: %x\n", updateConfirmationData.ResStar, ausfCurrentContext.XresStar)
	if strings.Compare(updateConfirmationData.ResStar, ausfCurrentContext.XresStar) == 0 {
		ausfCurrentContext.AuthStatus = models.AuthResult_SUCCESS
		responseBody.AuthResult = models.AuthResult_SUCCESS
		success = true
		logger.Auth5gAkaComfirmLog.Infoln("5G AKA confirmation succeeded")
		responseBody.Kseaf = ausfCurrentContext.Kseaf
	} else {
		ausfCurrentContext.AuthStatus = models.AuthResult_FAILURE
		responseBody.AuthResult = models.AuthResult_FAILURE
		logConfirmFailureAndInformUDM(ConfirmationDataResponseID, models.AuthType__5_G_AKA, servingNetworkName,
			"5G AKA confirmation failed", ausfCurrentContext.UdmUeauUrl)
	}

	if sendErr := sendAuthResultToUDM(currentSupi, models.AuthType__5_G_AKA, success, servingNetworkName,
		ausfCurrentContext.UdmUeauUrl); sendErr != nil {
		logger.Auth5gAkaComfirmLog.Infoln(sendErr.Error())
		var problemDetails models.ProblemDetails
		problemDetails.Status = http.StatusInternalServerError
		problemDetails.Cause = UPSTREAM_SERVER_ERROR

		return nil, &problemDetails
	}

	responseBody.Supi = currentSupi
	return &responseBody, nil
}

// return response, problemDetails
func EapAuthComfirmRequestProcedure(updateEapSession models.EapSession, eapSessionID string) (*models.EapSession,
	*models.ProblemDetails,
) {
	var responseBody models.EapSession

	if !ausf_context.CheckIfSuciSupiPairExists(eapSessionID) {
		logger.Auth5gAkaComfirmLog.Infoln("supiSuciPair does not exist, confirmation failed")
		var problemDetails models.ProblemDetails
		problemDetails.Cause = USER_NOT_FOUND_ERROR
		return nil, &problemDetails
	}

	currentSupi := ausf_context.GetSupiFromSuciSupiMap(eapSessionID)
	if !ausf_context.CheckIfAusfUeContextExists(currentSupi) {
		logger.Auth5gAkaComfirmLog.Infoln("SUPI does not exist, confirmation failed")
		var problemDetails models.ProblemDetails
		problemDetails.Cause = USER_NOT_FOUND_ERROR
		return nil, &problemDetails
	}

	ausfCurrentContext := ausf_context.GetAusfUeContext(currentSupi)
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
		logConfirmFailureAndInformUDM(eapSessionID, models.AuthType_EAP_AKA_PRIME, servingNetworkName,
			"eap packet code error", ausfCurrentContext.UdmUeauUrl)
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
			logConfirmFailureAndInformUDM(eapSessionID, models.AuthType_EAP_AKA_PRIME, servingNetworkName,
				"eap packet decode error", ausfCurrentContext.UdmUeauUrl)
			failEapAkaNoti := ConstructFailEapAkaNotification(eapContent.Id)
			responseBody.EapPayload = failEapAkaNoti
		} else if XRES == string(RES) { // decodeOK && XRES == res, auth success
			logger.EapAuthComfirmLog.Infoln("Correct RES value, EAP-AKA' auth succeed")
			responseBody.AuthResult = models.AuthResult_SUCCESS
			eapSuccPkt := ConstructEapNoTypePkt(radius.EapCodeSuccess, eapContent.Id)
			responseBody.EapPayload = eapSuccPkt
			udmUrl := ausfCurrentContext.UdmUeauUrl
			if sendErr := sendAuthResultToUDM(eapSessionID, models.AuthType_EAP_AKA_PRIME, true, servingNetworkName,
				udmUrl); sendErr != nil {
				logger.EapAuthComfirmLog.Infoln(sendErr.Error())
				var problemDetails models.ProblemDetails
				problemDetails.Cause = UPSTREAM_SERVER_ERROR
				return nil, &problemDetails
			}
			ausfCurrentContext.AuthStatus = models.AuthResult_SUCCESS
		} else {
			ausfCurrentContext.AuthStatus = models.AuthResult_FAILURE
			responseBody.AuthResult = models.AuthResult_ONGOING
			logConfirmFailureAndInformUDM(eapSessionID, models.AuthType_EAP_AKA_PRIME, servingNetworkName,
				"Wrong RES value, EAP-AKA' auth failed", ausfCurrentContext.UdmUeauUrl)
			failEapAkaNoti := ConstructFailEapAkaNotification(eapContent.Id)
			responseBody.EapPayload = failEapAkaNoti
		}

	case models.AuthResult_FAILURE:
		eapFailPkt := ConstructEapNoTypePkt(radius.EapCodeFailure, eapPayload[1])
		responseBody.EapPayload = eapFailPkt
		responseBody.AuthResult = models.AuthResult_FAILURE
	}

	return &responseBody, nil
}
