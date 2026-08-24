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

// serviceRequest encodes a ciphered SERVICE REQUEST of svcType for this fixture's UE.
func (f connectedModeFixture) serviceRequest(t *testing.T, svcType fgs.ServiceType) []byte {
	t.Helper()

	m, err := buildTestServiceRequestCiphered(f.algo, f.key, f.ue.ULCount(), svcType)
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	return encSR(t, m)
}

// connectedModeUe builds a registered, secured UE whose connection carries the UE Context
// Request the gNB set on the INITIAL UE MESSAGE, with one PDU session (PSI 12 — the one
// buildTestServiceRequestCiphered names in its Uplink data status IE).
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
	// The SERVICE REQUEST was integrity checked before dispatch, which is what establishes
	// secure exchange on the connection (TS 24.501 §4.4.4.3).
	ue.Conn().MarkSecureExchangeEstablished()

	return connectedModeFixture{amf: amfInstance, ue: ue, ngapSender: ngapSender, key: key, algo: algo}
}

// The Initial Context Setup carries the KgNB derived from the uplink NAS COUNT of the NAS
// message that took the UE from CM-IDLE to CM-CONNECTED (TS 33.501 §6.8.1.2.2), and the AMF
// triggers it because the UE Context Request IE arrived on the INITIAL UE MESSAGE
// (TS 38.413 §8.6.1.2). A SERVICE REQUEST the UE sends later in 5GMM-CONNECTED mode
// (TS 24.501 §5.6.1.1 cases e) and j)) is not that message: it must neither repeat the
// procedure nor re-derive the key, and its user-plane resources go up in a standalone PDU
// Session Resource Setup instead.
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

// TS 24.501 §5.6.1.8 b): a SERVICE REJECT for a protocol error leaves the AMF in its current
// 5GMM mode. The UE starts T3540 to let the network release the N1 NAS signalling connection
// only when it sent the request from 5GMM-IDLE mode (§5.3.1.3 a1), §5.6.1.7 i)) — a UE that
// sent it in 5GMM-CONNECTED mode keeps its connection and its user plane.
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

// TS 24.501 §5.3.1.3 d): a SERVICE REJECT with 5GMM cause #9 has the UE start T3540 whatever
// mode it sent the request from, so the network releases the connection either way.
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

// TS 24.501 §5.6.1.4.1 conditions the AMF's user-plane re-establishment on the Uplink data
// status IE being present, not on the service type, and §5.6.1.2.1 case c) has a UE with an
// always-on PDU session include that IE alongside service type "signalling". Such a request
// must re-establish the named PDU session and report the result in the SERVICE ACCEPT.
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

// The N1 NAS signalling connection is established by the NGAP INITIAL UE MESSAGE that
// carries a connection's first uplink NAS message (TS 38.413 §8.6.1.1, TS 24.501 §5.3.1.1),
// so HandleNAS — the one entry point every uplink NAS PDU passes through — is where the AMF
// learns which 5GMM mode the UE sent each message from.
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
