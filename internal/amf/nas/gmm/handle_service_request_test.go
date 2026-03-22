package gmm

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"slices"
	"testing"
	"time"

	amfContext "github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/nas/security"
)

func TestServiceTypeToString(t *testing.T) {
	type Testcase struct {
		name    string
		svcType uint8
	}

	testcases := []Testcase{
		{"Signalling", nasMessage.ServiceTypeSignalling},
		{"Data", nasMessage.ServiceTypeData},
		{"Mobile Terminated Services", nasMessage.ServiceTypeMobileTerminatedServices},
		{"Emergency Services", nasMessage.ServiceTypeEmergencyServices},
		{"Emergency Services Fallback", nasMessage.ServiceTypeEmergencyServicesFallback},
		{"High Priority Access", nasMessage.ServiceTypeHighPriorityAccess},
		{"Unknown", 200},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ret := serviceTypeToString(tc.svcType)
			if ret != tc.name {
				t.Fatalf("expected: %s, got: %s", tc.name, ret)
			}
		})
	}
}

func TestHandleServiceRequest_WrongStateError(t *testing.T) {
	testcases := []amfContext.StateType{amfContext.SecurityMode, amfContext.Authentication, amfContext.ContextSetup}
	for _, tc := range testcases {
		t.Run(string(tc), func(t *testing.T) {
			expected := fmt.Sprintf("state mismatch: receive Service Request message in state %s", tc)

			err := handleServiceRequest(t.Context(), amfContext.New(nil, nil, nil), &amfContext.AmfUe{State: tc}, nil)
			if err == nil || err.Error() != expected {
				t.Fatalf("expected error: %s, got: %v", expected, err)
			}
		})
	}
}

func TestHandleServiceRequest_InvalidSecurityContext_ServiceReject(t *testing.T) {
	amf := amfContext.New(
		&FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		&FakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: "testkey",
		},
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.State = amfContext.Registered
	ue.SecurityContextAvailable = false

	m := buildTestServiceRequest()

	err = handleServiceRequest(t.Context(), amf, ue, m.ServiceRequest)
	if err != nil {
		t.Fatalf("expected no errors, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected a plain NAS message")
	}

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode plain NAS message")
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeServiceReject {
		t.Fatalf("expected a service reject essage, got '%v'", nm.GmmHeader.GetMessageType())
	}
}

func TestHandleServiceRequest_MacFailed_ServiceReject(t *testing.T) {
	amf := amfContext.New(
		&FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		&FakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: "testkey",
		},
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.State = amfContext.Registered
	ue.SecurityContextAvailable = true
	ue.MacFailed = true

	m := buildTestServiceRequest()

	err = handleServiceRequest(t.Context(), amf, ue, m.ServiceRequest)
	if err != nil {
		t.Fatalf("expected no errors, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected a plain NAS message")
	}

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode plain NAS message")
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeServiceReject {
		t.Fatalf("expected a service reject essage, got '%v'", nm.GmmHeader.GetMessageType())
	}

	if ue.SecurityContextAvailable {
		t.Fatalf("expected security context to change to not available")
	}
}

func TestHandleServiceRequest_NASContainer_DecryptFailure_ServiceReject(t *testing.T) {
	amf := amfContext.New(
		&FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		&FakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: "testkey",
		},
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.State = amfContext.Registered
	ue.Tai = ue.RanUe.Tai
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2
	ue.KnasEnc = key
	ue.KnasInt = key
	ue.CipheringAlg = algo
	ue.IntegrityAlg = security.AlgIntegrity128NIA0

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount.Get(), nasMessage.ServiceTypeSignalling)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	ue.CipheringAlg = 200

	err = handleServiceRequest(t.Context(), amf, ue, m.ServiceRequest)
	if err != nil {
		t.Fatalf("expected no errors, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected a plain NAS message")
	}

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode plain NAS message")
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeServiceReject {
		t.Fatalf("expected a service reject essage, got '%v'", nm.GmmHeader.GetMessageType())
	}
}

func TestHandleServiceRequest_UnknownUE_NASMessage_ServiceReject(t *testing.T) {
	amf := amfContext.New(
		&FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		&FakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: "testkey",
		},
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ranUe := ue.RanUe
	ue = amfContext.NewAmfUe()
	ue.AttachRanUe(ranUe)

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount.Get(), nasMessage.ServiceTypeData)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	err = handleServiceRequest(t.Context(), amf, ue, m.ServiceRequest)
	if err != nil {
		t.Fatalf("expected no errors, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected a plain NAS message")
	}

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode plain NAS message")
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeServiceReject {
		t.Fatalf("expected a service reject essage, got '%v'", nm.GmmHeader.GetMessageType())
	}
}

func TestHandleServiceRequest_ServiceTypeSignaling_ServiceAccept(t *testing.T) {
	amf := amfContext.New(
		&FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		&FakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: "testkey",
		},
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.State = amfContext.Registered
	ue.SecurityContextAvailable = true
	ue.MacFailed = false
	ue.T3513 = amfContext.NewTimer(6*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.SetOnGoing(amfContext.OnGoingProcedurePaging)

	m := buildTestServiceRequest()

	err = handleServiceRequest(t.Context(), amf, ue, m.ServiceRequest)
	if err != nil {
		t.Fatalf("expected no errors, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Fatalf("expected a ciphered NAS message")
	}

	decodedMessage, err := ue.DecodeNASMessage(resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message")
	}

	if decodedMessage.GmmHeader.GetMessageType() != nas.MsgTypeServiceAccept {
		t.Fatalf("expected a service accept message, got '%v'", decodedMessage.GmmHeader.GetMessageType())
	}

	if ue.T3513 != nil {
		t.Fatalf("expected timer T3513 to be stopped and cleared")
	}

	if ue.GetOnGoing() != amfContext.OnGoingProcedureNothing {
		t.Fatalf("expected paging procedure to be completed")
	}
}

func TestHandleServiceRequest_NASContainerServiceTypeSignaling_ServiceAccept(t *testing.T) {
	amf := amfContext.New(
		&FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		&FakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: "testkey",
		},
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.T3565 = amfContext.NewTimer(6*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.State = amfContext.Registered
	ue.Tai = ue.RanUe.Tai
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2
	ue.KnasEnc = key
	ue.KnasInt = key
	ue.CipheringAlg = algo
	ue.IntegrityAlg = security.AlgIntegrity128NIA0

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount.Get(), nasMessage.ServiceTypeSignalling)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	err = handleServiceRequest(t.Context(), amf, ue, m.ServiceRequest)
	if err != nil {
		t.Fatalf("expected no errors, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	payload := make([]byte, len(resp.NasPdu))
	copy(payload, resp.NasPdu)
	payload = payload[7:]

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Fatalf("expected a ciphered NAS message")
	}

	if err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get(), security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
		t.Fatalf("could not decrypt NAS message: %v", err)
	}

	err = nm.PlainNasDecode(&payload)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeServiceAccept {
		t.Fatalf("expected a service accept message, got '%v'", nm.GmmHeader.GetMessageType())
	}

	if ue.T3565 != nil {
		t.Fatalf("expected timer T3565 to be stopped and cleared")
	}
}

func TestHandleServiceRequest_NASContainerServiceTypeData_ServiceAccept(t *testing.T) {
	amf := amfContext.New(
		&FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		&FakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: "testkey",
		},
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.T3565 = amfContext.NewTimer(6*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.State = amfContext.Registered
	ue.Tai = ue.RanUe.Tai
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2
	ue.KnasEnc = key
	ue.KnasInt = key
	ue.CipheringAlg = algo
	ue.IntegrityAlg = security.AlgIntegrity128NIA0

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount.Get(), nasMessage.ServiceTypeData)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	err = handleServiceRequest(t.Context(), amf, ue, m.ServiceRequest)
	if err != nil {
		t.Fatalf("expected no errors, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	payload := make([]byte, len(resp.NasPdu))
	copy(payload, resp.NasPdu)
	payload = payload[7:]

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Fatalf("expected a ciphered NAS message")
	}

	if err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get(), security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
		t.Fatalf("could not decrypt NAS message: %v", err)
	}

	err = nm.PlainNasDecode(&payload)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeServiceAccept {
		t.Fatalf("expected a service accept message, got '%v'", nm.GmmHeader.GetMessageType())
	}

	if ue.T3565 != nil {
		t.Fatalf("expected timer T3565 to be stopped and cleared")
	}
}

func TestHandleServiceRequest_NASContainerServiceTypeMT_ServiceAccept(t *testing.T) {
	amf := amfContext.New(
		&FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		&FakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: "testkey",
		},
		&FakeSmf{},
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	oldguti := mustTestGuti("001", "01", "cafe42", 0x00000001)

	ue.T3513 = amfContext.NewTimer(6*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.SetOnGoing(amfContext.OnGoingProcedurePaging)
	ue.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}
	ue.State = amfContext.Registered
	ue.Guti = oldguti
	ue.Tai = ue.RanUe.Tai
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2
	ue.KnasEnc = key
	ue.KnasInt = key
	ue.CipheringAlg = algo
	ue.IntegrityAlg = security.AlgIntegrity128NIA0

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount.Get(), nasMessage.ServiceTypeMobileTerminatedServices)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	err = handleServiceRequest(t.Context(), amf, ue, m.ServiceRequest)
	if err != nil {
		t.Fatalf("expected no errors, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) < 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	payload := make([]byte, len(resp.NasPdu))
	copy(payload, resp.NasPdu)
	payload = payload[7:]

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Fatalf("expected a ciphered NAS message")
	}

	if err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get(), security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
		t.Fatalf("could not decrypt NAS message: %v", err)
	}

	err = nm.PlainNasDecode(&payload)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeServiceAccept {
		t.Fatalf("expected a service accept message, got '%v'", nm.GmmHeader.GetMessageType())
	}

	if ue.T3513 != nil {
		t.Fatalf("expected timer T3513 to be stopped and cleared")
	}

	if ue.Guti == oldguti {
		t.Fatal("expected new GUTI to be allocated")
	}

	if ue.OldGuti != oldguti {
		t.Fatal("expected old GUTI to still be valid")
	}
}

func TestHandleServiceRequest_NASContainerServiceTypeMT_N1N2Message_NoPDUSession_Error(t *testing.T) {
	amf := amfContext.New(
		&FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		&FakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: "testkey",
		},
		nil,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.T3513 = amfContext.NewTimer(6*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.SetOnGoing(amfContext.OnGoingProcedurePaging)
	ue.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}
	ue.State = amfContext.Registered
	ue.Guti = mustTestGuti("001", "01", "cafe42", 0x00000001)
	ue.Tai = ue.RanUe.Tai
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2
	ue.KnasEnc = key
	ue.KnasInt = key
	ue.CipheringAlg = algo
	ue.IntegrityAlg = security.AlgIntegrity128NIA0
	ue.N1N2Message = &models.N1N2MessageTransferRequest{PduSessionID: 1}

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount.Get(), nasMessage.ServiceTypeMobileTerminatedServices)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	expected := "service Request triggered by Network for pduSessionID that does not exist"

	err = handleServiceRequest(t.Context(), amf, ue, m.ServiceRequest)
	if err == nil || err.Error() != expected {
		t.Fatalf("expected error: %s, got: %v", expected, err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("should not have sent a Downlink NAS Transport message")
	}
}

func TestHandleServiceRequest_NASContainerServiceTypeMT_N1N2Message_ExistingPDUSession_ServiceAccept(t *testing.T) {
	amf := amfContext.New(
		&FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		&FakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: "testkey",
		},
		&FakeSmf{},
	)
	amf.T3555Cfg = amfContext.TimerValue{Enable: true, ExpireTime: 5 * time.Minute, MaxRetryTimes: 5}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	oldguti := mustTestGuti("001", "01", "cafe42", 0x00000001)

	snssai := models.Snssai{Sst: 1, Sd: "102030"}
	ue.T3513 = amfContext.NewTimer(6*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.SetOnGoing(amfContext.OnGoingProcedurePaging)
	ue.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}
	ue.State = amfContext.Registered
	ue.Guti = oldguti
	ue.Tai = ue.RanUe.Tai
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2
	ue.KnasEnc = key
	ue.KnasInt = key
	ue.CipheringAlg = algo
	ue.IntegrityAlg = security.AlgIntegrity128NIA0
	ue.Ambr = &models.Ambr{Uplink: "100mbps", Downlink: "100mbps"}
	_ = ue.CreateSmContext(1, "testref", &snssai)
	ue.N1N2Message = &models.N1N2MessageTransferRequest{PduSessionID: 1, SNssai: &snssai}

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount.Get(), nasMessage.ServiceTypeMobileTerminatedServices)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	err = handleServiceRequest(t.Context(), amf, ue, m.ServiceRequest)
	if err != nil {
		t.Fatalf("expected no errors, got: %v", err)
	}

	if len(ngapSender.SentPDUSessionResourceSetupRequest) < 1 {
		t.Fatalf("should have sent a PDU Session Resource Setup Request message")
	}

	pduResp := ngapSender.SentPDUSessionResourceSetupRequest[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(pduResp.NasPdu) & 0x0f

	payload := make([]byte, len(pduResp.NasPdu))
	copy(payload, pduResp.NasPdu)
	payload = payload[7:]

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Fatalf("expected a ciphered NAS message")
	}

	if err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get(), security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
		t.Fatalf("could not decrypt NAS message: %v", err)
	}

	err = nm.PlainNasDecode(&payload)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message: %v, %v", err, payload)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeServiceAccept {
		t.Fatalf("expected a service accept message, got '%v'", nm.GmmHeader.GetMessageType())
	}

	if len(ngapSender.SentDownlinkNASTransport) < 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm = new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	payload = make([]byte, len(resp.NasPdu))
	copy(payload, resp.NasPdu)
	payload = payload[7:]

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Fatalf("expected a ciphered NAS message")
	}

	if err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get()+1, security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
		t.Fatalf("could not decrypt NAS message: %v", err)
	}

	err = nm.PlainNasDecode(&payload)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message: %v, %v", err, payload)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeConfigurationUpdateCommand {
		t.Fatalf("expected a configuration update command message, got '%v'", nm.GmmHeader.GetMessageType())
	}

	if ue.T3513 != nil {
		t.Fatalf("expected timer T3513 to be stopped and cleared")
	}

	if ue.T3555 == nil {
		t.Fatalf("expected timer T3555 to be started")
	}

	if ue.Guti == oldguti {
		t.Fatal("expected new GUTI to be allocated")
	}

	if ue.OldGuti != oldguti {
		t.Fatal("expected old GUTI to still be valid")
	}
}

func TestHandleServiceRequest_NASContainerServiceTypeMT_N1N2MessageN2_ExistingPDUSession_ServiceAccept_UplinkPDUError(t *testing.T) {
	amf := amfContext.New(
		&FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		&FakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: "testkey",
		},
		&FakeSmf{Error: fmt.Errorf("error activating PDU session")},
	)
	amf.T3555Cfg = amfContext.TimerValue{Enable: true, ExpireTime: 5 * time.Minute, MaxRetryTimes: 5}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	oldguti := mustTestGuti("001", "01", "cafe42", 0x00000001)
	snssai := models.Snssai{Sst: 1, Sd: "102030"}
	ue.T3513 = amfContext.NewTimer(6*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.SetOnGoing(amfContext.OnGoingProcedurePaging)
	ue.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}
	ue.State = amfContext.Registered
	ue.Guti = oldguti
	ue.Tai = ue.RanUe.Tai
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA1
	ue.KnasEnc = key
	ue.KnasInt = key
	ue.CipheringAlg = algo
	ue.IntegrityAlg = security.AlgIntegrity128NIA0
	ue.Ambr = &models.Ambr{Uplink: "100mbps", Downlink: "100mbps"}
	_ = ue.CreateSmContext(1, "testref", &snssai)
	_ = ue.CreateSmContext(12, "testrefuplink", &snssai)
	ue.N1N2Message = &models.N1N2MessageTransferRequest{PduSessionID: 1, SNssai: &snssai, BinaryDataN2Information: []byte{}}

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount.Get(), nasMessage.ServiceTypeMobileTerminatedServices)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	err = handleServiceRequest(t.Context(), amf, ue, m.ServiceRequest)
	if err != nil {
		t.Fatalf("expected no errors, got: %v", err)
	}

	if len(ngapSender.SentPDUSessionResourceSetupRequest) < 1 {
		t.Fatalf("should have sent a PDU Session Resource Setup Request message")
	}

	pduResp := ngapSender.SentPDUSessionResourceSetupRequest[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(pduResp.NasPdu) & 0x0f

	payload := make([]byte, len(pduResp.NasPdu))
	copy(payload, pduResp.NasPdu)
	payload = payload[7:]

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Fatalf("expected a ciphered NAS message")
	}

	if err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get(), security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
		t.Fatalf("could not decrypt NAS message: %v", err)
	}

	err = nm.PlainNasDecode(&payload)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message: %v, %v", err, payload)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeServiceAccept {
		t.Fatalf("expected a service accept message, got '%v'", nm.GmmHeader.GetMessageType())
	}

	if nm.ServiceAccept.PDUSessionReactivationResult.GetPSI12() != 1 {
		t.Fatalf("should have failed to reactivate PDU Session ID 12")
	}

	if len(ngapSender.SentDownlinkNASTransport) < 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm = new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	payload = make([]byte, len(resp.NasPdu))
	copy(payload, resp.NasPdu)
	payload = payload[7:]

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Fatalf("expected a ciphered NAS message")
	}

	if err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get()+1, security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
		t.Fatalf("could not decrypt NAS message: %v", err)
	}

	err = nm.PlainNasDecode(&payload)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message: %v, %v", err, payload)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeConfigurationUpdateCommand {
		t.Fatalf("expected a configuration update command message, got '%v'", nm.GmmHeader.GetMessageType())
	}

	if ue.T3513 != nil {
		t.Fatalf("expected timer T3513 to be stopped and cleared")
	}

	if ue.T3555 == nil {
		t.Fatalf("expected timer T3555 to be started")
	}

	if ue.Guti == oldguti {
		t.Fatal("expected new GUTI to be allocated")
	}

	if ue.OldGuti != oldguti {
		t.Fatal("expected old GUTI to still be valid")
	}
}

func TestHandleServiceRequest_NASContainerServiceTypeMT_N1N2MessageN2_ExistingPDUSession_ServiceAccept_UplinkPDUSuccess(t *testing.T) {
	amf := amfContext.New(
		&FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		&FakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: "testkey",
		},
		&FakeSmf{Error: nil},
	)
	amf.T3555Cfg = amfContext.TimerValue{Enable: true, ExpireTime: 5 * time.Minute, MaxRetryTimes: 5}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	oldguti := mustTestGuti("001", "01", "cafe42", 0x00000001)
	snssai := models.Snssai{Sst: 1, Sd: "102030"}
	ue.T3513 = amfContext.NewTimer(6*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.SetOnGoing(amfContext.OnGoingProcedurePaging)
	ue.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}
	ue.State = amfContext.Registered
	ue.Guti = oldguti
	ue.Tai = ue.RanUe.Tai
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2
	ue.KnasEnc = key
	ue.KnasInt = key
	ue.CipheringAlg = algo
	ue.IntegrityAlg = security.AlgIntegrity128NIA0
	ue.Ambr = &models.Ambr{Uplink: "100mbps", Downlink: "100mbps"}
	_ = ue.CreateSmContext(1, "testref", &snssai)
	_ = ue.CreateSmContext(12, "testrefuplink", &snssai)
	ue.N1N2Message = &models.N1N2MessageTransferRequest{PduSessionID: 1, SNssai: &snssai, BinaryDataN2Information: []byte{}}

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount.Get(), nasMessage.ServiceTypeMobileTerminatedServices)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	err = handleServiceRequest(t.Context(), amf, ue, m.ServiceRequest)
	if err != nil {
		t.Fatalf("expected no errors, got: %v", err)
	}

	if len(ngapSender.SentPDUSessionResourceSetupRequest) < 1 {
		t.Fatalf("should have sent a PDU Session Resource Setup Request message")
	}

	pduResp := ngapSender.SentPDUSessionResourceSetupRequest[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(pduResp.NasPdu) & 0x0f

	payload := make([]byte, len(pduResp.NasPdu))
	copy(payload, pduResp.NasPdu)
	payload = payload[7:]

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Fatalf("expected a ciphered NAS message")
	}

	if err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get(), security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
		t.Fatalf("could not decrypt NAS message: %v", err)
	}

	err = nm.PlainNasDecode(&payload)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message: %v, %v", err, payload)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeServiceAccept {
		t.Fatalf("expected a service accept message, got '%v'", nm.GmmHeader.GetMessageType())
	}

	if nm.ServiceAccept.PDUSessionReactivationResult.GetPSI12() != 0 {
		t.Fatalf("should not have failed to reactivate PDU Session ID 12")
	}

	if nm.ServiceAccept.PDUSessionStatus.GetPSI1() != 1 {
		t.Fatalf("should have indicated PDU Session ID 1 is active in network")
	}

	if nm.ServiceAccept.PDUSessionStatus.GetPSI13() != 0 {
		t.Fatalf("should have indicated PDU Session ID 13 is inactive in network")
	}

	if len(ngapSender.SentDownlinkNASTransport) < 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm = new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	payload = make([]byte, len(resp.NasPdu))
	copy(payload, resp.NasPdu)
	payload = payload[7:]

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Fatalf("expected a ciphered NAS message")
	}

	if err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get()+1, security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
		t.Fatalf("could not decrypt NAS message: %v", err)
	}

	err = nm.PlainNasDecode(&payload)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message: %v, %v", err, payload)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeConfigurationUpdateCommand {
		t.Fatalf("expected a configuration update command message, got '%v'", nm.GmmHeader.GetMessageType())
	}

	if ue.T3513 != nil {
		t.Fatalf("expected timer T3513 to be stopped and cleared")
	}

	if ue.T3555 == nil {
		t.Fatalf("expected timer T3555 to be started")
	}

	if ue.Guti == oldguti {
		t.Fatal("expected new GUTI to be allocated")
	}

	if ue.OldGuti != oldguti {
		t.Fatal("expected old GUTI to still be valid")
	}
}

func TestHandleServiceRequest_NASContainerServiceTypeMT_N1N2MessageN2_UeCtxReq_ExistingPDUSession_ServiceAccept_UplinkPDUSuccess(t *testing.T) {
	amf := amfContext.New(
		&FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		&FakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: "testkey",
		},
		&FakeSmf{Error: nil},
	)
	amf.T3555Cfg = amfContext.TimerValue{Enable: true, ExpireTime: 5 * time.Minute, MaxRetryTimes: 5}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	oldguti := mustTestGuti("001", "01", "cafe42", 0x00000001)
	snssai := models.Snssai{Sst: 1, Sd: "102030"}
	ue.T3513 = amfContext.NewTimer(6*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.SetOnGoing(amfContext.OnGoingProcedurePaging)
	ue.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}
	ue.State = amfContext.Registered
	ue.Guti = oldguti
	ue.Tai = ue.RanUe.Tai
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2
	ue.KnasEnc = key
	ue.KnasInt = key
	ue.CipheringAlg = algo
	ue.IntegrityAlg = security.AlgIntegrity128NIA0
	ue.Ambr = &models.Ambr{Uplink: "100mbps", Downlink: "100mbps"}
	_ = ue.CreateSmContext(1, "testref", &snssai)
	_ = ue.CreateSmContext(12, "testrefuplink", &snssai)
	ue.N1N2Message = &models.N1N2MessageTransferRequest{PduSessionID: 1, SNssai: &snssai, BinaryDataN2Information: []byte{}}
	ue.RanUe.UeContextRequest = true

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount.Get(), nasMessage.ServiceTypeMobileTerminatedServices)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	err = handleServiceRequest(t.Context(), amf, ue, m.ServiceRequest)
	if err != nil {
		t.Fatalf("expected no errors, got: %v", err)
	}

	if len(ngapSender.SentInitialContextSetupRequest) < 1 {
		t.Fatalf("should have sent a PDU Session Resource Setup Request message")
	}

	pduResp := ngapSender.SentInitialContextSetupRequest[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(pduResp.NasPdu) & 0x0f

	payload := make([]byte, len(pduResp.NasPdu))
	copy(payload, pduResp.NasPdu)
	payload = payload[7:]

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Fatalf("expected a ciphered NAS message")
	}

	if err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get(), security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
		t.Fatalf("could not decrypt NAS message: %v", err)
	}

	err = nm.PlainNasDecode(&payload)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message: %v, %v", err, payload)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeServiceAccept {
		t.Fatalf("expected a service accept message, got '%v'", nm.GmmHeader.GetMessageType())
	}

	if len(ngapSender.SentDownlinkNASTransport) < 1 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm = new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	payload = make([]byte, len(resp.NasPdu))
	copy(payload, resp.NasPdu)
	payload = payload[7:]

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Fatalf("expected a ciphered NAS message")
	}

	if err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get()+1, security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
		t.Fatalf("could not decrypt NAS message: %v", err)
	}

	err = nm.PlainNasDecode(&payload)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message: %v, %v", err, payload)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeConfigurationUpdateCommand {
		t.Fatalf("expected a configuration update command message, got '%v'", nm.GmmHeader.GetMessageType())
	}

	if ue.T3513 != nil {
		t.Fatalf("expected timer T3513 to be stopped and cleared")
	}

	if ue.T3555 == nil {
		t.Fatalf("expected timer T3555 to be started")
	}

	if ue.Guti == oldguti {
		t.Fatal("expected new GUTI to be allocated")
	}

	if ue.OldGuti != oldguti {
		t.Fatal("expected old GUTI to still be valid")
	}
}

func TestHandleServiceRequest_NASContainerServiceTypeMT_DownlinkSignalingOnly_ServiceAccept(t *testing.T) {
	amf := amfContext.New(
		&FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		&FakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: "testkey",
		},
		&FakeSmf{Error: nil},
	)
	amf.T3555Cfg = amfContext.TimerValue{Enable: true, ExpireTime: 5 * time.Minute, MaxRetryTimes: 5}

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	oldguti := mustTestGuti("001", "01", "cafe42", 0x00000001)
	snssai := models.Snssai{Sst: 1, Sd: "102030"}
	ue.T3513 = amfContext.NewTimer(6*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.SetOnGoing(amfContext.OnGoingProcedurePaging)
	ue.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}
	ue.State = amfContext.Registered
	ue.Guti = oldguti
	ue.Tai = ue.RanUe.Tai
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2
	ue.KnasEnc = key
	ue.KnasInt = key
	ue.CipheringAlg = algo
	ue.IntegrityAlg = security.AlgIntegrity128NIA0
	ue.Ambr = &models.Ambr{Uplink: "100mbps", Downlink: "100mbps"}
	_ = ue.CreateSmContext(1, "testref", &snssai)
	_ = ue.CreateSmContext(12, "testrefuplink", &snssai)

	n1msg, err := buildN1PDUSessionModificationCommand()
	if err != nil {
		t.Fatalf("could not build N1 message: %v", err)
	}

	ue.N1N2Message = &models.N1N2MessageTransferRequest{
		PduSessionID: 1,
		SNssai:       &snssai,
		// BinaryDataN2Information: []byte{},
		BinaryDataN1Message: n1msg,
	}

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount.Get(), nasMessage.ServiceTypeMobileTerminatedServices)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	err = handleServiceRequest(t.Context(), amf, ue, m.ServiceRequest)
	if err != nil {
		t.Fatalf("expected no errors, got: %v", err)
	}

	if len(ngapSender.SentPDUSessionResourceSetupRequest) < 1 {
		t.Fatalf("should have sent a PDU Session Resource Setup Request message")
	}

	pduResp := ngapSender.SentPDUSessionResourceSetupRequest[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(pduResp.NasPdu) & 0x0f

	payload := make([]byte, len(pduResp.NasPdu))
	copy(payload, pduResp.NasPdu)
	payload = payload[7:]

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Fatalf("expected a ciphered NAS message")
	}

	if err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get(), security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
		t.Fatalf("could not decrypt NAS message: %v", err)
	}

	err = nm.PlainNasDecode(&payload)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message: %v, %v", err, payload)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeServiceAccept {
		t.Fatalf("expected a service accept message, got '%v'", nm.GmmHeader.GetMessageType())
	}

	if len(ngapSender.SentDownlinkNASTransport) < 2 {
		t.Fatalf("should have sent a Downlink NAS Transport message")
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm = new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	payload = make([]byte, len(resp.NasPdu))
	copy(payload, resp.NasPdu)
	payload = payload[7:]

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Fatalf("expected a ciphered NAS message")
	}

	if err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get()+1, security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
		t.Fatalf("could not decrypt NAS message: %v", err)
	}

	err = nm.PlainNasDecode(&payload)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message: %v, %v", err, payload)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeDLNASTransport {
		t.Fatalf("expected a DL NAS transport message, got '%v'", nm.GmmHeader.GetMessageType())
	}

	if nm.DLNASTransport.GetPayloadContainerType() != nasMessage.PayloadContainerTypeN1SMInfo {
		t.Fatalf("expected payload container to be for N1SMInfo, got: %v", nm.DLNASTransport.GetPayloadContainerType())
	}

	if !slices.Equal(nm.DLNASTransport.GetPayloadContainerContents(), n1msg) {
		t.Fatalf("expected payload to match N1 message stored for UE, %v, %v", nm.DLNASTransport.GetPayloadContainerContents(), n1msg)
	}

	resp = ngapSender.SentDownlinkNASTransport[1]
	nm = new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	payload = make([]byte, len(resp.NasPdu))
	copy(payload, resp.NasPdu)
	payload = payload[7:]

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Fatalf("expected a ciphered NAS message")
	}

	if err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get()+2, security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
		t.Fatalf("could not decrypt NAS message: %v", err)
	}

	err = nm.PlainNasDecode(&payload)
	if err != nil {
		t.Fatalf("could not decode ciphered NAS message: %v, %v", err, payload)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeConfigurationUpdateCommand {
		t.Fatalf("expected a configuration update command message, got '%v'", nm.GmmHeader.GetMessageType())
	}

	if ue.T3513 != nil {
		t.Fatalf("expected timer T3513 to be stopped and cleared")
	}

	if ue.T3555 == nil {
		t.Fatalf("expected timer T3555 to be started")
	}

	if ue.Guti == oldguti {
		t.Fatal("expected new GUTI to be allocated")
	}

	if ue.OldGuti != oldguti {
		t.Fatal("expected old GUTI to still be valid")
	}
}

// TestHandleServiceRequest_OutOfRangePduSessionID_UplinkDataStatus verifies that a
// ServiceRequest with UplinkDataStatus does NOT panic when SmContextList contains
// a PDU session ID >= 16 (outside the [16]bool PSI array bounds).
// This is a regression test for an index-out-of-range crash (DoS vulnerability).
func TestHandleServiceRequest_OutOfRangePduSessionID_UplinkDataStatus(t *testing.T) {
	amf := amfContext.New(
		&FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		&FakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: "testkey",
		},
		&FakeSmf{},
	)

	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	snssai := models.Snssai{Sst: 1, Sd: "102030"}
	ue.State = amfContext.Registered
	ue.Tai = ue.RanUe.Tai
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2
	ue.KnasEnc = key
	ue.KnasInt = key
	ue.CipheringAlg = algo
	ue.IntegrityAlg = security.AlgIntegrity128NIA0

	// Inject an out-of-range PDU session ID (255) directly into SmContextList,
	// bypassing CreateSmContext validation. This simulates a malicious UE that
	// somehow stored an invalid session ID (e.g., via a hypothetical future bug).
	// The read-side bounds checks in handleServiceRequest must still prevent a panic.
	ue.SmContextList[255] = &amfContext.SmContext{Ref: "malicious-ref", Snssai: &snssai}

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount.Get(), nasMessage.ServiceTypeData)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handleServiceRequest panicked with out-of-range PDU session ID: %v", r)
		}
	}()

	err = handleServiceRequest(t.Context(), amf, ue, m.ServiceRequest)
	if err != nil {
		t.Logf("handleServiceRequest returned error (acceptable): %v", err)
	}
}

// TestHandleServiceRequest_OutOfRangePduSessionID_PDUSessionStatus verifies that a
// ServiceRequest with PDUSessionStatus does NOT panic when SmContextList contains
// a PDU session ID >= 16 (outside the [16]bool PSI array bounds).
func TestHandleServiceRequest_OutOfRangePduSessionID_PDUSessionStatus(t *testing.T) {
	amf := amfContext.New(
		&FakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				Sst:           1,
				SupportedTACs: "[\"000001\"]",
			},
		},
		&FakeAusf{
			AvKgAka: &ausf.AuthResult{
				Rand: hex.EncodeToString(make([]byte, 16)),
				Autn: hex.EncodeToString(make([]byte, 16)),
			},
			Supi:  mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf: "testkey",
		},
		&FakeSmf{},
	)

	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	snssai := models.Snssai{Sst: 1, Sd: "102030"}
	ue.State = amfContext.Registered
	ue.Tai = ue.RanUe.Tai
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2
	ue.KnasEnc = key
	ue.KnasInt = key
	ue.CipheringAlg = algo
	ue.IntegrityAlg = security.AlgIntegrity128NIA0

	// Inject an out-of-range PDU session ID (200) directly into SmContextList,
	// bypassing CreateSmContext validation to test the read-side safety net.
	ue.SmContextList[200] = &amfContext.SmContext{Ref: "malicious-ref", Snssai: &snssai}

	m, err := buildTestServiceRequestCiphered(algo, key, ue.ULCount.Get(), nasMessage.ServiceTypeData)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	// Ensure PDUSessionStatus is set in the inner message (it is by default in
	// buildTestServiceRequestCiphered). The panic occurs when iterating SmContextList
	// and indexing into the [16]bool psiArray with pduSessionID >= 16.

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handleServiceRequest panicked with out-of-range PDU session ID: %v", r)
		}
	}()

	err = handleServiceRequest(t.Context(), amf, ue, m.ServiceRequest)
	if err != nil {
		t.Logf("handleServiceRequest returned error (acceptable): %v", err)
	}
}

func buildTestServiceRequest() *nas.GmmMessage {
	m := nas.NewGmmMessage()

	serviceRequest := nasMessage.NewServiceRequest(0)
	serviceRequest.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	serviceRequest.SetSpareHalfOctet(0x00)
	serviceRequest.SetMessageType(nas.MsgTypeServiceRequest)
	serviceRequest.SetAMFPointer(0)
	serviceRequest.SetAMFSetID(0)
	serviceRequest.SetTMSI5G([4]uint8{0xDE, 0xAD, 0xBE, 0xEF})
	serviceRequest.SetServiceTypeValue(nasMessage.ServiceTypeSignalling)

	m.ServiceRequest = serviceRequest

	return m
}

func buildTestServiceRequestCiphered(cipherAlg uint8, key [16]uint8, ulcount uint32, svc_type uint8) (*nas.GmmMessage, error) {
	m := nas.NewGmmMessage()

	innerServiceRequest := nasMessage.NewServiceRequest(0)
	innerServiceRequest.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	innerServiceRequest.SetSecurityHeaderType(nas.SecurityHeaderTypeIntegrityProtectedAndCiphered)
	innerServiceRequest.SetSpareHalfOctet(0x00)
	innerServiceRequest.SetMessageType(nas.MsgTypeServiceRequest)
	innerServiceRequest.SetAMFPointer(0)
	innerServiceRequest.SetAMFSetID(0)
	innerServiceRequest.SetTMSI5G([4]uint8{0xDE, 0xAD, 0xBE, 0xEF})
	innerServiceRequest.SetServiceTypeValue(svc_type)
	innerServiceRequest.SetNasKeySetIdentifiler(1)
	innerServiceRequest.TMSI5GS.SetLen(7)
	innerServiceRequest.SetTypeOfIdentity(4) // 5G-S-TMSI
	innerServiceRequest.UplinkDataStatus = &nasType.UplinkDataStatus{}
	innerServiceRequest.UplinkDataStatus.SetIei(nasMessage.ServiceRequestUplinkDataStatusType)
	innerServiceRequest.UplinkDataStatus.SetLen(2)
	innerServiceRequest.UplinkDataStatus.SetPSI12(1)
	innerServiceRequest.PDUSessionStatus = &nasType.PDUSessionStatus{}
	innerServiceRequest.PDUSessionStatus.SetIei(nasMessage.ServiceRequestPDUSessionStatusType)
	innerServiceRequest.PDUSessionStatus.SetLen(2)
	innerServiceRequest.PDUSessionStatus.SetPSI1(1)
	innerServiceRequest.PDUSessionStatus.SetPSI13(1)

	m.ServiceRequest = innerServiceRequest
	m.SetMessageType(nas.MsgTypeServiceRequest)

	data := new(bytes.Buffer)

	err := m.EncodeServiceRequest(data)
	if err != nil {
		return nil, fmt.Errorf("could not encode registration request: %v", err)
	}

	nasPdu := data.Bytes()

	if err = security.NASEncrypt(cipherAlg, key, ulcount, security.Bearer3GPP, security.DirectionUplink, nasPdu); err != nil {
		return nil, fmt.Errorf("could not encrypt NAS message: %v", err)
	}

	serviceRequest := nasMessage.NewServiceRequest(0)
	serviceRequest.NASMessageContainer = nasType.NewNASMessageContainer(nasMessage.ServiceRequestNASMessageContainerType)
	serviceRequest.NASMessageContainer.SetLen(uint16(len(nasPdu)))
	serviceRequest.SetNASMessageContainerContents(nasPdu)
	serviceRequest.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	serviceRequest.SetSpareHalfOctet(0x00)
	serviceRequest.SetMessageType(svc_type)
	serviceRequest.SetAMFPointer(0)
	serviceRequest.SetAMFSetID(0)
	serviceRequest.SetTMSI5G([4]uint8{0xDE, 0xAD, 0xBE, 0xEF})
	serviceRequest.SetServiceTypeValue(nasMessage.ServiceTypeSignalling)
	serviceRequest.SetNasKeySetIdentifiler(1)
	serviceRequest.TMSI5GS.SetLen(7)
	serviceRequest.SetTypeOfIdentity(4) // 5G-S-TMSI
	serviceRequest.UplinkDataStatus = &nasType.UplinkDataStatus{}
	serviceRequest.UplinkDataStatus.SetIei(nasMessage.ServiceRequestUplinkDataStatusType)
	serviceRequest.UplinkDataStatus.SetLen(2)
	serviceRequest.UplinkDataStatus.SetPSI12(1)
	serviceRequest.PDUSessionStatus = &nasType.PDUSessionStatus{}
	serviceRequest.PDUSessionStatus.SetIei(nasMessage.ServiceRequestPDUSessionStatusType)
	serviceRequest.PDUSessionStatus.SetLen(2)
	serviceRequest.PDUSessionStatus.SetPSI1(1)
	serviceRequest.PDUSessionStatus.SetPSI13(1)

	m.ServiceRequest = serviceRequest

	return m, nil
}

func buildN1PDUSessionModificationCommand() ([]byte, error) {
	m := nas.NewGsmMessage()

	pduModCmd := nasMessage.NewPDUSessionModificationCommand(0)
	pduModCmd.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	pduModCmd.SetMessageType(nas.MsgTypePDUSessionModificationCommand)
	pduModCmd.SetPDUSessionID(1)

	m.PDUSessionModificationCommand = pduModCmd
	m.SetMessageType(nas.MsgTypePDUSessionModificationCommand)

	data := new(bytes.Buffer)

	err := m.EncodePDUSessionModificationCommand(data)
	if err != nil {
		return nil, fmt.Errorf("could not encode PDU session modification command: %v", err)
	}

	return data.Bytes(), nil
}
