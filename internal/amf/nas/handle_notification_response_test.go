// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
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

	handleNotificationResponse(t.Context(), amfInstance, ue, buildTestNotificationResponse(nil))

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
	_ = ue.CreateSmContext(1, "1", &models.Snssai{})
	_ = ue.CreateSmContext(5, "5", &models.Snssai{})
	_ = ue.CreateSmContext(8, "8", &models.Snssai{})
	_ = ue.CreateSmContext(11, "11", &models.Snssai{})
	_ = ue.CreateSmContext(15, "15", &models.Snssai{})

	// Only PSI 11 is active, so the inactive sessions 1, 5, 8, 15 are released.
	var psi [16]bool

	psi[11] = true

	handleNotificationResponse(t.Context(), amfInstance, ue, buildTestNotificationResponse(fgs.PSIToBytes(psi)))

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
}

// buildTestNotificationResponse builds the plain NOTIFICATION RESPONSE wire bytes,
// optionally carrying a PDU session status IE (IEI 0x50, TS 24.501 §8.2.24).
func buildTestNotificationResponse(pduSessionStatus []byte) []byte {
	b := []byte{fgs.EPD5GMM, 0x00, uint8(fgs.MsgNotificationResponse)}

	if pduSessionStatus != nil {
		b = append(b, 0x50, uint8(len(pduSessionStatus)))
		b = append(b, pduSessionStatus...)
	}

	return b
}
