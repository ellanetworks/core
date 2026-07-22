// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/nas/fgs"
)

func TestHandleStatus5GMM_UEDeregistered_Ignored(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.Deregister(context.TODO())

	handleStatus5GMM(context.Background(), ue, buildTestStatus5gmm(t))

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("expected Status 5GMM in Deregistered state to be ignored, but a downlink was sent")
	}
}

func TestHandleStatus5GMM_Registered_Ignored(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)

	handleStatus5GMM(context.Background(), ue, buildTestStatus5gmm(t))

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("expected no downlink for Status 5GMM, but a downlink was sent")
	}
}

func buildTestStatus5gmm(t *testing.T) []byte {
	t.Helper()

	b, err := (&fgs.Status5GMM{Cause: 0x6f}).Marshal()
	if err != nil {
		t.Fatalf("build Status 5GMM: %v", err)
	}

	return b
}
