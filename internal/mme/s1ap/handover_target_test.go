// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/s1ap"
)

type targetIDCase struct {
	targetID s1ap.TargetID
	cause    s1ap.Cause
}

func TestHandoverRequiredTargetIDAlternatives(t *testing.T) {
	plmn := s1ap.PLMNIdentity{0x00, 0xf1, 0x10}
	rac := s1ap.RAC(7)
	notComprehended := s1ap.CauseProtocolAbstractSyntaxErrorReject.Cause()

	cases := map[string]targetIDCase{
		"targetRNC-ID": {
			s1ap.TargetID{TargetRNCID: &s1ap.TargetRNCID{LAI: s1ap.LAI{PLMNIdentity: plmn, LAC: 1}, RAC: &rac, RNCID: 12}},
			causeUnknownTargetID,
		},
		"cGI": {
			s1ap.TargetID{CGI: &s1ap.CGI{PLMNIdentity: plmn, LAC: 1, CI: 2, RAC: &rac}},
			causeUnknownTargetID,
		},
		"targetgNgRanNode-ID": {
			s1ap.TargetID{TargetNgRanNodeID: &s1ap.TargetNgRanNodeID{
				GlobalRANNodeID: s1ap.GlobalRANNodeID{GNB: &s1ap.GNB{GlobalGNBID: s1ap.GlobalGNBID{
					PLMNIdentity: plmn,
					GNBID:        s1ap.GNBIdentity{GNBID: &s1ap.GNBID{Value: 1, Bits: 24}},
				}}},
				SelectedTAI: s1ap.FiveGSTAI{PLMNIdentity: plmn, TAC: 1},
			}},
			causeUnknownTargetID,
		},
		"unknown extension": {
			s1ap.TargetID{Ext: &per.ExtensionAlternative{Index: 1, Value: "\x00"}},
			notComprehended,
		},
		"targeteNB-ID with unknown eNB-ID alternative": {
			s1ap.TargetID{TargeteNBID: &s1ap.TargeteNBID{
				GlobalENBID: s1ap.GlobalENBID{PLMNIdentity: plmn, ENBID: s1ap.ENBID{Ext: &per.ExtensionAlternative{Index: 2, Value: "\x00"}}},
				SelectedTAI: s1ap.TAI{PLMNIdentity: plmn, TAC: 1},
			}},
			notComprehended,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := newTestMME(t)
			ue, source, target := handoverUE(t, m)

			req := sampleHandoverRequired(ue)
			req.TargetID = tc.targetID

			handleHandoverRequired(context.Background(), m, mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, req.Marshal)))

			fail := preparationFailureOn(t, source)
			if fail.Cause == nil || !fail.Cause.Equal(tc.cause) {
				t.Fatalf("cause = %+v, want %+v", fail.Cause, tc.cause)
			}

			target.mu.Lock()
			sent := len(target.sent)
			target.mu.Unlock()

			if sent != 0 {
				t.Fatalf("target eNB received %d messages, want none", sent)
			}

			if ue.HasHandoverForTest() {
				t.Fatal("handover context left set on failure")
			}
		})
	}
}

func TestHandoverRequiredToFiveGSTargetIDAlternatives(t *testing.T) {
	plmn := s1ap.PLMNIdentity{0x00, 0xf1, 0x10}
	notComprehended := s1ap.CauseProtocolAbstractSyntaxErrorReject.Cause()

	cases := map[string]targetIDCase{
		"targeteNB-ID": {
			s1ap.TargetID{TargeteNBID: &s1ap.TargeteNBID{GlobalENBID: targetGlobalENBID, SelectedTAI: s1ap.TAI{PLMNIdentity: plmn, TAC: 1}}},
			causeUnknownTargetID,
		},
		"targetRNC-ID": {
			s1ap.TargetID{TargetRNCID: &s1ap.TargetRNCID{LAI: s1ap.LAI{PLMNIdentity: plmn, LAC: 1}, RNCID: 12}},
			causeUnknownTargetID,
		},
		"cGI": {
			s1ap.TargetID{CGI: &s1ap.CGI{PLMNIdentity: plmn, LAC: 1, CI: 2}},
			causeUnknownTargetID,
		},
		"unknown extension": {
			s1ap.TargetID{Ext: &per.ExtensionAlternative{Index: 1, Value: "\x00"}},
			notComprehended,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := newTestMME(t)
			ue, source, _ := handoverUE(t, m)

			req := sampleHandoverRequired(ue)
			req.HandoverType = s1ap.HandoverTypeEPSToFiveGS
			req.TargetID = tc.targetID

			handleHandoverRequired(context.Background(), m, mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, req.Marshal)))

			fail := preparationFailureOn(t, source)
			if fail.Cause == nil || !fail.Cause.Equal(tc.cause) {
				t.Fatalf("cause = %+v, want %+v", fail.Cause, tc.cause)
			}
		})
	}
}
