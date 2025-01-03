// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package security_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/util/nas/security"
)

func TestSetterGetter(t *testing.T) {
	testCases := []struct {
		overflow uint16
		sqn      uint8
	}{
		{1, 2},
		{0, 0},
		{170, 35},
		{65535, 255},
	}

	count := security.Count{}

	for _, testCase := range testCases {
		count.Set(testCase.overflow, testCase.sqn)
		expected := (uint32(testCase.overflow) << 8) + uint32(testCase.sqn)
		if expected != count.Get() {
			t.Errorf("Get() Failed")
		}
		if testCase.overflow != count.Overflow() {
			t.Errorf("Overflow() Failed")
		}
		if testCase.sqn != count.SQN() {
			t.Errorf("SQN() Failed")
		}
	}
}

func TestAddOne(t *testing.T) {
	count := security.Count{}

	count.Set(0, 0)

	for i := uint32(0); i < 4567; i++ {
		count.AddOne()
		if i+1 != count.Get() {
			t.Errorf("AddOne() Test Failed")
		}
	}
}
