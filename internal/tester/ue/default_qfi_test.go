// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"testing"

	"github.com/ellanetworks/core/nas/fgs"
)

func TestDefaultQFIFollowsTheDefaultQoSRule(t *testing.T) {
	dedicated := fgs.QoSRule{
		Identifier:    2,
		OperationCode: fgs.QoSRuleOpCreate,
		Parameters:    &fgs.QoSRuleParameters{Precedence: 10, QFI: 2},
	}

	acc := &fgs.PDUSessionEstablishmentAccept{
		QoSRules: fgs.QoSRules{dedicated, fgs.DefaultQoSRule(1, 1)},
		QoSFlowDescriptions: fgs.QoSFlowDescriptions{
			fgs.FiveQIQoSFlow(2, 69, fgs.QoSFlowOpCreate),
			fgs.FiveQIQoSFlow(1, 9, fgs.QoSFlowOpCreate),
		},
	}

	qfi, err := defaultQFI(acc)
	if err != nil {
		t.Fatalf("defaultQFI: %v", err)
	}

	if qfi != 1 {
		t.Errorf("default QFI = %d, want 1: the default QoS rule names it, not the first flow description", qfi)
	}
}

func TestDefaultQFIFallsBackToTheFirstFlowDescription(t *testing.T) {
	acc := &fgs.PDUSessionEstablishmentAccept{
		QoSFlowDescriptions: fgs.QoSFlowDescriptions{fgs.FiveQIQoSFlow(5, 9, fgs.QoSFlowOpCreate)},
	}

	if qfi, err := defaultQFI(acc); err != nil || qfi != 5 {
		t.Errorf("defaultQFI = %d, %v, want 5", qfi, err)
	}

	if _, err := defaultQFI(&fgs.PDUSessionEstablishmentAccept{}); err == nil {
		t.Error("defaultQFI found a QFI in an accept without QoS rule or flow description")
	}
}
