// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
)

func TestHandleDeregistrationAccept_T3522Stopped_UEContextReleaseCommand(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)
	conn := ue.Conn()
	conn.NASGuardForTest().Arm(5*time.Minute, 5, func(expireTimes int32) {}, func() {})

	handleDeregistrationAccept(t.Context(), ue)

	if ue.State() != amf.Deregistered {
		t.Fatalf("expected UE to be deregistered, but was: %s", ue.State())
	}

	if conn.NASGuardForTest().Active() {
		t.Fatal("expected timer T3522 to be stopped and cleared")
	}

	if len(ngapSender.SentUEContextReleaseCommand) != 1 {
		t.Fatal("should have sent a UE Context Release Command message")
	}
}

func TestHandleDeregistrationAccept_NilRanUE_NoMessage(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test UE and radio: %v", err)
	}

	ue.Conn().AMFForTest().ReleaseNasConnection(ue, nil)

	handleDeregistrationAccept(t.Context(), ue)

	if len(ngapSender.SentUEContextReleaseCommand) != 0 {
		t.Fatal("should not have sent a UE Context Release Command message")
	}
}
