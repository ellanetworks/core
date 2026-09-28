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

func TestUEContextModificationFailureForgetsTheUEAMBR(t *testing.T) {
	m := newTestMME(t)
	conn := &captureConn{}
	ue := m.NewUe(t.Context(), conn, 7)
	m.RegisterUEForTest(ue, "001010000000001")

	ueConn := ue.Conn()

	if err := ueConn.SendUEContextModification(context.Background(), models.Ambr{
		Uplink:   models.MustParseBitRate("50 Mbps"),
		Downlink: models.MustParseBitRate("50 Mbps"),
	}); err != nil {
		t.Fatal(err)
	}

	if _, ok := ueConn.HeldUEAMBR(); !ok {
		t.Fatal("the sent UE-AMBR was not recorded")
	}

	wire, err := (&s1ap.UEContextModificationFailure{
		MMEUES1APID: s1ap.Ptr(ueConn.MMEUES1APID),
		ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(7)),
		Cause:       &s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := s1ap.Unmarshal(wire)
	if err != nil {
		t.Fatal(err)
	}

	handleUEContextModificationFailure(context.Background(), m, mme.NewRadioForTest(conn), pdu.(*s1ap.UnsuccessfulOutcome).Value)

	if _, ok := ueConn.HeldUEAMBR(); ok {
		t.Fatal("the UE-AMBR the eNB refused is still recorded as held")
	}
}

func TestUEContextModificationResponseKeepsTheUEAMBR(t *testing.T) {
	m := newTestMME(t)
	conn := &captureConn{}
	ue := m.NewUe(t.Context(), conn, 7)
	m.RegisterUEForTest(ue, "001010000000001")

	ueConn := ue.Conn()

	if err := ueConn.SendUEContextModification(context.Background(), models.Ambr{
		Uplink:   models.MustParseBitRate("50 Mbps"),
		Downlink: models.MustParseBitRate("50 Mbps"),
	}); err != nil {
		t.Fatal(err)
	}

	wire, err := (&s1ap.UEContextModificationResponse{
		MMEUES1APID: s1ap.Ptr(ueConn.MMEUES1APID),
		ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(7)),
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := s1ap.Unmarshal(wire)
	if err != nil {
		t.Fatal(err)
	}

	handleUEContextModificationResponse(context.Background(), m, mme.NewRadioForTest(conn), pdu.(*s1ap.SuccessfulOutcome).Value)

	if _, ok := ueConn.HeldUEAMBR(); !ok {
		t.Fatal("an accepted UE-AMBR was dropped")
	}
}
