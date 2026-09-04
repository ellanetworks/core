// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
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

func TestHandleServiceRequest_SessionReportedInactive_ReleasedOnBothSides(t *testing.T) {
	smf := &fakeSmf{}
	f := connectedModeUe(t, smf)

	snssai := models.Snssai{Sst: 1, Sd: "102030"}
	if err := f.ue.CreateSmContext(9, "ref-9", &snssai, "internet"); err != nil {
		t.Fatalf("could not create the sm context: %v", err)
	}

	idleToActive(t, f, fgs.ServiceTypeData)

	if len(smf.ReleasedSmContext) != 1 || smf.ReleasedSmContext[0] != "ref-9" {
		t.Fatalf("released %v in the SMF, want [ref-9]", smf.ReleasedSmContext)
	}

	if _, ok := f.ue.SmContextFindByPDUSessionID(9); ok {
		t.Error("TS 24.501 §5.6.1.4: PDU session 9 was reported inactive and must be released on the AMF side too, not only in the SMF")
	}

	if _, ok := f.ue.SmContextFindByPDUSessionID(12); !ok {
		t.Error("PDU session 12 was reported active and must be kept")
	}
}

func serviceRequestWithStatus(t *testing.T, f connectedModeFixture, svcType fgs.ServiceType, uplinkData, status []byte) []byte {
	t.Helper()

	inner := &fgs.ServiceRequest{
		ServiceType:      svcType,
		NgKSI:            nas.KeySetIdentifier{Value: 1},
		MobileIdentity:   serviceRequest5GSTMSI(),
		UplinkDataStatus: mustBitmap(uplinkData),
		PDUSessionStatus: mustBitmap(status),
	}

	innerBytes, err := inner.MarshalBinary()
	if err != nil {
		t.Fatalf("could not encode the inner service request: %v", err)
	}

	ciph, err := nas.CipherFor(f.algo)
	if err != nil {
		t.Fatalf("could not build the cipher: %v", err)
	}

	container, err := ciph.Apply(f.key, f.ue.ULCount(), nas.Bearer3GPP, nas.DirectionUplink, innerBytes)
	if err != nil {
		t.Fatalf("could not encrypt the service request: %v", err)
	}

	return encSR(t, &fgs.ServiceRequest{
		ServiceType:         fgs.ServiceTypeSignalling,
		NgKSI:               nas.KeySetIdentifier{Value: 1},
		MobileIdentity:      serviceRequest5GSTMSI(),
		NASMessageContainer: container,
	})
}

// TS 24.501 §5.6.1.4.1, §5.6.1.8 case i)
func TestHandleServiceRequest_BufferedPayloadForASessionReportedInactive_IsNotSetUpOnTheRAN(t *testing.T) {
	smf := &fakeSmf{}
	f := connectedModeUe(t, smf)

	snssai := models.Snssai{Sst: 1, Sd: "102030"}
	if err := f.ue.CreateSmContext(9, "ref-9", &snssai, "internet"); err != nil {
		t.Fatalf("could not create the sm context: %v", err)
	}

	f.ue.SetN1N2Message(&models.N1N2MessageTransferRequest{
		N1Class:                 models.N1ClassSM,
		N2Class:                 models.N2ClassSM,
		PduSessionID:            9,
		SNssai:                  &snssai,
		BinaryDataN2Information: []byte{0x01, 0x02, 0x03},
	})

	sr := serviceRequestWithStatus(t, f, fgs.ServiceTypeMobileTerminatedServices, []byte{0x00, 0x10}, []byte{0x00, 0x10})

	handleServiceRequest(t.Context(), f.amf, f.ue, sr, true)

	if len(smf.ReleasedSmContext) != 1 || smf.ReleasedSmContext[0] != "ref-9" {
		t.Fatalf("released %v in the SMF, want [ref-9]", smf.ReleasedSmContext)
	}

	if _, ok := f.ue.SmContextFindByPDUSessionID(9); ok {
		t.Error("PDU session 9 was reported inactive and must be released on the AMF side too")
	}

	if len(f.ngapSender.SentInitialContextSetupRequest) != 1 {
		t.Fatalf("initial context setup requests = %d, want 1: the service request must still be accepted (TS 24.501 §5.6.1.8 case i)",
			len(f.ngapSender.SentInitialContextSetupRequest))
	}

	for _, item := range f.ngapSender.SentInitialContextSetupRequest[0].PDUSessionResourceSetup {
		if item.PDUSessionID == 9 {
			t.Error("the NG-RAN node was asked to set up PDU session 9, which the AMF releases in the same SERVICE ACCEPT")
		}
	}

	if got := len(f.ngapSender.SentInitialContextSetupRequest[0].PDUSessionResourceSetup); got != 1 {
		t.Errorf("PDU session resource setup list length = %d, want 1: PDU session 12 was still requested by the UE", got)
	}

	if !f.conn().N2Setup(amf.N2SetupInitialContext).ClaimSession(9) {
		t.Error("PDU session 9 is still claimed on the NG-RAN node although it was never set up")
	}

	if f.ue.N1N2Message() != nil {
		t.Error("the buffered downlink payload for the released session must be discarded")
	}
}

// TS 24.501 §5.6.1.4.1 a)
func TestHandleServiceRequest_SmfReleaseFails_SessionIsStillReleasedOnTheAMFSide(t *testing.T) {
	smf := &fakeSmf{ReleaseSmContextError: errors.New("pfcp session deletion failed")}
	f := connectedModeUe(t, smf)

	snssai := models.Snssai{Sst: 1, Sd: "102030"}
	if err := f.ue.CreateSmContext(9, "ref-9", &snssai, "internet"); err != nil {
		t.Fatalf("could not create the sm context: %v", err)
	}

	idleToActive(t, f, fgs.ServiceTypeData)

	if len(smf.ReleaseSmContextCalls) != 1 || smf.ReleaseSmContextCalls[0].SmContextRef != "ref-9" {
		t.Fatalf("release calls = %v, want one for ref-9", smf.ReleaseSmContextCalls)
	}

	if _, ok := f.ue.SmContextFindByPDUSessionID(9); ok {
		t.Error("the AMF-side local release is not conditional on the SMF request succeeding (TS 24.501 §5.6.1.4.1 a)1))")
	}

	if got := f.ue.AllTransferableEPSSessions(); len(got) != 0 {
		t.Errorf("AllTransferableEPSSessions = %+v, want none: a released session must not move to EPS", got)
	}
}

// TS 24.501 §9.11.3.57 table 9.11.3.57.1 forbids this combination: a PDU session in
// PDU SESSION INACTIVE state is coded 0 in the Uplink data status IE.
func TestHandleServiceRequest_UplinkDataForASessionReportedInactive_IsNotSetUpOnTheRAN(t *testing.T) {
	smf := &fakeSmf{}
	f := connectedModeUe(t, smf)

	sr := serviceRequestWithStatus(t, f, fgs.ServiceTypeData, []byte{0x00, 0x10}, []byte{0x00, 0x00})

	handleServiceRequest(t.Context(), f.amf, f.ue, sr, true)

	if len(smf.ActivateSmContextCalls) != 0 {
		t.Errorf("ActivateSmContext calls = %v, want none for a session the UE reports inactive", smf.ActivateSmContextCalls)
	}

	for _, ics := range f.ngapSender.SentInitialContextSetupRequest {
		for _, item := range ics.PDUSessionResourceSetup {
			if item.PDUSessionID == 12 {
				t.Error("the NG-RAN node was asked to set up PDU session 12, which the AMF releases in the same SERVICE ACCEPT")
			}
		}
	}

	if _, ok := f.ue.SmContextFindByPDUSessionID(12); ok {
		t.Error("PDU session 12 was reported inactive and must be released on the AMF side too")
	}
}
