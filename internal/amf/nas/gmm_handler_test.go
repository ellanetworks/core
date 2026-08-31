// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/fgs"
)

// TS 24.501 §7.4
func TestHandleGmmMessage_UnhandledMessageType_StatusNotImplemented(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)

	amfInstance := amf.New(nil, nil, nil)

	got := HandleGmmMessage(context.Background(), amfInstance, ue, uint8(fgs.MsgConfigurationUpdateCommand), []byte{0x7e, 0x00, 0x54}, true, false)
	if want := nasreply.StatusMM(nasreply.CauseMessageTypeNotImplemented); got != want {
		t.Fatalf("disposition = %+v, want %+v", got, want)
	}
}

func TestHandleGmmMessage_UndecodablePayload_StatusInvalidMandatoryInfo(t *testing.T) {
	ue := amf.NewUeContext()
	amfInstance := amf.New(nil, nil, nil)

	got := HandleGmmMessage(context.Background(), amfInstance, ue, 0xFF, nil, true, false)
	if want := nasreply.StatusMM(nasreply.CauseInvalidMandatoryInfo); got != want {
		t.Fatalf("disposition = %+v, want %+v", got, want)
	}
}

func TestHandleGmmMessage_DispatchesToConfigurationUpdateComplete(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)
	ue.Conn().NASGuardForTest().Arm(6*time.Minute, 5, func(expireTimes int32) {}, func() {})

	amfInstance := amf.New(nil, nil, nil)

	got := HandleGmmMessage(context.Background(), amfInstance, ue, uint8(fgs.MsgConfigurationUpdateComplete), []byte{0x7e, 0x00, 0x55}, true, false)
	if got != nasreply.Handled() {
		t.Fatalf("disposition = %+v, want %+v", got, nasreply.Handled())
	}

	if ue.Conn().NASGuardForTest().Active() {
		t.Fatal("the completion must stop T3555")
	}
}

func TestHandleGmmMessage_DispatchesToStatus5GMM(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)

	amfInstance := amf.New(nil, nil, nil)

	got := HandleGmmMessage(context.Background(), amfInstance, ue, uint8(fgs.MsgGMMStatus), buildTestStatus5gmmPlain(t), true, false)
	if got != nasreply.Handled() {
		t.Fatalf("disposition = %+v, want %+v", got, nasreply.Handled())
	}
}

func TestHandleGmmMessage_DispatchesToServiceRequest(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: `["000001"]`},
		},
		nil,
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)
	ue.SetSecuredForTest(true)

	plain := encSR(t, buildTestServiceRequest())

	got := HandleGmmMessage(context.Background(), amfInstance, ue, uint8(fgs.MsgServiceRequest), plain, true, false)

	if got.Action == nasreply.ActionStatus && got.Cause == nasreply.CauseMessageTypeNotImplemented {
		t.Fatal("SERVICE REQUEST answered with 5GMM STATUS #97; it must run the service request procedure")
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("downlinks = %d, want 1 (SERVICE ACCEPT)", len(ngapSender.SentDownlinkNASTransport))
	}
}

func TestHandleNAS_PlainServiceRequest_Discarded(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: `["000001"]`},
		},
		nil,
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)
	ue.SetSecuredForTest(true)

	HandleNAS(context.Background(), amfInstance, ue.Conn(), encSR(t, buildTestServiceRequest()))

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("plain SERVICE REQUEST drew %d downlinks, want 0 (TS 24.501 §4.4.4.3)", len(ngapSender.SentDownlinkNASTransport))
	}
}
