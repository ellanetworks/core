// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/db"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

// TestSecurityMode_BlockedByConflict verifies the security mode procedure is
// claimed before the security context is mutated: while an N2 handover holds the
// key-changing mutual exclusion, the re-key is refused and no NAS keys are
// derived (TS 33.501 §6.9.5.1).
func TestSecurityMode_BlockedByConflict(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("build UE and radio: %v", err)
	}

	conn := ue.Conn()
	if conn == nil {
		t.Fatal("UE has no NAS connection")
	}

	// An in-flight N2 handover holds the key-changing mutual exclusion.
	if _, err := conn.Parent().Procedures().Begin(conn.Ctx(), procedure.Procedure{Type: procedure.N2Handover}); err != nil {
		t.Fatalf("start N2 handover: %v", err)
	}

	before := ue.KnasEncForTest()

	// While a handover holds the key chain the re-key is refused: no security mode
	// procedure is claimed and no NAS keys are derived (asserted below).
	securityMode(context.Background(), amf.New(nil, nil, nil), ue)

	if conn.Parent().Procedures().Active(procedure.SecurityMode) {
		t.Fatal("security mode must not be claimed when blocked by a handover")
	}

	if ue.KnasEncForTest() != before {
		t.Fatal("a blocked security mode must not derive the NAS keys")
	}
}

// TestSecurityMode_NoCommonAlgorithm_RejectsAndDeregisters verifies that when the
// UE and the operator policy share no NAS algorithm, the AMF rejects the
// registration (5GMM cause #23) and releases the UE, leaving no
// half-registered UE with an open RAN connection (mirrors the MME's ATTACH REJECT).
func TestSecurityMode_NoCommonAlgorithm_RejectsAndDeregisters(t *testing.T) {
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: `["000001"]`,
			Ciphering:     `["AES"]`,
			Integrity:     `["AES"]`,
		},
	}, nil, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("build UE and radio: %v", err)
	}

	ue.Conn().RegistrationType5GS = nasMessage.RegistrationType5GSInitialRegistration
	// The UE advertises no supported NAS algorithm, so it shares none with the
	// operator's AES-only policy.
	ue.SetUESecurityCapabilityForTest(newUESecCaps(0x00, 0x00))

	securityMode(context.Background(), amfInstance, ue)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected one REGISTRATION REJECT, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if err := nm.PlainNasDecode(&resp.NasPdu); err != nil {
		t.Fatalf("could not decode REGISTRATION REJECT: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationReject {
		t.Fatalf("expected REGISTRATION REJECT, got %v", nm.GmmHeader.GetMessageType())
	}

	if got := nm.RegistrationReject.GetCauseValue(); got != nasMessage.Cause5GMMUESecurityCapabilitiesMismatch {
		t.Fatalf("cause = %d, want #%d (UE security capabilities mismatch)", got, nasMessage.Cause5GMMUESecurityCapabilitiesMismatch)
	}

	if ue.State() != amf.Deregistered {
		t.Fatalf("UE must be released to Deregistered, got %v", ue.State())
	}
}
