// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pcf

import (
	"reflect"
	"slices"
	"testing"

	"github.com/ellanetworks/core/diameter/rx"
	"github.com/ellanetworks/core/internal/models"
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

	flows := s.applyReports([]smf.RuleReport{{Rule: modified, Active: &enforced}, {Rule: added}}, func() uint64 {
		t.Fatal("no rule needs a new version")
		return 0
	})

	if len(flows) != 2 || flows[0].MediaComponentNumber != 1 || flows[1].MediaComponentNumber != 2 {
		t.Fatalf("flows %+v, want components 1 and 2", flows)
	}

	if st := flows[0].MediaComponentStatus; st == nil || *st != rx.MediaComponentActive || !slices.Equal(flows[0].FlowNumbers, []uint32{1}) {
		t.Fatalf("component 1 flows %v status %v, want flow 1 ACTIVE: its previous rule is still enforced (TS 29.214 §4.4.2, §5.3.48)", flows[0].FlowNumbers, st)
	}

	if st := flows[1].MediaComponentStatus; st == nil || *st != rx.MediaComponentInactive {
		t.Fatalf("component 2 status %v, want INACTIVE", st)
	}

	if r, ok := s.rules[flowKey{1, 1}]; !ok || !r.Equal(enforced) || len(s.rules) != 2 {
		t.Fatalf("session rules %+v, want the enforced RTP version and the untouched RTCP rule", s.rules)
	}
}

func TestFailedRulesKeepTheEnforcedQoSUnderTheAuthorizedGates(t *testing.T) {
	closed := models.GateStatus{ULGate: models.GateClose, DLGate: models.GateClose}
	rate := func(bps uint64) models.Ambr {
		return models.Ambr{Uplink: models.BitRateFromBps(bps), Downlink: models.BitRateFromBps(bps)}
	}

	rtp := smf.PCCRule{ID: "af;1#1.1", Version: 1, QCI: 1, MBR: rate(41000), Gate: closed}
	rtcp := smf.PCCRule{ID: "af;1#1.2", Version: 2, QCI: 1, MBR: rate(2000)}
	video := smf.PCCRule{ID: "af;1#2.1", Version: 3, QCI: 2, MBR: rate(512000), Gate: closed}

	answeredRTP, answeredRTCP, answeredVideo := rtp, rtcp, video
	answeredRTP.Version, answeredRTP.MBR, answeredRTP.Gate = 4, rate(30000), models.GateStatus{}
	answeredRTCP.Version, answeredRTCP.MBR = 5, rate(1500)
	answeredVideo.Version, answeredVideo.Gate = 6, models.GateStatus{}

	s := &rxSession{rules: map[flowKey]smf.PCCRule{{1, 1}: answeredRTP, {1, 2}: answeredRTCP, {2, 1}: answeredVideo}}

	versions := []uint64{7}
	flows := s.applyReports([]smf.RuleReport{{Rule: answeredRTP, Active: &rtp}, {Rule: answeredRTCP, Active: &rtcp}, {Rule: answeredVideo}}, func() uint64 {
		if len(versions) == 0 {
			t.Fatal("more new rule versions than gate changes")
		}

		v := versions[0]
		versions = versions[1:]

		return v
	})

	want := rtp
	want.Version, want.Gate = 7, models.GateStatus{}

	if got := s.rules[flowKey{1, 1}]; !reflect.DeepEqual(got, want) {
		t.Fatalf("RTP rule %+v, want the enforced rates under the open gate at a new version %+v (TS 29.212 §4.5.12, TS 29.214 §4.4.3)", got, want)
	}

	if got := s.rules[flowKey{1, 2}]; !reflect.DeepEqual(got, rtcp) {
		t.Fatalf("RTCP rule %+v, want the enforced version %+v: its gate did not change", got, rtcp)
	}

	if _, ok := s.rules[flowKey{2, 1}]; ok || len(s.rules) != 2 {
		t.Fatalf("session rules %+v, want the video rule with no enforced version removed", s.rules)
	}

	if len(flows) != 2 || *flows[0].MediaComponentStatus != rx.MediaComponentActive || *flows[1].MediaComponentStatus != rx.MediaComponentInactive {
		t.Fatalf("flows %+v, want component 1 ACTIVE and component 2 INACTIVE", flows)
	}
}
