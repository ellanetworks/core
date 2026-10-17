// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pcf_test

import (
	"context"
	"reflect"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/rx"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/pcf"
	"github.com/ellanetworks/core/internal/smf"
)

type notifyingEnforcer struct {
	pcf       *pcf.PCF
	decisions chan *smf.PolicyDecision
	revision  uint64
}

func (n *notifyingEnforcer) UpdateNotify(_ context.Context, _ string, d *smf.PolicyDecision) error {
	n.decisions <- d
	return nil
}

func (n *notifyingEnforcer) next(t *testing.T) *smf.PolicyDecision {
	t.Helper()

	select {
	case d := <-n.decisions:
		n.revision = max(n.revision, d.Revision)
		return d
	case <-time.After(5 * time.Second):
		t.Fatal("no policy decision pushed")
		return nil
	}
}

func (n *notifyingEnforcer) none(t *testing.T) {
	t.Helper()

	revision := n.probe(t)
	if revision != n.revision+1 {
		t.Fatalf("policy decisions up to revision %d produced since revision %d, want none", revision-1, n.revision)
	}

	n.revision = revision
}

func (n *notifyingEnforcer) probe(t *testing.T) uint64 {
	t.Helper()

	d, err := n.pcf.CreateAssociation(context.Background(), "probe", smf.PolicyContext{Supi: supi(t, "001010000000009"), Dnn: models.IMSDataNetworkName})
	if err != nil {
		t.Fatalf("probe association: %v", err)
	}

	return d.Revision
}

func newEnforcedPCF(t *testing.T) (*pcf.PCF, *notifyingEnforcer) {
	t.Helper()

	p := newPCF(t)
	n := &notifyingEnforcer{pcf: p, decisions: make(chan *smf.PolicyDecision, 8)}
	p.SetEnforcer(n)
	n.revision = n.probe(t)

	return p, n
}

func callAAR(port uint16, status rx.ServiceInfoStatus) rx.AARequest {
	r := signallingAAR(testUEv4)
	r.ServiceInfoStatus = status
	r.SpecificActions = []rx.SpecificAction{rx.ActionIndicationOfFailedResourcesAllocation}
	r.MediaComponents = []rx.MediaComponent{{
		Number:                  1,
		Type:                    &audio,
		MaxRequestedBandwidthUL: ptr(rx.Bandwidth(41000)),
		MaxRequestedBandwidthDL: ptr(rx.Bandwidth(41000)),
		SubComponents: []rx.MediaSubComponent{{
			FlowNumber: 1,
			FlowDescriptions: []string{
				"permit out 17 from 192.0.2.10 49000 to 10.60.0.1 " + itoa(port),
				"permit in 17 from 10.60.0.1 " + itoa(port) + " to 192.0.2.10 49000",
			},
		}},
	}}

	return r
}

func ptr[T any](v T) *T { return &v }

func itoa(n uint16) string { return strconv.Itoa(int(n)) }

func TestCallAARInstallsAVoiceRule(t *testing.T) {
	p, n := newEnforcedPCF(t)

	requireResult(t, aa(t, p, "af;call", callAAR(50000, rx.ServiceInfoFinal)), success)

	d := n.next(t)
	if len(d.Rules) != 1 || d.Rules[0].ID != "af;call#1.1" || d.Rules[0].QCI != 1 || d.Rules[0].GBR.Uplink.Bps() != 41000 {
		t.Fatalf("pushed rules %+v, want one QCI 1 rule with a 41 kbps GBR", d.Rules)
	}

	if d.PolicyID != "policy" {
		t.Fatalf("pushed policy %q, want the session rule kept", d.PolicyID)
	}

	update := rx.RequestUpdate
	modified := callAAR(50002, rx.ServiceInfoFinal)
	modified.RequestType = &update
	requireResult(t, aa(t, p, "af;call", modified), success)

	m := n.next(t)
	if len(m.Rules) != 1 || m.Rules[0].Filters[0].LocalPort != 50002 || m.Revision <= d.Revision {
		t.Fatalf("pushed %+v, want the modified rule at a later revision", m)
	}

	requireResult(t, aa(t, p, "af;call", modified), success)
	n.none(t)

	requireResult(t, str(t, p, "af;call"), success)

	if r := n.next(t); len(r.Rules) != 0 || r.Revision <= m.Revision {
		t.Fatalf("pushed %+v after STR, want no rules", r)
	}
}

func TestFinalServiceInfoAuthorizesThePreliminaryMedia(t *testing.T) {
	p, n := newEnforcedPCF(t)

	requireResult(t, aa(t, p, "af;call", callAAR(50000, rx.ServiceInfoPreliminary)), success)
	n.none(t)

	update := rx.RequestUpdate
	final := signallingAAR(testUEv4)
	final.RequestType = &update
	final.MediaComponents = nil
	requireResult(t, aa(t, p, "af;call", final), success)

	if d := n.next(t); len(d.Rules) != 1 || d.Rules[0].QCI != 1 || d.Rules[0].Filters[0].LocalPort != 50000 {
		t.Fatalf("pushed rules %+v, want the preliminary voice flow now authorized (TS 29.214 §5.3.7)", d.Rules)
	}
}

func TestFinalServiceInfoKeepsThePreliminaryAVPsItOmits(t *testing.T) {
	p, n := newEnforcedPCF(t)

	requireResult(t, aa(t, p, "af;call", callAAR(50000, rx.ServiceInfoPreliminary)), success)

	update := rx.RequestUpdate
	final := signallingAAR(testUEv4)
	final.RequestType = &update
	final.MediaComponents = []rx.MediaComponent{{
		Number: 1,
		SubComponents: []rx.MediaSubComponent{{
			FlowNumber: 1,
			FlowDescriptions: []string{
				"permit out 17 from 192.0.2.10 49000 to 10.60.0.1 50002",
				"permit in 17 from 10.60.0.1 50002 to 192.0.2.10 49000",
			},
		}},
	}}
	requireResult(t, aa(t, p, "af;call", final), success)

	if d := n.next(t); len(d.Rules) != 1 || d.Rules[0].QCI != 1 || d.Rules[0].GBR.Uplink.Bps() != 41000 || d.Rules[0].Filters[0].LocalPort != 50002 {
		t.Fatalf("pushed rules %+v, want the answered port with the preliminary media type and bandwidth", d.Rules)
	}
}

func TestBadFlowDescriptionIsAFilterRestriction(t *testing.T) {
	for _, status := range []rx.ServiceInfoStatus{rx.ServiceInfoFinal, rx.ServiceInfoPreliminary} {
		t.Run(status.String(), func(t *testing.T) {
			p, n := newEnforcedPCF(t)

			r := callAAR(50000, status)
			r.MediaComponents[0].SubComponents[0].FlowDescriptions = []string{"deny out 17 from 192.0.2.10 49000 to 10.60.0.1 50000"}

			requireResult(t, aa(t, p, "af;call", r), tgpp.Experimental(tgpp.ResultFilterRestrictions))
			n.none(t)
		})
	}
}

func TestRemovedMediaComponentRemovesItsRule(t *testing.T) {
	p, n := newEnforcedPCF(t)

	requireResult(t, aa(t, p, "af;call", callAAR(50000, rx.ServiceInfoFinal)), success)
	n.next(t)

	update := rx.RequestUpdate
	removed := rx.FlowStatusRemoved
	held := callAAR(50000, rx.ServiceInfoFinal)
	held.RequestType = &update
	held.MediaComponents[0].FlowStatus = &removed
	requireResult(t, aa(t, p, "af;call", held), success)

	if d := n.next(t); len(d.Rules) != 0 {
		t.Fatalf("pushed rules %+v, want none", d.Rules)
	}
}

func TestFailedVoiceRuleAbortsTheCall(t *testing.T) {
	p, n := newEnforcedPCF(t)

	requireResult(t, aa(t, p, "af;call", callAAR(50000, rx.ServiceInfoFinal)), success)
	d := n.next(t)

	stale := d.Rules[0]
	stale.Version--
	p.ReportEnforcementFailure(testRef, failedRules(stale), smf.ResourcesNotAllocated)
	n.none(t)

	p.ReportEnforcementFailure(testRef, failedRules(d.Rules...), smf.ResourcesNotAllocated)

	if r := n.next(t); len(r.Rules) != 0 {
		t.Fatalf("pushed rules %+v after the failure, want none", r.Rules)
	}

	update := rx.RequestUpdate
	requireResult(t, aa(t, p, "af;call", rx.AARequest{RequestType: &update}), tgpp.Result{Code: diameter.ResultUnknownSessionID})
}

func TestLosingEveryFlowAbortsWithoutASubscription(t *testing.T) {
	p, n := newEnforcedPCF(t)

	r := callAAR(50000, rx.ServiceInfoFinal)
	r.SpecificActions = []rx.SpecificAction{rx.ActionIndicationOfLossOfBearer}
	requireResult(t, aa(t, p, "af;call", r), success)
	d := n.next(t)

	p.ReportEnforcementFailure(testRef, failedRules(d.Rules...), smf.BearerReleased)
	n.next(t)

	update := rx.RequestUpdate
	requireResult(t, aa(t, p, "af;call", rx.AARequest{RequestType: &update}), tgpp.Result{Code: diameter.ResultUnknownSessionID})
}

func TestUpdateOmittingUnchangedAVPsKeepsTheRule(t *testing.T) {
	p, n := newEnforcedPCF(t)

	requireResult(t, aa(t, p, "af;call", callAAR(50000, rx.ServiceInfoFinal)), success)
	n.next(t)

	update := rx.RequestUpdate
	requireResult(t, aa(t, p, "af;call", rx.AARequest{RequestType: &update, MediaComponents: []rx.MediaComponent{{Number: 1}}}), success)
	n.none(t)

	requireResult(t, aa(t, p, "af;call", callAAR(50000, rx.ServiceInfoFinal)), success)
	n.none(t)
}

func TestFailureRestoresTheVersionTheSMFEnforces(t *testing.T) {
	p, n := newEnforcedPCF(t)

	requireResult(t, aa(t, p, "af;call", callAAR(50000, rx.ServiceInfoFinal)), success)
	enforced := n.next(t).Rules[0]

	for _, port := range []uint16{50002, 50004} {
		update := callAAR(port, rx.ServiceInfoFinal)
		update.RequestType = ptr(rx.RequestUpdate)
		requireResult(t, aa(t, p, "af;call", update), success)
	}

	latest := n.next(t).Rules[0]
	if other := n.next(t).Rules[0]; other.Version > latest.Version {
		latest = other
	}

	if samePorts(latest, enforced) || latest.Version <= enforced.Version {
		t.Fatalf("updated rule %+v, want new ports under a newer version than %+v", latest, enforced)
	}

	p.ReportEnforcementFailure(testRef, []smf.RuleReport{{Rule: latest, Active: &enforced}}, smf.ResourcesNotAllocated)

	if d := n.next(t); len(d.Rules) != 1 || !reflect.DeepEqual(d.Rules[0], enforced) {
		t.Fatalf("rules after the failure = %+v, want the enforced version %+v, not an intermediate one", d.Rules, enforced)
	}

	requireResult(t, aa(t, p, "af;call", rx.AARequest{RequestType: ptr(rx.RequestUpdate)}), success)
}

func TestFailedModificationKeepsTheEnforcedQoSUnderTheAuthorizedGate(t *testing.T) {
	p, n := newEnforcedPCF(t)

	ringing := callAAR(50000, rx.ServiceInfoFinal)
	ringing.MediaComponents[0].FlowStatus = ptr(rx.FlowStatusDisabled)
	requireResult(t, aa(t, p, "af;call", ringing), success)
	enforced := n.next(t).Rules[0]

	answered := callAAR(50000, rx.ServiceInfoFinal)
	answered.RequestType = ptr(rx.RequestUpdate)
	answered.MediaComponents[0].FlowStatus = ptr(rx.FlowStatusEnabled)
	answered.MediaComponents[0].MaxRequestedBandwidthUL = ptr(rx.Bandwidth(30000))
	requireResult(t, aa(t, p, "af;call", answered), success)
	modified := n.next(t).Rules[0]

	if modified.Gate != (models.GateStatus{}) || modified.MBR.Equal(enforced.MBR) {
		t.Fatalf("answered rule %+v, want the gates open and new rates", modified)
	}

	p.ReportEnforcementFailure(testRef, []smf.RuleReport{{Rule: modified, Active: &enforced}}, smf.ResourcesNotAllocated)

	d := n.next(t)
	if len(d.Rules) != 1 {
		t.Fatalf("rules after the failure = %+v, want one", d.Rules)
	}

	gated := d.Rules[0]
	want := enforced
	want.Gate, want.Version = models.GateStatus{}, gated.Version

	if !reflect.DeepEqual(gated, want) || gated.Version <= modified.Version {
		t.Fatalf("rule after the failure = %+v, want the enforced QoS and filters %+v with the gates the AF opened at a newer version (TS 29.212 §4.5.12, TS 29.214 §4.4.3)", gated, enforced)
	}

	p.ReportEnforcementFailure(testRef, []smf.RuleReport{{Rule: gated, Active: &enforced}}, smf.ResourcesNotAllocated)

	if d := n.next(t); len(d.Rules) != 1 || !reflect.DeepEqual(d.Rules[0], enforced) {
		t.Fatalf("rules after the gate change failed = %+v, want the enforced version %+v", d.Rules, enforced)
	}

	p.ReportEnforcementFailure(testRef, []smf.RuleReport{{Rule: gated, Active: &enforced}}, smf.ResourcesNotAllocated)
	n.none(t)
}

func TestReportForASupersededVersionIsIgnored(t *testing.T) {
	p, n := newEnforcedPCF(t)

	requireResult(t, aa(t, p, "af;call", callAAR(50000, rx.ServiceInfoFinal)), success)
	first := n.next(t).Rules[0]

	update := callAAR(50002, rx.ServiceInfoFinal)
	update.RequestType = ptr(rx.RequestUpdate)
	requireResult(t, aa(t, p, "af;call", update), success)
	n.next(t)

	p.ReportEnforcementFailure(testRef, failedRules(first), smf.ResourcesNotAllocated)
	n.none(t)
}

func failedRules(rules ...smf.PCCRule) []smf.RuleReport {
	reports := make([]smf.RuleReport, 0, len(rules))
	for _, r := range rules {
		reports = append(reports, smf.RuleReport{Rule: r})
	}

	return reports
}

func samePorts(a, b smf.PCCRule) bool {
	return slices.EqualFunc(a.Filters, b.Filters, func(x, y models.SDFFilter) bool { return x.LocalPort == y.LocalPort })
}

func TestHoldGatesTheMediaRuleInPlace(t *testing.T) {
	p, n := newEnforcedPCF(t)

	requireResult(t, aa(t, p, "af;call", callAAR(50000, rx.ServiceInfoFinal)), success)
	d := n.next(t)

	update := rx.RequestUpdate
	sendOnly := rx.FlowStatusEnabledUplink
	held := callAAR(50000, rx.ServiceInfoFinal)
	held.RequestType = &update
	held.MediaComponents[0].FlowStatus = &sendOnly
	requireResult(t, aa(t, p, "af;call", held), success)

	h := n.next(t)
	if len(h.Rules) != 1 || h.Rules[0].ID != d.Rules[0].ID || h.Rules[0].Gate != (models.GateStatus{DLGate: models.GateClose}) || h.Rules[0].Version == d.Rules[0].Version {
		t.Fatalf("pushed %+v on hold, want the same rule at a new version with its downlink gate closed (TS 29.214 §5.3.11)", h.Rules)
	}

	if !slices.Equal(h.Rules[0].Filters, d.Rules[0].Filters) || !h.Rules[0].GBR.Uplink.Equal(d.Rules[0].GBR.Uplink) {
		t.Fatalf("held rule %+v, want filters and QoS unchanged (TS 29.213 Table 6.3.1)", h.Rules[0])
	}

	enabled := rx.FlowStatusEnabled
	resumed := callAAR(50000, rx.ServiceInfoFinal)
	resumed.RequestType = &update
	resumed.MediaComponents[0].FlowStatus = &enabled
	requireResult(t, aa(t, p, "af;call", resumed), success)

	if r := n.next(t); len(r.Rules) != 1 || r.Rules[0].Gate != (models.GateStatus{}) {
		t.Fatalf("pushed %+v on resume, want the gates open", r.Rules)
	}
}

func TestForkedDialogueKeepsEnabledGatesAndAuthorisedMedia(t *testing.T) {
	p, n := newEnforcedPCF(t)

	requireResult(t, aa(t, p, "af;call", callAAR(50000, rx.ServiceInfoFinal)), success)
	d := n.next(t)

	update := rx.RequestUpdate
	disabled := rx.FlowStatusDisabled
	forked := callAAR(50002, rx.ServiceInfoFinal)
	forked.RequestType = &update
	forked.SIPForkingIndication = rx.ForkingSeveralDialogues
	forked.MediaComponents[0].FlowStatus = &disabled
	requireResult(t, aa(t, p, "af;call", forked), success)

	f := n.next(t)
	if len(f.Rules) != 1 || f.Rules[0].Gate != (models.GateStatus{}) || len(f.Rules[0].Filters) != 4 || !slices.Equal(f.Rules[0].Filters[:2], d.Rules[0].Filters) {
		t.Fatalf("pushed %+v for a forked dialogue, want the gates kept open, the first dialogue's filters kept in place and the second's appended (TS 29.214 Annex A.3.1)", f.Rules)
	}

	removed := rx.FlowStatusRemoved
	forked.MediaComponents[0].FlowStatus = &removed
	requireResult(t, aa(t, p, "af;call", forked), success)
	n.none(t)

	if f.Rules[0].ID != d.Rules[0].ID {
		t.Fatalf("forked rule %s, want the authorised rule %s extended", f.Rules[0].ID, d.Rules[0].ID)
	}
}

func TestComponentFlowStatusReplacesASubComponentOne(t *testing.T) {
	p, n := newEnforcedPCF(t)

	held := callAAR(50000, rx.ServiceInfoFinal)
	held.MediaComponents[0].SubComponents[0].FlowStatus = ptr(rx.FlowStatusEnabledUplink)
	requireResult(t, aa(t, p, "af;call", held), success)

	if d := n.next(t); d.Rules[0].Gate == (models.GateStatus{}) {
		t.Fatalf("pushed %+v, want the held RTP gated", d.Rules)
	}

	update := rx.RequestUpdate
	resumed := rx.AARequest{RequestType: &update, ServiceInfoStatus: rx.ServiceInfoFinal, MediaComponents: []rx.MediaComponent{{Number: 1, FlowStatus: ptr(rx.FlowStatusEnabled)}}}
	requireResult(t, aa(t, p, "af;call", resumed), success)

	if d := n.next(t); d.Rules[0].Gate != (models.GateStatus{}) {
		t.Fatalf("pushed %+v, want the component-level ENABLED to open the RTP gate (TS 29.214 §5.3.7)", d.Rules)
	}
}
