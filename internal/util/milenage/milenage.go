// SPDX-FileCopyrightText: Ella Networks Inc.
// Copyright 2019 Communication Service/Software Laboratory, National Chiao Tung University (free5gc.org)
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package milenage implements the 3GPP TS 35.205/35.206 MILENAGE authentication
// functions used by the test UE to derive keys and handle resynchronisation.
package milenage

import (
	"bytes"
	"crypto/aes"
	"fmt"
)

const (
	kLen           = 16
	opcLen         = 16
	sqnLen         = 6
	randLen        = 16
	amfLen         = 2
	macLen         = 8
	resLen         = 8
	ckLen          = 16
	ikLen          = 16
	akLen          = 6
	autnLen        = 16
	cipherBlockLen = 16
)

// Rotation constants r1..r5 from TS 35.206 §4.1.
const (
	r1 = 8
	r3 = 4
	r4 = 8
	r5 = 12
)

// The AMF used to calculate MAC-S assumes a dummy value of all zeros.
var resynchAMF = []byte{0x00, 0x00}

type ParameterLengthError struct {
	Name     string
	Exact    int
	Expected int
}

func (e *ParameterLengthError) Error() string {
	return fmt.Sprintf("parameter[%s] length should be %d byte(s), not %d byte(s)", e.Name, e.Expected, e.Exact)
}

type MACFailureError struct {
	MACName     string
	ExpectedMAC []byte
	ExactMAC    []byte
}

func (m *MACFailureError) Error() string {
	return fmt.Sprintf("X%s[%x] not match %s[%x]", m.MACName, m.ExpectedMAC, m.MACName, m.ExactMAC)
}

func validateArg(arg []byte, argName string, expectedLen int) error {
	if len(arg) != expectedLen {
		return &ParameterLengthError{Name: argName, Exact: len(arg), Expected: expectedLen}
	}

	return nil
}

// xor returns (a XOR b) truncated to the shorter operand.
func xor(a, b []byte) []byte {
	outLen := len(a)
	if len(b) < outLen {
		outLen = len(b)
	}

	out := make([]byte, outLen)
	for i := 0; i < outLen; i++ {
		out[i] = a[i] ^ b[i]
	}

	return out
}

// f1 computes the network (MAC-A) and resync (MAC-S) authentication codes.
func f1(opc, k, _rand, sqn, amf []byte) (macA, macS []byte, err error) {
	macA, macS = make([]byte, macLen), make([]byte, macLen)

	rijndaelInput := make([]byte, cipherBlockLen)
	for i := 0; i < cipherBlockLen; i++ {
		rijndaelInput[i] = _rand[i] ^ opc[i]
	}

	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, nil, err
	}

	tmp1 := make([]byte, block.BlockSize())
	block.Encrypt(tmp1, rijndaelInput)

	tmp2 := make([]byte, cipherBlockLen)
	copy(tmp2[0:], sqn[0:6])
	copy(tmp2[6:], amf[0:2])
	copy(tmp2[8:], tmp2[0:8])

	tmp3 := make([]byte, cipherBlockLen)
	for i := 0; i < cipherBlockLen; i++ {
		tmp3[(i+(cipherBlockLen-r1))%cipherBlockLen] = tmp2[i] ^ opc[i]
	}

	for i := 0; i < cipherBlockLen; i++ {
		tmp3[i] ^= tmp1[i]
	}

	tmp1 = make([]byte, cipherBlockLen)
	block.Encrypt(tmp1, tmp3)

	for i := 0; i < cipherBlockLen; i++ {
		tmp1[i] ^= opc[i]
	}

	copy(macA[0:], tmp1[0:8])
	copy(macS[0:], tmp1[8:16])

	return macA, macS, nil
}

// f2345 computes RES (f2), CK (f3), IK (f4), AK (f5) and AK* (f5*).
func f2345(opc, k, _rand []byte) (res, ck, ik, ak, akstar []byte, err error) {
	res = make([]byte, resLen)
	ck, ik = make([]byte, ckLen), make([]byte, ikLen)
	ak, akstar = make([]byte, akLen), make([]byte, akLen)

	tmp1 := make([]byte, cipherBlockLen)
	for i := 0; i < cipherBlockLen; i++ {
		tmp1[i] = _rand[i] ^ opc[i]
	}

	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	tmp2 := make([]byte, cipherBlockLen)
	block.Encrypt(tmp2, tmp1)

	/* f2 and f5: rotate by r2 (= 0, i.e. NOP) */
	for i := 0; i < cipherBlockLen; i++ {
		tmp1[i] = tmp2[i] ^ opc[i]
	}

	tmp1[15] ^= 1 // XOR c2 (= ..01)

	tmp3 := make([]byte, block.BlockSize())
	block.Encrypt(tmp3, tmp1)

	for i := 0; i < cipherBlockLen; i++ {
		tmp3[i] ^= opc[i]
	}

	copy(res[0:], tmp3[8:16]) // f2
	copy(ak[0:], tmp3[0:6])   // f5

	/* f3: rotate by r3 */
	for i := 0; i < cipherBlockLen; i++ {
		tmp1[(i+(cipherBlockLen-r3))%cipherBlockLen] = tmp2[i] ^ opc[i]
	}

	tmp1[15] ^= 2 // XOR c3 (= ..02)

	block.Encrypt(ck, tmp1)

	for i := 0; i < cipherBlockLen; i++ {
		ck[i] ^= opc[i]
	}

	/* f4: rotate by r4 */
	for i := 0; i < ikLen; i++ {
		tmp1[(i+(cipherBlockLen-r4))%cipherBlockLen] = tmp2[i] ^ opc[i]
	}

	tmp1[15] ^= 4 // XOR c4 (= ..04)

	block.Encrypt(ik, tmp1)

	for i := 0; i < ikLen; i++ {
		ik[i] ^= opc[i]
	}

	/* f5*: rotate by r5 */
	for i := 0; i < cipherBlockLen; i++ {
		tmp1[(i+(cipherBlockLen-r5))%cipherBlockLen] = tmp2[i] ^ opc[i]
	}

	tmp1[15] ^= 8 // XOR c5 (= ..08)

	block.Encrypt(tmp1, tmp1)

	for i := 0; i < akLen; i++ {
		akstar[i] = tmp1[i] ^ opc[i]
	}

	return res, ck, ik, ak, akstar, nil
}

// cutAUTN splits AUTN into (SQN XOR AK) || AMF || MAC-A.
func cutAUTN(autn []byte) (sqn, amf, mac []byte) {
	sqn = make([]byte, sqnLen)
	amf = make([]byte, amfLen)
	mac = make([]byte, macLen)

	copy(sqn, autn[0:sqnLen])
	copy(amf, autn[sqnLen:sqnLen+amfLen])
	copy(mac, autn[sqnLen+amfLen:sqnLen+amfLen+macLen])

	return sqn, amf, mac
}

// GenerateKeysWithAUTN derives SQN, AK, IK, CK and RES from AUTN and verifies MAC-A.
func GenerateKeysWithAUTN(opc, k, rand, autn []byte) (sqnhe, ak, ik, ck, res []byte, err error) {
	for _, a := range []struct {
		val  []byte
		name string
		size int
	}{{opc, "OPc", opcLen}, {k, "K", kLen}, {rand, "RAND", randLen}, {autn, "AUTN", autnLen}} {
		if err = validateArg(a.val, a.name, a.size); err != nil {
			return nil, nil, nil, nil, nil, err
		}
	}

	res, ck, ik, ak, _, err = f2345(opc, k, rand)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("calculate F2345 failed: %w", err)
	}

	concSQNhe, amf, xmac := cutAUTN(autn)

	sqnhe = xor(concSQNhe, ak)

	mac, _, err := f1(opc, k, rand, sqnhe, amf)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("calculate F1 failed: %w", err)
	}

	if !bytes.Equal(xmac, mac) {
		return nil, nil, nil, nil, nil, &MACFailureError{MACName: "MAC-A", ExpectedMAC: xmac, ExactMAC: mac}
	}

	return sqnhe, ak, ik, ck, res, nil
}

// GenerateAUTS builds the resynchronisation token AUTS = (SQNms XOR AK*) || MAC-S.
func GenerateAUTS(opc, k, rand, sqnms []byte) (auts []byte, err error) {
	for _, a := range []struct {
		val  []byte
		name string
		size int
	}{{opc, "OPc", opcLen}, {k, "K", kLen}, {rand, "RAND", randLen}, {sqnms, "SQNms", sqnLen}} {
		if err = validateArg(a.val, a.name, a.size); err != nil {
			return nil, err
		}
	}

	_, _, _, _, akstar, err := f2345(opc, k, rand) //nolint:dogsled
	if err != nil {
		return nil, fmt.Errorf("calculate F2345 failed: %w", err)
	}

	concSQNms := xor(sqnms, akstar)

	_, macS, err := f1(opc, k, rand, sqnms, resynchAMF)
	if err != nil {
		return nil, fmt.Errorf("calculate F1 failed: %w", err)
	}

	return append(concSQNms, macS...), nil
}
