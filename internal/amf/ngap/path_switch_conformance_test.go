// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/ngap"
)

var errSmfRefused = errors.New("SMF refused the path switch")

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

func switchedDLItem(t *testing.T) ngap.PDUSessionResourceToBeSwitchedDLItem {
	t.Helper()

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatalf("build transfer: %v", err)
	}

	return ngap.PDUSessionResourceToBeSwitchedDLItem{
		PDUSessionID: 1,
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

	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(10)),
		ngap.Ptr(ngap.RANUENGAPID(2)),
		ngap.PDUSessionResourceToBeSwitchedDLList{switchedDLItem(t)},
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

	bad := switchedDLItem(t)
	bad.PDUSessionID = 0

	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(10)),
		ngap.Ptr(ngap.RANUENGAPID(2)),
		ngap.PDUSessionResourceToBeSwitchedDLList{switchedDLItem(t), bad},
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

func TestPathSwitchFailedSessionDeactivatesUserPlane(t *testing.T) {
	fakeSmf := &fakeSmfSbi{PathSwitchResponse: []byte{0xAA}, PathSwitchErr: errSmfRefused}
	amfInstance, amfUe, targetRan := pathSwitchTestUE(t, fakeSmf, 1)

	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(10)),
		ngap.Ptr(ngap.RANUENGAPID(2)),
		ngap.PDUSessionResourceToBeSwitchedDLList{switchedDLItem(t)},
		nil, nil,
	)

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

	if !hasRef(fakeSmf.DeactivateSmContextCalls, smContextRefFor(1)) {
		t.Errorf("user plane not deactivated; DeactivateSmContext calls = %v", fakeSmf.DeactivateSmContextCalls)
	}

	if hasRef(fakeSmf.ReleaseSmContextCalls, smContextRefFor(1)) {
		t.Errorf("session was released where 3GPP asks for UP deactivation: %v", fakeSmf.ReleaseSmContextCalls)
	}

	sc, still := amfUe.SmContextFindByPDUSessionID(1)
	if !still {
		t.Fatal("the PDU session must survive a failed switch")
	}

	if !sc.PduSessionInactive {
		t.Error("the PDU session must be marked user-plane inactive")
	}
}

// TS 38.413 §8.4.4.3
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
		ngap.PDUSessionResourceToBeSwitchedDLList{switchedDLItem(t)},
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

const (
	ieIDUESecurityCapabilities  = 119
	ieIDUserLocationInformation = 121
)

func dropNGAPIEs(t *testing.T, value []byte, ids ...uint16) []byte {
	t.Helper()

	drop := make(map[uint16]bool, len(ids))
	for _, id := range ids {
		drop[id] = true
	}

	if len(value) < 3 {
		t.Fatalf("message body too short: %d bytes", len(value))
	}

	preamble, count, pos := value[0], binary.BigEndian.Uint16(value[1:3]), 3

	kept := make([]byte, 0, len(value))
	keptCount := uint16(0)

	for i := uint16(0); i < count; i++ {
		start := pos
		if pos+4 > len(value) {
			t.Fatalf("truncated IE header at offset %d", pos)
		}

		id := binary.BigEndian.Uint16(value[pos : pos+2])
		pos += 3

		n := int(value[pos])

		switch {
		case n < 0x80:
			pos++
		case n < 0xC0:
			n = (n&0x3F)<<8 | int(value[pos+1])
			pos += 2
		default:
			t.Fatalf("fragmented IE length at offset %d is not supported", pos)
		}

		if pos+n > len(value) {
			t.Fatalf("IE 0x%04x claims %d bytes, only %d remain", id, n, len(value)-pos)
		}

		pos += n

		if drop[id] {
			continue
		}

		kept = append(kept, value[start:pos]...)
		keptCount++
	}

	out := make([]byte, 3, 3+len(kept))
	out[0] = preamble
	binary.BigEndian.PutUint16(out[1:3], keptCount)

	return append(out, kept...)
}

func TestPathSwitchAckCarriesMandatoryIEs(t *testing.T) {
	fakeSmf := &fakeSmfSbi{PathSwitchResponse: []byte{0xAA}}
	amfInstance, _, targetRan := pathSwitchTestUE(t, fakeSmf, 1)

	targetSender, ok := targetRan.Conn.(*fakeNGAPSender)
	if !ok {
		t.Fatalf("target conn is %T", targetRan.Conn)
	}

	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(10)),
		ngap.Ptr(ngap.RANUENGAPID(2)),
		ngap.PDUSessionResourceToBeSwitchedDLList{switchedDLItem(t)},
		nil, nil,
	)

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

	if len(targetSender.SentPathSwitchRequestAcknowledges) != 1 {
		t.Fatalf("expected one acknowledge, got %d", len(targetSender.SentPathSwitchRequestAcknowledges))
	}

	ack := targetSender.SentPathSwitchRequestAcknowledges[0]

	if ack.AMFUENGAPID == nil {
		t.Error("acknowledge carries no AMF UE NGAP ID")
	}

	if ack.RANUENGAPID == nil {
		t.Error("acknowledge carries no RAN UE NGAP ID")
	}

	if len(ack.AllowedNSSAI) == 0 {
		t.Error("acknowledge carries no Allowed NSSAI (mandatory, reject criticality)")
	}

	if len(ack.PDUSessionResourceSwitchedList) == 0 {
		t.Error("acknowledge carries an empty PDU Session Resource Switched List (Range 1)")
	}
}

func TestPathSwitchFailureCarriesMandatoryIEs(t *testing.T) {
	for _, tt := range []struct {
		name    string
		fakeSmf *fakeSmfSbi
		sourceA int64
	}{
		{name: "no session switched", fakeSmf: &fakeSmfSbi{PathSwitchErr: errSmfRefused}, sourceA: 10},
		{name: "unknown source AMF UE NGAP ID", fakeSmf: &fakeSmfSbi{PathSwitchResponse: []byte{0xAA}}, sourceA: 0x7ffffff0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			amfInstance, _, targetRan := pathSwitchTestUE(t, tt.fakeSmf, 1)

			targetSender, ok := targetRan.Conn.(*fakeNGAPSender)
			if !ok {
				t.Fatalf("target conn is %T", targetRan.Conn)
			}

			msg := buildPathSwitchRequest(
				ngap.Ptr(ngap.AMFUENGAPID(tt.sourceA)),
				ngap.Ptr(ngap.RANUENGAPID(2)),
				ngap.PDUSessionResourceToBeSwitchedDLList{switchedDLItem(t)},
				nil, nil,
			)

			HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

			if len(targetSender.SentPathSwitchRequestFailures) != 1 {
				t.Fatalf("expected one failure, got %d", len(targetSender.SentPathSwitchRequestFailures))
			}

			fail := targetSender.SentPathSwitchRequestFailures[0]

			if fail.AMFUENGAPID == nil {
				t.Error("failure carries no AMF UE NGAP ID")
			}

			if fail.RANUENGAPID == nil {
				t.Error("failure carries no RAN UE NGAP ID")
			}

			if len(fail.PDUSessionResourceReleased) == 0 {
				t.Error("failure carries an empty PDU Session Resource Released List (Range 1)")
			}
		})
	}
}

func TestPathSwitchAckCarriesFreshlyDerivedNH(t *testing.T) {
	fakeSmf := &fakeSmfSbi{PathSwitchResponse: []byte{0xAA}}
	amfInstance, amfUe, targetRan := pathSwitchTestUE(t, fakeSmf, 1)

	targetSender, ok := targetRan.Conn.(*fakeNGAPSender)
	if !ok {
		t.Fatalf("target conn is %T", targetRan.Conn)
	}

	beforeNH, beforeNCC := amfUe.NHForTest(), amfUe.NCCForTest()

	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(10)),
		ngap.Ptr(ngap.RANUENGAPID(2)),
		ngap.PDUSessionResourceToBeSwitchedDLList{switchedDLItem(t)},
		nil, nil,
	)

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

	if len(targetSender.SentPathSwitchRequestAcknowledges) != 1 {
		t.Fatalf("expected one acknowledge, got %d", len(targetSender.SentPathSwitchRequestAcknowledges))
	}

	ack := targetSender.SentPathSwitchRequestAcknowledges[0]

	if ack.SecurityContext.NextHopChainingCount != (beforeNCC+1)%8 {
		t.Errorf("acknowledge NCC = %d, want %d", ack.SecurityContext.NextHopChainingCount, (beforeNCC+1)%8)
	}

	if ack.SecurityContext.NextHopNH == beforeNH {
		t.Error("acknowledge replayed the current NH instead of a freshly derived one")
	}

	if amfUe.NHForTest() != ack.SecurityContext.NextHopNH {
		t.Error("the pair sent to the target must be the UE's committed chain")
	}
}

func TestPathSwitchNCCWrapsToZero(t *testing.T) {
	fakeSmf := &fakeSmfSbi{PathSwitchResponse: []byte{0xAA}}
	amfInstance, amfUe, targetRan := pathSwitchTestUE(t, fakeSmf, 1)

	amfUe.SetNCCForTest(7)

	targetSender, ok := targetRan.Conn.(*fakeNGAPSender)
	if !ok {
		t.Fatalf("target conn is %T", targetRan.Conn)
	}

	msg := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(10)),
		ngap.Ptr(ngap.RANUENGAPID(2)),
		ngap.PDUSessionResourceToBeSwitchedDLList{switchedDLItem(t)},
		nil, nil,
	)

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

	if len(targetSender.SentPathSwitchRequestAcknowledges) != 1 {
		t.Fatalf("expected one acknowledge, got %d", len(targetSender.SentPathSwitchRequestAcknowledges))
	}

	if got := targetSender.SentPathSwitchRequestAcknowledges[0].SecurityContext.NextHopChainingCount; got != 0 {
		t.Errorf("NCC after 7 = %d, want 0", got)
	}

	if amfUe.NCCForTest() != 0 {
		t.Errorf("stored NCC after 7 = %d, want 0", amfUe.NCCForTest())
	}
}

func TestPathSwitchToleratesAbsentIgnoreCriticalityIEs(t *testing.T) {
	fakeSmf := &fakeSmfSbi{PathSwitchResponse: []byte{0xAA}}
	amfInstance, _, targetRan := pathSwitchTestUE(t, fakeSmf, 1)

	targetSender, ok := targetRan.Conn.(*fakeNGAPSender)
	if !ok {
		t.Fatalf("target conn is %T", targetRan.Conn)
	}

	full := buildPathSwitchRequest(
		ngap.Ptr(ngap.AMFUENGAPID(10)),
		ngap.Ptr(ngap.RANUENGAPID(2)),
		ngap.PDUSessionResourceToBeSwitchedDLList{switchedDLItem(t)},
		nil,
		ngap.Ptr(ngap.UESecurityCapabilities{NREncryptionAlgorithms: 0xc000, NRIntegrityProtectionAlgorithms: 0xc000}),
	)

	encoded, err := full.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	pdu, err := ngap.Unmarshal(encoded)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	initiating, ok := pdu.(*ngap.InitiatingMessage)
	if !ok {
		t.Fatalf("expected InitiatingMessage, got %T", pdu)
	}

	stripped, err := ngap.ParsePathSwitchRequest(
		dropNGAPIEs(t, initiating.Value, ieIDUserLocationInformation, ieIDUESecurityCapabilities))
	if err != nil {
		t.Fatalf("a message missing only ignore-criticality IEs must still parse: %v", err)
	}

	if stripped.UserLocationInformation != nil || stripped.UESecurityCapabilities != nil {
		t.Fatal("dropNGAPIEs did not strip the ignore-criticality IEs")
	}

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, stripped)

	if len(targetSender.SentPathSwitchRequestAcknowledges) != 1 {
		t.Fatalf("path switch did not complete: %d acknowledges, %d failures",
			len(targetSender.SentPathSwitchRequestAcknowledges), len(targetSender.SentPathSwitchRequestFailures))
	}
}
