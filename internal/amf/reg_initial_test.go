// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

type emptyPolicyDB struct {
	*FakeDBInstance
}

func (fdb *emptyPolicyDB) ListPoliciesByProfile(_ context.Context, _ string) ([]db.Policy, error) {
	return []db.Policy{}, nil
}

func TestHandleInitialRegistration_EmptyAllowedNssai_RejectsRegistration(t *testing.T) {
	ctx := context.TODO()

	amfInstance := New(&emptyPolicyDB{FakeDBInstance: &FakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}}, nil, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.supi = mustSUPIFromPrefixed("imsi-001019756139935")
	ue.kamf = "0000000000000000000000000000000000000000000000000000000000000000"

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	ue.NasConn().RegistrationRequest = m.RegistrationRequest
	ue.NasConn().RegistrationType5GS = nasMessage.RegistrationType5GSInitialRegistration

	err = HandleInitialRegistration(ctx, amfInstance, ue)
	if err == nil {
		t.Fatal("expected registration reject for empty AllowedNssai, got nil")
	}

	if got, want := err.Error(), "registration Reject [No allowed S-NSSAI in subscription]"; got != want {
		t.Fatalf("expected error %q, got %q", want, got)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 Downlink NAS Transport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected plain NAS, got security header type %d", nm.SecurityHeaderType)
	}

	if err := nm.PlainNasDecode(&resp.NasPdu); err != nil {
		t.Fatalf("could not decode plain NAS message: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationReject {
		t.Fatalf("expected RegistrationReject, got %v", nm.GmmHeader.GetMessageType())
	}

	if nm.RegistrationReject == nil {
		t.Fatal("expected RegistrationReject payload")
	}

	if got, want := nm.RegistrationReject.GetCauseValue(), nasMessage.Cause5GMM5GSServicesNotAllowed; got != want {
		t.Fatalf("expected cause %d, got %d", want, got)
	}
}
