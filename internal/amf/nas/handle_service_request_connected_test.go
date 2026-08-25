// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

type connectedModeFixture struct {
	amf        *amf.AMF
	ue         *amf.UeContext
	ngapSender *fakeNGAPSender
	key        [16]uint8
	algo       nas.CipheringAlgorithm
}

func (f connectedModeFixture) conn() *amf.UeConn { return f.ue.Conn() }

func (f connectedModeFixture) serviceRequest(t *testing.T, svcType fgs.ServiceType) []byte {
	t.Helper()

	m, err := buildTestServiceRequestCiphered(f.algo, f.key, f.ue.ULCount(), svcType)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	return encSR(t, m)
}

func connectedModeUe(t *testing.T, smf amf.SmfSbi) connectedModeFixture {
	t.Helper()

	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: `["000001"]`},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{Rand: hex.EncodeToString(make([]byte, 16)), Autn: hex.EncodeToString(make([]byte, 16))},
			Supi:    mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf:   []byte("testkey"),
		},
		smf,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	snssai := models.Snssai{Sst: 1, Sd: "102030"}

	ue.AllowedNssai = []models.Snssai{{Sst: 1, Sd: "010203"}}
	setTestUESecurityCapability(ue)

	ue.PlmnID = models.PlmnID{Mcc: "001", Mnc: "01"}
	ue.ForceStateForTest(amf.Registered)
	ue.SetGutiForTest(mustTestGuti("001", "01", "cafe42", 0x00000001))
	ue.Tai = ue.Conn().Tai
	ue.SetSecuredForTest(true)

	ng := ue.NgKsiForTest()
	ng.Ksi = 1
	ue.SetNgKsiForTest(ng)

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := nas.CipheringAES

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)
	ue.Ambr = &models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("100 Mbps")}

	if err := ue.CreateSmContext(12, "testrefuplink", &snssai, "internet"); err != nil {
		t.Fatalf("could not create sm context: %v", err)
	}

	ue.Conn().UeContextRequest = true
	ue.Conn().MarkSecureExchangeEstablished()

	return connectedModeFixture{amf: amfInstance, ue: ue, ngapSender: ngapSender, key: key, algo: algo}
}

func TestHandleServiceRequest_ContextAlreadySetUp_NoSecondInitialContextSetup(t *testing.T) {
	for _, tc := range []struct {
		name      string
		connState func(*amf.UeConn)
		wantICS   int
		wantSUReq int
	}{
		{
			name:      "5GMM-IDLE: the request that establishes the AS context",
			connState: func(*amf.UeConn) {},
			wantICS:   1,
			wantSUReq: 0,
		},
		{
			name:      "5GMM-CONNECTED: the AS context is already up",
			connState: func(c *amf.UeConn) { c.MarkICSCompleted() },
			wantICS:   0,
			wantSUReq: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := connectedModeUe(t, &fakeSmf{})

			tc.connState(f.conn())

			before := append([]byte(nil), f.ue.Kgnb()...)

			handleServiceRequest(t.Context(), f.amf, f.ue, f.serviceRequest(t, fgs.ServiceTypeData), true)

			if got := len(f.ngapSender.SentInitialContextSetupRequest); got != tc.wantICS {
				t.Fatalf("initial context setup requests = %d, want %d", got, tc.wantICS)
			}

			if got := len(f.ngapSender.SentPDUSessionResourceSetupRequest); got != tc.wantSUReq {
				t.Fatalf("pdu session resource setup requests = %d, want %d", got, tc.wantSUReq)
			}

			if rederived := !bytes.Equal(before, f.ue.Kgnb()); rederived != (tc.wantICS == 1) {
				t.Fatalf("KgNB re-derived = %v, want %v (TS 33.501 §6.8.1.2.2)", rederived, tc.wantICS == 1)
			}
		})
	}
}

func TestHandleServiceRequest_ProtocolErrorReject_ReleasesOnlyFrom5GMMIdle(t *testing.T) {
	const unassignedServiceType = fgs.ServiceType(0x07)

	for _, tc := range []struct {
		name        string
		inboundNAS  int
		wantRelease int
	}{
		{name: "started from 5GMM-IDLE", inboundNAS: 1, wantRelease: 1},
		{name: "started from 5GMM-CONNECTED", inboundNAS: 2, wantRelease: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := connectedModeUe(t, &fakeSmf{})

			for range tc.inboundNAS {
				f.conn().NoteInboundNAS()
			}

			handleServiceRequest(t.Context(), f.amf, f.ue, f.serviceRequest(t, unassignedServiceType), true)

			assertServiceReject(t, f, fgs.GMMCauseProtocolErrorUnspecified)

			if got := len(f.ngapSender.SentUEContextReleaseCommand); got != tc.wantRelease {
				t.Fatalf("UE context release commands = %d, want %d", got, tc.wantRelease)
			}
		})
	}
}

func TestHandleServiceRequest_Cause9Reject_ReleasesFrom5GMMConnected(t *testing.T) {
	f := connectedModeUe(t, &fakeSmf{})

	f.ue.ForceStateForTest(amf.Deregistered)
	f.conn().NoteInboundNAS()
	f.conn().NoteInboundNAS()

	handleServiceRequest(t.Context(), f.amf, f.ue, f.serviceRequest(t, fgs.ServiceTypeData), true)

	assertServiceReject(t, f, fgs.GMMCauseUEIdentityCannotBeDerived)

	if len(f.ngapSender.SentUEContextReleaseCommand) != 1 {
		t.Fatalf("UE context release commands = %d, want 1", len(f.ngapSender.SentUEContextReleaseCommand))
	}
}

func TestHandleServiceRequest_SignallingWithUplinkDataStatus_ReactivatesPDUSession(t *testing.T) {
	f := connectedModeUe(t, &fakeSmf{})

	f.conn().MarkICSCompleted()

	handleServiceRequest(t.Context(), f.amf, f.ue, f.serviceRequest(t, fgs.ServiceTypeSignalling), true)

	if len(f.ngapSender.SentPDUSessionResourceSetupRequest) != 1 {
		t.Fatalf("pdu session resource setup requests = %d, want 1", len(f.ngapSender.SentPDUSessionResourceSetupRequest))
	}

	setup := f.ngapSender.SentPDUSessionResourceSetupRequest[0]
	plain := decipherGmm(t, f.ue, *setup.NASPDU, uint8(fgs.MsgServiceAccept))

	accept, err := fgs.ParseServiceAccept(plain)
	if err != nil {
		t.Fatalf("could not parse service accept: %v", err)
	}

	if accept.PDUSessionReactivationResult == nil {
		t.Fatal("service accept carries no PDU session reactivation result IE (TS 24.501 §5.6.1.4.1)")
	}

	if psiSet(accept.PDUSessionReactivationResult, 12) {
		t.Fatal("PDU session 12 reported as failed; it was re-established")
	}

	if accept.PDUSessionStatus == nil {
		t.Fatal("service accept carries no PDU session status IE (TS 24.501 §5.6.1.4.1)")
	}
}

func TestHandleNAS_CountsTheConnectionsFirstMessageAsSentFrom5GMMIdle(t *testing.T) {
	f := connectedModeUe(t, &fakeSmf{})

	HandleNAS(t.Context(), f.amf, f.conn(), encSR(t, buildTestServiceRequest()))

	if !f.conn().SentFrom5GMMIdle() {
		t.Fatal("the connection's first NAS message must count as sent from 5GMM-IDLE mode")
	}

	HandleNAS(t.Context(), f.amf, f.conn(), encSR(t, buildTestServiceRequest()))

	if f.conn().SentFrom5GMMIdle() {
		t.Fatal("a later NAS message on the same connection was sent in 5GMM-CONNECTED mode")
	}
}

func assertServiceReject(t *testing.T, f connectedModeFixture, want fgs.GMMCause) {
	t.Helper()

	if len(f.ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("downlink NAS transports = %d, want 1 (SERVICE REJECT)", len(f.ngapSender.SentDownlinkNASTransport))
	}

	plain := decipherGmm(t, f.ue, f.ngapSender.SentDownlinkNASTransport[0].NASPDU, uint8(fgs.MsgServiceReject))

	reject, err := fgs.ParseServiceReject(plain)
	if err != nil {
		t.Fatalf("could not parse service reject: %v", err)
	}

	if reject.Cause != want {
		t.Fatalf("service reject cause = %v, want %v", reject.Cause, want)
	}
}
