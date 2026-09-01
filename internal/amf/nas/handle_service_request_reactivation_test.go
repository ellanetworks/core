// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
)

func idleToActive(t *testing.T, f connectedModeFixture, svcType fgs.ServiceType) connectedModeFixture {
	t.Helper()

	handleServiceRequest(t.Context(), f.amf, f.ue, f.serviceRequest(t, svcType), true)

	return f
}

func TestHandleServiceRequest_IdleToActive_ReactivatesPDUSession(t *testing.T) {
	f := idleToActive(t, connectedModeUe(t, &fakeSmf{}), fgs.ServiceTypeData)

	if len(f.ngapSender.SentInitialContextSetupRequest) != 1 {
		t.Fatalf("initial context setup requests = %d, want 1", len(f.ngapSender.SentInitialContextSetupRequest))
	}

	if got := len(f.ngapSender.SentInitialContextSetupRequest[0].PDUSessionResourceSetup); got != 1 {
		t.Fatalf("PDU session resource setup list length = %d, want 1", got)
	}
}

func TestHandleServiceRequest_BufferedN1N2_DoesNotSuppressReactivation(t *testing.T) {
	for _, svcType := range []fgs.ServiceType{
		fgs.ServiceTypeData,
		fgs.ServiceTypeSignalling,
		fgs.ServiceTypeHighPriorityAccess,
		fgs.ServiceTypeMobileTerminatedServices,
	} {
		t.Run(svcType.String(), func(t *testing.T) {
			f := connectedModeUe(t, &fakeSmf{})

			snssai := models.Snssai{Sst: 1, Sd: "102030"}
			f.ue.SetN1N2Message(&models.N1N2MessageTransferRequest{
				N1Class:                 models.N1ClassSM,
				N2Class:                 models.N2ClassSM,
				PduSessionID:            12,
				SNssai:                  &snssai,
				BinaryDataN2Information: []byte{0x01, 0x02, 0x03},
			})

			idleToActive(t, f, svcType)

			if len(f.ngapSender.SentInitialContextSetupRequest) != 1 {
				t.Fatalf("initial context setup requests = %d, want 1", len(f.ngapSender.SentInitialContextSetupRequest))
			}

			if got := len(f.ngapSender.SentInitialContextSetupRequest[0].PDUSessionResourceSetup); got != 1 {
				t.Fatalf("PDU session resource setup list length = %d, want 1", got)
			}
		})
	}
}

func TestHandleServiceRequest_BufferedN1N2_IsConsumed(t *testing.T) {
	f := connectedModeUe(t, &fakeSmf{})

	snssai := models.Snssai{Sst: 1, Sd: "102030"}
	f.ue.SetN1N2Message(&models.N1N2MessageTransferRequest{
		N1Class:                 models.N1ClassSM,
		N2Class:                 models.N2ClassSM,
		PduSessionID:            12,
		SNssai:                  &snssai,
		BinaryDataN2Information: []byte{0x01, 0x02, 0x03},
	})

	idleToActive(t, f, fgs.ServiceTypeData)

	if f.ue.N1N2Message() != nil {
		t.Fatal("buffered N1N2 message still present after the service accept")
	}
}

func TestHandleServiceRequest_SecondRequestAfterBufferedN1N2_StillReactivates(t *testing.T) {
	f := connectedModeUe(t, &fakeSmf{})

	snssai := models.Snssai{Sst: 1, Sd: "102030"}
	f.ue.SetN1N2Message(&models.N1N2MessageTransferRequest{
		N1Class:                 models.N1ClassSM,
		N2Class:                 models.N2ClassSM,
		PduSessionID:            12,
		SNssai:                  &snssai,
		BinaryDataN2Information: []byte{0x01, 0x02, 0x03},
	})

	idleToActive(t, f, fgs.ServiceTypeData)

	f.conn().AbortICS()
	f.ngapSender.SentInitialContextSetupRequest = nil

	idleToActive(t, f, fgs.ServiceTypeData)

	if len(f.ngapSender.SentInitialContextSetupRequest) != 1 {
		t.Fatalf("initial context setup requests = %d, want 1", len(f.ngapSender.SentInitialContextSetupRequest))
	}

	if got := len(f.ngapSender.SentInitialContextSetupRequest[0].PDUSessionResourceSetup); got != 1 {
		t.Fatalf("PDU session resource setup list length = %d, want 1", got)
	}
}

func TestHandleServiceRequest_ItemBuildFailure_DoesNotStrandTheClaim(t *testing.T) {
	f := connectedModeUe(t, &fakeSmf{})

	if err := f.ue.CreateSmContext(12, "testrefuplink", nil, "internet"); err != nil {
		t.Fatalf("could not recreate the sm context: %v", err)
	}

	f.conn().UeContextRequest = false
	f.conn().MarkICSCompleted()

	idleToActive(t, f, fgs.ServiceTypeData)

	if f.conn().N2SetupOpen(amf.N2SetupPDUSession) {
		t.Error("a transaction that sent no setup request must not stay open")
	}

	if !f.conn().N2Setup(amf.N2SetupPDUSession).ClaimSession(12) {
		t.Error("the PDU session is still claimed although no setup request reached the RAN")
	}
}

func TestHandleServiceRequest_AlreadySetUpSession_DoesNotActivateTheSmContext(t *testing.T) {
	smf := &fakeSmf{}
	f := connectedModeUe(t, smf)

	f.conn().SetN2SessionActive(12)
	f.conn().UeContextRequest = false
	f.conn().MarkICSCompleted()

	idleToActive(t, f, fgs.ServiceTypeData)

	if len(smf.ActivateSmContextCalls) != 0 {
		t.Errorf("ActivateSmContext calls = %d, want 0: the SMF must not be driven through UP activation for a session the NG-RAN node already holds",
			len(smf.ActivateSmContextCalls))
	}

	if len(f.ngapSender.SentPDUSessionResourceSetupRequest) != 0 {
		t.Errorf("PDU session resource setup requests = %d, want 0", len(f.ngapSender.SentPDUSessionResourceSetupRequest))
	}
}

func TestHandleServiceRequest_AlreadyEstablishedSession_IsNotReportedAsAReactivationFailure(t *testing.T) {
	f := connectedModeUe(t, &fakeSmf{})

	f.conn().SetN2SessionActive(12)
	f.conn().UeContextRequest = false
	f.conn().MarkICSCompleted()

	handleServiceRequest(t.Context(), f.amf, f.ue, f.serviceRequest(t, fgs.ServiceTypeData), true)

	if len(f.ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("downlink nas transports = %d, want 1", len(f.ngapSender.SentDownlinkNASTransport))
	}

	plain := decipherGmm(t, f.ue, f.ngapSender.SentDownlinkNASTransport[0].NASPDU, uint8(fgs.MsgServiceAccept))

	accept, err := fgs.ParseServiceAccept(plain)
	if err != nil {
		t.Fatalf("could not parse service accept: %v", err)
	}

	if accept.PDUSessionReactivationResult != nil && psiSet(accept.PDUSessionReactivationResult, 12) {
		t.Error("PDU session 12 reported as a reactivation failure although its user-plane resources are established (TS 24.501 9.11.3.42)")
	}
}
