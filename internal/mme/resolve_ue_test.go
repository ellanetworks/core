// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

// parseOutboundErrorIndication decodes an ERROR INDICATION the MME sent.
func parseOutboundErrorIndication(t *testing.T, pdu []byte) *s1ap.ErrorIndication {
	t.Helper()

	msg, err := s1ap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := msg.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcErrorIndication {
		t.Fatalf("expected Error Indication, got %T", msg)
	}

	ind, err := s1ap.ParseErrorIndication(im.Value)
	if err != nil {
		t.Fatalf("parse Error Indication: %v", err)
	}

	return ind
}

func TestResolveUEUnknownMMEUES1APIDSendsErrorIndication(t *testing.T) {
	m := newTestMME(t)

	conn := &captureConn{}
	if ue, ok := m.resolveUE(conn, 4242, 7); ok || ue != nil {
		t.Fatalf("expected resolution to fail for an unknown MME-UE-S1AP-ID")
	}

	if len(conn.sent) != 1 {
		t.Fatalf("expected one Error Indication, got %d", len(conn.sent))
	}

	ind := parseOutboundErrorIndication(t, conn.sent[0])

	if ind.MMEUES1APID == nil || *ind.MMEUES1APID != 4242 {
		t.Fatalf("expected MME-UE-S1AP-ID 4242, got %v", ind.MMEUES1APID)
	}

	if ind.ENBUES1APID == nil || *ind.ENBUES1APID != 7 {
		t.Fatalf("expected eNB-UE-S1AP-ID 7, got %v", ind.ENBUES1APID)
	}

	if ind.Cause == nil || *ind.Cause != causeUnknownMMEUES1APID {
		t.Fatalf("expected cause unknown-mme-ue-s1ap-id, got %v", ind.Cause)
	}
}

func TestResolveUEInconsistentENBUES1APIDSendsErrorIndication(t *testing.T) {
	m := newTestMME(t)
	ue, conn := securedUE(t, m) // ENBUES1APID == 7

	if got, ok := m.resolveUE(conn, ue.s1.MMEUES1APID, 99); ok || got != nil {
		t.Fatalf("expected resolution to fail for a mismatched eNB-UE-S1AP-ID")
	}

	if len(conn.sent) != 1 {
		t.Fatalf("expected one Error Indication, got %d", len(conn.sent))
	}

	ind := parseOutboundErrorIndication(t, conn.sent[0])

	if ind.MMEUES1APID == nil || *ind.MMEUES1APID != ue.s1.MMEUES1APID {
		t.Fatalf("expected MME-UE-S1AP-ID %d, got %v", ue.s1.MMEUES1APID, ind.MMEUES1APID)
	}

	if ind.ENBUES1APID == nil || *ind.ENBUES1APID != 99 {
		t.Fatalf("expected the received eNB-UE-S1AP-ID 99, got %v", ind.ENBUES1APID)
	}

	if ind.Cause == nil || *ind.Cause != causeUnknownPairUES1APID {
		t.Fatalf("expected cause unknown-pair-ue-s1ap-id, got %v", ind.Cause)
	}
}

func TestResolveUEValidPairResolves(t *testing.T) {
	m := newTestMME(t)
	ue, conn := securedUE(t, m) // ENBUES1APID == 7

	got, ok := m.resolveUE(conn, ue.s1.MMEUES1APID, ue.s1.ENBUES1APID)
	if !ok || got != ue {
		t.Fatalf("expected the valid AP-ID pair to resolve to the UE")
	}

	if len(conn.sent) != 0 {
		t.Fatalf("expected no Error Indication for a valid pair, got %d", len(conn.sent))
	}
}
