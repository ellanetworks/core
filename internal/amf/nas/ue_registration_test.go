// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/nas/fgs"
)

type failingRegistrar struct{}

func (failingRegistrar) Register(context.Context, string) (int64, error) {
	return 0, errors.New("no leader")
}

func (failingRegistrar) Confirmed(context.Context, string, int64) bool { return false }

func (failingRegistrar) Purge(string) {}

func (failingRegistrar) Reconcile(context.Context, string, func() int64, func(context.Context)) {
}

func TestMobilityReg_AcceptedWhenRegistrationFails(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)
	amfInstance.Registrations = failingRegistrar{}

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NASPDU, 0)
	if nm[2] != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("expected RegistrationAccept, got %v", nm[2])
	}
}
