// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

var servedPLMN = models.PlmnID{Mcc: "001", Mnc: "01"}

func servedPLMNIdentity(t *testing.T) s1ap.PLMNIdentity {
	t.Helper()

	p, err := mme.EncodePLMN(servedPLMN)
	if err != nil {
		t.Fatal(err)
	}

	return p
}

// TS 36.413 §8.7.4
func TestENBConfigUpdateAcknowledged(t *testing.T) {
	cases := []struct {
		name string
		req  *s1ap.ENBConfigurationUpdate
	}{
		{"name only", &s1ap.ENBConfigurationUpdate{ENBName: new("enb-renamed")}},
		{"served TA", &s1ap.ENBConfigurationUpdate{
			SupportedTAs: s1ap.SupportedTAs{{TAC: 7, BroadcastPLMNs: s1ap.BPLMNs{servedPLMNIdentity(t)}}},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, out, accepted, _, err := enbConfigUpdateOutcomeFor(tc.req, servedPLMNIdentity(t), []uint16{7})
			if err != nil {
				t.Fatal(err)
			}

			if !accepted {
				t.Fatal("update rejected, want acknowledged")
			}

			pdu, err := s1ap.Unmarshal(out)
			if err != nil {
				t.Fatal(err)
			}

			so, ok := pdu.(*s1ap.SuccessfulOutcome)
			if !ok || so.ProcedureCode != s1ap.ProcENBConfigurationUpdate {
				t.Fatalf("expected ENB Configuration Update Acknowledge, got %T", pdu)
			}
		})
	}
}

func TestENBConfigUpdateRejectedUnknownPLMN(t *testing.T) {
	foreign := s1ap.PLMNIdentity{0x09, 0xf9, 0x99}

	req := &s1ap.ENBConfigurationUpdate{
		SupportedTAs: s1ap.SupportedTAs{{TAC: 7, BroadcastPLMNs: s1ap.BPLMNs{foreign}}},
	}

	_, out, accepted, _, err := enbConfigUpdateOutcomeFor(req, servedPLMNIdentity(t), []uint16{7})
	if err != nil {
		t.Fatal(err)
	}

	if accepted {
		t.Fatal("update acknowledged, want rejected for Unknown PLMN")
	}

	pdu, err := s1ap.Unmarshal(out)
	if err != nil {
		t.Fatal(err)
	}

	uo, ok := pdu.(*s1ap.UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != s1ap.ProcENBConfigurationUpdate {
		t.Fatalf("expected ENB Configuration Update Failure, got %T", pdu)
	}

	failure, err := s1ap.ParseENBConfigurationUpdateFailure(uo.Value)
	if err != nil {
		t.Fatal(err)
	}

	if failure.Cause == nil || *failure.Cause != causeUnknownPLMN {
		t.Fatalf("cause = %+v, want Unknown PLMN %+v", failure.Cause, causeUnknownPLMN)
	}
}

func TestENBConfigUpdateRejectedUnknownTAC(t *testing.T) {
	req := &s1ap.ENBConfigurationUpdate{
		SupportedTAs: s1ap.SupportedTAs{{TAC: 7, BroadcastPLMNs: s1ap.BPLMNs{servedPLMNIdentity(t)}}},
	}

	_, out, accepted, _, err := enbConfigUpdateOutcomeFor(req, servedPLMNIdentity(t), []uint16{1})
	if err != nil {
		t.Fatal(err)
	}

	if accepted {
		t.Fatal("update acknowledged, want rejected for unserved TAC")
	}

	pdu, err := s1ap.Unmarshal(out)
	if err != nil {
		t.Fatal(err)
	}

	uo, ok := pdu.(*s1ap.UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != s1ap.ProcENBConfigurationUpdate {
		t.Fatalf("expected ENB Configuration Update Failure, got %T", pdu)
	}

	failure, err := s1ap.ParseENBConfigurationUpdateFailure(uo.Value)
	if err != nil {
		t.Fatal(err)
	}

	if failure.Cause == nil || *failure.Cause != causeNoServedTAC {
		t.Fatalf("cause = %+v, want Misc/unspecified %+v", failure.Cause, causeNoServedTAC)
	}
}

// TS 36.413 §8.7.4.2
func TestHandleENBConfigurationUpdate_AbsentTAsPreservesAndAcks(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	radio := mme.NewRadioForTest(cc)
	radio.BindMMEForTest(m)

	stored := []mme.SupportedTAI{{Tai: models.Tai{Tac: "0007", PlmnID: &servedPLMN}}}
	m.UpdateRadioSupportedTAs(radio, stored)
	m.UpdateRadioName(radio, "enb-old")

	b, err := (&s1ap.ENBConfigurationUpdate{ENBName: s1ap.Ptr("enb-new")}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handleENBConfigurationUpdate(m, context.Background(), radio, initiatingValue(t, b))

	if cc.count() != 1 {
		t.Fatalf("expected 1 response, got %d", cc.count())
	}

	assertSuccessfulOutcome(t, cc.sent[0])

	if tais := m.RadioSupportedTAsForTest(radio); len(tais) != 1 || tais[0].Tai.Tac != "0007" {
		t.Fatalf("absent Supported TAs must leave the stored TAs unchanged, got %+v", tais)
	}

	if name := radio.NodeName(); name != "enb-new" {
		t.Fatalf("eNB name = %q, want enb-new", name)
	}
}

func TestHandleENBConfigurationUpdate_RejectPreservesTAs(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	radio := mme.NewRadioForTest(cc)
	radio.BindMMEForTest(m)

	stored := []mme.SupportedTAI{{Tai: models.Tai{Tac: "0007", PlmnID: &servedPLMN}}}
	m.UpdateRadioSupportedTAs(radio, stored)

	b, err := (&s1ap.ENBConfigurationUpdate{
		SupportedTAs: s1ap.SupportedTAs{{TAC: 0xFF, BroadcastPLMNs: s1ap.BPLMNs{servedPLMNIdentity(t)}}},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handleENBConfigurationUpdate(m, context.Background(), radio, initiatingValue(t, b))

	if cc.count() != 1 {
		t.Fatalf("expected 1 response, got %d", cc.count())
	}

	assertUnsuccessfulOutcome(t, cc.sent[0])

	if tais := m.RadioSupportedTAsForTest(radio); len(tais) != 1 || tais[0].Tai.Tac != "0007" {
		t.Fatalf("a rejected update must not discard the stored TAs, got %+v", tais)
	}
}

func assertSuccessfulOutcome(t *testing.T, b []byte) {
	t.Helper()

	pdu, err := s1ap.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	if so, ok := pdu.(*s1ap.SuccessfulOutcome); !ok || so.ProcedureCode != s1ap.ProcENBConfigurationUpdate {
		t.Fatalf("expected ENB Configuration Update Acknowledge, got %T", pdu)
	}
}

func assertUnsuccessfulOutcome(t *testing.T, b []byte) {
	t.Helper()

	pdu, err := s1ap.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	if uo, ok := pdu.(*s1ap.UnsuccessfulOutcome); !ok || uo.ProcedureCode != s1ap.ProcENBConfigurationUpdate {
		t.Fatalf("expected ENB Configuration Update Failure, got %T", pdu)
	}
}
