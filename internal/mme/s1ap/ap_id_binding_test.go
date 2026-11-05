// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

func handoverAck(targetMME s1ap.MMEUES1APID, enbID s1ap.ENBUES1APID) *s1ap.HandoverRequestAcknowledge {
	return &s1ap.HandoverRequestAcknowledge{
		MMEUES1APID: s1ap.Ptr(targetMME),
		ENBUES1APID: s1ap.Ptr(enbID),
		ERABAdmitted: []s1ap.ERABAdmittedItem{{
			ERABID:                s1ap.ERABID(mme.DefaultERABID),
			TransportLayerAddress: s1ap.TransportLayerAddress{10, 4, 0, 2},
			GTPTEID:               0x99,
		}},
		TargetToSource: s1ap.TransparentContainer{0xaa},
	}
}

func TestHandoverRequestAcknowledgeReusingAnAllocatedENBUES1APIDFailsTheHandover(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	holder := m.NewUeConn(target, 55)

	handleHandoverRequired(context.Background(), m, mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	targetMME := targetMMEUEID(t, target)
	before := target.count()

	handleHandoverRequestAcknowledge(context.Background(), m, mme.NewRadioForTest(target), successfulValue(t, mustMarshal(t, handoverAck(targetMME, 55).Marshal)))

	if n := errorIndicationsSince(t, target, before); n != 1 {
		t.Fatalf("sent %d Error Indications to the target, want 1", n)
	}

	ind := parseOutboundErrorIndication(t, lastSent(t, target))
	if ind.MMEUES1APID == nil || *ind.MMEUES1APID != targetMME || ind.ENBUES1APID == nil || *ind.ENBUES1APID != 55 {
		t.Errorf("Error Indication carries (%v, %v), want the received (%d, 55)", ind.MMEUES1APID, ind.ENBUES1APID, targetMME)
	}

	if heldConn(m, target, holder) {
		t.Error("the connection already holding the eNB-UE-S1AP-ID was not released")
	}

	if ue.HasHandoverForTest() {
		t.Error("the handover was not failed")
	}

	preparationFailureOn(t, source)

	if !ue.Connected() || ue.Conn().ConnForTest() != source {
		t.Error("the UE did not stay on its source connection")
	}

	if got := m.ConnCountForTest(); got != 1 {
		t.Errorf("%d S1 connections held, want only the source", got)
	}
}

func TestHandoverRequestAcknowledgeReusingTheENBUES1APIDOfAConnectionBeingReleasedCompletesThatRelease(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	holder := m.NewUeConn(target, 55)
	if err := holder.SendUEContextReleaseCommand(t.Context(), mme.CauseNASNormalRelease); err != nil {
		t.Fatal(err)
	}

	handleHandoverRequired(context.Background(), m, mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	targetMME := targetMMEUEID(t, target)
	before := target.count()

	handleHandoverRequestAcknowledge(context.Background(), m, mme.NewRadioForTest(target), successfulValue(t, mustMarshal(t, handoverAck(targetMME, 55).Marshal)))

	if n := errorIndicationsSince(t, target, before); n != 0 {
		t.Fatalf("sent %d Error Indications for an eNB-UE-S1AP-ID the eNB freed, want none", n)
	}

	if heldConn(m, target, holder) {
		t.Error("the commanded release was not completed")
	}

	handoverCommandOn(t, source)

	if !ue.HasHandoverForTest() {
		t.Error("the handover did not stay prepared")
	}

	if got, ok := m.LookupUe(targetMME); !ok || got != ue {
		t.Error("the handover target connection was released")
	}
}

func TestPathSwitchRequestReusingAnAllocatedENBUES1APIDIsRejectedAndReleasesTheHolder(t *testing.T) {
	m := newTestMME(t)
	ue := pathSwitchUE(t, m)
	source := ue.Conn().ConnForTest()
	target := &captureConn{}

	holder := m.NewUeConn(target, 42)

	handlePathSwitchRequest(t.Context(), m, mme.NewRadioForTest(target), initiatingValue(t, mustMarshal(t, samplePathSwitchRequest(ue).Marshal)))

	if target.count() != 1 {
		t.Fatalf("sent %d messages, want only the Error Indication", target.count())
	}

	ind := parseOutboundErrorIndication(t, target.sent[0])
	if ind.MMEUES1APID != nil || ind.ENBUES1APID == nil || *ind.ENBUES1APID != 42 {
		t.Errorf("Error Indication carries (%v, %v), want only the received eNB-UE-S1AP-ID 42", ind.MMEUES1APID, ind.ENBUES1APID)
	}

	if ind.Cause == nil || *ind.Cause != causeUnknownENBUES1APID {
		t.Errorf("Error Indication cause = %v, want unknown-enb-ue-s1ap-id", ind.Cause)
	}

	if heldConn(m, target, holder) {
		t.Error("the connection already holding the eNB-UE-S1AP-ID was not released")
	}

	if !ue.Connected() || ue.Conn().ConnForTest() != source {
		t.Error("the UE was moved off its source connection")
	}
}

func TestPathSwitchRequestOnTheUEsOwnENBUES1APIDIsAccepted(t *testing.T) {
	m := newTestMME(t)
	ue := pathSwitchUE(t, m)
	conn := ue.Conn().ConnForTest().(*captureConn)

	req := samplePathSwitchRequest(ue)
	req.ENBUES1APID = ue.Conn().ENBUES1APID()

	handlePathSwitchRequest(t.Context(), m, mme.NewRadioForTest(conn), initiatingValue(t, mustMarshal(t, req.Marshal)))

	if n := errorIndicationsSince(t, conn, 0); n != 0 {
		t.Fatalf("sent %d Error Indications for the UE's own eNB-UE-S1AP-ID, want none", n)
	}

	parsePathSwitchAck(t, lastSent(t, conn))
}

func TestHandoverSourceIsReapedWhenItsReleaseCompleteIsLost(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := newTestMME(t)
		ue, source, target := handoverUE(t, m)

		targetMME, targetENB := driveToPrepared(t, m, ue, source, target)

		handleHandoverNotify(context.Background(), m, mme.NewRadioForTest(target),
			initiatingValue(t, mustMarshal(t, handoverNotify(targetMME, targetENB).Marshal)))

		parseUEContextReleaseCommand(t, lastSent(t, source))

		time.Sleep(time.Minute)
		synctest.Wait()

		if got := m.ConnCountForTest(); got != 1 {
			t.Fatalf("%d S1 connections held after the source's UE Context Release Command went unanswered, want only the target", got)
		}

		if got, ok := m.LookupUe(targetMME); !ok || got != ue {
			t.Fatal("reaping the source disturbed the UE on its target")
		}
	})
}
