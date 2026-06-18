// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package udm is Ella Core's converged home credential authority: the HSS/AuC
// (4G) and UDM/ARPF (5G) role. It owns subscriber credentials and the single
// SQN per subscriber, and generates authentication vectors for both the AUSF
// (5G HE AV) and the MME (EPS-AKA vector). Serving NFs are consumers; the
// long-term key K never leaves this package.
package udm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/etsi"
)

// Subscriber holds the authentication material the credential authority needs.
type Subscriber struct {
	PermanentKey   string
	Opc            string
	SequenceNumber string
}

// SubscriberStore provides credential access and SQN persistence (the AuC/ARPF
// database). The SQN update is the single per-subscriber counter shared by 4G
// and 5G.
type SubscriberStore interface {
	GetSubscriber(ctx context.Context, imsi string) (*Subscriber, error)
	UpdateSequenceNumber(ctx context.Context, imsi string, sqn string) error
}

// Service is the home credential authority.
type Service struct {
	store SubscriberStore
	keys  KeyResolver
}

// New returns a credential authority backed by store, using keys to deconceal
// SUCIs (SIDF).
func New(store SubscriberStore, keys KeyResolver) *Service {
	return &Service{store: store, keys: keys}
}

// HEAV5G is a 5G home-environment authentication vector (TS 33.501 §6.1.3.2.0):
// the UDM/ARPF output the AUSF transforms into the 5G AV.
type HEAV5G struct {
	SUPI     etsi.SUPI
	RAND     string // hex
	AUTN     string // hex
	XresStar string // hex
	Kausf    []byte
}

// authAMF is the authentication management field with the separation bit set
// (TS 33.501 §6.1.3.2.0 step 1, TS 33.102).
const authAMF = "8000"

// Generate5GHEAV deconceals the SUCI, advances and persists the subscriber's
// SQN, and returns a 5G HE AV. For a re-synchronisation pass the UE's AUTS and
// the RAND from the preceding challenge; otherwise pass empty strings.
func (s *Service) Generate5GHEAV(ctx context.Context, suci, servingNetwork, resyncAuts, resyncRand string) (*HEAV5G, error) {
	supi, err := ToSupi(suci, s.keys)
	if err != nil {
		return nil, fmt.Errorf("couldn't convert suci to supi: %w", err)
	}

	k, opc, sqn, err := s.advance(ctx, supi.IMSI(), resyncAuts, resyncRand)
	if err != nil {
		return nil, err
	}

	amf, err := hex.DecodeString(authAMF)
	if err != nil {
		return nil, fmt.Errorf("amf decode error: %w", err)
	}

	randBytes := make([]byte, 16)
	if _, err = rand.Read(randBytes); err != nil {
		return nil, fmt.Errorf("rand read error: %w", err)
	}

	macA, macS := make([]byte, 8), make([]byte, 8)
	ck, ik := make([]byte, 16), make([]byte, 16)
	res := make([]byte, 8)
	ak := make([]byte, 6)

	if err = F1(opc, k, randBytes, sqn, amf, macA, macS); err != nil {
		return nil, fmt.Errorf("milenage F1 err: %w", err)
	}

	if err = F2345(opc, k, randBytes, res, ck, ik, ak, nil); err != nil {
		return nil, fmt.Errorf("milenage F2345 err: %w", err)
	}

	sqnXorAK := make([]byte, 6)
	for i := range sqn {
		sqnXorAK[i] = sqn[i] ^ ak[i]
	}

	autn := append(append(append([]byte{}, sqnXorAK...), amf...), macA...)

	xresStar, err := DeriveXresStar(ck, ik, servingNetwork, randBytes, res)
	if err != nil {
		return nil, fmt.Errorf("XRES* derivation failed: %w", err)
	}

	kausf, err := DeriveKausf(ck, ik, servingNetwork, sqnXorAK)
	if err != nil {
		return nil, fmt.Errorf("kausf derivation failed: %w", err)
	}

	return &HEAV5G{
		SUPI:     supi,
		RAND:     hex.EncodeToString(randBytes),
		AUTN:     hex.EncodeToString(autn),
		XresStar: hex.EncodeToString(xresStar),
		Kausf:    kausf,
	}, nil
}

// advance fetches the subscriber's K/OPc, advances the SQN (re-synchronising
// from AUTS when provided), persists the new SQN, and returns K, OPc, and the
// SQN to use for the vector.
func (s *Service) advance(ctx context.Context, imsi, resyncAuts, resyncRand string) (k, opc, sqn []byte, err error) {
	sub, err := s.store.GetSubscriber(ctx, imsi)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("couldn't get subscriber %s: %w", imsi, err)
	}

	if sub.PermanentKey == "" || sub.Opc == "" {
		return nil, nil, nil, fmt.Errorf("subscriber %s missing key material", imsi)
	}

	if k, err = hex.DecodeString(sub.PermanentKey); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode k: %w", err)
	}

	if opc, err = hex.DecodeString(sub.Opc); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode opc: %w", err)
	}

	var nextSQN string

	if resyncAuts != "" {
		auts, err := hex.DecodeString(resyncAuts)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("could not decode auts: %w", err)
		}

		randBytes, err := hex.DecodeString(resyncRand)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("could not decode rand: %w", err)
		}

		sqnMsHex, err := resyncSQN(opc, k, auts, randBytes)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("SQN resync failed for %s: %w", imsi, err)
		}

		// TS 33.102 §C.3.4: after resync, advance by IND+1 to the next IND slot.
		if nextSQN, err = AdvanceSQN(sqnMsHex, IndStep+1); err != nil {
			return nil, nil, nil, fmt.Errorf("SQN advance failed: %w", err)
		}
	} else if nextSQN, err = AdvanceSQN(strictHex(sub.SequenceNumber, 12), IndStep); err != nil {
		return nil, nil, nil, fmt.Errorf("SQN increment failed: %w", err)
	}

	if sqn, err = hex.DecodeString(nextSQN); err != nil {
		return nil, nil, nil, fmt.Errorf("error decoding sqn: %w", err)
	}

	if err = s.store.UpdateSequenceNumber(ctx, imsi, nextSQN); err != nil {
		return nil, nil, nil, fmt.Errorf("couldn't update subscriber %s: %w", imsi, err)
	}

	return k, opc, sqn, nil
}
