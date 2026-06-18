// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

// servedPLMN is the operator PLMN the fake bearer store serves (MCC 001 / MNC 01).
var servedPLMN = models.PlmnID{Mcc: "001", Mnc: "01"}

func servedPLMNIdentity(t *testing.T) s1ap.PLMNIdentity {
	t.Helper()

	p, err := encodePLMN(servedPLMN)
	if err != nil {
		t.Fatal(err)
	}

	return p
}

// TestENBConfigUpdateAcknowledged confirms an update whose TAs still broadcast a
// served PLMN (or that changes only the name) is acknowledged (TS 36.413 §8.7.4).
func TestENBConfigUpdateAcknowledged(t *testing.T) {
	cases := []struct {
		name string
		req  *s1ap.ENBConfigurationUpdate
	}{
		{"name only", &s1ap.ENBConfigurationUpdate{ENBName: "enb-renamed"}},
		{"served TA", &s1ap.ENBConfigurationUpdate{
			SupportedTAs: s1ap.SupportedTAs{{TAC: 7, BroadcastPLMNs: s1ap.BPLMNs{servedPLMNIdentity(t)}}},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, accepted, err := enbConfigUpdateOutcomeFor(tc.req, servedPLMN)
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

// TestENBConfigUpdateRejectedUnknownPLMN confirms an update whose TAs broadcast
// no served PLMN draws an ENB CONFIGURATION UPDATE FAILURE with Unknown PLMN.
func TestENBConfigUpdateRejectedUnknownPLMN(t *testing.T) {
	foreign := s1ap.PLMNIdentity{0x09, 0xf9, 0x99} // MCC 999 / MNC 99

	req := &s1ap.ENBConfigurationUpdate{
		SupportedTAs: s1ap.SupportedTAs{{TAC: 7, BroadcastPLMNs: s1ap.BPLMNs{foreign}}},
	}

	out, accepted, err := enbConfigUpdateOutcomeFor(req, servedPLMN)
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

	if failure.Cause != causeUnknownPLMN {
		t.Fatalf("cause = %+v, want Unknown PLMN %+v", failure.Cause, causeUnknownPLMN)
	}
}
