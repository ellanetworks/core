// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

// TestResolveAttachQoSDefaultWhenNoAPN: with no requested APN the attach uses the
// subscriber's default policy (TS 24.301 §6.5.1.3).
func TestResolveAttachQoSDefaultWhenNoAPN(t *testing.T) {
	m := newTestMME(t)
	ue := &UeContext{imsi: testSubscriber.IMSI}

	qos, err := m.resolveAttachQoS(context.Background(), ue)
	if err != nil {
		t.Fatalf("resolveAttachQoS: %v", err)
	}

	if qos.APN != "internet" {
		t.Errorf("APN = %q, want the default %q", qos.APN, "internet")
	}
}

// TestResolveAttachQoSSelectsRequestedAPN: a requested non-default APN selects the
// policy bound to that data network.
func TestResolveAttachQoSSelectsRequestedAPN(t *testing.T) {
	m := newTestMME(t)
	ue := &UeContext{imsi: testSubscriber.IMSI, requestedAPN: "ims"}

	qos, err := m.resolveAttachQoS(context.Background(), ue)
	if err != nil {
		t.Fatalf("resolveAttachQoS: %v", err)
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
	ue := &UeContext{imsi: testSubscriber.IMSI, requestedAPN: "nonexistent"}

	if _, err := m.resolveAttachQoS(context.Background(), ue); !errors.Is(err, ErrUnknownAPN) {
		t.Fatalf("resolveAttachQoS error = %v, want ErrUnknownAPN", err)
	}
}

// TestIngestAttachRequestExtractsAPN: the requested APN in the attach's PDN
// Connectivity Request is parsed into the UE context.
func TestIngestAttachRequestExtractsAPN(t *testing.T) {
	apnIE, err := eps.EncodeAPN("ims")
	if err != nil {
		t.Fatalf("EncodeAPN: %v", err)
	}

	esm, err := (&eps.PDNConnectivityRequest{
		ProcedureTransactionIdentity: 1, RequestType: 1, PDNType: eps.PDNTypeIPv4, AccessPointName: apnIE,
	}).Marshal()
	if err != nil {
		t.Fatalf("marshal PDN Connectivity Request: %v", err)
	}

	ue := &UeContext{}
	(&MME{}).ingestAttachRequest(ue, &eps.AttachRequest{ESMMessageContainer: esm})

	if ue.requestedAPN != "ims" {
		t.Errorf("requestedAPN = %q, want %q", ue.requestedAPN, "ims")
	}

	// No APN IE → empty (use the default policy).
	esm2, err := (&eps.PDNConnectivityRequest{ProcedureTransactionIdentity: 1, RequestType: 1, PDNType: eps.PDNTypeIPv4}).Marshal()
	if err != nil {
		t.Fatalf("marshal PDN Connectivity Request (no APN): %v", err)
	}

	ue2 := &UeContext{}
	(&MME{}).ingestAttachRequest(ue2, &eps.AttachRequest{ESMMessageContainer: esm2})

	if ue2.requestedAPN != "" {
		t.Errorf("requestedAPN = %q, want empty for an attach without an APN", ue2.requestedAPN)
	}
}
