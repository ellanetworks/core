// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
)

// idleToActive drives a SERVICE REQUEST from a UE coming out of CM-IDLE, so the AMF
// answers with an INITIAL CONTEXT SETUP REQUEST rather than a standalone PDU session
// resource setup.
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

// A session-scoped payload buffered by an unanswered paging must not suppress the
// user-plane re-establishment the UE asked for (TS 24.501 §5.6.1.4.1). The service type
// a UE uses to come back is "data", not "mobile terminated services" (§5.6.1.2.1), so
// gating reactivation on the latter strands the UE with a signalling-only connection.
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

// The buffered payload is consumed whatever the service type. Leaving it behind would
// strand every later SERVICE REQUEST from the same UE.
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

// A second SERVICE REQUEST after a buffered payload was consumed must still bring the
// user plane up: this is the loop the Pixel was stuck in.
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

	f.conn().ResetICS()
	f.ngapSender.SentInitialContextSetupRequest = nil

	idleToActive(t, f, fgs.ServiceTypeData)

	if len(f.ngapSender.SentInitialContextSetupRequest) != 1 {
		t.Fatalf("initial context setup requests = %d, want 1", len(f.ngapSender.SentInitialContextSetupRequest))
	}

	if got := len(f.ngapSender.SentInitialContextSetupRequest[0].PDUSessionResourceSetup); got != 1 {
		t.Fatalf("PDU session resource setup list length = %d, want 1", got)
	}
}
