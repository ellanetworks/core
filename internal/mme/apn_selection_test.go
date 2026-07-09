// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"testing"
)

// TestResolveAttachQoSDefaultWhenNoAPN: with no requested APN the attach uses the
// subscriber's default policy (TS 24.301 §6.5.1.3).
func TestResolveAttachQoSDefaultWhenNoAPN(t *testing.T) {
	m := newTestMME(t)
	ue := &UeContext{supi: mustSUPI(testSubscriber.IMSI)}

	qos, err := ResolveAttachQoS(context.Background(), m, ue)
	if err != nil {
		t.Fatalf("ResolveAttachQoS: %v", err)
	}

	if qos.APN != "internet" {
		t.Errorf("APN = %q, want the default %q", qos.APN, "internet")
	}
}

// TestResolveAttachQoSSelectsRequestedAPN: a requested non-default APN selects the
// policy bound to that data network.
func TestResolveAttachQoSSelectsRequestedAPN(t *testing.T) {
	m := newTestMME(t)
	ue := &UeContext{supi: mustSUPI(testSubscriber.IMSI), RequestedAPN: "ims"}

	qos, err := ResolveAttachQoS(context.Background(), m, ue)
	if err != nil {
		t.Fatalf("ResolveAttachQoS: %v", err)
	}

	if qos.APN != "ims" {
		t.Errorf("APN = %q, want the requested %q", qos.APN, "ims")
	}

	if qos.IPv4Pool != "10.46.0.0/16" {
		t.Errorf("IPv4Pool = %q, want the ims pool 10.46.0.0/16", qos.IPv4Pool)
	}
}

// TestResolveAttachQoSRejectsUnknownAPN: a requested APN not bound to any policy in
// the profile returns ErrUnknownAPN, which the attach path maps to a reject.
func TestResolveAttachQoSRejectsUnknownAPN(t *testing.T) {
	m := newTestMME(t)
	ue := &UeContext{supi: mustSUPI(testSubscriber.IMSI), RequestedAPN: "nonexistent"}

	if _, err := ResolveAttachQoS(context.Background(), m, ue); !errors.Is(err, ErrUnknownAPN) {
		t.Fatalf("ResolveAttachQoS error = %v, want ErrUnknownAPN", err)
	}
}

// TestIngestAttachRequestExtractsAPN: the requested APN in the attach's PDN
