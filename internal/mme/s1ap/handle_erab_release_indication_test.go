// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

func releaseIndication(t *testing.T, ue *mme.UeContext, ebi uint8) []byte {
	t.Helper()

	msg := &s1ap.ERABReleaseIndication{
		MMEUES1APID:  ue.Conn().MMEUES1APID,
		ENBUES1APID:  ue.Conn().ENBUES1APID(),
		ERABReleased: []s1ap.ERABItem{{ERABID: s1ap.ERABID(ebi), Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkRadioConnectionWithUELost}}},
	}

	return initiatingValue(t, mustMarshal(t, msg.Marshal))
}

func TestERABReleaseIndicationReleasesTheVoiceBearerWithoutNAS(t *testing.T) {
	m := newTestMME(t)
	ue, source, _ := handoverUE(t, m)

	m.LookupPDN(ue, mme.DefaultERABID).Dedicated = map[uint8]*mme.DedicatedBearer{6: {DedicatedBearerInfo: mme.DedicatedBearerInfo{Ebi: 6, QCI: 1}}}

	sent := source.count()

	handleERABReleaseIndication(context.Background(), m, mme.NewRadioForTest(source), releaseIndication(t, ue, 6))

	if _, b := m.LookupDedicated(ue, 6); b != nil {
		t.Fatal("the voice bearer the eNB released is still held")
	}

	if source.count() != sent {
		t.Fatal("the core signalled a bearer the eNB already released (TS 23.401 §5.4.4.2)")
	}
}

func TestERABReleaseIndicationOfADefaultBearerDisconnectsItsPDN(t *testing.T) {
	m := newTestMME(t)
	ue, source, _ := handoverUE(t, m)
	secondPDN(ue)

	handleERABReleaseIndication(context.Background(), m, mme.NewRadioForTest(source), releaseIndication(t, ue, 6))

	requirePDNDisconnected(t, m, ue, 6)

	if m.LookupPDN(ue, mme.DefaultERABID) == nil {
		t.Fatal("the other PDN connection was released too")
	}

	pdu, err := s1ap.Unmarshal(lastSent(t, source))
	if err != nil {
		t.Fatal(err)
	}

	if im, ok := pdu.(*s1ap.InitiatingMessage); !ok || im.ProcedureCode != s1ap.ProcERABRelease {
		t.Fatalf("sent %T, want an E-RAB Release Command carrying the PDN deactivation (TS 23.401 §5.10.3)", pdu)
	}
}
