package gmm

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
)

func TestHandleNotificationResponse_NotRegisteredError(t *testing.T) {
	testcases := []context.StateType{context.Authentication, context.Deregistered, context.ContextSetup, context.SecurityMode}

	for _, tc := range testcases {
		t.Run(string(tc), func(t *testing.T) {
			ue := context.NewAmfUe()
			ue.State = tc

			expected := fmt.Sprintf("state mismatch: receive Notification Response message in state %s", tc)

			err := handleNotificationResponse(t.Context(), nil, ue, nil)
			if err == nil || err.Error() != expected {
				t.Fatalf("expected error: %s, got %v", expected, err)
			}
		})
	}
}

func TestHandleNotificationResponse_MacFailed(t *testing.T) {
	ue := context.NewAmfUe()
	ue.State = context.Registered
	ue.MacFailed = true

	expected := "NAS message integrity check failed"

	err := handleNotificationResponse(t.Context(), nil, ue, nil)
	if err == nil || err.Error() != expected {
		t.Fatalf("expected error: %s, got %v", expected, err)
	}
}

func TestHandleNotificationResponse_T3565Stopped_NoPDUSessionStatus_NoSmContextReleased(t *testing.T) {
	smf := FakeSmf{Error: nil, ReleasedSmContext: make([]string, 0)}
	amf := &context.AMF{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		UEs: make(map[string]*context.AmfUe),
		Smf: &smf,
	}

	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test UE and radio: %v", err)
	}

	ue.State = context.Registered
	ue.T3565 = context.NewTimer(5*time.Minute, 5, func(expireTimes int32) {}, func() {})

	m := buildTestNotifationResponse()

	err = handleNotificationResponse(t.Context(), amf, ue, m.NotificationResponse)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if ue.T3565 != nil {
		t.Fatal("expected timer T3565 to be stopped and cleared")
	}

	if len(smf.ReleasedSmContext) != 0 {
		t.Fatalf("should not have released any SM Context")
	}
}

func TestHandleNotificationResponse_T3565Stopped_PDUSessionStatus_SmContextReleased(t *testing.T) {
	smf := FakeSmf{Error: nil, ReleasedSmContext: make([]string, 0)}
	amf := &context.AMF{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		UEs: make(map[string]*context.AmfUe),
		Smf: &smf,
	}

	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test UE and radio: %v", err)
	}

	ue.State = context.Registered
	ue.T3565 = context.NewTimer(5*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.CreateSmContext(1, "1", &models.Snssai{})
	ue.CreateSmContext(5, "5", &models.Snssai{})
	ue.CreateSmContext(8, "8", &models.Snssai{})
	ue.CreateSmContext(11, "11", &models.Snssai{})
	ue.CreateSmContext(15, "15", &models.Snssai{})

	m := buildTestNotifationResponse()

	m.NotificationResponse.PDUSessionStatus = &nasType.PDUSessionStatus{}
	m.NotificationResponse.SetIei(nasMessage.NotificationResponsePDUSessionStatusType)
	m.NotificationResponse.SetLen(2)
	m.NotificationResponse.SetPSI1(0)
	m.NotificationResponse.SetPSI5(0)
	m.NotificationResponse.SetPSI8(0)
	m.NotificationResponse.SetPSI11(1)
	m.NotificationResponse.SetPSI15(0)

	err = handleNotificationResponse(t.Context(), amf, ue, m.NotificationResponse)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if ue.T3565 != nil {
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

func buildTestNotifationResponse() *nas.GmmMessage {
	m := nas.NewGmmMessage()

	notificationResponse := nasMessage.NewNotificationResponse(0)
	notificationResponse.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	notificationResponse.SetSpareHalfOctet(0x00)
	notificationResponse.SetMessageType(nas.MsgTypeNotificationResponse)

	m.NotificationResponse = notificationResponse
	m.SetMessageType(nas.MsgTypeNotificationResponse)

	return m
}
