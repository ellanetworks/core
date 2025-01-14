// Copyright 2024 Ella Networks
// Copyright 2019 Communication Service/Software Laboratory, National Chiao Tung University (free5gc.org)
// SPDX-License-Identifier: Apache-2.0

package udm

import (
	"crypto/aes"
)

/**
 * milenage_f1 - Milenage f1 and f1* algorithms
 * @opc: OPc = 128-bit value derived from OP and K
 * @k: K = 128-bit subscriber key
 * @_rand: RAND = 128-bit random challenge
 * @sqn: SQN = 48-bit sequence number
 * @amf: AMF = 16-bit authentication management field
 * @mac_a: Buffer for MAC-A = 64-bit network authentication code, or %NULL
 * @mac_s: Buffer for MAC-S = 64-bit resync authentication code, or %NULL
 * Returns: 0 on success, -1 on failure
 */
func milenageF1(opc, k, _rand, sqn, amf, mac_a, mac_s []uint8) error {
	tmp2, tmp3 := make([]uint8, 16), make([]uint8, 16)
	// var tmp1, tmp2, tmp3 [16]uint8

	rijndaelInput := make([]uint8, 16)

	/* tmp1 = TEMP = E_K(RAND XOR OP_C) */
	for i := 0; i < 16; i++ {
		rijndaelInput[i] = _rand[i] ^ opc[i]
	}
	// RijndaelEncrypt( OP, op_c );

	block, err := aes.NewCipher(k)
	if err != nil {
		return err
	}

	tmp1 := make([]byte, block.BlockSize())
	block.Encrypt(tmp1, rijndaelInput)

	// fmt.Printf("tmp1: %x\n", tmp1)

	/* tmp2 = IN1 = SQN || AMF || SQN || AMF */
	copy(tmp2[0:], sqn[0:6])
	copy(tmp2[6:], amf[0:2])
	copy(tmp2[8:], tmp2[0:8])
	/*
		os_memcpy(tmp2, sqn, 6);
		os_memcpy(tmp2 + 6, amf, 2);
		os_memcpy(tmp2 + 8, tmp2, 8);
	*/

	/* OUT1 = E_K(TEMP XOR rot(IN1 XOR OP_C, r1) XOR c1) XOR OP_C */

	/* rotate (tmp2 XOR OP_C) by r1 (= 0x40 = 8 bytes) */
	for i := 0; i < 16; i++ {
		tmp3[(i+8)%16] = tmp2[i] ^ opc[i]
	}

	// fmt.Printf("tmp3: %x\n", tmp3)

	/* XOR with TEMP = E_K(RAND XOR OP_C) */
	for i := 0; i < 16; i++ {
		tmp3[i] ^= tmp1[i]
	}
	// fmt.Printf("tmp3 XOR with TEMP: %x\n", tmp3)

	/* XOR with c1 (= ..00, i.e., NOP) */
	/* f1 || f1* = E_K(tmp3) XOR OP_c */

	tmp1 = make([]byte, block.BlockSize())
	block.Encrypt(tmp1, tmp3)

	// fmt.Printf("XOR with c1 (: %x\n", tmp1)

	for i := 0; i < 16; i++ {
		tmp1[i] ^= opc[i]
	}
	// fmt.Printf("tmp1[i] ^= opc[i] %x\n", tmp1)
	if mac_a != nil {
		copy(mac_a[0:], tmp1[0:8])
	}

	if mac_s != nil {
		copy(mac_s[0:], tmp1[8:16])
	}

	return nil
}

/**
 * milenage_f2345 - Milenage f2, f3, f4, f5, f5* algorithms
 * @opc: OPc = 128-bit value derived from OP and K
 * @k: K = 128-bit subscriber key
 * @_rand: RAND = 128-bit random challenge
 * @res: Buffer for RES = 64-bit signed response (f2), or %NULL
 * @ck: Buffer for CK = 128-bit confidentiality key (f3), or %NULL
 * @ik: Buffer for IK = 128-bit integrity key (f4), or %NULL
 * @ak: Buffer for AK = 48-bit anonymity key (f5), or %NULL
 * @akstar: Buffer for AK = 48-bit anonymity key (f5*), or %NULL
 * Returns: 0 on success, -1 on failure
 */
func milenageF2345(opc, k, _rand, res, ck, ik, ak, akstar []uint8) error {
	tmp1 := make([]uint8, 16)

	/* tmp2 = TEMP = E_K(RAND XOR OP_C) */
	for i := 0; i < 16; i++ {
		tmp1[i] = _rand[i] ^ opc[i]
	}

	block, err := aes.NewCipher(k)
	if err != nil {
		return err
	}

	tmp2 := make([]byte, block.BlockSize())
	block.Encrypt(tmp2, tmp1)

	/* OUT2 = E_K(rot(TEMP XOR OP_C, r2) XOR c2) XOR OP_C */
	/* OUT3 = E_K(rot(TEMP XOR OP_C, r3) XOR c3) XOR OP_C */
	/* OUT4 = E_K(rot(TEMP XOR OP_C, r4) XOR c4) XOR OP_C */
	/* OUT5 = E_K(rot(TEMP XOR OP_C, r5) XOR c5) XOR OP_C */

	/* f2 and f5 */
	/* rotate by r2 (= 0, i.e., NOP) */
	for i := 0; i < 16; i++ {
		tmp1[i] = tmp2[i] ^ opc[i]
	}
	tmp1[15] ^= 1 // XOR c2 (= ..01)
	/*
		for (i = 0; i < 16; i++)
			tmp1[i] = tmp2[i] ^ opc[i];
		tmp1[15] ^= 1; // XOR c2 (= ..01)
	*/

	/* f5 || f2 = E_K(tmp1) XOR OP_c */
	tmp3 := make([]byte, block.BlockSize())
	block.Encrypt(tmp3, tmp1)

	for i := 0; i < 16; i++ {
		tmp3[i] ^= opc[i]
	}

	if res != nil {
		copy(res[0:], tmp3[8:16]) // f2
	}

	if ak != nil {
		copy(ak[0:], tmp3[0:6]) // f5
	}
	/*
		if (aes_128_encrypt_block(k, tmp1, tmp3))
			return -1;
		for (i = 0; i < 16; i++)
			tmp3[i] ^= opc[i];
		if (res)
			os_memcpy(res, tmp3 + 8, 8); // f2
		if (ak)
			os_memcpy(ak, tmp3, 6); // f5
	*/

	/* f3 */
	if ck != nil {
		// rotate by r3 = 0x20 = 4 bytes
		for i := 0; i < 16; i++ {
			tmp1[(i+12)%16] = tmp2[i] ^ opc[i]
		}
		tmp1[15] ^= 2 // XOR c3 (= ..02)

		block.Encrypt(ck, tmp1)

		for i := 0; i < 16; i++ {
			ck[i] ^= opc[i]
		}
	}
	/*
		if (ck) {
			// rotate by r3 = 0x20 = 4 bytes
			for (i = 0; i < 16; i++)
				tmp1[(i + 12) % 16] = tmp2[i] ^ opc[i];
			tmp1[15] ^= 2; // XOR c3 (= ..02)
			if (aes_128_encrypt_block(k, tmp1, ck))
				return -1;
			for (i = 0; i < 16; i++)
				ck[i] ^= opc[i];
		}
	*/

	/* f4 */
	if ik != nil {
		// rotate by r4 = 0x40 = 8 bytes
		for i := 0; i < 16; i++ {
			tmp1[(i+8)%16] = tmp2[i] ^ opc[i]
		}
		tmp1[15] ^= 4 // XOR c4 (= ..04)

		block.Encrypt(ik, tmp1)

		for i := 0; i < 16; i++ {
			ik[i] ^= opc[i]
		}
	}
	/*
		if (ik) {
			//rotate by r4 = 0x40 = 8 bytes
			for (i = 0; i < 16; i++)
				tmp1[(i + 8) % 16] = tmp2[i] ^ opc[i];
			tmp1[15] ^= 4; // XOR c4 (= ..04)
			if (aes_128_encrypt_block(k, tmp1, ik))
				return -1;
			for (i = 0; i < 16; i++)
				ik[i] ^= opc[i];
		}
	*/

	/* f5* */
	if akstar != nil {
		// rotate by r5 = 0x60 = 12 bytes
		for i := 0; i < 16; i++ {
			tmp1[(i+4)%16] = tmp2[i] ^ opc[i]
		}
		tmp1[15] ^= 8 // XOR c5 (= ..08)

		block.Encrypt(tmp1, tmp1)

		for i := 0; i < 6; i++ {
			akstar[i] = tmp1[i] ^ opc[i]
		}
	}
	/*
		if (akstar) {
			// rotate by r5 = 0x60 = 12 bytes
			for (i = 0; i < 16; i++)
				tmp1[(i + 4) % 16] = tmp2[i] ^ opc[i];
			tmp1[15] ^= 8; // XOR c5 (= ..08)
			if (aes_128_encrypt_block(k, tmp1, tmp1))
				return -1;
			for (i = 0; i < 6; i++)
				akstar[i] = tmp1[i] ^ opc[i];
		}
	*/

	return nil
}

func F1(opc, k, _rand, sqn, amf, mac_a, mac_s []uint8) error {
	return milenageF1(opc, k, _rand, sqn, amf, mac_a, mac_s)
}

func F2345(opc, k, _rand, res, ck, ik, ak, akstar []uint8) error {
	return milenageF2345(opc, k, _rand, res, ck, ik, ak, akstar)
}

func GenerateOPC(k, op []uint8) ([]uint8, error) {
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}

	opc := make([]byte, block.BlockSize())

	block.Encrypt(opc, op)

	for i := 0; i < 16; i++ {
		opc[i] ^= op[i]
	}

	return opc, nil
}
