// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas/fgs"
)

// TS 24.501 §5.5.1.2.5, §5.5.1.3.5
func TestRegistrationRejectForAuthFailure(t *testing.T) {
	unknown := fmt.Errorf("%w: %w", udm.ErrSubscriberUnknown, db.ErrNotFound)

	tests := []struct {
		name        string
		err         error
		wantCause   fgs.GMMCause
		wantBackoff bool
	}{
		{"unknown subscriber", unknown, fgs.GMMCauseIllegalUE, false},
		{"unknown subscriber wrapped by serving NFs", fmt.Errorf("failed to send ue authentication request: %w", fmt.Errorf("ausf UE amf.Authentication Authenticate Request failed: %w", unknown)), fgs.GMMCauseIllegalUE, false},
		{"raft commit timeout", fmt.Errorf("advance sqn: %w", db.ErrProposeTimeout), fgs.GMMCauseCongestion, true},
		{"forwarded write outcome unknown", fmt.Errorf("advance sqn: %w", db.ErrOutcomeUnknown), fgs.GMMCauseCongestion, true},
		{"migration pending", fmt.Errorf("advance sqn: %w", db.ErrMigrationPending), fgs.GMMCauseCongestion, true},
		{"context cancelled", context.Canceled, fgs.GMMCauseCongestion, true},
		{"opaque error defaults to transient", errors.New("boom"), fgs.GMMCauseCongestion, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cause, backoff := registrationRejectForAuthFailure(tt.err)

			if cause != tt.wantCause {
				t.Errorf("cause = %v, want %v", cause, tt.wantCause)
			}

			if got := backoff != 0; got != tt.wantBackoff {
				t.Errorf("backoff = %v, want non-zero: %v", backoff, tt.wantBackoff)
			}
		})
	}
}

func TestRegistrationRejectForAuthFailureNeverUsesIdentityCause(t *testing.T) {
	for _, err := range []error{
		errors.New("boom"),
		fmt.Errorf("%w: %w", udm.ErrSubscriberUnknown, db.ErrNotFound),
		db.ErrProposeTimeout,
	} {
		if cause, _ := registrationRejectForAuthFailure(err); cause == fgs.GMMCauseUEIdentityCannotBeDerived {
			t.Errorf("registrationRejectForAuthFailure(%v) = %v, which deregisters a UE on a mobility registration update", err, cause)
		}
	}
}

func registrationRejectOnAuthError(t *testing.T, ausfErr error) *fgs.RegistrationReject {
	t.Helper()

	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, &fakeAusf{Error: ausfErr}, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = "testsuci"
	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001019756139935"))

	if err := amfInstance.CommitUEIdentity(t.Context(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(t.Context(), amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("sent %d downlink NAS messages, want 1", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgRegistrationReject))

	reject, err := fgs.ParseRegistrationReject(resp.NASPDU)
	if err != nil {
		t.Fatalf("ParseRegistrationReject: %v", err)
	}

	return reject
}

// TS 24.501 §5.5.1.3.5
func TestHandleRegistrationRequest_TransientAuthFailureRejectsWithCongestion(t *testing.T) {
	reject := registrationRejectOnAuthError(t, fmt.Errorf("advance sqn: %w", db.ErrProposeTimeout))

	if reject.Cause != fgs.GMMCauseCongestion {
		t.Errorf("cause = %v, want %v", reject.Cause, fgs.GMMCauseCongestion)
	}

	if reject.T3346 == nil {
		t.Fatal("T3346 absent; §5.5.1.3.5 makes #22 without it an abnormal case")
	}

	if reject.T3346.Value == 0 {
		t.Error("T3346 is zero; §5.5.1.3.5 makes #22 with a zero timer an abnormal case")
	}
}

// TS 24.501 §5.5.1.2.5
func TestHandleRegistrationRequest_UnknownSubscriberRejectsWithIllegalUE(t *testing.T) {
	unknown := fmt.Errorf("%w: %w", udm.ErrSubscriberUnknown, db.ErrNotFound)

	reject := registrationRejectOnAuthError(t, unknown)

	if reject.Cause != fgs.GMMCauseIllegalUE {
		t.Errorf("cause = %v, want %v", reject.Cause, fgs.GMMCauseIllegalUE)
	}

	if reject.T3346 != nil {
		t.Errorf("T3346 = %v, want nil on a permanent reject", reject.T3346)
	}
}
