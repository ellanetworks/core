// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// --- Fakes ---

type fakeNGAPSender struct {
	pduSessionSetupCalls      int
	initialContextSetupCalls  int
	downlinkNasTransportCalls int
	pagingCalls               int
}

// WriteMsg counts the sent NGAP PDU by procedure, standing in for a gNB
// association. The NGAP-PDU APER header is byte 0 = outcome choice (0x00 is an
// InitiatingMessage) and byte 1 = procedure code (TS 38.413); the transparent N2
// payloads carried here are opaque, so the message is identified from the header
// without a full decode.
func (f *fakeNGAPSender) WriteMsg(b []byte, _ *sctp.SndRcvInfo) (int, error) {
	if len(b) >= 2 && b[0] == 0x00 {
		switch int64(b[1]) {
		case ngapType.ProcedureCodePaging:
			f.pagingCalls++
		case ngapType.ProcedureCodePDUSessionResourceSetup:
			f.pduSessionSetupCalls++
		case ngapType.ProcedureCodeInitialContextSetup:
			f.initialContextSetupCalls++
		case ngapType.ProcedureCodeDownlinkNASTransport:
			f.downlinkNasTransportCalls++
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

type fakeSmf struct{}

func (f *fakeSmf) GetSession(string) *smf.SMContext      { return nil }
func (f *fakeSmf) SessionsByDNN(string) []*smf.SMContext { return nil }
func (f *fakeSmf) SessionCount() int                     { return 0 }
func (f *fakeSmf) CreateSmContext(context.Context, etsi.SUPI, uint8, string, *models.Snssai, []byte) (string, []byte, error) {
	return "", nil, nil
}
func (f *fakeSmf) ActivateSmContext(context.Context, string) ([]byte, error) { return nil, nil }
func (f *fakeSmf) DeactivateSmContext(context.Context, string) error         { return nil }
func (f *fakeSmf) ReleaseSmContext(context.Context, string) error            { return nil }
func (f *fakeSmf) UpdateSmContextN1Msg(context.Context, string, []byte) (*smf.UpdateResult, error) {
	return nil, nil
}

func (f *fakeSmf) UpdateSmContextN2InfoPduResSetupRsp(context.Context, string, []byte) error {
	return nil
}

func (f *fakeSmf) UpdateSmContextN2InfoPduResSetupFail(context.Context, string, []byte) error {
	return nil
}
func (f *fakeSmf) UpdateSmContextN2InfoPduResRelRsp(context.Context, string) error { return nil }
func (f *fakeSmf) UpdateSmContextCauseDuplicatePDUSessionID(context.Context, string) ([]byte, error) {
	return nil, nil
}

func (f *fakeSmf) UpdateSmContextN2HandoverPreparing(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}

func (f *fakeSmf) UpdateSmContextN2HandoverPrepared(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}

func (f *fakeSmf) UpdateSmContextN2HandoverComplete(context.Context, string) error { return nil }

func (f *fakeSmf) UpdateSmContextXnHandoverPathSwitchReq(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}

func (f *fakeSmf) UpdateSmContextN2ModifyIndication(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}
func (f *fakeSmf) UpdateSmContextHandoverFailed(context.Context, string, []byte) error { return nil }

func (f *fakeSmf) ReconcileSmContext(context.Context, *models.SessionReconcileRequest) error {
	return nil
}

func (f *fakeSmf) GetSessionPolicy(context.Context, etsi.SUPI, *models.Snssai, string) (*smf.Policy, error) {
	return nil, nil
}

// --- Helpers ---

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

// --- TransferN1N2Message tests ---

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
		u.Ambr = &models.Ambr{Uplink: "1000000 bps", Downlink: "1000000 bps"}
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
		u.Ambr = &models.Ambr{Uplink: "1000000 bps", Downlink: "1000000 bps"}
		u.AllowedNssai = []models.Snssai{{Sst: 1, Sd: "010203"}}
		u.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}

		secCap := &nasType.UESecurityCapability{}
		secCap.SetLen(2)
		u.SetUESecurityCapabilityForTest(secCap)
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

	// UE has no UeConn attached → CM-IDLE
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

	// Must be the handover guard, not the idle path — the modification is deferred
	// for the reconcile backstop, not treated as unreachable.
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

	// UE has no UeConn attached → CM-IDLE
	err := amfInstance.ReleaseSessionMessage(context.Background(), supi, 1, []byte{0x01}, []byte{0x02})
	if err == nil {
		t.Fatal("expected ErrUENotReachable for idle UE")
	}

	if err != amf.ErrUENotReachable {
		t.Fatalf("expected ErrUENotReachable, got: %v", err)
	}
}

// --- N2MessageTransferOrPage tests ---

func TestN2MessageTransferOrPage_UENotFound(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	supi := mustSUPIFromIMSI(t, "001010000000005")

	err := amfInstance.N2MessageTransferOrPage(context.Background(), supi, newReq())
	if err == nil {
		t.Fatal("expected error for missing UE")
	}
}

// TestSendPaging_IdleUE_ArmsPersistentTimer guards the paging-timer scope fix: an
// idle UE has no NAS connection, so paging must not touch a connection-scoped
// timer (previously a nil-connection crash) and must arm the persistent per-UE
// paging timer instead (T3513, TS 24.501 §5.4.3).
// TestIdleTimers_ArmedAndStoppedUnderRegistryLock verifies the AMF's idle-mode
// supervision timers are driven through the registry lock (`(a *AMF)` receivers,
// matching the MME): StartMobileReachable arms the timer and UE teardown
// cancels it (TS 24.501 §5.3.7).
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

// TestArmRegistrationAcceptGuard_ArmsT3550 verifies that a GUTI-bearing
// REGISTRATION ACCEPT delivered on a mobility/periodic update outside
// SendRegistrationAccept (embedded in a PDU Session Resource Setup, or a plain DL
// NAS Transport) is still supervised by T3550 (TS 24.501 §5.5.1.3.4).
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

	// Drop the NAS connection: paging targets an ECM-IDLE UE, which has none.
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
		u.Ambr = &models.Ambr{Uplink: "1000000 bps", Downlink: "1000000 bps"}
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

	// A registered UE in CM-IDLE has no NAS connection; downlink data must page it.
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

// --- TransferN1Msg tests ---

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
