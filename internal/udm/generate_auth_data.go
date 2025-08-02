// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package udm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/milenage"
	"github.com/ellanetworks/core/internal/util/suci"
	"github.com/ellanetworks/core/internal/util/ueauth"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

const (
	SqnMAx    int64 = 0x7FFFFFFFFFF
	ind       int64 = 32
	keyStrLen int   = 32
	opStrLen  int   = 32
	opcStrLen int   = 32
)

const (
	AuthenticationManagementField = "8000"
	EncryptionAlgorithm           = 0
	EncryptionKey                 = 0
	OpValue                       = ""
)

func aucSQN(opc, k, auts, rand []byte) ([]byte, []byte, error) {
	AK, SQNms := make([]byte, 6), make([]byte, 6)
	macS := make([]byte, 8)
	ConcSQNms := auts[:6]
	AMF, err := hex.DecodeString("0000")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode AMF: %w", err)
	}

	err = milenage.F2345(opc, k, rand, nil, nil, nil, nil, AK)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate AK: %w", err)
	}

	for i := 0; i < 6; i++ {
		SQNms[i] = AK[i] ^ ConcSQNms[i]
	}

	err = milenage.F1(opc, k, rand, SQNms, AMF, nil, macS)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate macS: %w", err)
	}

	return SQNms, macS, nil
}

func strictHex(s string, n int) string {
	l := len(s)
	if l < n {
		return strings.Repeat("0", n-l) + s
	} else {
		return s[l-n : l]
	}
}

func EditAuthenticationSubscription(ctx context.Context, ueID string, sequenceNumber string) error {
	subscriber, err := udmContext.DBInstance.GetSubscriber(ctx, ueID)
	if err != nil {
		return fmt.Errorf("couldn't get subscriber %s: %v", ueID, err)
	}
	subscriber.SequenceNumber = sequenceNumber
	err = udmContext.DBInstance.UpdateSubscriber(ctx, subscriber)
	if err != nil {
		return fmt.Errorf("couldn't update subscriber %s: %v", ueID, err)
	}
	return nil
}

func convertDBAuthSubsDataToModel(opc string, key string, sequenceNumber string) *models.AuthenticationSubscription {
	authSubsData := &models.AuthenticationSubscription{}
	authSubsData.AuthenticationManagementField = AuthenticationManagementField
	authSubsData.AuthenticationMethod = models.AuthMethod5GAka
	authSubsData.Milenage = &models.Milenage{
		Op: &models.Op{
			EncryptionAlgorithm: EncryptionAlgorithm,
			EncryptionKey:       EncryptionKey,
			OpValue:             OpValue,
		},
	}
	authSubsData.Opc = &models.Opc{
		EncryptionAlgorithm: EncryptionAlgorithm,
		EncryptionKey:       EncryptionKey,
		OpcValue:            opc,
	}
	authSubsData.PermanentKey = &models.PermanentKey{
		EncryptionAlgorithm: EncryptionAlgorithm,
		EncryptionKey:       EncryptionKey,
		PermanentKeyValue:   key,
	}
	authSubsData.SequenceNumber = sequenceNumber

	return authSubsData
}

func GetAuthSubsData(ctx context.Context, ueID string) (*models.AuthenticationSubscription, error) {
	subscriber, err := udmContext.DBInstance.GetSubscriber(ctx, ueID)
	if err != nil {
		return nil, fmt.Errorf("couldn't get subscriber %s: %v", ueID, err)
	}
	authSubsData := convertDBAuthSubsDataToModel(subscriber.Opc, subscriber.PermanentKey, subscriber.SequenceNumber)
	return authSubsData, nil
}

func CreateAuthData(ctx context.Context, authInfoRequest models.AuthenticationInfoRequest, suc string) (*models.AuthenticationInfoResult, error) {
	ctx, span := tracer.Start(ctx, "UDM CreateAuthData")
	defer span.End()
	span.SetAttributes(
		attribute.String("suci", suc),
	)
	if udmContext.DBInstance == nil {
		return nil, fmt.Errorf("db instance is nil")
	}
	hnPrivateKey, err := udmContext.DBInstance.GetHomeNetworkPrivateKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("couldn't get home network private key: %w", err)
	}

	supi, err := suci.ToSupi(suc, hnPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("couldn't convert suci to supi: %w", err)
	}

	authSubs, err := GetAuthSubsData(ctx, supi)
	if err != nil {
		return nil, fmt.Errorf("couldn't get authentication subscriber data: %w", err)
	}

	/*
		K, RAND, CK, IK: 128 bits (16 bytes) (hex len = 32)
		SQN, AK: 48 bits (6 bytes) (hex len = 12) TS33.102 - 6.3.2
		AMF: 16 bits (2 bytes) (hex len = 4) TS33.102 - Annex H
	*/

	hasOP, hasOPC := false, false

	var kStr, opStr, opcStr string

	var k []byte
	op := make([]byte, 16)
	opc := make([]byte, 16)

	if authSubs.PermanentKey == nil {
		return nil, fmt.Errorf("permanent key is nil")
	}

	kStr = authSubs.PermanentKey.PermanentKeyValue
	if len(kStr) != keyStrLen {
		return nil, fmt.Errorf("kStr length is %d", len(kStr))
	}
	k, err = hex.DecodeString(kStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode k: %w", err)
	}

	if authSubs.Milenage == nil {
		return nil, fmt.Errorf("milenage is nil")
	}

	if authSubs.Milenage.Op != nil {
		opStr = authSubs.Milenage.Op.OpValue
		if len(opStr) == opStrLen {
			op, err = hex.DecodeString(opStr)
			if err != nil {
				return nil, fmt.Errorf("failed to decode op: %w", err)
			}
			hasOP = true
		}
	}

	if authSubs.Opc != nil && authSubs.Opc.OpcValue != "" {
		opcStr = authSubs.Opc.OpcValue
		if len(opcStr) == opcStrLen {
			opc, err = hex.DecodeString(opcStr)
			if err != nil {
				return nil, fmt.Errorf("failed to decode opc: %w", err)
			}
			hasOPC = true
		} else {
			logger.UdmLog.Error("opcStr length is not 32", zap.Int("len", len(opcStr)))
		}
	} else {
		logger.UdmLog.Info("nil Opc")
	}

	if !hasOPC && !hasOP {
		return nil, fmt.Errorf("unable to derive OP")
	}

	if hasOP && !hasOPC {
		opc, err = milenage.GenerateOPC(k, op)
		if err != nil {
			return nil, fmt.Errorf("failed to generate OPC: %w", err)
		}
	}

	sqnStr := strictHex(authSubs.SequenceNumber, 12)

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
		Auts, err := hex.DecodeString(authInfoRequest.ResynchronizationInfo.Auts)
		if err != nil {
			return nil, fmt.Errorf("could not decode auts: %w", err)
		}

		randHex, err := hex.DecodeString(authInfoRequest.ResynchronizationInfo.Rand)
		if err != nil {
			return nil, fmt.Errorf("could not decode rand: %w", err)
		}

		SQNms, macS, err := aucSQN(opc, k, Auts, randHex)
		if err != nil {
			return nil, fmt.Errorf("failed to re-sync SQN with supi %s: %w", supi, err)
		}
		if !reflect.DeepEqual(macS, Auts[6:]) {
			return nil, fmt.Errorf("failed to re-sync MAC with supi %s, macS %x, auts[6:] %x, sqn %x", supi, macS, Auts[6:], SQNms)
		}
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
	}

	// increment sqn
	bigSQN := big.NewInt(0)
	sqn, err := hex.DecodeString(sqnStr)
	if err != nil {
		return nil, fmt.Errorf("error decoding sqn: %w", err)
	}

	bigSQN.SetString(sqnStr, 16)

	bigInc := big.NewInt(1)
	bigSQN = bigInc.Add(bigSQN, bigInc)

	SQNheStr := fmt.Sprintf("%x", bigSQN)
	SQNheStr = strictHex(SQNheStr, 12)

	err = EditAuthenticationSubscription(ctx, supi, SQNheStr)
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
		return nil, fmt.Errorf("milenage F1 err: %w", err)
	}

	// Generate RES, CK, IK, AK, AKstar
	// RES == XRES (expected RES) for server
	err = milenage.F2345(opc, k, RAND, RES, CK, IK, AK, AKstar)
	if err != nil {
		return nil, fmt.Errorf("milenage F2345 err: %w", err)
	}

	// Generate AUTN
	SQNxorAK := make([]byte, 6)
	for i := 0; i < len(sqn); i++ {
		SQNxorAK[i] = sqn[i] ^ AK[i]
	}
	AUTN := append(append(SQNxorAK, AMF...), macA...)
	response := &models.AuthenticationInfoResult{}
	var av models.AuthenticationVector
	if authSubs.AuthenticationMethod == models.AuthMethod5GAka {
		response.AuthType = models.AuthType5GAka

		// derive XRES*
		key := append(CK, IK...)
		FC := ueauth.FCForResStarXresStarDerivation
		P0 := []byte(authInfoRequest.ServingNetworkName)
		P1 := RAND
		P2 := RES

		kdfValForXresStar, err := ueauth.GetKDFValue(key, FC, P0, ueauth.KDFLen(P0), P1, ueauth.KDFLen(P1), P2, ueauth.KDFLen(P2))
		if err != nil {
			return nil, fmt.Errorf("failed to get KDF value: %w", err)
		}
		xresStar := kdfValForXresStar[len(kdfValForXresStar)/2:]

		// derive Kausf
		FC = ueauth.FCForKausfDerivation
		P0 = []byte(authInfoRequest.ServingNetworkName)
		P1 = SQNxorAK
		kdfValForKausf, err := ueauth.GetKDFValue(key, FC, P0, ueauth.KDFLen(P0), P1, ueauth.KDFLen(P1))
		if err != nil {
			return nil, fmt.Errorf("failed to get KDF value: %w", err)
		}

		// Fill in rand, xresStar, autn, kausf
		av.Rand = hex.EncodeToString(RAND)
		av.XresStar = hex.EncodeToString(xresStar)
		av.Autn = hex.EncodeToString(AUTN)
		av.Kausf = hex.EncodeToString(kdfValForKausf)
	} else { // EAP-AKA'
		response.AuthType = models.AuthTypeEAPAkaPrime

		// derive CK' and IK'
		key := append(CK, IK...)
		FC := ueauth.FCForCkPrimeIkPrimeDerivation
		P0 := []byte(authInfoRequest.ServingNetworkName)
		P1 := SQNxorAK
		kdfVal, err := ueauth.GetKDFValue(key, FC, P0, ueauth.KDFLen(P0), P1, ueauth.KDFLen(P1))
		if err != nil {
			return nil, fmt.Errorf("failed to get KDF value: %w", err)
		}

		// For TS 35.208 test set 19 & RFC 5448 test vector 1
		// CK': 0093 962d 0dd8 4aa5 684b 045c 9edf fa04
		// IK': ccfc 230c a74f cc96 c0a5 d611 64f5 a76

		ckPrime := kdfVal[:len(kdfVal)/2]
		ikPrime := kdfVal[len(kdfVal)/2:]

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
