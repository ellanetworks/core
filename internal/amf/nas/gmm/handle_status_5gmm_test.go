package gmm

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

func TestHandleStatus5GMM_UEDeregistered_Error(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.Deregister()

	m := buildTestStatus5gmm()

	expected := "UE is in Deregistered state, ignore Status 5GMM message"

	err = handleStatus5GMM(ue, m.Status5GMM)
	if err == nil || err.Error() != expected {
		t.Fatalf("expected error: %s, got: %v", expected, err)
	}
}

func TestHandleStatus5GMM_MacFailed_Error(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.State = context.Registered
	ue.MacFailed = true

	m := buildTestStatus5gmm()

	expected := "NAS message integrity check failed"

	err = handleStatus5GMM(ue, m.Status5GMM)
	if err == nil || err.Error() != expected {
		t.Fatalf("expected error: %s, got: %v", expected, err)
	}
}

func TestHandleStatus5GMM_NoErrror(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.State = context.Registered

	m := buildTestStatus5gmm()

	err = handleStatus5GMM(ue, m.Status5GMM)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func buildTestStatus5gmm() *nas.GmmMessage {
	m := nas.NewGmmMessage()

	status5gmm := nasMessage.NewStatus5GMM(0)
	status5gmm.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	status5gmm.SetSpareHalfOctet(0x00)
	status5gmm.SetMessageType(nas.MsgTypeStatus5GMM)

	m.Status5GMM = status5gmm
	m.SetMessageType(nas.MsgTypeStatus5GMM)

	return m
}
