// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

type fakeNGAPSender struct {
	pduSessionSetupCalls          int
	initialContextSetupCalls      int
	downlinkNasTransportCalls     int
	pagingCalls                   int
	locationReportingControlCalls int
	nrppaTransportCalls           int
}

func (f *fakeNGAPSender) WriteMsg(b []byte, _ *sctp.SndRcvInfo) (int, error) {
	pdu, err := ngap.Unmarshal(b)
	if err != nil {
		panic(fmt.Sprintf("fakeNGAPSender: unmarshal NGAP PDU: %v", err))
	}

	if m, ok := pdu.(*ngap.InitiatingMessage); ok {
		switch m.ProcedureCode {
		case ngap.ProcPaging:
			f.pagingCalls++
		case ngap.ProcPDUSessionResourceSetup:
			f.pduSessionSetupCalls++
		case ngap.ProcInitialContextSetup:
			f.initialContextSetupCalls++
		case ngap.ProcDownlinkNASTransport:
			f.downlinkNasTransportCalls++
		case ngap.ProcLocationReportingControl:
			f.locationReportingControlCalls++
		case ngap.ProcDownlinkUEAssociatedNRPPaTransport:
			f.nrppaTransportCalls++
		}
	}

	return len(b), nil
}

type fakeDBInstance struct {
	operator *db.Operator
}

func (f *fakeDBInstance) GetOperator(context.Context) (*db.Operator, error) {
	return f.operator, nil
}

func (f *fakeDBInstance) GetSubscriber(context.Context, string) (*db.Subscriber, error) {
	return nil, nil
}

func (f *fakeDBInstance) GetDataNetworkByID(context.Context, string) (*db.DataNetwork, error) {
	return nil, nil
}

func (f *fakeDBInstance) GetNetworkSliceByID(context.Context, string) (*db.NetworkSlice, error) {
	return nil, nil
}

func (f *fakeDBInstance) ListNetworkSlicesByIDs(context.Context, []string) ([]db.NetworkSlice, error) {
	return nil, nil
}

func (f *fakeDBInstance) GetProfileByID(context.Context, string) (*db.Profile, error) {
	return nil, nil
}

func (f *fakeDBInstance) ListAllNetworkSlices(context.Context) ([]db.NetworkSlice, error) {
	return nil, nil
}

func (f *fakeDBInstance) GetPolicyByProfileAndSlice(context.Context, string, string) (*db.Policy, error) {
	return nil, nil
}

func (f *fakeDBInstance) ListPoliciesByProfile(context.Context, string) ([]db.Policy, error) {
	return nil, nil
}

func (f *fakeDBInstance) NodeID() int { return 0 }

type fakeSmf struct {
	mu              sync.Mutex
	DeactivateCalls []string
}

func (f *fakeSmf) deactivated() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.DeactivateCalls...)
}

func (f *fakeSmf) GetSession(string) *smf.SMContext      { return nil }
func (f *fakeSmf) SessionsByDNN(string) []*smf.SMContext { return nil }
func (f *fakeSmf) SessionCount() int                     { return 0 }
func (f *fakeSmf) CreateSmContext(context.Context, etsi.SUPI, uint8, string, *models.Snssai, fgs.RequestType, []byte, uint8) (string, []byte, error) {
	return "", nil, nil
}
func (f *fakeSmf) ActivateSmContext(context.Context, string) ([]byte, error) { return nil, nil }
func (f *fakeSmf) DeactivateSmContext(_ context.Context, ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.DeactivateCalls = append(f.DeactivateCalls, ref)

	return nil
}
func (f *fakeSmf) HandlePagingFailure(context.Context, etsi.SUPI, uint8) error { return nil }

func (f *fakeSmf) ClearPagingSuppression(context.Context, etsi.SUPI, uint8) error { return nil }
func (f *fakeSmf) ReleaseSmContext(context.Context, string) error                 { return nil }
func (f *fakeSmf) UpdateSmContextN1Msg(context.Context, string, []byte) (*smf.UpdateResult, error) {
	return nil, nil
}

func (f *fakeSmf) UpdateSmContextN2InfoPduResSetupRsp(context.Context, string, []byte) error {
	return nil
}

func (f *fakeSmf) UpdateSmContextN2InfoPduResSetupFail(context.Context, string, []byte) error {
	return nil
}

func (f *fakeSmf) UpdateSmContextN2InfoPduResRelRsp(context.Context, string) (bool, error) {
	return false, nil
}

func (f *fakeSmf) UpdateSmContextCauseDuplicatePDUSessionID(context.Context, string) ([]byte, error) {
	return nil, nil
}

func (f *fakeSmf) TransferIdleTo5GS(context.Context, etsi.SUPI, uint8, uint8, string, *models.Snssai) (string, error) {
	return "", nil
}

func (f *fakeSmf) PrepareSmContextFromEPS(context.Context, etsi.SUPI, uint8, uint8, string, *models.Snssai) (string, []byte, error) {
	return "", nil, nil
}

func (f *fakeSmf) UpdateSmContextN2HandoverPreparing(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}

func (f *fakeSmf) UpdateSmContextN2HandoverPrepared(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}

func (f *fakeSmf) UpdateSmContextN2HandoverComplete(context.Context, string) error { return nil }

func (f *fakeSmf) UpdateSmContextN2HandoverCanceled(context.Context, string) error { return nil }

func (f *fakeSmf) UpdateSmContextXnHandoverPathSwitchReq(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}

func (f *fakeSmf) UpdateSmContextN2ModifyIndication(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}

func (f *fakeSmf) UpdateSmContextXnHandoverFailed(context.Context, string, []byte) error { return nil }

func (f *fakeSmf) UpdateSmContextN2HandoverFailed(context.Context, string, []byte) error { return nil }

func (f *fakeSmf) ReconcileSmContext(context.Context, *models.SessionReconcileRequest) error {
	return nil
}

func (f *fakeSmf) GetSessionPolicy(context.Context, etsi.SUPI, *models.Snssai, string) (*smf.Policy, error) {
	return nil, nil
}

func mustSUPIFromIMSI(t *testing.T, imsi string) etsi.SUPI {
	t.Helper()

	s, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		t.Fatalf("bad IMSI: %v", err)
	}

	return s
}

func addUE(t *testing.T, amfInstance *amf.AMF, imsi string, setup func(*amf.UeContext)) *amf.UeContext {
	t.Helper()
	supi := mustSUPIFromIMSI(t, imsi)
	ue := amf.NewUeContext()
	ue.SetSupiForTest(supi)

	if setup != nil {
		setup(ue)
	}

	if err := amfInstance.CommitUEIdentity(context.Background(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	return ue
}

func testGUTI(t *testing.T) etsi.GUTI5G {
	t.Helper()

	tmsi, err := etsi.NewTMSI(0x01020304)
	if err != nil {
		t.Fatalf("build TMSI: %v", err)
	}

	guti, err := etsi.NewGUTI5G("001", "01", "cafe00", tmsi)
	if err != nil {
		t.Fatalf("build GUTI: %v", err)
	}

	return guti
}

func newReq() models.N1N2MessageTransferRequest {
	return models.N1N2MessageTransferRequest{
		PduSessionID:            1,
		SNssai:                  &models.Snssai{Sst: 1, Sd: "010203"},
		BinaryDataN1Message:     []byte{0x01, 0x02},
		BinaryDataN2Information: []byte{0x03, 0x04},
	}
}

func TestTransferN1N2Message_UENotFound(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	supi := mustSUPIFromIMSI(t, "001010000000001")

	err := amfInstance.TransferN1N2Message(context.Background(), supi, newReq())
	if err == nil {
		t.Fatal("expected error for missing UE")
	}
}

func TestTransferN1N2Message_UENotConnected(t *testing.T) {
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000002", nil)

	err := amfInstance.TransferN1N2Message(context.Background(), ue.SupiForTest(), newReq())
	if err == nil {
		t.Fatal("expected error for UE not connected to RAN")
	}
}

func TestTransferN1N2Message_InitialContextAlreadySent(t *testing.T) {
	sender := &fakeNGAPSender{}
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000003", func(u *amf.UeContext) {
		u.Ambr = &models.Ambr{Uplink: models.MustParseBitRate("1000000 bps"), Downlink: models.MustParseBitRate("1000000 bps")}
	})

	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.MarkICSPending()
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	err := amfInstance.TransferN1N2Message(context.Background(), ue.SupiForTest(), newReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.pduSessionSetupCalls != 1 {
		t.Fatalf("expected 1 PDUSessionResourceSetupRequest, got %d", sender.pduSessionSetupCalls)
	}
}

func TestTransferN1N2Message_InitialContextNotYetSent(t *testing.T) {
	sender := &fakeNGAPSender{}
	fakeDB := &fakeDBInstance{
		operator: &db.Operator{
			Mcc: "001",
			Mnc: "01",
		},
	}
	amfInstance := amf.New(fakeDB, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000004", func(u *amf.UeContext) {
		u.Ambr = &models.Ambr{Uplink: models.MustParseBitRate("1000000 bps"), Downlink: models.MustParseBitRate("1000000 bps")}
		u.AllowedNssai = []models.Snssai{{Sst: 1, Sd: "010203"}}
		u.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}

		u.SetUESecurityCapabilityForTest(&fgs.UESecurityCapability{EA: 0x00, IA: 0x00})
		u.SetKgnbForTest(make([]byte, 32))
	})

	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.ResetICS()
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	err := amfInstance.TransferN1N2Message(context.Background(), ue.SupiForTest(), newReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.initialContextSetupCalls != 1 {
		t.Fatalf("expected 1 InitialContextSetupRequest, got %d", sender.initialContextSetupCalls)
	}

	if ueConn.ICS() != amf.ICSPending {
		t.Fatalf("expected ueConn.ICS == ICSPending, got %v", ueConn.ICS())
	}
}

func TestModifyN1N2Message_IdleRegisteredUE_ReturnsNotReachable(t *testing.T) {
	sender := &fakeNGAPSender{}
	amfInstance := amf.New(nil, nil, &fakeSmf{})
	amfInstance.ClearRadiosForTest()

	ue := addUE(t, amfInstance, "001010000000014", func(u *amf.UeContext) {
		u.ForceStateForTest(amf.Registered)
		u.SetGutiForTest(testGUTI(t))
		u.RegistrationArea = []models.Tai{{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"}}
	})

	radio := &amf.Radio{
		Conn: sender,
	}
	radio.BindAMFForTest(amfInstance)
	amfInstance.UpdateRadioSupportedTAIs(radio, []amf.SupportedTAI{{
		Tai: models.Tai{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"},
	}})
	amfInstance.SetRadioForTest(nil, radio)

	err := amfInstance.ModifyN1N2Message(context.Background(), ue.SupiForTest(), 1, []byte{0x01, 0x02}, []byte{0x03, 0x04})
	if err == nil {
		t.Fatal("expected ErrUENotReachable for idle UE")
	}

	if err != amf.ErrUENotReachable {
		t.Fatalf("expected ErrUENotReachable, got: %v", err)
	}

	if sender.pagingCalls != 0 {
		t.Fatalf("expected 0 paging calls, got %d", sender.pagingCalls)
	}

	if ue.N1N2Message() != nil {
		t.Fatal("expected no stored N1N2 message")
	}
}

func TestModifyN1N2Message_OngoingN2Handover_Deferred(t *testing.T) {
	sender := &fakeNGAPSender{}
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000016", func(u *amf.UeContext) {
		u.ForceStateForTest(amf.Registered)
	})

	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	if err := ue.Procedures().Begin(procedure.N2Handover); err != nil {
		t.Fatal(err)
	}

	err := amfInstance.ModifyN1N2Message(context.Background(), ue.SupiForTest(), 1, []byte{0x01, 0x02}, []byte{0x03, 0x04})
	if err == nil {
		t.Fatal("expected a temporary reject while an N2 handover is in flight")
	}

	if err == amf.ErrUENotReachable {
		t.Fatalf("expected the handover guard, got the idle path: %v", err)
	}

	if sender.downlinkNasTransportCalls != 0 || sender.pduSessionSetupCalls != 0 {
		t.Fatalf("expected no NGAP send while deferring, got dlnas=%d setup=%d",
			sender.downlinkNasTransportCalls, sender.pduSessionSetupCalls)
	}
}

func TestReleaseSessionMessage_IdleUE_ReturnsNotReachable(t *testing.T) {
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	addUE(t, amfInstance, "001010000000015", func(u *amf.UeContext) {
		u.ForceStateForTest(amf.Registered)
	})

	supi := mustSUPIFromIMSI(t, "001010000000015")

	err := amfInstance.ReleaseSessionMessage(context.Background(), supi, 1, []byte{0x01}, []byte{0x02})
	if err == nil {
		t.Fatal("expected ErrUENotReachable for idle UE")
	}

	if err != amf.ErrUENotReachable {
		t.Fatalf("expected ErrUENotReachable, got: %v", err)
	}
}

func TestN2MessageTransferOrPage_UENotFound(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	supi := mustSUPIFromIMSI(t, "001010000000005")

	err := amfInstance.N2MessageTransferOrPage(context.Background(), supi, newReq())
	if err == nil {
		t.Fatal("expected error for missing UE")
	}
}

// TS 24.501 §5.4.3
func TestIdleTimers_ArmedAndStoppedUnderRegistryLock(t *testing.T) {
	amfInstance := amf.New(nil, nil, &fakeSmf{})
	ue := addUE(t, amfInstance, "001010000000031", nil)

	amfInstance.StartMobileReachable(ue)

	if !ue.MobileReachableActiveForTest() {
		t.Fatal("StartMobileReachable must arm the mobile reachable timer")
	}

	amfInstance.DeregisterAndRemoveUeContext(context.Background(), ue)

	if ue.MobileReachableActiveForTest() {
		t.Fatal("UE teardown must cancel the mobile reachable timer")
	}
}

// TS 24.501 §5.5.1.3.4
func TestArmRegistrationAcceptGuard_ArmsT3550(t *testing.T) {
	amfInstance := amf.New(nil, nil, &fakeSmf{})
	ue := addUE(t, amfInstance, "001010000000030", nil)

	sender := &fakeNGAPSender{}
	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	amf.ArmRegistrationAcceptGuard(amfInstance, ue, []byte{0x7e, 0x00, 0x42})

	if !ue.Conn().NASGuardForTest().Active() {
		t.Fatal("ArmRegistrationAcceptGuard must arm T3550 for a GUTI-bearing accept")
	}

	ue.Conn().NASGuardForTest().Stop()
}

func TestSendPaging_IdleUE_ArmsPersistentTimer(t *testing.T) {
	amfInstance := amf.New(nil, nil, &fakeSmf{})
	ue := addUE(t, amfInstance, "001010000000021", nil)

	if conn := ue.Conn(); conn != nil {
		conn.Release()
	}

	if ue.Conn() != nil {
		t.Fatal("precondition: idle UE must have no NAS connection")
	}

	if err := amfInstance.SendPaging(context.Background(), ue, []byte{0x00}); err != nil {
		t.Fatalf("SendPaging: %v", err)
	}

	if !ue.PagingActiveForTest() {
		t.Fatal("SendPaging must arm the persistent per-UE paging timer")
	}

	ue.StopPaging()
}

func TestN2MessageTransferOrPage_OnGoingPaging(t *testing.T) {
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000006", nil)

	ue.ArmPagingForTest(1*time.Hour, 3)

	err := amfInstance.N2MessageTransferOrPage(context.Background(), ue.SupiForTest(), newReq())
	if err == nil {
		t.Fatal("expected error for ongoing paging")
	}
}

func TestN2MessageTransferOrPage_OnGoingRegistration(t *testing.T) {
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000007", nil)

	ue.ForceStateForTest(amf.RegistrationInitiated)

	err := amfInstance.N2MessageTransferOrPage(context.Background(), ue.SupiForTest(), newReq())
	if err == nil {
		t.Fatal("expected error for ongoing registration")
	}
}

func TestN2MessageTransferOrPage_OnGoingN2Handover(t *testing.T) {
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000008", nil)

	if err := ue.Procedures().Begin(procedure.N2Handover); err != nil {
		t.Fatal(err)
	}

	err := amfInstance.N2MessageTransferOrPage(context.Background(), ue.SupiForTest(), newReq())
	if err == nil {
		t.Fatal("expected error for ongoing N2 handover")
	}
}

func TestN2MessageTransferOrPage_ConnectedUE_InitialCtxSent(t *testing.T) {
	sender := &fakeNGAPSender{}
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000009", func(u *amf.UeContext) {
		u.Ambr = &models.Ambr{Uplink: models.MustParseBitRate("1000000 bps"), Downlink: models.MustParseBitRate("1000000 bps")}
	})

	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.MarkICSPending()
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	err := amfInstance.N2MessageTransferOrPage(context.Background(), ue.SupiForTest(), newReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.pduSessionSetupCalls != 1 {
		t.Fatalf("expected 1 PDUSessionResourceSetupRequest, got %d", sender.pduSessionSetupCalls)
	}
}

func TestN2MessageTransferOrPage_IdleRegisteredUE_Pages(t *testing.T) {
	sender := &fakeNGAPSender{}
	fakeDB := &fakeDBInstance{operator: &db.Operator{Mcc: "001", Mnc: "01"}}
	amfInstance := amf.New(fakeDB, nil, &fakeSmf{})
	amfInstance.ClearRadiosForTest()

	ue := addUE(t, amfInstance, "001010000000030", func(u *amf.UeContext) {
		u.ForceStateForTest(amf.Registered)
		u.SetGutiForTest(testGUTI(t))
		u.RegistrationArea = []models.Tai{{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"}}
	})

	if conn := ue.Conn(); conn != nil {
		conn.Release()
	}

	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	amfInstance.UpdateRadioSupportedTAIs(radio, []amf.SupportedTAI{{
		Tai: models.Tai{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"},
	}})

	req := newReq()

	err := amfInstance.N2MessageTransferOrPage(context.Background(), ue.SupiForTest(), req)
	if err != nil {
		t.Fatalf("expected idle registered UE to be paged, got error: %v", err)
	}

	if sender.pagingCalls != 1 {
		t.Fatalf("expected 1 paging call, got %d", sender.pagingCalls)
	}

	if !ue.PagingActiveForTest() {
		t.Fatal("expected the persistent per-UE paging timer to be armed")
	}

	buffered := ue.N1N2Message()
	if buffered == nil {
		t.Fatal("expected the N1N2 message to be buffered on the persistent UE context")
	}

	if buffered.PduSessionID != req.PduSessionID {
		t.Fatalf("buffered PDU session id: expected %d, got %d", req.PduSessionID, buffered.PduSessionID)
	}

	ue.StopPaging()
}

func TestN2MessageTransferOrPage_NotRegistered_NoPaging(t *testing.T) {
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000010", nil)

	err := amfInstance.N2MessageTransferOrPage(context.Background(), ue.SupiForTest(), newReq())
	if err == nil {
		t.Fatal("expected error for UE not in registered state")
	}
}

func TestTransferN1Msg_UENotFound(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	supi := mustSUPIFromIMSI(t, "001010000000011")

	err := amfInstance.TransferN1Msg(context.Background(), supi, []byte{0x01}, 1)
	if err == nil {
		t.Fatal("expected error for missing UE")
	}
}

func TestTransferN1Msg_UENotConnected(t *testing.T) {
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000012", nil)

	err := amfInstance.TransferN1Msg(context.Background(), ue.SupiForTest(), []byte{0x01}, 1)
	if err == nil {
		t.Fatal("expected error for UE not connected to RAN")
	}
}

func TestTransferN1Msg_Success(t *testing.T) {
	sender := &fakeNGAPSender{}
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000013", nil)

	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	err := amfInstance.TransferN1Msg(context.Background(), ue.SupiForTest(), []byte{0x01}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.downlinkNasTransportCalls != 1 {
		t.Fatalf("expected 1 DownlinkNasTransport, got %d", sender.downlinkNasTransportCalls)
	}
}

// TS 38.413 §8.3.1
func TestN2MessageTransferOrPage_SetupItemFailureReleasesICSClaim(t *testing.T) {
	sender := &fakeNGAPSender{}
	fakeDB := &fakeDBInstance{operator: &db.Operator{Mcc: "001", Mnc: "01"}}
	amfInstance := amf.New(fakeDB, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000021", func(u *amf.UeContext) {
		u.Ambr = &models.Ambr{Uplink: models.MustParseBitRate("1000000 bps"), Downlink: models.MustParseBitRate("1000000 bps")}
	})

	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	req := newReq()
	req.SNssai = nil

	if err := amfInstance.N2MessageTransferOrPage(context.Background(), ue.SupiForTest(), req); err == nil {
		t.Fatal("expected an error building the PDU session setup item")
	}

	if got := ueConn.ICS(); got != amf.ICSNotStarted {
		t.Fatalf("ICS = %v, want %v: the claim was not released", got, amf.ICSNotStarted)
	}
}

// TS 23.501 §5.17.2.2.1
func TestRegistrationAcceptGuardExpiryDropsTheEPSRegistration(t *testing.T) {
	amfInstance := amf.New(nil, nil, &fakeSmf{})
	amfInstance.NASGuardCfg = guard.TimerValue{Enable: true, ExpireTime: time.Millisecond, MaxRetryTimes: 1}

	peer := &cancelCountingEPSPeer{}
	amfInstance.EPS = peer

	ue := addUE(t, amfInstance, "001010000000031", nil)

	sender := &fakeNGAPSender{}
	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	ue.TransitionTo(amf.RegistrationInitiated)

	amf.ArmRegistrationAcceptGuard(amfInstance, ue, []byte{0x7e, 0x00, 0x42})

	deadline := time.Now().Add(2 * time.Second)
	for peer.cancels.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("EPS registrations cancelled = 0, want 1; UE state = %s", ue.State())
		}

		time.Sleep(time.Millisecond)
	}

	if got := peer.cancels.Load(); got != 1 {
		t.Errorf("EPS registrations cancelled = %d, want 1", got)
	}

	if got := ue.State(); got != amf.Registered {
		t.Errorf("UE state = %s, want %s", got, amf.Registered)
	}
}

type cancelCountingEPSPeer struct {
	cancels atomic.Int64
}

func (p *cancelCountingEPSPeer) CancelRegistration(context.Context, etsi.SUPI) {
	p.cancels.Add(1)
}

func (*cancelCountingEPSPeer) ForwardRelocation(context.Context, interworking.ForwardRelocationRequest) (interworking.ForwardRelocationResponse, error) {
	return interworking.ForwardRelocationResponse{}, nil
}

func (*cancelCountingEPSPeer) RelocationCancel(context.Context, etsi.SUPI, interworking.RelocationID) error {
	return nil
}

func (*cancelCountingEPSPeer) RelocationComplete(context.Context, etsi.SUPI, interworking.RelocationID) error {
	return nil
}

func (*cancelCountingEPSPeer) MMContext(context.Context, interworking.MMContextRequest) (interworking.MMContextResponse, error) {
	return interworking.MMContextResponse{}, nil
}

func (*cancelCountingEPSPeer) MMContextAck(context.Context, etsi.SUPI, []uint8) error {
	return nil
}

func TestN2MessageTransferOrPage_DoesNotResetupASessionAlreadyInFlight(t *testing.T) {
	sender := &fakeNGAPSender{}
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000021", func(u *amf.UeContext) {
		u.Ambr = &models.Ambr{Uplink: models.MustParseBitRate("1000000 bps"), Downlink: models.MustParseBitRate("1000000 bps")}
	})

	if err := ue.CreateSmContext(1, "ref-1", &models.Snssai{Sst: 1}, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.MarkICSPending()
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	if err := amfInstance.N2MessageTransferOrPage(context.Background(), ue.SupiForTest(), newReq()); err != nil {
		t.Fatalf("first transfer: %v", err)
	}

	if sender.pduSessionSetupCalls != 1 {
		t.Fatalf("PDUSessionResourceSetupRequest count = %d, want 1", sender.pduSessionSetupCalls)
	}

	if err := amfInstance.N2MessageTransferOrPage(context.Background(), ue.SupiForTest(), newReq()); err != nil {
		t.Fatalf("second transfer: %v", err)
	}

	if sender.pduSessionSetupCalls != 1 {
		t.Fatalf("PDUSessionResourceSetupRequest count = %d after a repeat N2 transfer, want 1", sender.pduSessionSetupCalls)
	}
}

func TestReleaseNasConnectionClearsOutstandingSetups(t *testing.T) {
	sender := &fakeNGAPSender{}
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000022", func(u *amf.UeContext) {
		u.Ambr = &models.Ambr{Uplink: models.MustParseBitRate("1000000 bps"), Downlink: models.MustParseBitRate("1000000 bps")}
	})

	if err := ue.CreateSmContext(1, "ref-1", &models.Snssai{Sst: 1}, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if !ue.ClaimSmContextN2(1) {
		t.Fatal("the first claim must succeed")
	}

	if ue.ClaimSmContextN2(1) {
		t.Fatal("a second claim must fail while the setup is outstanding")
	}

	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	amfInstance.ReleaseNasConnection(ue, ueConn)

	if !ue.ClaimSmContextN2(1) {
		t.Fatal("the UE dropping to CM-IDLE must clear the outstanding setup")
	}
}

func TestSmContextN2StateTransitions(t *testing.T) {
	amfInstance := amf.New(nil, nil, &fakeSmf{})
	ue := addUE(t, amfInstance, "001010000000023", nil)

	if err := ue.CreateSmContext(1, "ref-1", &models.Snssai{Sst: 1}, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	sc, ok := ue.SmContextFindByPDUSessionID(1)
	if !ok {
		t.Fatal("SM context not found")
	}

	if !sc.Inactive() {
		t.Error("a session the RAN has not set up must start inactive")
	}

	if ue.HasActivePduSessions() {
		t.Error("a session with no RAN resources must not count as active")
	}

	if !ue.ClaimSmContextN2(1) {
		t.Fatal("an inactive session must be claimable")
	}

	if ue.ClaimSmContextN2(1) {
		t.Error("a pending session must not be claimable")
	}

	ue.SetSmContextActive(1)

	if ue.ClaimSmContextN2(1) {
		t.Error("an active session must not be claimable")
	}

	if !ue.HasActivePduSessions() {
		t.Error("a session the RAN confirmed must count as active")
	}

	ue.ReleaseSmContextN2(1)

	if !ue.ClaimSmContextN2(1) {
		t.Error("a released session must be claimable again")
	}
}

func TestN2SetupTransaction_GuardExpiryReleasesTheClaim(t *testing.T) {
	sender := &fakeNGAPSender{}
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000024", nil)

	if err := ue.CreateSmContext(1, "ref-1", &models.Snssai{Sst: 1}, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	if !ueConn.ClaimN2SetupSession(amf.N2SetupPDUSession, 1) {
		t.Fatal("could not claim the PDU session")
	}

	ueConn.ArmN2Setup(amf.N2SetupPDUSession, guard.TimerValue{Enable: true, ExpireTime: 10 * time.Millisecond})

	deadline := time.Now().Add(2 * time.Second)
	for ueConn.N2SetupOpen(amf.N2SetupPDUSession) && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	if ueConn.N2SetupOpen(amf.N2SetupPDUSession) {
		t.Fatal("the transaction is still open after the guard timer expired")
	}

	if !ueConn.ClaimN2SetupSession(amf.N2SetupPDUSession, 1) {
		t.Error("an NG-RAN node that never answers must not hold the claim forever")
	}
}

func TestN2SetupTransaction_TerminalKeepsConfirmedSessions(t *testing.T) {
	sender := &fakeNGAPSender{}
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000025", nil)

	if err := ue.CreateSmContext(1, "ref-1", &models.Snssai{Sst: 1}, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	if !ueConn.ClaimN2SetupSession(amf.N2SetupPDUSession, 1) {
		t.Fatal("could not claim the PDU session")
	}

	ue.SetSmContextActive(1)
	ueConn.EndN2Setup(amf.N2SetupPDUSession)

	if ue.ClaimSmContextN2(1) {
		t.Error("a session the NG-RAN node confirmed must stay active when its transaction closes")
	}
}

func TestTransferN1N2Message_SessionAlreadySetUp_ReleasesTheICSClaim(t *testing.T) {
	sender := &fakeNGAPSender{}
	fakeDB := &fakeDBInstance{operator: &db.Operator{Mcc: "001", Mnc: "01"}}
	amfInstance := amf.New(fakeDB, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000026", func(u *amf.UeContext) {
		u.Ambr = &models.Ambr{Uplink: models.MustParseBitRate("1000000 bps"), Downlink: models.MustParseBitRate("1000000 bps")}
		u.AllowedNssai = []models.Snssai{{Sst: 1, Sd: "010203"}}
		u.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}

		u.SetUESecurityCapabilityForTest(&fgs.UESecurityCapability{EA: 0x00, IA: 0x00})
		u.SetKgnbForTest(make([]byte, 32))
	})

	if err := ue.CreateSmContext(1, "ref-1", &models.Snssai{Sst: 1}, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	ue.SetSmContextActive(1)

	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.ResetICS()
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	if err := amfInstance.TransferN1N2Message(context.Background(), ue.SupiForTest(), newReq()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.initialContextSetupCalls != 0 {
		t.Fatalf("InitialContextSetupRequest count = %d, want 0", sender.initialContextSetupCalls)
	}

	if !ueConn.ClaimICS() {
		t.Error("the initial context setup claim was not released, so the UE context can never be set up on this connection")
	}
}

func TestStoreN1N2AndPage_RejectsASecondTransferWhilePaging(t *testing.T) {
	sender := &fakeNGAPSender{}
	fakeDB := &fakeDBInstance{operator: &db.Operator{Mcc: "001", Mnc: "01"}}
	amfInstance := amf.New(fakeDB, nil, &fakeSmf{})
	amfInstance.ClearRadiosForTest()

	ue := addUE(t, amfInstance, "001010000000027", func(u *amf.UeContext) {
		u.ForceStateForTest(amf.Registered)
		u.SetGutiForTest(testGUTI(t))
		u.RegistrationArea = []models.Tai{{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"}}
	})

	if conn := ue.Conn(); conn != nil {
		conn.Release()
	}

	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	amfInstance.UpdateRadioSupportedTAIs(radio, []amf.SupportedTAI{{
		Tai: models.Tai{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"},
	}})

	first := newReq()
	if err := amfInstance.N2MessageTransferOrPage(context.Background(), ue.SupiForTest(), first); err != nil {
		t.Fatalf("first transfer: %v", err)
	}

	if !ue.PagingActiveForTest() {
		t.Fatal("expected paging supervision to be running")
	}

	buffered := ue.N1N2Message()
	if buffered == nil {
		t.Fatal("the first transfer was not buffered")
	}

	second := newReq()
	second.BinaryDataN2Information = []byte{0xAA, 0xBB}

	err := amfInstance.N2MessageTransferOrPage(context.Background(), ue.SupiForTest(), second)
	if err == nil {
		t.Fatal("a second transfer while paging must be rejected (TS 23.502 4.2.3.3 step 3b)")
	}

	if got := ue.N1N2Message(); got == nil || !bytes.Equal(got.BinaryDataN2Information, buffered.BinaryDataN2Information) {
		t.Error("the buffered request was displaced; the SMF will never resend the 5GSM message it carried")
	}
}

func TestN2SetupGuardExpiry_DeactivatesTheSessionAtTheSMF(t *testing.T) {
	sender := &fakeNGAPSender{}
	smfStub := &fakeSmf{}
	amfInstance := amf.New(nil, nil, smfStub)

	ue := addUE(t, amfInstance, "001010000000028", nil)

	if err := ue.CreateSmContext(1, "ref-1", &models.Snssai{Sst: 1}, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	if !ueConn.ClaimN2SetupSession(amf.N2SetupPDUSession, 1) {
		t.Fatal("could not claim the PDU session")
	}

	ueConn.ArmN2Setup(amf.N2SetupPDUSession, guard.TimerValue{Enable: true, ExpireTime: 10 * time.Millisecond})

	deadline := time.Now().Add(2 * time.Second)
	for len(smfStub.deactivated()) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	got := smfStub.deactivated()
	if len(got) != 1 || got[0] != "ref-1" {
		t.Fatalf("DeactivateSmContext calls = %v, want [ref-1]: an unanswered setup leaves the SMF activating (TS 23.502 4.2.3.2 steps 12/16)", got)
	}
}

func TestN2SetupTerminal_DoesNotDeactivateAConfirmedSession(t *testing.T) {
	sender := &fakeNGAPSender{}
	smfStub := &fakeSmf{}
	amfInstance := amf.New(nil, nil, smfStub)

	ue := addUE(t, amfInstance, "001010000000029", nil)

	if err := ue.CreateSmContext(1, "ref-1", &models.Snssai{Sst: 1}, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	if !ueConn.ClaimN2SetupSession(amf.N2SetupPDUSession, 1) {
		t.Fatal("could not claim the PDU session")
	}

	ue.SetSmContextActive(1)
	ueConn.EndN2Setup(amf.N2SetupPDUSession)

	if got := smfStub.deactivated(); len(got) != 0 {
		t.Errorf("DeactivateSmContext calls = %v, want none: the NG-RAN node confirmed the session", got)
	}
}
