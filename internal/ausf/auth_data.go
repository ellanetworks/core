package ausf

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/ueauth"
	"go.opentelemetry.io/otel/attribute"
)

const (
	SqnMAx int64 = 0x7FFFFFFFFFF
	ind    int64 = 32
)

func aucSQN(opc, k, auts, rand []byte) ([]byte, []byte, error) {
	AK, SQNms := make([]byte, 6), make([]byte, 6)
	macS := make([]byte, 8)
	ConcSQNms := auts[:6]

	AMF, err := hex.DecodeString("0000")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode AMF: %w", err)
	}

	err = F2345(opc, k, rand, nil, nil, nil, nil, AK)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate AK: %w", err)
	}

	for i := range 6 {
		SQNms[i] = AK[i] ^ ConcSQNms[i]
	}

	err = F1(opc, k, rand, SQNms, AMF, nil, macS)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate macS: %w", err)
	}

	return SQNms, macS, nil
}

func strictHex(s string, n int) string {
	l := len(s)

	if l < n {
		return strings.Repeat("0", n-l) + s
	}

	return s[l-n : l]
}

func CreateAuthData(ctx context.Context, authInfoRequest models.AuthenticationInfoRequest, suci string) (*models.AuthenticationInfoResult, error) {
	ctx, span := tracer.Start(ctx, "AUSF CreateAuthData")
	defer span.End()

	span.SetAttributes(
		attribute.String("suci", suci),
	)

	if ausfContext.DBInstance == nil {
		return nil, fmt.Errorf("db instance is nil")
	}

	hnPrivateKey, err := ausfContext.DBInstance.GetHomeNetworkPrivateKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("couldn't get home network private key: %w", err)
	}

	supi, err := ToSupi(suci, hnPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("couldn't convert suci to supi: %w", err)
	}

	subscriber, err := ausfContext.DBInstance.GetSubscriber(ctx, supi)
	if err != nil {
		return nil, fmt.Errorf("couldn't get subscriber %s: %v", supi, err)
	}

	/*
		K, RAND, CK, IK: 128 bits (16 bytes) (hex len = 32)
		SQN, AK: 48 bits (6 bytes) (hex len = 12) TS33.102 - 6.3.2
		AMF: 16 bits (2 bytes) (hex len = 4) TS33.102 - Annex H
	*/

	if subscriber.PermanentKey == "" {
		return nil, fmt.Errorf("permanent key is nil")
	}

	if subscriber.Opc == "" {
		return nil, fmt.Errorf("opc is nil")
	}

	k, err := hex.DecodeString(subscriber.PermanentKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode k: %w", err)
	}

	opc, err := hex.DecodeString(subscriber.Opc)
	if err != nil {
		return nil, fmt.Errorf("failed to decode opc: %w", err)
	}

	sqnStr := strictHex(subscriber.SequenceNumber, 12)

	RAND := make([]byte, 16)

	_, err = rand.Read(RAND)
	if err != nil {
		return nil, fmt.Errorf("rand read error: %w", err)
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

	err = ausfContext.DBInstance.EditSubscriberSequenceNumber(ctx, supi, SQNheStr)
	if err != nil {
		return nil, fmt.Errorf("couldn't update subscriber %s: %v", supi, err)
	}

	// Run milenage
	macA, macS := make([]byte, 8), make([]byte, 8)
	CK, IK := make([]byte, 16), make([]byte, 16)
	RES := make([]byte, 8)
	AK, AKstar := make([]byte, 6), make([]byte, 6)

	amf, err := hex.DecodeString("8000")
	if err != nil {
		return nil, fmt.Errorf("amf decode error: %w", err)
	}

	// Generate macA, macS
	err = F1(opc, k, RAND, sqn, amf, macA, macS)
	if err != nil {
		return nil, fmt.Errorf("milenage F1 err: %w", err)
	}

	// Generate RES, CK, IK, AK, AKstar
	// RES == XRES (expected RES) for server
	err = F2345(opc, k, RAND, RES, CK, IK, AK, AKstar)
	if err != nil {
		return nil, fmt.Errorf("milenage F2345 err: %w", err)
	}

	// Generate AUTN
	SQNxorAK := make([]byte, 6)

	for i := range sqn {
		SQNxorAK[i] = sqn[i] ^ AK[i]
	}

	AUTN := append(append(SQNxorAK, amf...), macA...)

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

	return &models.AuthenticationInfoResult{
		AuthenticationVector: &models.AuthenticationVector{
			Rand:     hex.EncodeToString(RAND),
			XresStar: hex.EncodeToString(xresStar),
			Autn:     hex.EncodeToString(AUTN),
			Kausf:    hex.EncodeToString(kdfValForKausf),
		},
		Supi: supi,
	}, nil
}
