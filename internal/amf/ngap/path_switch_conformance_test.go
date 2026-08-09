// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/ngap"
)

// Conformance tests for the Xn handover Path Switch procedure.
var errSmfRefused = errors.New("SMF refused the path switch")

// pathSwitchTestUE builds a secured UE with the given PDU sessions, its source
// association, and an AMF wired to fakeSmf. It returns the UE and the target radio
// the PATH SWITCH REQUEST arrives on.
func pathSwitchTestUE(t *testing.T, fakeSmf *fakeSmfSbi, pduSessionIDs ...uint8) (*amf.AMF, *amf.UeContext, *amf.Radio) {
	t.Helper()

	const sourceAmfUeNgapID = int64(10)

	sourceRan := &amf.Radio{Log: logger.AmfLog, Conn: &fakeNGAPSender{}}
	targetRan := &amf.Radio{Log: logger.AmfLog, Conn: &fakeNGAPSender{}}

	amfUe := newValidUeContext()

	for _, id := range pduSessionIDs {
		amfUe.SmContextList[id] = &amf.SmContext{
			Ref:    smContextRefFor(id),
			Snssai: &models.Snssai{Sst: 1},
		}
	}

	sourceUe := amf.NewUeConnForTest(sourceRan, 1, models.AmfUeNgapID(sourceAmfUeNgapID), logger.AmfLog)
	sourceUe.AMFForTest().AttachUeConn(amfUe, sourceUe)

	amfInstance := newTestAMFWithSmf(fakeSmf)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)
	sourceRan.BindAMFForTest(amfInstance)
	targetRan.BindAMFForTest(amfInstance)

	return amfInstance, amfUe, targetRan
}

func smContextRefFor(pduSessionID uint8) string {
	return "imsi-001010000000001-" + string(rune('0'+pduSessionID))
}

func switchedDLItem(t *testing.T, pduSessionID uint8, teid uint32) ngap.PDUSessionResourceToBeSwitchedDLItem {
	t.Helper()

	transfer, err := buildPathSwitchRequestTransfer(teid, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("build transfer: %v", err)
	}

	return ngap.PDUSessionResourceToBeSwitchedDLItem{
		PDUSessionID: ngap.PDUSessionID(pduSessionID),
		Transfer:     transfer,
	}
}

func hasRef(refs []string, want string) bool {
	for _, r := range refs {
		if r == want {
			return true
		}
	}

	return false
}

func TestPathSwitchKeepsSessionsTheGNBDidNotList(t *testing.T) {
	fakeSmf := &fakeSmfSbi{PathSwitchResponse: []byte{0xAA}}
	amfInstance, amfUe, targetRan := pathSwitchTestUE(t, fakeSmf, 1, 2)

	// The target gNB reports only session 1; session 2 is omitted.
	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(10)),
		ngap.Ptr(ngap.RANUENGAPID(2)),
		ngap.PDUSessionResourceToBeSwitchedDLList{switchedDLItem(t, 1, 5000)},
		nil, nil,
	)

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

	if hasRef(fakeSmf.ReleaseSmContextCalls, smContextRefFor(2)) {
		t.Errorf("PDU session the gNB did not list was released; ReleaseSmContext calls = %v", fakeSmf.ReleaseSmContextCalls)
	}

	if _, kept := amfUe.SmContextFindByPDUSessionID(2); !kept {
		t.Error("PDU session the gNB did not list was dropped from the UE context")
	}

	if _, kept := amfUe.SmContextFindByPDUSessionID(1); !kept {
		t.Error("the switched PDU session must be kept")
	}
}

// TS 38.413 §9.2.3.8
func TestPathSwitchReportsUndecodablePDUSessionID(t *testing.T) {
	fakeSmf := &fakeSmfSbi{PathSwitchResponse: []byte{0xAA}}
	amfInstance, _, targetRan := pathSwitchTestUE(t, fakeSmf, 1)

	targetSender, ok := targetRan.Conn.(*fakeNGAPSender)
	if !ok {
		t.Fatalf("target conn is %T", targetRan.Conn)
	}

	bad := switchedDLItem(t, 1, 5000)
	bad.PDUSessionID = 0 // outside the valid 1..15 range

	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(10)),
		ngap.Ptr(ngap.RANUENGAPID(2)),
		ngap.PDUSessionResourceToBeSwitchedDLList{switchedDLItem(t, 1, 5000), bad},
		nil, nil,
	)

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

	if len(targetSender.SentPathSwitchRequestAcknowledges) != 1 {
		t.Fatalf("expected one acknowledge, got %d", len(targetSender.SentPathSwitchRequestAcknowledges))
	}

	ack := targetSender.SentPathSwitchRequestAcknowledges[0]
	if len(ack.PDUSessionResourceReleased) != 1 {
		t.Fatalf("released list = %+v, want the undecodable session reported", ack.PDUSessionResourceReleased)
	}
}

// TS 38.413 §8.4.4.2 / TS 23.502 §4.9.1.2.2
func TestPathSwitchFailedSessionReleasesCoreResources(t *testing.T) {
	fakeSmf := &fakeSmfSbi{PathSwitchResponse: []byte{0xAA}, PathSwitchErr: errSmfRefused}
	amfInstance, amfUe, targetRan := pathSwitchTestUE(t, fakeSmf, 1)

	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(10)),
		ngap.Ptr(ngap.RANUENGAPID(2)),
		ngap.PDUSessionResourceToBeSwitchedDLList{switchedDLItem(t, 1, 5000)},
		nil, nil,
	)

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

	if !hasRef(fakeSmf.ReleaseSmContextCalls, smContextRefFor(1)) {
		t.Errorf("a PDU session that failed to switch kept its core context; ReleaseSmContext calls = %v", fakeSmf.ReleaseSmContextCalls)
	}

	if _, still := amfUe.SmContextFindByPDUSessionID(1); still {
		t.Error("a PDU session that failed to switch is still in the UE context")
	}
}

// TS 38.413 §8.4.4.3: when no session could be switched the AMF answers with PATH
// SWITCH REQUEST FAILURE. Unlike EPS, no detach follows — TS 23.502 §4.9.1.2 leaves
// the UE's fate to the RAN, where TS 23.401 §5.5.1.1.2 step 6 mandates one.
func TestPathSwitchNoSessionSwitchedSendsFailure(t *testing.T) {
	fakeSmf := &fakeSmfSbi{PathSwitchErr: errSmfRefused}
	amfInstance, _, targetRan := pathSwitchTestUE(t, fakeSmf, 1)

	targetSender, ok := targetRan.Conn.(*fakeNGAPSender)
	if !ok {
		t.Fatalf("target conn is %T", targetRan.Conn)
	}

	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(10)),
		ngap.Ptr(ngap.RANUENGAPID(2)),
		ngap.PDUSessionResourceToBeSwitchedDLList{switchedDLItem(t, 1, 5000)},
		nil, nil,
	)

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

	if len(targetSender.SentPathSwitchRequestFailures) != 1 {
		t.Fatalf("expected one PathSwitchRequestFailure, got %d", len(targetSender.SentPathSwitchRequestFailures))
	}

	if len(targetSender.SentPathSwitchRequestAcknowledges) != 0 {
		t.Error("a path switch that switched nothing must not be acknowledged")
	}
}
