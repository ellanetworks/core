// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func TestServiceTypeToString(t *testing.T) {
	type Testcase struct {
		name    string
		svcType fgs.ServiceType
	}

	testcases := []Testcase{
		{"Signalling", fgs.ServiceTypeSignalling},
		{"Data", fgs.ServiceTypeData},
		{"Mobile terminated services", fgs.ServiceTypeMobileTerminatedServices},
		{"Emergency services", fgs.ServiceTypeEmergencyServices},
		{"Emergency services fallback", fgs.ServiceTypeEmergencyServicesFallback},
		{"High priority access", fgs.ServiceTypeHighPriorityAccess},
		{"unknown (200)", 200},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ret := tc.svcType.String()
			if ret != tc.name {
				t.Fatalf("expected: %s, got: %s", tc.name, ret)
			}
		})
	}
}

func TestHandleServiceRequest_WrongStateError(t *testing.T) {
	testcases := []amf.StateType{amf.RegistrationInitiated, amf.DeregistrationInitiated}
	for _, tc := range testcases {
		t.Run(string(tc), func(t *testing.T) {
			ue := amf.NewUeContext()
			ue.ForceStateForTest(tc)

			handleServiceRequest(t.Context(), amf.New(nil, nil, nil), ue, nil, true)

			if ue.State() != tc {
				t.Fatalf("expected out-of-state Service Request to leave state %s unchanged, got %s", tc, ue.State())
			}
		})
	}
}

func TestHandleServiceRequest_InvalidSecurityContext_ServiceReject(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				SupportedTACs: "[\"000001\"]",
			},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: []byte("testkey"),
		},
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)
	ue.SetSecuredForTest(false)

	m := buildTestServiceRequest()

	handleServiceRequest(t.Context(), amfInstance, ue, encSR(t, m), true)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgServiceReject))
}

func TestHandleServiceRequest_MacFailed_ServiceReject(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				SupportedTACs: "[\"000001\"]",
			},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: []byte("testkey"),
		},
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)
	ue.SetSecuredForTest(true)

	m := buildTestServiceRequest()

	handleServiceRequest(t.Context(), amfInstance, ue, encSR(t, m), false)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgServiceReject))

	if !ue.SecuredForTest() {
		t.Fatalf("security context must be kept unchanged on a mac-failed service request (TS 24.501)")
	}
}

func TestHandleServiceRequest_NASContainer_DecryptFailure_ServiceReject(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				SupportedTACs: "[\"000001\"]",
			},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: []byte("testkey"),
		},
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)
	ue.Tai = ue.Conn().Tai
	ue.SetSecuredForTest(true)
	{
		ng := ue.NgKsiForTest()
		ng.Ksi = 1
		ue.SetNgKsiForTest(ng)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := nas.CipheringAES

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount(), fgs.ServiceTypeSignalling)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	ue.SetCipheringAlgForTest(200)

	handleServiceRequest(t.Context(), amfInstance, ue, encSR(t, m), true)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgServiceReject))
}

func TestHandleServiceRequest_UnknownUE_NASMessage_ServiceReject(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				SupportedTACs: "[\"000001\"]",
			},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: []byte("testkey"),
		},
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ueConn := ue.Conn()
	ue = amf.NewUeContext()
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := nas.CipheringAES

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount(), fgs.ServiceTypeData)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	handleServiceRequest(t.Context(), amfInstance, ue, encSR(t, m), true)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgServiceReject))
}

func TestHandleServiceRequest_ServiceTypeSignaling_ServiceAccept(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				SupportedTACs: "[\"000001\"]",
			},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: []byte("testkey"),
		},
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)
	ue.SetSecuredForTest(true)

	ue.ArmPagingForTest(6*time.Minute, 5)

	m := buildTestServiceRequest()

	handleServiceRequest(t.Context(), amfInstance, ue, encSR(t, m), true)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	if len(resp.NASPDU) < 7 || fgs.SecurityHeaderType(resp.NASPDU[1]&0x0f) != fgs.SHTIntegrityProtectedCiphered {
		t.Fatalf("expected a ciphered NAS message")
	}

	decoded, err := amf.DecodeNASMessage(ue, resp.NASPDU)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message")
	}

	if decoded.MessageType != uint8(fgs.MsgServiceAccept) {
		t.Fatalf("expected a service accept message, got %d", decoded.MessageType)
	}

	if ue.PagingActiveForTest() {
		t.Fatalf("expected timer T3513 to be stopped and cleared")
	}
}

// TS 24.501 §5.6.1.5
func TestHandleServiceRequest_ServiceTypeReplies(t *testing.T) {
	tests := []struct {
		name        string
		serviceType uint8
		wantMsgType uint8
	}{
		{"high-priority access is accepted", uint8(fgs.ServiceTypeHighPriorityAccess), uint8(fgs.MsgServiceAccept)},
		{"elevated signalling is accepted", uint8(fgs.ServiceTypeElevatedSignalling), uint8(fgs.MsgServiceAccept)},
		{"emergency is rejected (unsupported)", uint8(fgs.ServiceTypeEmergencyServices), uint8(fgs.MsgServiceReject)},
		{"emergency fallback is rejected", uint8(fgs.ServiceTypeEmergencyServicesFallback), uint8(fgs.MsgServiceReject)},
		{"unknown service type is rejected", 0x07, uint8(fgs.MsgServiceReject)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			amfInstance := amf.New(
				&fakeDBInstance{Operator: &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: "[\"000001\"]"}},
				&fakeAusf{
					AvKgAka: &ausf.AuthResult{Rand: hex.EncodeToString(make([]byte, 16)), Autn: hex.EncodeToString(make([]byte, 16))},
					Supi:    mustSUPIFromPrefixed("imsi-001019756139935"),
					Kseaf:   []byte("testkey"),
				},
				nil,
			)

			ue, ngapSender, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not build UE and radio: %v", err)
			}

			ue.ForceStateForTest(amf.Registered)
			ue.SetSecuredForTest(true)

			m := buildTestServiceRequest()
			m.ServiceType = fgs.ServiceType(tc.serviceType)

			handleServiceRequest(t.Context(), amfInstance, ue, encSR(t, m), true)

			if len(ngapSender.SentDownlinkNASTransport) != 1 {
				t.Fatalf("service type %d: want exactly 1 downlink reply (never a silent drop), got %d", tc.serviceType, len(ngapSender.SentDownlinkNASTransport))
			}

			pdu := ngapSender.SentDownlinkNASTransport[0].NASPDU

			var gotType uint8
			if fgs.SecurityHeaderType(pdu[1]&0x0f) == fgs.SHTPlain {
				gotType = pdu[2]
			} else {
				decoded, err := amf.DecodeNASMessage(ue, pdu)
				if err != nil {
					t.Fatalf("could not decode ciphered downlink reply: %v", err)
				}

				gotType = decoded.MessageType
			}

			if gotType != tc.wantMsgType {
				t.Fatalf("reply message type = %d, want %d", gotType, tc.wantMsgType)
			}
		})
	}
}

func TestHandleServiceRequest_NASContainerServiceTypeSignaling_ServiceAccept(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				SupportedTACs: "[\"000001\"]",
			},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: []byte("testkey"),
		},
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.Conn().NASGuardForTest().Arm(6*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.ForceStateForTest(amf.Registered)
	ue.Tai = ue.Conn().Tai
	ue.SetSecuredForTest(true)
	{
		ng := ue.NgKsiForTest()
		ng.Ksi = 1
		ue.SetNgKsiForTest(ng)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := nas.CipheringAES

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount(), fgs.ServiceTypeSignalling)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	handleServiceRequest(t.Context(), amfInstance, ue, encSR(t, m), true)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	decipherGmm(t, ue, resp.NASPDU, uint8(fgs.MsgServiceAccept))

	if ue.Conn().NASGuardForTest().Active() {
		t.Fatalf("expected timer T3565 to be stopped and cleared")
	}
}

func TestHandleServiceRequest_NASContainerServiceTypeData_ServiceAccept(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				SupportedTACs: "[\"000001\"]",
			},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: []byte("testkey"),
		},
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.Conn().NASGuardForTest().Arm(6*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.ForceStateForTest(amf.Registered)
	ue.Tai = ue.Conn().Tai
	ue.SetSecuredForTest(true)
	{
		ng := ue.NgKsiForTest()
		ng.Ksi = 1
		ue.SetNgKsiForTest(ng)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := nas.CipheringAES

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount(), fgs.ServiceTypeData)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	handleServiceRequest(t.Context(), amfInstance, ue, encSR(t, m), true)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	decipherGmm(t, ue, resp.NASPDU, uint8(fgs.MsgServiceAccept))

	if ue.Conn().NASGuardForTest().Active() {
		t.Fatalf("expected timer T3565 to be stopped and cleared")
	}
}

type countingDBInstance struct {
	*fakeDBInstance
	operatorReads atomic.Int64
}

func (c *countingDBInstance) GetOperator(ctx context.Context) (*db.Operator, error) {
	c.operatorReads.Add(1)

	return c.fakeDBInstance.GetOperator(ctx)
}

func TestHandleServiceRequestMTReadsOperatorOnce(t *testing.T) {
	dbInstance := &countingDBInstance{fakeDBInstance: &fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}}

	amfInstance := amf.New(
		dbInstance,
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: []byte("testkey"),
		},
		&fakeSmf{},
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	oldguti := mustTestGuti("001", "01", "cafe42", 0x00000001)

	ue.ArmPagingForTest(6*time.Minute, 5)

	ue.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}
	ue.ForceStateForTest(amf.Registered)
	ue.SetGutiForTest(oldguti)
	ue.Tai = ue.Conn().Tai
	ue.SetSecuredForTest(true)
	{
		ng := ue.NgKsiForTest()
		ng.Ksi = 1
		ue.SetNgKsiForTest(ng)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := nas.CipheringAES

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount(), fgs.ServiceTypeMobileTerminatedServices)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	handleServiceRequest(t.Context(), amfInstance, ue, encSR(t, m), true)

	if len(ngapSender.SentDownlinkNASTransport) < 2 {
		t.Fatalf("expected a Service Accept and a Configuration Update Command, got %d downlinks", len(ngapSender.SentDownlinkNASTransport))
	}

	if ue.TmsiForTest() == oldguti.Tmsi {
		t.Fatal("expected new GUTI to be allocated")
	}

	if got := dbInstance.operatorReads.Load(); got != 1 {
		t.Fatalf("the mobile-terminated service request read the operator row %d times, want 1", got)
	}
}

func TestHandleServiceRequest_NASContainerServiceTypeMT_ServiceAccept(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				SupportedTACs: "[\"000001\"]",
			},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: []byte("testkey"),
		},
		&fakeSmf{},
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	oldguti := mustTestGuti("001", "01", "cafe42", 0x00000001)

	ue.ArmPagingForTest(6*time.Minute, 5)

	ue.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}
	ue.ForceStateForTest(amf.Registered)
	ue.SetGutiForTest(oldguti)
	ue.Tai = ue.Conn().Tai
	ue.SetSecuredForTest(true)
	{
		ng := ue.NgKsiForTest()
		ng.Ksi = 1
		ue.SetNgKsiForTest(ng)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := nas.CipheringAES

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount(), fgs.ServiceTypeMobileTerminatedServices)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	handleServiceRequest(t.Context(), amfInstance, ue, encSR(t, m), true)

	if len(ngapSender.SentDownlinkNASTransport) < 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	decipherGmm(t, ue, resp.NASPDU, uint8(fgs.MsgServiceAccept))

	if ue.PagingActiveForTest() {
		t.Fatalf("expected timer T3513 to be stopped and cleared")
	}

	if ue.TmsiForTest() == oldguti.Tmsi {
		t.Fatal("expected new GUTI to be allocated")
	}

	if ue.OldTmsi() != oldguti.Tmsi {
		t.Fatal("expected old GUTI to still be valid")
	}
}

func TestHandleServiceRequest_NASContainerServiceTypeMT_N1N2Message_NoPDUSession_Error(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				SupportedTACs: "[\"000001\"]",
			},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: []byte("testkey"),
		},
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ArmPagingForTest(6*time.Minute, 5)

	ue.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}
	ue.ForceStateForTest(amf.Registered)
	ue.SetGutiForTest(mustTestGuti("001", "01", "cafe42", 0x00000001))
	ue.Tai = ue.Conn().Tai
	ue.SetSecuredForTest(true)
	{
		ng := ue.NgKsiForTest()
		ng.Ksi = 1
		ue.SetNgKsiForTest(ng)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := nas.CipheringAES

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)
	ue.SetN1N2Message(&models.N1N2MessageTransferRequest{PduSessionID: 1})

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount(), fgs.ServiceTypeMobileTerminatedServices)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	handleServiceRequest(t.Context(), amfInstance, ue, encSR(t, m), true)

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("should not have sent a Downlink NAS Transport message")
	}
}

func TestHandleServiceRequest_NASContainerServiceTypeMT_N1N2Message_ExistingPDUSession_ServiceAccept(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				SupportedTACs: "[\"000001\"]",
			},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: []byte("testkey"),
		},
		&fakeSmf{},
	)
	amfInstance.NASGuardCfg = guard.TimerValue{Enable: true, ExpireTime: 5 * time.Minute, MaxRetryTimes: 5}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	oldguti := mustTestGuti("001", "01", "cafe42", 0x00000001)

	snssai := models.Snssai{Sst: 1, Sd: "102030"}

	ue.ArmPagingForTest(6*time.Minute, 5)

	ue.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}
	ue.ForceStateForTest(amf.Registered)
	ue.SetGutiForTest(oldguti)
	ue.Tai = ue.Conn().Tai
	ue.SetSecuredForTest(true)
	{
		ng := ue.NgKsiForTest()
		ng.Ksi = 1
		ue.SetNgKsiForTest(ng)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := nas.CipheringAES

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)
	ue.Ambr = &models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("100 Mbps")}
	_ = ue.CreateSmContext(1, "testref", &snssai, "internet")
	ue.SetN1N2Message(&models.N1N2MessageTransferRequest{PduSessionID: 1, SNssai: &snssai})

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount(), fgs.ServiceTypeMobileTerminatedServices)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	handleServiceRequest(t.Context(), amfInstance, ue, encSR(t, m), true)

	if len(ngapSender.SentPDUSessionResourceSetupRequest) < 1 {
		t.Fatalf("should have sent a PDU Session Resource Setup Request message")
	}

	pduResp := ngapSender.SentPDUSessionResourceSetupRequest[0]
	decipherGmm(t, ue, *pduResp.NASPDU, uint8(fgs.MsgServiceAccept))

	if len(ngapSender.SentDownlinkNASTransport) < 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	decipherGmmCount(t, ue, resp.NASPDU, ue.ULCount()+1, uint8(fgs.MsgConfigurationUpdateCommand))

	if ue.PagingActiveForTest() {
		t.Fatalf("expected timer T3513 to be stopped and cleared")
	}

	if !ue.Conn().NASGuardForTest().Active() {
		t.Fatalf("expected timer T3555 to be started")
	}

	if ue.TmsiForTest() == oldguti.Tmsi {
		t.Fatal("expected new GUTI to be allocated")
	}

	if ue.OldTmsi() != oldguti.Tmsi {
		t.Fatal("expected old GUTI to still be valid")
	}
}

func TestHandleServiceRequest_NASContainerServiceTypeMT_N1N2MessageN2_ExistingPDUSession_ServiceAccept_UplinkPDUError(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				SupportedTACs: "[\"000001\"]",
			},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: []byte("testkey"),
		},
		&fakeSmf{Error: fmt.Errorf("error activating PDU session")},
	)
	amfInstance.NASGuardCfg = guard.TimerValue{Enable: true, ExpireTime: 5 * time.Minute, MaxRetryTimes: 5}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	oldguti := mustTestGuti("001", "01", "cafe42", 0x00000001)
	snssai := models.Snssai{Sst: 1, Sd: "102030"}

	ue.ArmPagingForTest(6*time.Minute, 5)

	ue.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}
	ue.ForceStateForTest(amf.Registered)
	ue.SetGutiForTest(oldguti)
	ue.Tai = ue.Conn().Tai
	ue.SetSecuredForTest(true)
	{
		ng := ue.NgKsiForTest()
		ng.Ksi = 1
		ue.SetNgKsiForTest(ng)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := nas.CipheringSNOW3G

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)
	ue.Ambr = &models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("100 Mbps")}
	_ = ue.CreateSmContext(1, "testref", &snssai, "internet")
	_ = ue.CreateSmContext(12, "testrefuplink", &snssai, "internet")
	ue.SetN1N2Message(&models.N1N2MessageTransferRequest{PduSessionID: 1, SNssai: &snssai, BinaryDataN2Information: []byte{}})

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount(), fgs.ServiceTypeMobileTerminatedServices)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	handleServiceRequest(t.Context(), amfInstance, ue, encSR(t, m), true)

	if len(ngapSender.SentPDUSessionResourceSetupRequest) < 1 {
		t.Fatalf("should have sent a PDU Session Resource Setup Request message")
	}

	pduResp := ngapSender.SentPDUSessionResourceSetupRequest[0]
	plain := decipherGmm(t, ue, *pduResp.NASPDU, uint8(fgs.MsgServiceAccept))

	accept, err := fgs.ParseServiceAccept(plain)
	if err != nil {
		t.Fatalf("could not parse service accept: %v", err)
	}

	if !psiSet(accept.PDUSessionReactivationResult, 12) {
		t.Fatalf("should have failed to reactivate PDU Session ID 12")
	}

	if len(ngapSender.SentDownlinkNASTransport) < 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	decipherGmmCount(t, ue, resp.NASPDU, ue.ULCount()+1, uint8(fgs.MsgConfigurationUpdateCommand))

	if ue.PagingActiveForTest() {
		t.Fatalf("expected timer T3513 to be stopped and cleared")
	}

	if !ue.Conn().NASGuardForTest().Active() {
		t.Fatalf("expected timer T3555 to be started")
	}

	if ue.TmsiForTest() == oldguti.Tmsi {
		t.Fatal("expected new GUTI to be allocated")
	}

	if ue.OldTmsi() != oldguti.Tmsi {
		t.Fatal("expected old GUTI to still be valid")
	}
}

func TestHandleServiceRequest_NASContainerServiceTypeMT_N1N2MessageN2_ExistingPDUSession_ServiceAccept_UplinkPDUSuccess(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				SupportedTACs: "[\"000001\"]",
			},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: []byte("testkey"),
		},
		&fakeSmf{Error: nil},
	)
	amfInstance.NASGuardCfg = guard.TimerValue{Enable: true, ExpireTime: 5 * time.Minute, MaxRetryTimes: 5}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	oldguti := mustTestGuti("001", "01", "cafe42", 0x00000001)
	snssai := models.Snssai{Sst: 1, Sd: "102030"}

	ue.ArmPagingForTest(6*time.Minute, 5)

	ue.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}
	ue.ForceStateForTest(amf.Registered)
	ue.SetGutiForTest(oldguti)
	ue.Tai = ue.Conn().Tai
	ue.SetSecuredForTest(true)
	{
		ng := ue.NgKsiForTest()
		ng.Ksi = 1
		ue.SetNgKsiForTest(ng)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := nas.CipheringAES

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)
	ue.Ambr = &models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("100 Mbps")}
	_ = ue.CreateSmContext(1, "testref", &snssai, "internet")
	_ = ue.CreateSmContext(12, "testrefuplink", &snssai, "internet")
	ue.SetN1N2Message(&models.N1N2MessageTransferRequest{PduSessionID: 1, SNssai: &snssai, BinaryDataN2Information: []byte{}})

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount(), fgs.ServiceTypeMobileTerminatedServices)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	handleServiceRequest(t.Context(), amfInstance, ue, encSR(t, m), true)

	if len(ngapSender.SentPDUSessionResourceSetupRequest) < 1 {
		t.Fatalf("should have sent a PDU Session Resource Setup Request message")
	}

	pduResp := ngapSender.SentPDUSessionResourceSetupRequest[0]
	plain := decipherGmm(t, ue, *pduResp.NASPDU, uint8(fgs.MsgServiceAccept))

	accept, err := fgs.ParseServiceAccept(plain)
	if err != nil {
		t.Fatalf("could not parse service accept: %v", err)
	}

	if psiSet(accept.PDUSessionReactivationResult, 12) {
		t.Fatalf("should not have failed to reactivate PDU Session ID 12")
	}

	if !psiSet(accept.PDUSessionStatus, 1) {
		t.Fatalf("should have indicated PDU Session ID 1 is active in network")
	}

	if psiSet(accept.PDUSessionStatus, 13) {
		t.Fatalf("should have indicated PDU Session ID 13 is inactive in network")
	}

	for _, pduSessionID := range []uint8{1, 12} {
		if _, ok := ue.SmContextFindByPDUSessionID(pduSessionID); !ok {
			t.Errorf("PDU session %d was reported active and must be kept", pduSessionID)
		}
	}

	if len(ngapSender.SentDownlinkNASTransport) < 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	decipherGmmCount(t, ue, resp.NASPDU, ue.ULCount()+1, uint8(fgs.MsgConfigurationUpdateCommand))

	if ue.PagingActiveForTest() {
		t.Fatalf("expected timer T3513 to be stopped and cleared")
	}

	if !ue.Conn().NASGuardForTest().Active() {
		t.Fatalf("expected timer T3555 to be started")
	}

	if ue.TmsiForTest() == oldguti.Tmsi {
		t.Fatal("expected new GUTI to be allocated")
	}

	if ue.OldTmsi() != oldguti.Tmsi {
		t.Fatal("expected old GUTI to still be valid")
	}
}

func TestHandleServiceRequest_NASContainerServiceTypeMT_N1N2MessageN2_UeCtxReq_ExistingPDUSession_ServiceAccept_UplinkPDUSuccess(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				SupportedTACs: "[\"000001\"]",
			},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: []byte("testkey"),
		},
		&fakeSmf{Error: nil},
	)
	amfInstance.NASGuardCfg = guard.TimerValue{Enable: true, ExpireTime: 5 * time.Minute, MaxRetryTimes: 5}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.AllowedNssai = []models.Snssai{{Sst: 1, Sd: "010203"}}
	setTestUESecurityCapability(ue)

	oldguti := mustTestGuti("001", "01", "cafe42", 0x00000001)
	snssai := models.Snssai{Sst: 1, Sd: "102030"}

	ue.ArmPagingForTest(6*time.Minute, 5)

	ue.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}
	ue.ForceStateForTest(amf.Registered)
	ue.SetGutiForTest(oldguti)
	ue.Tai = ue.Conn().Tai
	ue.SetSecuredForTest(true)
	{
		ng := ue.NgKsiForTest()
		ng.Ksi = 1
		ue.SetNgKsiForTest(ng)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := nas.CipheringAES

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)
	ue.Ambr = &models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("100 Mbps")}
	_ = ue.CreateSmContext(1, "testref", &snssai, "internet")
	_ = ue.CreateSmContext(12, "testrefuplink", &snssai, "internet")
	ue.SetN1N2Message(&models.N1N2MessageTransferRequest{PduSessionID: 1, SNssai: &snssai, BinaryDataN2Information: []byte{}})
	ue.Conn().UeContextRequest = true

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount(), fgs.ServiceTypeMobileTerminatedServices)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	handleServiceRequest(t.Context(), amfInstance, ue, encSR(t, m), true)

	if len(ngapSender.SentInitialContextSetupRequest) < 1 {
		t.Fatalf("should have sent a PDU Session Resource Setup Request message")
	}

	pduResp := ngapSender.SentInitialContextSetupRequest[0]
	decipherGmm(t, ue, *pduResp.NASPDU, uint8(fgs.MsgServiceAccept))

	if len(ngapSender.SentDownlinkNASTransport) < 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	decipherGmmCount(t, ue, resp.NASPDU, ue.ULCount()+1, uint8(fgs.MsgConfigurationUpdateCommand))

	if ue.PagingActiveForTest() {
		t.Fatalf("expected timer T3513 to be stopped and cleared")
	}

	if !ue.Conn().NASGuardForTest().Active() {
		t.Fatalf("expected timer T3555 to be started")
	}

	if ue.TmsiForTest() == oldguti.Tmsi {
		t.Fatal("expected new GUTI to be allocated")
	}

	if ue.OldTmsi() != oldguti.Tmsi {
		t.Fatal("expected old GUTI to still be valid")
	}
}

func TestHandleServiceRequest_OutOfRangePduSessionID(t *testing.T) {
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				SupportedTACs: "[\"000001\"]",
			},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: []byte("testkey"),
		},
		&fakeSmf{},
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	snssai := models.Snssai{Sst: 1, Sd: "102030"}

	ue.ForceStateForTest(amf.Registered)
	ue.Tai = ue.Conn().Tai
	ue.SetSecuredForTest(true)
	{
		ng := ue.NgKsiForTest()
		ng.Ksi = 1
		ue.SetNgKsiForTest(ng)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := nas.CipheringAES

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)

	ue.SmContextList[255] = &amf.SmContext{Ref: "malicious-ref", Snssai: &snssai}

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount(), fgs.ServiceTypeData)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	handleServiceRequest(t.Context(), amfInstance, ue, encSR(t, m), true)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected the out-of-range session to be skipped and a Service Accept sent, got %d downlinks", len(ngapSender.SentDownlinkNASTransport))
	}

	decipherGmm(t, ue, ngapSender.SentDownlinkNASTransport[0].NASPDU, uint8(fgs.MsgServiceAccept))
}

func buildTestServiceRequest() *fgs.ServiceRequest {
	return &fgs.ServiceRequest{
		ServiceType:    fgs.ServiceTypeSignalling,
		MobileIdentity: serviceRequest5GSTMSI(),
	}
}

func encSR(t *testing.T, sr *fgs.ServiceRequest) []byte {
	t.Helper()

	b, err := sr.MarshalBinary()
	if err != nil {
		t.Fatalf("could not encode Service Request: %v", err)
	}

	return b
}

func buildTestServiceRequestCiphered(cipherAlg nas.CipheringAlgorithm, key [16]uint8, ulcount uint32, svcType fgs.ServiceType) (*fgs.ServiceRequest, error) {
	inner := &fgs.ServiceRequest{
		ServiceType:      svcType,
		NgKSI:            nas.KeySetIdentifier{Value: 1},
		MobileIdentity:   serviceRequest5GSTMSI(),
		UplinkDataStatus: mustBitmap([]byte{0x00, 0x10}),
		PDUSessionStatus: mustBitmap([]byte{0x02, 0x30}),
	}

	innerBytes, err := inner.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("could not encode service request: %v", err)
	}

	ciph, err := nas.CipherFor(cipherAlg)
	if err != nil {
		return nil, err
	}

	container, err := ciph.Apply(key, ulcount, nas.Bearer3GPP, nas.DirectionUplink, innerBytes)
	if err != nil {
		return nil, fmt.Errorf("could not encrypt NAS message: %v", err)
	}

	return &fgs.ServiceRequest{
		ServiceType:         fgs.ServiceTypeSignalling,
		NgKSI:               nas.KeySetIdentifier{Value: 1},
		MobileIdentity:      serviceRequest5GSTMSI(),
		UplinkDataStatus:    mustBitmap([]byte{0x00, 0x10}),
		PDUSessionStatus:    mustBitmap([]byte{0x02, 0x20}),
		NASMessageContainer: container,
	}, nil
}

func serviceRequest5GSTMSI() fgs.MobileIdentity {
	return fgs.STMSIIdentity(fgs.STMSI{TMSI: [4]byte{0xDE, 0xAD, 0xBE, 0xEF}})
}
