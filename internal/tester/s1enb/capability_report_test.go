// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import "testing"

func TestCapabilityReportIsClaimedOncePerUEAndPrunedOnRelease(t *testing.T) {
	e := &ENB{UERadioCapability: DefaultUERadioCapability}

	if !e.claimCapabilityReport(3) {
		t.Fatal("the first report for a UE must be claimable")
	}

	if e.claimCapabilityReport(3) {
		t.Error("the eNB reports the capability once per UE context setup")
	}

	e.dropCapabilityReport(3)

	if !e.claimCapabilityReport(3) {
		t.Error("a released S1 context must not keep its report claimed")
	}
}

func TestCapabilityReportDisabledWhenEmpty(t *testing.T) {
	e := &ENB{}

	if e.claimCapabilityReport(3) {
		t.Error("an empty capability must disable the report")
	}
}
