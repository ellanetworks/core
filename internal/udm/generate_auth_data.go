// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package udm

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/udr"
	"github.com/ellanetworks/core/internal/util/milenage"
	"github.com/ellanetworks/core/internal/util/suci"
	"github.com/ellanetworks/core/internal/util/ueauth"
	"github.com/omec-project/openapi/models"
)

const (
	SqnMAx    int64 = 0x7FFFFFFFFFF
	ind       int64 = 32
	keyStrLen int   = 32
	opStrLen  int   = 32
	opcStrLen int   = 32
)

func aucSQN(opc, k, auts, rand []byte) ([]byte, []byte) {
	AK, SQNms := make([]byte, 6), make([]byte, 6)
	macS := make([]byte, 8)
	ConcSQNms := auts[:6]
	AMF, err := hex.DecodeString("0000")
	if err != nil {
		return nil, nil
	}

	logger.UdmLog.Debugln("ConcSQNms", ConcSQNms)

	err = milenage.F2345(opc, k, rand, nil, nil, nil, nil, AK)
	if err != nil {
		logger.UdmLog.Errorln("milenage F2345 err ", err)
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
		logger.UdmLog.Errorln("milenage F1 err ", err)
	}
	// fmt.Printf("macS %x\n", macS)

	logger.UdmLog.Debugln("SQNms", SQNms)
	logger.UdmLog.Debugln("macS", macS)
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

func CreateAuthData(authInfoRequest models.AuthenticationInfoRequest, supiOrSuci string) (
	*models.AuthenticationInfoResult, error,
) {
	response := &models.AuthenticationInfoResult{}
	supi, err := suci.ToSupi(supiOrSuci, udmContext.SuciProfiles)
	if err != nil {
		return nil, fmt.Errorf("suciToSupi error: %w", err)
	}

	authSubs, err := udr.GetAuthSubsData(supi)
	if err != nil {
		return nil, fmt.Errorf("couldn't get authentication subscriber data: %w", err)
	}

	/*
		K, RAND, CK, IK: 128 bits (16 bytes) (hex len = 32)
		SQN, AK: 48 bits (6 bytes) (hex len = 12) TS33.102 - 6.3.2
		AMF: 16 bits (2 bytes) (hex len = 4) TS33.102 - Annex H
	*/

	hasK, hasOP, hasOPC := false, false, false

	var kStr, opStr, opcStr string

	k, op, opc := make([]byte, 16), make([]byte, 16), make([]byte, 16)

	logger.UdmLog.Debugln("K", k)

	if authSubs.PermanentKey != nil {
		kStr = authSubs.PermanentKey.PermanentKeyValue
		if len(kStr) == keyStrLen {
			k, err = hex.DecodeString(kStr)
			if err != nil {
				logger.UdmLog.Errorln("err", err)
			} else {
				hasK = true
			}
		} else {
			return nil, fmt.Errorf("kStr length is %d", len(kStr))
		}
	} else {
		return nil, fmt.Errorf("nil PermanentKey")
	}

	if authSubs.Milenage != nil {
		if authSubs.Milenage.Op != nil {
			opStr = authSubs.Milenage.Op.OpValue
			if len(opStr) == opStrLen {
				op, err = hex.DecodeString(opStr)
				if err != nil {
					logger.UdmLog.Errorln("err", err)
				} else {
					hasOP = true
				}
			} else {
				logger.UdmLog.Warnf("opStr is of length %d", len(opStr))
			}
		} else {
			logger.UdmLog.Infoln("milenage Op is nil")
		}
	} else {
		return nil, fmt.Errorf("nil Milenage")
	}

	if authSubs.Opc != nil && authSubs.Opc.OpcValue != "" {
		opcStr = authSubs.Opc.OpcValue
		if len(opcStr) == opcStrLen {
			opc, err = hex.DecodeString(opcStr)
			if err != nil {
				logger.UdmLog.Errorln("err", err)
			} else {
				hasOPC = true
			}
		} else {
			logger.UdmLog.Errorln("opcStr length is ", len(opcStr))
		}
	} else {
		logger.UdmLog.Infoln("Nil Opc")
	}

	if !hasOPC && !hasOP {
		return nil, fmt.Errorf("unable to derive OP")
	}

	if !hasOPC {
		if hasK && hasOP {
			opc, err = milenage.GenerateOPC(k, op)
			if err != nil {
				logger.UdmLog.Errorln("milenage GenerateOPC err ", err)
			}
		} else {
			return nil, fmt.Errorf("unable to derive OPC")
		}
	}

	sqnStr := strictHex(authSubs.SequenceNumber, 12)
	logger.UdmLog.Debugln("sqnStr", sqnStr)
	sqn, err := hex.DecodeString(sqnStr)
	if err != nil {
		return nil, fmt.Errorf("sqnStr decode error: %w", err)
	}

	logger.UdmLog.Debugln("sqn", sqn)
	// fmt.Printf("K=%x\nsqn=%x\nOP=%x\nOPC=%x\n", K, sqn, OP, OPC)

	RAND := make([]byte, 16)
	_, err = rand.Read(RAND)
	if err != nil {
		return nil, fmt.Errorf("rand read error: %w", err)
	}

	AMF, err := hex.DecodeString("8000")
	if err != nil {
		return nil, fmt.Errorf("AMF decode error: %w", err)
	}

	// re-synchroniztion
	if authInfoRequest.ResynchronizationInfo != nil {
		Auts, deCodeErr := hex.DecodeString(authInfoRequest.ResynchronizationInfo.Auts)
		if deCodeErr != nil {
			return nil, fmt.Errorf("auts decode error: %w", deCodeErr)
		}

		randHex, deCodeErr := hex.DecodeString(authInfoRequest.ResynchronizationInfo.Rand)
		if deCodeErr != nil {
			return nil, fmt.Errorf("randHex decode error: %w", deCodeErr)
		}

		SQNms, macS := aucSQN(opc, k, Auts, randHex)
		if reflect.DeepEqual(macS, Auts[6:]) {
			_, err = rand.Read(RAND)
			if err != nil {
				return nil, fmt.Errorf("rand read error: %w", err)
			}

			// increment sqn authSubs.SequenceNumber
			bigSQN := big.NewInt(0)
			sqnStr = hex.EncodeToString(SQNms)
			bigSQN.SetString(sqnStr, 16)

			bigInc := big.NewInt(ind + 1)

			bigP := big.NewInt(SqnMAx)
			bigSQN = bigInc.Add(bigSQN, bigInc)
			bigSQN = bigSQN.Mod(bigSQN, bigP)
			sqnStr = fmt.Sprintf("%x", bigSQN)
			sqnStr = strictHex(sqnStr, 12)
		} else {
			logger.UdmLog.Errorln("Re-Sync MAC failed ", supi)
			logger.UdmLog.Errorln("MACS ", macS)
			logger.UdmLog.Errorln("Auts[6:] ", Auts[6:])
			logger.UdmLog.Errorln("Sqn ", SQNms)
			return nil, fmt.Errorf("Re-Sync MAC failed")
		}
	}

	// increment sqn
	bigSQN := big.NewInt(0)
	sqn, err = hex.DecodeString(sqnStr)
	if err != nil {
		return nil, fmt.Errorf("sqn decode error: %w", err)
	}

	bigSQN.SetString(sqnStr, 16)

	bigInc := big.NewInt(1)
	bigSQN = bigInc.Add(bigSQN, bigInc)

	SQNheStr := fmt.Sprintf("%x", bigSQN)
	SQNheStr = strictHex(SQNheStr, 12)

	err = udr.EditAuthenticationSubscription(supi, SQNheStr)
	if err != nil {
		return nil, fmt.Errorf("update sqn error: %w", err)
	}

	// Run milenage
	macA, macS := make([]byte, 8), make([]byte, 8)
	CK, IK := make([]byte, 16), make([]byte, 16)
	RES := make([]byte, 8)
	AK, AKstar := make([]byte, 6), make([]byte, 6)

	// Generate macA, macS
	err = milenage.F1(opc, k, RAND, sqn, AMF, macA, macS)
	if err != nil {
		logger.UdmLog.Errorln("milenage F1 err ", err)
	}

	// Generate RES, CK, IK, AK, AKstar
	// RES == XRES (expected RES) for server
	err = milenage.F2345(opc, k, RAND, RES, CK, IK, AK, AKstar)
	if err != nil {
		logger.UdmLog.Errorln("milenage F2345 err ", err)
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
			logger.UdmLog.Error(err)
		}
		xresStar := kdfValForXresStar[len(kdfValForXresStar)/2:]

		// derive Kausf
		FC = ueauth.FC_FOR_KAUSF_DERIVATION
		P0 = []byte(authInfoRequest.ServingNetworkName)
		P1 = SQNxorAK
		kdfValForKausf, err := ueauth.GetKDFValue(key, FC, P0, ueauth.KDFLen(P0), P1, ueauth.KDFLen(P1))
		if err != nil {
			logger.UdmLog.Error(err)
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
			logger.UdmLog.Error(err)
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
