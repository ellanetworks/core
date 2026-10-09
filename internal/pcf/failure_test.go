// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pcf

import (
	"slices"
	"testing"

	"github.com/ellanetworks/core/diameter/rx"
	"github.com/ellanetworks/core/internal/smf"
)

func TestFailureActionFollowsTheFailureKind(t *testing.T) {
	both := []rx.SpecificAction{rx.ActionIndicationOfReleaseOfBearer, rx.ActionIndicationOfFailedResourcesAllocation}

	tests := []struct {
		name       string
		subscribed []rx.SpecificAction
		cause      smf.EnforcementFailure
		want       rx.SpecificAction
		ok         bool
	}{
		{"setup failure", both, smf.ResourcesNotAllocated, rx.ActionIndicationOfFailedResourcesAllocation, true},
		{"active bearer released", both, smf.BearerReleased, rx.ActionIndicationOfReleaseOfBearer, true},
		{"release reported as failure when only that is subscribed", []rx.SpecificAction{rx.ActionIndicationOfFailedResourcesAllocation}, smf.BearerReleased, rx.ActionIndicationOfFailedResourcesAllocation, true},
		{"GPRS loss of bearer is not an EPS report", []rx.SpecificAction{rx.ActionIndicationOfLossOfBearer}, smf.BearerReleased, 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := failureAction(tc.subscribed, tc.cause)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("failureAction = %v, %v; want %v, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestReportedFlowsCarryTheMediaComponentStatus(t *testing.T) {
	enforced := smf.PCCRule{ID: "af;1#1.1", Version: 1, QCI: 1}
	modified := smf.PCCRule{ID: "af;1#1.1", Version: 2, QCI: 1}
	rtcp := smf.PCCRule{ID: "af;1#1.2", Version: 1, QCI: 1}
	added := smf.PCCRule{ID: "af;1#2.1", Version: 3, QCI: 1}

	s := &rxSession{rules: map[flowKey]smf.PCCRule{{1, 1}: modified, {1, 2}: rtcp, {2, 1}: added}}

	flows := s.applyReports([]smf.RuleReport{{Rule: modified, Active: &enforced}, {Rule: added}})

	if len(flows) != 2 || flows[0].MediaComponentNumber != 1 || flows[1].MediaComponentNumber != 2 {
		t.Fatalf("flows %+v, want components 1 and 2", flows)
	}

	if st := flows[0].MediaComponentStatus; st == nil || *st != rx.MediaComponentActive || !slices.Equal(flows[0].FlowNumbers, []uint32{1}) {
		t.Fatalf("component 1 flows %v status %v, want flow 1 ACTIVE: its previous rule is still enforced (TS 29.214 §4.4.2, §5.3.48)", flows[0].FlowNumbers, st)
	}

	if st := flows[1].MediaComponentStatus; st == nil || *st != rx.MediaComponentInactive {
		t.Fatalf("component 2 status %v, want INACTIVE", st)
	}

	if r, ok := s.rules[flowKey{1, 1}]; !ok || !samePCCRule(r, enforced) || len(s.rules) != 2 {
		t.Fatalf("session rules %+v, want the enforced RTP version and the untouched RTCP rule", s.rules)
	}
}
