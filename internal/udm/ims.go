// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package udm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
)

var imsAuthAMF = []byte{0x00, 0x00}

type IMSAV struct {
	RAND []byte
	AUTN []byte
	XRES []byte
	CK   []byte
	IK   []byte
}

type IMSResync struct {
	RAND []byte
	AUTS []byte
}

type IMSSubscriberStore interface {
	AdvanceIMSSequenceNumber(ctx context.Context, imsi, resyncAuts, resyncRand string) (*AdvancedCredentials, error)
}

type IMSCredentials struct {
	store IMSSubscriberStore
}

func NewIMSCredentials(store IMSSubscriberStore) *IMSCredentials {
	return &IMSCredentials{store: store}
}

func (c *IMSCredentials) GenerateIMSVector(ctx context.Context, imsi string, resync *IMSResync) (*IMSAV, error) {
	var resyncAuts, resyncRand string
	if resync != nil {
		resyncAuts, resyncRand = hex.EncodeToString(resync.AUTS), hex.EncodeToString(resync.RAND)
	}

	creds, err := c.store.AdvanceIMSSequenceNumber(ctx, imsi, resyncAuts, resyncRand)
	if err != nil {
		return nil, fmt.Errorf("couldn't advance the IMS sequence number for subscriber %s: %w", imsi, err)
	}

	k, opc, sqn, err := decodeCredentials(creds)
	if err != nil {
		return nil, fmt.Errorf("subscriber %s: %w", imsi, err)
	}

	randBytes := make([]byte, 16)
	if _, err := rand.Read(randBytes); err != nil {
		return nil, fmt.Errorf("rand read error: %w", err)
	}

	return newIMSVector(k, opc, sqn, randBytes)
}

func newIMSVector(k, opc, sqn, randBytes []byte) (*IMSAV, error) {
	v, err := milenageVector(k, opc, sqn, imsAuthAMF, randBytes)
	if err != nil {
		return nil, err
	}

	return &IMSAV{RAND: randBytes, AUTN: v.autn, XRES: v.res, CK: v.ck, IK: v.ik}, nil
}

func decodeCredentials(creds *AdvancedCredentials) (k, opc, sqn []byte, err error) {
	if creds.PermanentKey == "" || creds.Opc == "" {
		return nil, nil, nil, errors.New("missing key material")
	}

	if k, err = hex.DecodeString(creds.PermanentKey); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode k: %w", err)
	}

	if opc, err = hex.DecodeString(creds.Opc); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode opc: %w", err)
	}

	if sqn, err = hex.DecodeString(creds.SequenceNumber); err != nil {
		return nil, nil, nil, fmt.Errorf("error decoding sqn: %w", err)
	}

	if len(sqn) != 6 {
		return nil, nil, nil, fmt.Errorf("%d-octet sqn", len(sqn))
	}

	return k, opc, sqn, nil
}
