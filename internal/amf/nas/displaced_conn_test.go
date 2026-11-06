// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
)

type clearPagingHookSmf struct {
	*fakeSmf

	onClear func(context.Context)
}

func (s *clearPagingHookSmf) ClearPagingSuppression(ctx context.Context, _ etsi.SUPI, _ uint8) error {
	if s.onClear != nil {
		hook := s.onClear
		s.onClear = nil

		hook(ctx)
	}

	return nil
}

func displacedTestAMF(s amf.SmfSbi) *amf.AMF {
	return amf.New(&fakeDBInstance{Operator: &db.Operator{
		Mcc: "001", Mnc: "01", AmfRegionID: 0xca, AmfSetID: 0x3f,
	}}, nil, s)
}

func newConnOnOwnRadio(t *testing.T, amfInstance *amf.AMF, address string, ranUeNgapID models.RanUeNgapID) (*amf.UeConn, *fakeNGAPSender) {
	t.Helper()

	sender := &fakeNGAPSender{}
	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	amf.BindRadioLogForTest(radio, address)

	conn, err := amfInstance.NewUeConn(radio, ranUeNgapID)
	if err != nil {
		t.Fatalf("NewUeConn: %v", err)
	}

	return conn, sender
}

func TestHandleNAS_ConnectionDisplacedDuringDispatchIsDiscardedWithoutPanic(t *testing.T) {
	hook := &clearPagingHookSmf{fakeSmf: &fakeSmf{}}
	amfInstance := displacedTestAMF(hook)

	gutiID, guti := gutiWithTMSI(t, [4]byte{0x00, 0x00, 0x00, 0x01})
	ue := securedUeForTest(t, amfInstance, "imsi-001010000000001", guti,
		[16]uint8{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	snssai := models.Snssai{Sst: 1, Sd: "102030"}
	if err := ue.CreateSmContext(1, "ref-1", &snssai, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	conn, sender := newConnOnOwnRadio(t, amfInstance, "gnb-a", 1)

	hook.onClear = func(ctx context.Context) {
		amfInstance.ReleaseNasConnection(ctx, ue, conn)
	}

	pdu := wrapIntegrityProtected(t, ue, plainDeregistrationWithGuti(t, gutiID), 0)

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("NAS dispatch panicked on a connection displaced mid-dispatch: %v", r)
			}
		}()

		HandleNAS(t.Context(), amfInstance, conn, pdu)
	}()

	if ue.State() != amf.Registered {
		t.Fatalf("a NAS message from a connection that no longer serves the UE was processed: state %v", ue.State())
	}

	if len(sender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("answered a NAS message on a connection that no longer serves the UE: %d downlink NAS transports", len(sender.SentDownlinkNASTransport))
	}
}

func TestHandleNAS_OnADisplacedConnectionDoesNotReclaimTheUE(t *testing.T) {
	amfInstance := displacedTestAMF(&fakeSmf{})

	ue := amf.NewUeContext()
	ue.ForceStateForTest(amf.Registered)

	connA, senderA := newConnOnOwnRadio(t, amfInstance, "gnb-a", 1)
	amfInstance.AttachUeConn(t.Context(), ue, connA)

	connB, senderB := newConnOnOwnRadio(t, amfInstance, "gnb-b", 2)
	amfInstance.AttachUeConn(t.Context(), ue, connB)

	HandleNAS(t.Context(), amfInstance, connA, encSR(t, buildTestServiceRequest()))

	if ue.Conn() != connB {
		t.Fatal("a NAS message on a displaced connection moved the UE back onto it")
	}

	if len(senderA.SentDownlinkNASTransport) != 0 || len(senderB.SentDownlinkNASTransport) != 0 {
		t.Fatalf("a NAS message on a displaced connection was answered: %d on the displaced, %d on the current",
			len(senderA.SentDownlinkNASTransport), len(senderB.SentDownlinkNASTransport))
	}
}

func TestHandleNAS_ArrivingOnAStaleConnectionIsNotJudgedAgainstTheCurrentOne(t *testing.T) {
	amfInstance := displacedTestAMF(&fakeSmf{})

	gutiID, guti := gutiWithTMSI(t, [4]byte{0x00, 0x00, 0x00, 0x02})
	ue := securedUeForTest(t, amfInstance, "imsi-001010000000002", guti,
		[16]uint8{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	connA, senderA := newConnOnOwnRadio(t, amfInstance, "gnb-a", 1)
	amfInstance.AttachUeConn(t.Context(), ue, connA)
	connA.MarkSecureExchangeEstablished()
	connA.Release(t.Context())

	connB, senderB := newConnOnOwnRadio(t, amfInstance, "gnb-b", 2)
	amfInstance.AttachUeConn(t.Context(), ue, connB)

	if connA.UeContext() != ue {
		t.Fatal("fixture: the stale connection must still name the UE")
	}

	HandleNAS(t.Context(), amfInstance, connA, plainRegistrationWithGuti(t, gutiID))

	if ue.State() != amf.Registered {
		t.Fatalf("a NAS message from a stale connection was processed as if it came on the current one: state %v", ue.State())
	}

	if len(senderA.SentDownlinkNASTransport) != 0 || len(senderB.SentDownlinkNASTransport) != 0 {
		t.Fatalf("a NAS message from a stale connection was answered: %d on the stale, %d on the current",
			len(senderA.SentDownlinkNASTransport), len(senderB.SentDownlinkNASTransport))
	}

	if ue.Conn() != connB {
		t.Fatal("the UE left its current connection")
	}
}
