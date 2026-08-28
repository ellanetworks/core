// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas/fgs"
)

// TS 24.501 §5.5.1.2.5, §5.5.1.3.5
func TestRegistrationRejectForAuthFailure(t *testing.T) {
	unknown := fmt.Errorf("%w: %w", udm.ErrSubscriberUnknown, db.ErrNotFound)
	underivable := fmt.Errorf("couldn't convert suci to supi: %w", fmt.Errorf("%w: %w", udm.ErrSUPIUnderivable, errors.New("profile A error: mac verification failed")))

	tests := []struct {
		name          string
		err           error
		wantCause     fgs.GMMCause
		wantPermanent bool
	}{
		{"unknown subscriber", unknown, fgs.GMMCauseIllegalUE, true},
		{"unknown subscriber wrapped by serving NFs", fmt.Errorf("failed to send ue authentication request: %w", fmt.Errorf("ausf UE amf.Authentication Authenticate Request failed: %w", unknown)), fgs.GMMCauseIllegalUE, true},
		{"suci does not deconceal", underivable, fgs.GMMCauseUEIdentityCannotBeDerived, true},
		{"suci does not deconceal wrapped by serving NFs", fmt.Errorf("failed to send ue authentication request: %w", fmt.Errorf("ausf UE amf.Authentication Authenticate Request failed: %w", underivable)), fgs.GMMCauseUEIdentityCannotBeDerived, true},
		{"raft commit timeout", fmt.Errorf("advance sqn: %w", db.ErrProposeTimeout), 0, false},
		{"forwarded write outcome unknown", fmt.Errorf("advance sqn: %w", db.ErrOutcomeUnknown), 0, false},
		{"migration pending", fmt.Errorf("advance sqn: %w", db.ErrMigrationPending), 0, false},
		{"context cancelled", context.Canceled, 0, false},
		{"opaque error defaults to transient", errors.New("boom"), 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cause, permanent := registrationRejectCauseForAuthFailure(tt.err)

			if permanent != tt.wantPermanent {
				t.Errorf("permanent = %v, want %v", permanent, tt.wantPermanent)
			}

			if cause != tt.wantCause {
				t.Errorf("cause = %v, want %v", cause, tt.wantCause)
			}
		})
	}
}

func TestRegistrationRejectForAuthFailureUsesIdentityCauseOnlyWhenSUPIUnderivable(t *testing.T) {
	for _, err := range []error{
		errors.New("boom"),
		fmt.Errorf("%w: %w", udm.ErrSubscriberUnknown, db.ErrNotFound),
		db.ErrProposeTimeout,
		db.ErrOutcomeUnknown,
		db.ErrMigrationPending,
		context.Canceled,
	} {
		if cause, _ := registrationRejectCauseForAuthFailure(err); cause == fgs.GMMCauseUEIdentityCannotBeDerived {
			t.Errorf("registrationRejectCauseForAuthFailure(%v) = %v, which deregisters a UE on a mobility registration update", err, cause)
		}
	}

	underivable := fmt.Errorf("%w: %w", udm.ErrSUPIUnderivable, errors.New("home network key not found"))
	if cause, _ := registrationRejectCauseForAuthFailure(underivable); cause != fgs.GMMCauseUEIdentityCannotBeDerived {
		t.Errorf("registrationRejectCauseForAuthFailure(%v) = %v, want %v", underivable, cause, fgs.GMMCauseUEIdentityCannotBeDerived)
	}
}

func registrationOnAuth(t *testing.T, ausfInstance amf.Authenticator, suci string) *fakeNGAPSender {
	t.Helper()

	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}, ausfInstance, nil)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.Suci = suci
	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001019756139935"))

	if err := amfInstance.CommitUEIdentity(t.Context(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(t.Context(), amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

	return ngapSender
}

func registrationRejectOnAuthError(t *testing.T, ausfErr error) *fgs.RegistrationReject {
	t.Helper()

	return registrationRejectOnAuth(t, &fakeAusf{Error: ausfErr}, "testsuci")
}

func registrationRejectOnAuth(t *testing.T, ausfInstance amf.Authenticator, suci string) *fgs.RegistrationReject {
	t.Helper()

	ngapSender := registrationOnAuth(t, ausfInstance, suci)

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

func TestHandleRegistrationRequest_TransientAuthFailureReleasesWithoutRejecting(t *testing.T) {
	ngapSender := registrationOnAuth(t, &fakeAusf{Error: fmt.Errorf("advance sqn: %w", db.ErrProposeTimeout)}, "testsuci")

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Errorf("sent %d downlink NAS messages, want 0; an unprotected reject would strand the UE on a 15-30 minute T3346", len(ngapSender.SentDownlinkNASTransport))
	}

	if len(ngapSender.SentUEContextReleaseCommand) != 1 {
		t.Errorf("sent %d UE Context Release Commands, want 1", len(ngapSender.SentUEContextReleaseCommand))
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

type rejectingSubscriberStore struct{ t *testing.T }

func (s *rejectingSubscriberStore) AdvanceSequenceNumber(_ context.Context, imsi, _, _ string) (*udm.AdvancedCredentials, error) {
	s.t.Errorf("subscriber store queried for %q, but the SUCI never deconcealed", imsi)
	return nil, errors.New("must not be reached")
}

func TestHandleRegistrationRequest_UndecipherableSUCIRejectsWithIdentityCause(t *testing.T) {
	hnPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate home network key: %v", err)
	}

	schemeOutput := make([]byte, 45)
	if _, err := rand.Read(schemeOutput); err != nil {
		t.Fatalf("read scheme output: %v", err)
	}

	suci := fmt.Sprintf("suci-0-001-01-0000-1-0-%s", hex.EncodeToString(schemeOutput))

	ausfInstance := ausf.New(&rejectingSubscriberStore{t: t}, func(scheme string, keyID int) (string, error) {
		return hex.EncodeToString(hnPriv.Bytes()), nil
	})

	reject := registrationRejectOnAuth(t, ausfInstance, suci)

	if reject.Cause != fgs.GMMCauseUEIdentityCannotBeDerived {
		t.Errorf("cause = %v, want %v", reject.Cause, fgs.GMMCauseUEIdentityCannotBeDerived)
	}

	if reject.T3346 != nil {
		t.Errorf("T3346 = %v, want nil on a permanent reject", reject.T3346)
	}
}

func registeredUEWithSession(t *testing.T, ausfErr error) (*amf.UeContext, *fakeNGAPSender) {
	t.Helper()

	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: "[\"000001\"]"},
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

	if err := ue.CreateSmContext(5, "smref-5", nil, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	ue.TransitionTo(amf.RegistrationInitiated)
	ue.TransitionTo(amf.Registered)

	m, err := buildTestRegistrationRequestMessage(0, nil, 0)
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	handleRegistrationRequest(t.Context(), amfInstance, ue, mustParseRegistrationRequest(t, m), m, true, false)

	return ue, ngapSender
}

// TS 24.501 §5.5.1.3.7 case e
func TestTransientAuthFailureRetainsRegistrationAndSessions(t *testing.T) {
	ue, ngapSender := registeredUEWithSession(t, fmt.Errorf("advance sqn: %w", db.ErrProposeTimeout))

	if got := ue.State(); got != amf.Registered {
		t.Errorf("state = %v, want Registered; the UE stays 5GMM-REGISTERED and retries at T3511", got)
	}

	if _, ok := ue.SmContextFindByPDUSessionID(5); !ok {
		t.Error("PDU session released on a transient failure; the UE still believes it exists")
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Errorf("sent %d downlink NAS messages, want 0", len(ngapSender.SentDownlinkNASTransport))
	}

	if len(ngapSender.SentUEContextReleaseCommand) != 1 {
		t.Errorf("sent %d UE Context Release Commands, want 1", len(ngapSender.SentUEContextReleaseCommand))
	}
}

// TS 24.501 §5.5.1.3.5
func TestPermanentRejectTearsDownRegistrationAndSessions(t *testing.T) {
	unknown := fmt.Errorf("%w: %w", udm.ErrSubscriberUnknown, db.ErrNotFound)

	ue, _ := registeredUEWithSession(t, unknown)

	if got := ue.State(); got != amf.Deregistered {
		t.Errorf("state = %v, want Deregistered", got)
	}

	if _, ok := ue.SmContextFindByPDUSessionID(5); ok {
		t.Error("PDU session retained on a permanent reject")
	}
}
