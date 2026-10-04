// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf

import (
	"testing"

	"github.com/ellanetworks/core/diameter/sgd"
	"github.com/ellanetworks/core/diameter/tgpp"
	"go.uber.org/zap/zapcore"
)

func logged(fields []zapcore.Field) map[string]any {
	enc := zapcore.NewMapObjectEncoder()
	for _, f := range fields {
		f.AddTo(enc)
	}

	return enc.Fields
}

func TestMTOutcomesLogTheCauseAndDiagnosticByName(t *testing.T) {
	failure := logged(deliveryFailure(sgd.CauseMemoryCapacityExceeded, nil).logFields())
	if failure["delivery_failure_cause"] != uint32(0) || failure["delivery_failure_cause_name"] != "MEMORY_CAPACITY_EXCEEDED" {
		t.Fatalf("delivery failure fields = %v", failure)
	}

	detached := logged(absent(tgpp.AbsentUserIMSIDetached).logFields())
	if detached["absent_user_diagnostic"] != uint32(1) || detached["absent_user_diagnostic_name"] != "IMSI_DETACHED" {
		t.Fatalf("absent user fields = %v", detached)
	}
}
