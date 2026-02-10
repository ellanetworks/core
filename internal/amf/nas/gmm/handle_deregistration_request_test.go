package gmm

import (
	"fmt"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

func TestHandleRegeristrationRequest(t *testing.T) {
	testcases := []context.StateType{context.Deregistered, context.Authentication, context.SecurityMode, context.ContextSetup}
	for _, tc := range testcases {
		t.Run(fmt.Sprintf("State-%s", tc), func(t *testing.T) {
			ue, ngapSender, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not build test ue: %v", err)
			}

			ue.State = tc

			expected := fmt.Sprintf("state mismatch: receive Deregistration Request (UE Originating Deregistration) message in state %s", tc)

			err = handleDeregistrationRequestUEOriginatingDeregistration(t.Context(), nil, ue, nil)
			if err == nil || err.Error() != expected {
				t.Fatalf("expected error: %s, got: %v", expected, err)
			}

			if len(ngapSender.SentUEContextReleaseCommand) != 0 {
				t.Fatalf("should not have sent a UE Context Release Command")
			}
		})
	}
}

func TestHandleRegistrationRequest_AllSmContextAreReleased(t *testing.T) {
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
	snssai := models.Snssai{Sst: 1, Sd: "102030"}

	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test ue: %v", err)
	}

	ue.State = context.Registered
	ue.CreateSmContext(1, "testref1", &snssai)
	ue.CreateSmContext(2, "testref2", &snssai)
	ue.CreateSmContext(3, "testref3", &snssai)
	ue.CreateSmContext(4, "testref4", &snssai)

	m := buildTestDeregistrationRequestUEOriginatingDeregistrationMessage()

	err = handleDeregistrationRequestUEOriginatingDeregistration(t.Context(), amf, ue, m.DeregistrationRequestUEOriginatingDeregistration)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	r := smf.ReleasedSmContext

	if len(r) != 4 {
		t.Fatalf("expected 4 SmContext to be relased, released: %d", len(r))
	}

	if !slices.Contains(r, "testref1") || !slices.Contains(r, "testref2") || !slices.Contains(r, "testref3") || !slices.Contains(r, "testref4") {
		t.Fatalf("expected all SM Contexts to be release, released: %v", r)
	}
}

func TestHandleDeregistrationRequest_NilRanUE(t *testing.T) {
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

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test ue: %v", err)
	}

	ue.State = context.Registered
	ue.RanUe = nil

	m := buildTestDeregistrationRequestUEOriginatingDeregistrationMessage()

	err = handleDeregistrationRequestUEOriginatingDeregistration(t.Context(), amf, ue, m.DeregistrationRequestUEOriginatingDeregistration)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatal("should not have sent a downlink NAS transport message")
	}

	if len(ngapSender.SentUEContextReleaseCommand) != 0 {
		t.Fatal("should not have sent a downlink NAS transport message")
	}
}

func TestHandleDeregistrationRequest_NotSwitchOff_DeregistrationAccept(t *testing.T) {
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

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test ue: %v", err)
	}

	ue.State = context.Registered

	m := buildTestDeregistrationRequestUEOriginatingDeregistrationMessage()

	err = handleDeregistrationRequestUEOriginatingDeregistration(t.Context(), amf, ue, m.DeregistrationRequestUEOriginatingDeregistration)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatal("should have sent a downlink NAS transport message")
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

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeDeregistrationAcceptUEOriginatingDeregistration {
		t.Fatalf("expected a deregistration accept message, got '%v'", nm.GmmHeader.GetMessageType())
	}

	if len(ngapSender.SentUEContextReleaseCommand) != 1 {
		t.Fatal("should have sent a UE Context Release Command message")
	}
}

func TestHandleDeregistrationRequest_SwitchOff_NoDeregistrationAccept(t *testing.T) {
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

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test ue: %v", err)
	}

	ue.State = context.Registered

	m := buildTestDeregistrationRequestUEOriginatingDeregistrationMessage()
	m.DeregistrationRequestUEOriginatingDeregistration.SetSwitchOff(1)

	err = handleDeregistrationRequestUEOriginatingDeregistration(t.Context(), amf, ue, m.DeregistrationRequestUEOriginatingDeregistration)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatal("should have sent a downlink NAS transport message")
	}

	if len(ngapSender.SentUEContextReleaseCommand) != 1 {
		t.Fatal("should have sent a UE Context Release Command message")
	}
}

func TestHandleDeregistrationRequest_Non3GPP_DeregistrationAccept(t *testing.T) {
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

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test ue: %v", err)
	}

	ue.State = context.Registered

	m := buildTestDeregistrationRequestUEOriginatingDeregistrationMessage()
	m.DeregistrationRequestUEOriginatingDeregistration.SetAccessType(nasMessage.AccessTypeNon3GPP)

	err = handleDeregistrationRequestUEOriginatingDeregistration(t.Context(), amf, ue, m.DeregistrationRequestUEOriginatingDeregistration)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatal("should have sent a downlink NAS transport message")
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

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeDeregistrationAcceptUEOriginatingDeregistration {
		t.Fatalf("expected a deregistration accept message, got '%v'", nm.GmmHeader.GetMessageType())
	}

	if len(ngapSender.SentUEContextReleaseCommand) != 0 {
		t.Fatal("should not have sent a UE Context Release Command message")
	}
}

func buildTestDeregistrationRequestUEOriginatingDeregistrationMessage() *nas.GmmMessage {
	m := nas.NewGmmMessage()

	deregistrationRequest := nasMessage.NewDeregistrationRequestUEOriginatingDeregistration(0)
	deregistrationRequest.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	deregistrationRequest.SetSpareHalfOctet(0x00)
	deregistrationRequest.SetMessageType(nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration)
	deregistrationRequest.SetAccessType(nasMessage.AccessType3GPP)

	m.DeregistrationRequestUEOriginatingDeregistration = deregistrationRequest

	return m
}
