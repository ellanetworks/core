// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/nas/fgs"
)

type emptyPolicyDB struct {
	*fakeDBInstance
}

func (fdb *emptyPolicyDB) ListPoliciesByProfile(_ context.Context, _ string) ([]db.Policy, error) {
	return []db.Policy{}, nil
}

func TestHandleInitialRegistration_EmptyAllowedNssai_RejectsRegistration(t *testing.T) {
	ctx := context.TODO()

	amfInstance := amf.New(&emptyPolicyDB{fakeDBInstance: &fakeDBInstance{
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

	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001019756139935"))
	ue.SetKamfForTest("0000000000000000000000000000000000000000000000000000000000000000")

	ue.Conn().RegistrationRequest = &fgs.RegistrationRequest{}
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeInitial

	HandleInitialRegistration(ctx, amfInstance, ue)

	if ue.State() != amf.Deregistered {
		t.Fatalf("UE should be released to Deregistered after the reject, got %v", ue.State())
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 Downlink NAS Transport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NasPdu, uint8(fgs.MsgRegistrationReject))

	reject, err := fgs.ParseRegistrationReject(resp.NasPdu)
	if err != nil {
		t.Fatalf("could not parse RegistrationReject: %v", err)
	}

	if got, want := int(reject.Cause), 0x07; got != want {
		t.Fatalf("expected cause %d, got %d", want, got)
	}
}
