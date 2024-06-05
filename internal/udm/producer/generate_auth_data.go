package producer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"reflect"
	"strings"

	"github.com/antihax/optional"
	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/Nudr_DataRepository"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	"github.com/omec-project/util/milenage"
	"github.com/omec-project/util/ueauth"
	"github.com/omec-project/util/util_3gpp/suci"
	udm_context "github.com/yeastengine/canard/internal/udm/context"
	"github.com/yeastengine/canard/internal/udm/logger"
	"github.com/yeastengine/canard/internal/udm/util"
)

const (
	SqnMAx    int64 = 0x7FFFFFFFFFF
	ind       int64 = 32
	keyStrLen int   = 32
	opStrLen  int   = 32
	opcStrLen int   = 32
)

const (
	authenticationRejected string = "AUTHENTICATION_REJECTED"
)

func aucSQN(opc, k, auts, rand []byte) ([]byte, []byte) {
	AK, SQNms := make([]byte, 6), make([]byte, 6)
	macS := make([]byte, 8)
	ConcSQNms := auts[:6]
	AMF, err := hex.DecodeString("0000")
	if err != nil {
		return nil, nil
	}

	logger.UeauLog.Traceln("ConcSQNms", ConcSQNms)

	err = milenage.F2345(opc, k, rand, nil, nil, nil, nil, AK)
	if err != nil {
		logger.UeauLog.Errorln("milenage F2345 err ", err)
	}

	for i := 0; i < 6; i++ {
		SQNms[i] = AK[i] ^ ConcSQNms[i]
	}

	// fmt.Printf("opc=%x\n", opc)
	// fmt.Printf("k=%x\n", k)
	// fmt.Printf("rand=%x\n", rand)
	// fmt.Printf("AMF %x\n", AMF)
	// fmt.Printf("SQNms %x\n", SQNms)
	err = milenage.F1(opc, k, rand, SQNms, AMF, nil, macS)
	if err != nil {
		logger.UeauLog.Errorln("milenage F1 err ", err)
	}
	// fmt.Printf("macS %x\n", macS)

	logger.UeauLog.Traceln("SQNms", SQNms)
	logger.UeauLog.Traceln("macS", macS)
	return SQNms, macS
}

func strictHex(s string, n int) string {
	l := len(s)
	if l < n {
		return fmt.Sprintf(strings.Repeat("0", n-l) + s)
	} else {
		return s[l-n : l]
	}
}

func HandleGenerateAuthDataRequest(request *httpwrapper.Request) *httpwrapper.Response {
	// step 1: log
	logger.UeauLog.Infoln("Handle GenerateAuthDataRequest")

	// step 2: retrieve request
	authInfoRequest := request.Body.(models.AuthenticationInfoRequest)
	supiOrSuci := request.Params["supiOrSuci"]

	// step 3: handle the message
	response, problemDetails := GenerateAuthDataProcedure(authInfoRequest, supiOrSuci)

	// step 4: process the return value from step 3
	if response != nil {
		// status code is based on SPEC, and option headers
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

func HandleConfirmAuthDataRequest(request *httpwrapper.Request) *httpwrapper.Response {
	logger.UeauLog.Infoln("Handle ConfirmAuthDataRequest")

	authEvent := request.Body.(models.AuthEvent)
	supi := request.Params["supi"]

	problemDetails := ConfirmAuthDataProcedure(authEvent, supi)

	if problemDetails != nil {
		return httpwrapper.NewResponse(int(problemDetails.Status), nil, problemDetails)
	} else {
		return httpwrapper.NewResponse(http.StatusCreated, nil, nil)
	}
}

func ConfirmAuthDataProcedure(authEvent models.AuthEvent, supi string) (problemDetails *models.ProblemDetails) {
	var createAuthParam Nudr_DataRepository.CreateAuthenticationStatusParamOpts
	optInterface := optional.NewInterface(authEvent)
	createAuthParam.AuthEvent = optInterface

	client, err := createUDMClientToUDR(supi)
	if err != nil {
		return util.ProblemDetailsSystemFailure(err.Error())
	}
	resp, err := client.AuthenticationStatusDocumentApi.CreateAuthenticationStatus(
		context.Background(), supi, &createAuthParam)
	if err != nil {
		problemDetails = &models.ProblemDetails{
			Status: int32(resp.StatusCode),
			Cause:  err.(openapi.GenericOpenAPIError).Model().(models.ProblemDetails).Cause,
			Detail: err.Error(),
		}

		logger.UeauLog.Errorln("[ConfirmAuth] ", err.Error())
		return problemDetails
	}
	defer func() {
		if rspCloseErr := resp.Body.Close(); rspCloseErr != nil {
			logger.UeauLog.Errorf("CreateAuthenticationStatus response body cannot close: %+v", rspCloseErr)
		}
	}()

	return nil
}

func GenerateAuthDataProcedure(authInfoRequest models.AuthenticationInfoRequest, supiOrSuci string) (
	response *models.AuthenticationInfoResult, problemDetails *models.ProblemDetails,
) {
	logger.UeauLog.Traceln("In GenerateAuthDataProcedure")

	response = &models.AuthenticationInfoResult{}
	supi, err := suci.ToSupi(supiOrSuci, udm_context.UDM_Self().SuciProfiles)
	if err != nil {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusForbidden,
			Cause:  authenticationRejected,
			Detail: err.Error(),
		}

		logger.UeauLog.Errorln("suciToSupi error: ", err.Error())
		return nil, problemDetails
	}

	logger.UeauLog.Tracef("supi conversion => %s\n", supi)

	client, err := createUDMClientToUDR(supi)
	if err != nil {
		return nil, util.ProblemDetailsSystemFailure(err.Error())
	}
	authSubs, res, err := client.AuthenticationDataDocumentApi.QueryAuthSubsData(context.Background(), supi, nil)
	if err != nil {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusForbidden,
			Cause:  authenticationRejected,
			Detail: err.Error(),
		}

		logger.UeauLog.Errorln("Return from UDR QueryAuthSubsData error")
		return nil, problemDetails
	}
	defer func() {
		if rspCloseErr := res.Body.Close(); rspCloseErr != nil {
			logger.SdmLog.Errorf("QueryAuthSubsData response body cannot close: %+v", rspCloseErr)
		}
	}()

	/*
		K, RAND, CK, IK: 128 bits (16 bytes) (hex len = 32)
		SQN, AK: 48 bits (6 bytes) (hex len = 12) TS33.102 - 6.3.2
		AMF: 16 bits (2 bytes) (hex len = 4) TS33.102 - Annex H
	*/

	hasK, hasOP, hasOPC := false, false, false

	var kStr, opStr, opcStr string

	k, op, opc := make([]byte, 16), make([]byte, 16), make([]byte, 16)

	logger.UeauLog.Traceln("K", k)

	if authSubs.PermanentKey != nil {
		kStr = authSubs.PermanentKey.PermanentKeyValue
		if len(kStr) == keyStrLen {
			k, err = hex.DecodeString(kStr)
			if err != nil {
				logger.UeauLog.Errorln("err", err)
			} else {
				hasK = true
			}
		} else {
			problemDetails = &models.ProblemDetails{
				Status: http.StatusForbidden,
				Cause:  authenticationRejected,
			}

			logger.UeauLog.Errorln("kStr length is ", len(kStr))
			return nil, problemDetails
		}
	} else {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusForbidden,
			Cause:  authenticationRejected,
		}

		logger.UeauLog.Errorln("Nil PermanentKey")
		return nil, problemDetails
	}

	if authSubs.Milenage != nil {
		if authSubs.Milenage.Op != nil {
			opStr = authSubs.Milenage.Op.OpValue
			if len(opStr) == opStrLen {
				op, err = hex.DecodeString(opStr)
				if err != nil {
					logger.UeauLog.Errorln("err", err)
				} else {
					hasOP = true
				}
			} else {
				logger.UeauLog.Errorln("opStr length is ", len(opStr))
			}
		} else {
			logger.UeauLog.Infoln("Nil Op")
		}
	} else {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusForbidden,
			Cause:  authenticationRejected,
		}

		logger.UeauLog.Infoln("Nil Milenage")
		return nil, problemDetails
	}

	if authSubs.Opc != nil && authSubs.Opc.OpcValue != "" {
		opcStr = authSubs.Opc.OpcValue
		if len(opcStr) == opcStrLen {
			opc, err = hex.DecodeString(opcStr)
			if err != nil {
				logger.UeauLog.Errorln("err", err)
			} else {
				hasOPC = true
			}
		} else {
			logger.UeauLog.Errorln("opcStr length is ", len(opcStr))
		}
	} else {
		logger.UeauLog.Infoln("Nil Opc")
	}

	if !hasOPC && !hasOP {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusForbidden,
			Cause:  authenticationRejected,
		}

		return nil, problemDetails
	}

	if !hasOPC {
		if hasK && hasOP {
			opc, err = milenage.GenerateOPC(k, op)
			if err != nil {
				logger.UeauLog.Errorln("milenage GenerateOPC err ", err)
			}
		} else {
			problemDetails = &models.ProblemDetails{
				Status: http.StatusForbidden,
				Cause:  authenticationRejected,
			}

			logger.UeauLog.Errorln("Unable to derive OPC")
			return nil, problemDetails
		}
	}

	sqnStr := strictHex(authSubs.SequenceNumber, 12)
	logger.UeauLog.Traceln("sqnStr", sqnStr)
	sqn, err := hex.DecodeString(sqnStr)
	if err != nil {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusForbidden,
			Cause:  authenticationRejected,
			Detail: err.Error(),
		}

		logger.UeauLog.Errorln("err", err)
		return nil, problemDetails
	}

	logger.UeauLog.Traceln("sqn", sqn)
	// fmt.Printf("K=%x\nsqn=%x\nOP=%x\nOPC=%x\n", K, sqn, OP, OPC)

	RAND := make([]byte, 16)
	_, err = rand.Read(RAND)
	if err != nil {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusForbidden,
			Cause:  authenticationRejected,
			Detail: err.Error(),
		}

		logger.UeauLog.Errorln("err", err)
		return nil, problemDetails
	}

	AMF, err := hex.DecodeString("8000")
	if err != nil {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusForbidden,
			Cause:  authenticationRejected,
			Detail: err.Error(),
		}

		logger.UeauLog.Errorln("err", err)
		return nil, problemDetails
	}

	// fmt.Printf("RAND=%x\nAMF=%x\n", RAND, AMF)

	// for test
	// RAND, _ = hex.DecodeString(TestGenAuthData.MilenageTestSet19.RAND)
	// AMF, _ = hex.DecodeString(TestGenAuthData.MilenageTestSet19.AMF)
	// fmt.Printf("For test: RAND=%x, AMF=%x\n", RAND, AMF)

	// re-synchroniztion
	if authInfoRequest.ResynchronizationInfo != nil {
		Auts, deCodeErr := hex.DecodeString(authInfoRequest.ResynchronizationInfo.Auts)
		if deCodeErr != nil {
			problemDetails = &models.ProblemDetails{
				Status: http.StatusForbidden,
				Cause:  authenticationRejected,
				Detail: deCodeErr.Error(),
			}

			logger.UeauLog.Errorln("err", deCodeErr)
			return nil, problemDetails
		}

		randHex, deCodeErr := hex.DecodeString(authInfoRequest.ResynchronizationInfo.Rand)
		if deCodeErr != nil {
			problemDetails = &models.ProblemDetails{
				Status: http.StatusForbidden,
				Cause:  authenticationRejected,
				Detail: deCodeErr.Error(),
			}

			logger.UeauLog.Errorln("err", deCodeErr)
			return nil, problemDetails
		}

		SQNms, macS := aucSQN(opc, k, Auts, randHex)
		if reflect.DeepEqual(macS, Auts[6:]) {
			_, err = rand.Read(RAND)
			if err != nil {
				problemDetails = &models.ProblemDetails{
					Status: http.StatusForbidden,
					Cause:  authenticationRejected,
					Detail: err.Error(),
				}

				logger.UeauLog.Errorln("err", err)
				return nil, problemDetails
			}

			// increment sqn authSubs.SequenceNumber
			bigSQN := big.NewInt(0)
			sqnStr = hex.EncodeToString(SQNms)
			fmt.Printf("SQNstr %s\n", sqnStr)
			bigSQN.SetString(sqnStr, 16)

			bigInc := big.NewInt(ind + 1)

			bigP := big.NewInt(SqnMAx)
			bigSQN = bigInc.Add(bigSQN, bigInc)
			bigSQN = bigSQN.Mod(bigSQN, bigP)
			sqnStr = fmt.Sprintf("%x", bigSQN)
			sqnStr = strictHex(sqnStr, 12)
		} else {
			logger.UeauLog.Errorln("Re-Sync MAC failed ", supi)
			logger.UeauLog.Errorln("MACS ", macS)
			logger.UeauLog.Errorln("Auts[6:] ", Auts[6:])
			logger.UeauLog.Errorln("Sqn ", SQNms)
			problemDetails = &models.ProblemDetails{
				Status: http.StatusForbidden,
				Cause:  "modification is rejected",
			}
			return nil, problemDetails
		}
	}

	// increment sqn
	bigSQN := big.NewInt(0)
	sqn, err = hex.DecodeString(sqnStr)
	if err != nil {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusForbidden,
			Cause:  authenticationRejected,
			Detail: err.Error(),
		}

		logger.UeauLog.Errorln("err", err)
		return nil, problemDetails
	}

	bigSQN.SetString(sqnStr, 16)

	bigInc := big.NewInt(1)
	bigSQN = bigInc.Add(bigSQN, bigInc)

	SQNheStr := fmt.Sprintf("%x", bigSQN)
	SQNheStr = strictHex(SQNheStr, 12)
	patchItemArray := []models.PatchItem{
		{
			Op:    models.PatchOperation_REPLACE,
			Path:  "/sequenceNumber",
			Value: SQNheStr,
		},
	}

	var rsp *http.Response
	rsp, err = client.AuthenticationDataDocumentApi.ModifyAuthentication(
		context.Background(), supi, patchItemArray)
	if err != nil {
		problemDetails = &models.ProblemDetails{
			Status: http.StatusForbidden,
			Cause:  "modification is rejected ",
			Detail: err.Error(),
		}

		logger.UeauLog.Errorln("update sqn error", err)
		return nil, problemDetails
	}
	defer func() {
		if rspCloseErr := rsp.Body.Close(); rspCloseErr != nil {
			logger.SdmLog.Errorf("ModifyAuthentication response body cannot close: %+v", rspCloseErr)
		}
	}()

	// Run milenage
	macA, macS := make([]byte, 8), make([]byte, 8)
	CK, IK := make([]byte, 16), make([]byte, 16)
	RES := make([]byte, 8)
	AK, AKstar := make([]byte, 6), make([]byte, 6)

	// Generate macA, macS
	err = milenage.F1(opc, k, RAND, sqn, AMF, macA, macS)
	if err != nil {
		logger.UeauLog.Errorln("milenage F1 err ", err)
	}

	// Generate RES, CK, IK, AK, AKstar
	// RES == XRES (expected RES) for server
	err = milenage.F2345(opc, k, RAND, RES, CK, IK, AK, AKstar)
	if err != nil {
		logger.UeauLog.Errorln("milenage F2345 err ", err)
	}
	// fmt.Printf("milenage RES = %s\n", hex.EncodeToString(RES))

	// Generate AUTN
	// fmt.Printf("SQN=%x\nAK =%x\n", SQN, AK)
	// fmt.Printf("AMF=%x, macA=%x\n", AMF, macA)
	SQNxorAK := make([]byte, 6)
	for i := 0; i < len(sqn); i++ {
		SQNxorAK[i] = sqn[i] ^ AK[i]
	}
	// fmt.Printf("SQN xor AK = %x\n", SQNxorAK)
	AUTN := append(append(SQNxorAK, AMF...), macA...)
	fmt.Printf("AUTN = %x\n", AUTN)

	var av models.AuthenticationVector
	if authSubs.AuthenticationMethod == models.AuthMethod__5_G_AKA {
		response.AuthType = models.AuthType__5_G_AKA

		// derive XRES*
		key := append(CK, IK...)
		FC := ueauth.FC_FOR_RES_STAR_XRES_STAR_DERIVATION
		P0 := []byte(authInfoRequest.ServingNetworkName)
		P1 := RAND
		P2 := RES

		kdfValForXresStar, err := ueauth.GetKDFValue(
			key, FC, P0, ueauth.KDFLen(P0), P1, ueauth.KDFLen(P1), P2, ueauth.KDFLen(P2))
		if err != nil {
			logger.UeauLog.Error(err)
		}
		xresStar := kdfValForXresStar[len(kdfValForXresStar)/2:]
		// fmt.Printf("xresStar = %x\n", xresStar)

		// derive Kausf
		FC = ueauth.FC_FOR_KAUSF_DERIVATION
		P0 = []byte(authInfoRequest.ServingNetworkName)
		P1 = SQNxorAK
		kdfValForKausf, err := ueauth.GetKDFValue(key, FC, P0, ueauth.KDFLen(P0), P1, ueauth.KDFLen(P1))
		if err != nil {
			logger.UeauLog.Error(err)
		}
		// fmt.Printf("Kausf = %x\n", kdfValForKausf)

		// Fill in rand, xresStar, autn, kausf
		av.Rand = hex.EncodeToString(RAND)
		av.XresStar = hex.EncodeToString(xresStar)
		av.Autn = hex.EncodeToString(AUTN)
		av.Kausf = hex.EncodeToString(kdfValForKausf)
	} else { // EAP-AKA'
		response.AuthType = models.AuthType_EAP_AKA_PRIME

		// derive CK' and IK'
		key := append(CK, IK...)
		FC := ueauth.FC_FOR_CK_PRIME_IK_PRIME_DERIVATION
		P0 := []byte(authInfoRequest.ServingNetworkName)
		P1 := SQNxorAK
		kdfVal, err := ueauth.GetKDFValue(key, FC, P0, ueauth.KDFLen(P0), P1, ueauth.KDFLen(P1))
		if err != nil {
			logger.UeauLog.Error(err)
		}
		// fmt.Printf("kdfVal = %x (len = %d)\n", kdfVal, len(kdfVal))

		// For TS 35.208 test set 19 & RFC 5448 test vector 1
		// CK': 0093 962d 0dd8 4aa5 684b 045c 9edf fa04
		// IK': ccfc 230c a74f cc96 c0a5 d611 64f5 a76

		ckPrime := kdfVal[:len(kdfVal)/2]
		ikPrime := kdfVal[len(kdfVal)/2:]
		// fmt.Printf("ckPrime: %x\nikPrime: %x\n", ckPrime, ikPrime)

		// Fill in rand, xres, autn, ckPrime, ikPrime
		av.Rand = hex.EncodeToString(RAND)
		av.Xres = hex.EncodeToString(RES)
		av.Autn = hex.EncodeToString(AUTN)
		av.CkPrime = hex.EncodeToString(ckPrime)
		av.IkPrime = hex.EncodeToString(ikPrime)
	}

	response.AuthenticationVector = &av
	response.Supi = supi
	return response, nil
}
