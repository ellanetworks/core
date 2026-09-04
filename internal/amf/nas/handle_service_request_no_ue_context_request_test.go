// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
)

func TestHandleServiceRequest_NoUeContextRequest_ResumesThroughInitialContextSetup(t *testing.T) {
	f := connectedModeUe(t, &fakeSmf{})
	f.conn().UeContextRequest = false

	idleToActive(t, f, fgs.ServiceTypeData)

	if len(f.ngapSender.SentPDUSessionResourceSetupRequest) != 0 {
		t.Errorf("PDU session resource setup requests = %d, want 0: the NG-RAN node holds no UE context for this connection",
			len(f.ngapSender.SentPDUSessionResourceSetupRequest))
	}

	if len(f.ngapSender.SentInitialContextSetupRequest) != 1 {
		t.Fatalf("initial context setup requests = %d, want 1", len(f.ngapSender.SentInitialContextSetupRequest))
	}

	ics := f.ngapSender.SentInitialContextSetupRequest[0]

	if got := len(ics.PDUSessionResourceSetup); got != 1 {
		t.Fatalf("PDU session resource setup list length = %d, want 1", got)
	}

	if got := uint8(ics.PDUSessionResourceSetup[0].PDUSessionID); got != 12 {
		t.Errorf("PDU session id = %d, want 12", got)
	}
}

func TestHandleServiceRequest_NoUeContextRequest_ContextAlreadySetUp_StaysStandalone(t *testing.T) {
	f := connectedModeUe(t, &fakeSmf{})
	f.conn().UeContextRequest = false
	f.conn().MarkICSCompleted()

	idleToActive(t, f, fgs.ServiceTypeData)

	if len(f.ngapSender.SentInitialContextSetupRequest) != 0 {
		t.Errorf("initial context setup requests = %d, want 0", len(f.ngapSender.SentInitialContextSetupRequest))
	}

	if len(f.ngapSender.SentPDUSessionResourceSetupRequest) != 1 {
		t.Fatalf("PDU session resource setup requests = %d, want 1", len(f.ngapSender.SentPDUSessionResourceSetupRequest))
	}
}

func TestHandleServiceRequest_NoUeContextRequest_SignallingOnly_NoContextSetup(t *testing.T) {
	f := connectedModeUe(t, &fakeSmf{})
	f.conn().UeContextRequest = false

	f.ue.DeleteSmContext(12)

	idleToActive(t, f, fgs.ServiceTypeSignalling)

	if len(f.ngapSender.SentInitialContextSetupRequest) != 0 {
		t.Errorf("initial context setup requests = %d, want 0", len(f.ngapSender.SentInitialContextSetupRequest))
	}

	if len(f.ngapSender.SentPDUSessionResourceSetupRequest) != 0 {
		t.Errorf("PDU session resource setup requests = %d, want 0", len(f.ngapSender.SentPDUSessionResourceSetupRequest))
	}

	if len(f.ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("downlink nas transports = %d, want 1", len(f.ngapSender.SentDownlinkNASTransport))
	}
}

func TestHandleServiceRequest_UeContextRequest_SignallingOnly_SetsUpContext(t *testing.T) {
	f := connectedModeUe(t, &fakeSmf{})

	f.ue.DeleteSmContext(12)

	idleToActive(t, f, fgs.ServiceTypeSignalling)

	if len(f.ngapSender.SentInitialContextSetupRequest) != 1 {
		t.Fatalf("initial context setup requests = %d, want 1", len(f.ngapSender.SentInitialContextSetupRequest))
	}
}

func TestSendPDUSessionResourceSetupRequest_RefusedBeforeInitialContextSetup(t *testing.T) {
	f := connectedModeUe(t, &fakeSmf{})

	if got := f.conn().ICS(); got != amf.ICSNotStarted {
		t.Fatalf("initial context setup state = %v, want ICSNotStarted", got)
	}

	item, err := amf.PDUSessionSetupItemSUReq(12, &models.Snssai{Sst: 1, Sd: "102030"}, nil, []byte{0x01, 0x02, 0x03})
	if err != nil {
		t.Fatalf("could not build PDU session setup item: %v", err)
	}

	list := ngap.PDUSessionResourceSetupListSUReq{item}

	if err := f.conn().SendPDUSessionResourceSetupRequest(t.Context(), f.ue.Ambr.Uplink, f.ue.Ambr.Downlink, nil, list); err == nil {
		t.Fatal("send succeeded, want a refusal: the NG-RAN node holds no UE context")
	}

	f.conn().MarkICSCompleted()

	if err := f.conn().SendPDUSessionResourceSetupRequest(t.Context(), f.ue.Ambr.Uplink, f.ue.Ambr.Downlink, nil, list); err != nil {
		t.Fatalf("send failed once the NG-RAN node holds the UE context: %v", err)
	}

	if len(f.ngapSender.SentPDUSessionResourceSetupRequest) != 1 {
		t.Errorf("PDU session resource setup requests = %d, want 1: only the send after the context setup reaches the NG-RAN node",
			len(f.ngapSender.SentPDUSessionResourceSetupRequest))
	}
}

func TestUEAssociatedSendsRefusedBeforeInitialContextSetup(t *testing.T) {
	f := connectedModeUe(t, &fakeSmf{})

	relList := ngap.PDUSessionResourceToReleaseListRelCmd{
		{PDUSessionID: 12, Transfer: ngap.TransferContainer([]byte{0x01, 0x02, 0x03})},
	}

	modList := ngap.PDUSessionResourceModifyListModReq{
		{PDUSessionID: 12, Transfer: ngap.TransferContainer([]byte{0x01, 0x02, 0x03})},
	}

	if err := f.conn().SendPDUSessionResourceReleaseCommand(t.Context(), nil, relList); err == nil {
		t.Error("release command send succeeded, want a refusal")
	}

	if err := f.conn().SendPDUSessionResourceModifyRequest(t.Context(), modList); err == nil {
		t.Error("modify request send succeeded, want a refusal")
	}

	f.conn().MarkICSCompleted()

	if err := f.conn().SendPDUSessionResourceReleaseCommand(t.Context(), nil, relList); err != nil {
		t.Errorf("release command failed once the NG-RAN node holds the UE context: %v", err)
	}

	if err := f.conn().SendPDUSessionResourceModifyRequest(t.Context(), modList); err != nil {
		t.Errorf("modify request failed once the NG-RAN node holds the UE context: %v", err)
	}

	if len(f.ngapSender.SentPDUSessionResourceReleaseCommand) != 1 {
		t.Errorf("release commands = %d, want 1", len(f.ngapSender.SentPDUSessionResourceReleaseCommand))
	}
}
