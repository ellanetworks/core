// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package udm

import (
	"github.com/ellanetworks/core/internal/milenage"
	"github.com/ellanetworks/core/internal/sqn"
)

const IndStep = sqn.IndStep

func AdvanceSQN(sqnHex string, delta uint64) (string, error) {
	return sqn.Advance(sqnHex, delta)
}

func ResyncSQN(opc, k, auts, rand []byte) ([]byte, error) {
	return sqn.ResyncNext(opc, k, auts, rand)
}

func resyncSQN(opc, k, auts, rand []byte) (string, error) {
	return sqn.Resync(opc, k, auts, rand)
}

func aucSQN(opc, k, auts, rand []byte) ([]byte, []byte, error) {
	return sqn.AucSQN(opc, k, auts, rand)
}

func strictHex(s string, n int) string {
	return sqn.StrictHex(s, n)
}

func F1(opc, k, rand, sqnBytes, amf, macA, macS []uint8) error {
	return milenage.F1(opc, k, rand, sqnBytes, amf, macA, macS)
}

func F2345(opc, k, rand, res, ck, ik, ak, akstar []uint8) error {
	return milenage.F2345(opc, k, rand, res, ck, ik, ak, akstar)
}
