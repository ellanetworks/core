// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
)

func TestHandleNotificationResponse_NotRegisteredIgnored(t *testing.T) {
	testcases := []amf.StateType{amf.Deregistered, amf.RegistrationInitiated, amf.DeregistrationInitiated}

	for _, tc := range testcases {
		t.Run(string(tc), func(t *testing.T) {
			ue, _, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not build test UE and radio: %v", err)
			}

			ue.ForceStateForTest(tc)
			ue.Conn().NASGuardForTest().Arm(5*time.Minute, 5, func(expireTimes int32) {}, func() {})

			handleNotificationResponse(t.Context(), nil, ue, nil)

			if !ue.Conn().NASGuardForTest().Active() {
				t.Fatal("expected out-of-state Notification Response to be ignored, leaving the NAS guard armed")
			}
		})
	}
}

func TestHandleNotificationResponse_T3565Stopped_NoPDUSessionStatus_NoSmContextReleased(t *testing.T) {
	smf := fakeSmf{Error: nil, ReleasedSmContext: make([]string, 0)}
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, nil, &smf)

	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)
	ue.Conn().NASGuardForTest().Arm(5*time.Minute, 5, func(expireTimes int32) {}, func() {})

	handleNotificationResponse(t.Context(), amfInstance, ue, buildTestNotificationResponse(t, nil))

	if ue.Conn().NASGuardForTest().Active() {
		t.Fatal("expected timer T3565 to be stopped and cleared")
	}

	if len(smf.ReleasedSmContext) != 0 {
		t.Fatalf("should not have released any SM Context")
	}
}

func TestHandleNotificationResponse_T3565Stopped_PDUSessionStatus_SmContextReleased(t *testing.T) {
	smf := fakeSmf{Error: nil, ReleasedSmContext: make([]string, 0)}
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, nil, &smf)

	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)
	ue.Conn().NASGuardForTest().Arm(5*time.Minute, 5, func(expireTimes int32) {}, func() {})
	_ = ue.CreateSmContext(1, "1", &models.Snssai{}, "internet")
	_ = ue.CreateSmContext(5, "5", &models.Snssai{}, "internet")
	_ = ue.CreateSmContext(8, "8", &models.Snssai{}, "internet")
	_ = ue.CreateSmContext(11, "11", &models.Snssai{}, "internet")
	_ = ue.CreateSmContext(15, "15", &models.Snssai{}, "internet")

	var psi [16]bool

	psi[11] = true

	handleNotificationResponse(t.Context(), amfInstance, ue, buildTestNotificationResponse(t, mustBytes(fgs.PSIBitmap{PSI: psi}.MarshalBinary())))

	if ue.Conn().NASGuardForTest().Active() {
		t.Fatal("expected timer T3565 to be stopped and cleared")
	}

	r := smf.ReleasedSmContext
	if len(r) != 4 {
		t.Fatalf("should have released 4 SM Context, released: %d", len(r))
	}

	if !slices.Contains(r, "1") || !slices.Contains(r, "5") || !slices.Contains(r, "8") || !slices.Contains(r, "15") {
		t.Fatalf("expected SM Context 1, 5, 8, and 15 to be released, released: %v", r)
	}

	for _, pduSessionID := range []uint8{1, 5, 8, 15} {
		if _, ok := ue.SmContextFindByPDUSessionID(pduSessionID); ok {
			t.Errorf("TS 24.501 §5.6.3.3: PDU session %d must be released on the AMF side too, not only in the SMF", pduSessionID)
		}
	}

	if _, ok := ue.SmContextFindByPDUSessionID(11); !ok {
		t.Error("PDU session 11 was reported active and must be kept")
	}
}

func buildTestNotificationResponse(t *testing.T, pduSessionStatus []byte) *fgs.NotificationResponse {
	t.Helper()

	m := &fgs.NotificationResponse{}

	if pduSessionStatus != nil {
		status, err := fgs.ParsePSIBitmap(pduSessionStatus)
		if err != nil {
			t.Fatalf("build PDU session status: %v", err)
		}

		m.PDUSessionStatus = &status
	}

	return m
}

func mustBytes(b []byte, err error) []byte {
	if err != nil {
		panic(err)
	}

	return b
}

func TestHandleNotificationResponse_SmfReleaseFails_EveryReportedSessionIsStillReleased(t *testing.T) {
	smf := fakeSmf{ReleaseSmContextError: errors.New("pfcp session deletion failed")}
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, nil, &smf)

	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)
	ue.Conn().NASGuardForTest().Arm(5*time.Minute, 5, func(expireTimes int32) {}, func() {})
	_ = ue.CreateSmContext(1, "1", &models.Snssai{}, "internet")
	_ = ue.CreateSmContext(5, "5", &models.Snssai{}, "internet")
	_ = ue.CreateSmContext(11, "11", &models.Snssai{}, "internet")

	var psi [16]bool

	psi[11] = true

	handleNotificationResponse(t.Context(), amfInstance, ue, buildTestNotificationResponse(t, mustBytes(fgs.PSIBitmap{PSI: psi}.MarshalBinary())))

	if len(smf.ReleaseSmContextCalls) != 2 {
		t.Fatalf("release calls = %v, want one per reported session: a failure must not abandon the rest", smf.ReleaseSmContextCalls)
	}

	for _, pduSessionID := range []uint8{1, 5} {
		if _, ok := ue.SmContextFindByPDUSessionID(pduSessionID); ok {
			t.Errorf("PDU session %d: the AMF-side local release is not conditional on the SMF request succeeding (TS 24.501 §5.6.3.3 a)1))", pduSessionID)
		}
	}

	if _, ok := ue.SmContextFindByPDUSessionID(11); !ok {
		t.Error("PDU session 11 was reported active and must be kept")
	}
}
