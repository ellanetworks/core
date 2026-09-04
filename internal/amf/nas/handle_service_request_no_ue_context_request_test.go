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

// An NG-RAN node may leave the UE Context Request IE out of the INITIAL UE MESSAGE and
// still hold no UE context on the connection the SERVICE REQUEST arrives on. TS 38.413
// §8.6.1.2 makes the IE an obligation to run the Initial Context Setup, not a condition
// for it, and §8.3.1.1 leaves the procedure to configuration only for signalling-only
// connections; a PDU SESSION RESOURCE SETUP REQUEST sent instead reaches a node holding
// no context and is answered with an Error Indication (§10.4).
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

// The standalone procedure stays available once the NG-RAN node holds the context, which
// is the 5GMM-CONNECTED case of TS 23.502 §4.2.3.2 step 12.
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

// A signalling-only request from a node that asked for no UE context sets none up
// (TS 38.413 §8.3.1.1).
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

// The UE Context Request IE keeps its own force: a node that sends it gets the Initial
// Context Setup even when the downlink carries no user-plane resources (§8.6.1.2).
func TestHandleServiceRequest_UeContextRequest_SignallingOnly_SetsUpContext(t *testing.T) {
	f := connectedModeUe(t, &fakeSmf{})

	f.ue.DeleteSmContext(12)

	idleToActive(t, f, fgs.ServiceTypeSignalling)

	if len(f.ngapSender.SentInitialContextSetupRequest) != 1 {
		t.Fatalf("initial context setup requests = %d, want 1", len(f.ngapSender.SentInitialContextSetupRequest))
	}
}

// The wire-level backstop: no PDU SESSION RESOURCE SETUP REQUEST reaches an NG-RAN node
// that has not been sent an INITIAL CONTEXT SETUP REQUEST.
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
