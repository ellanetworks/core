// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// TestIngestAttachRequestStoresDRX verifies the UE's requested DRX parameter is
// parsed and stored, mirroring the AMF's UESpecificDRX (parity; the value is
// otherwise unused today — 4G echoes it only in NB-IoT and neither stack pages
// with it yet).
func TestIngestAttachRequestStoresDRX(t *testing.T) {
	ue := &mme.UeContext{}
	drx := []byte{0x00, 0x08}

	ingestAttachRequest(context.Background(), ue, &eps.AttachRequest{Unrecognized: []nas.RawIE{{IEI: ieiDRXParameter, Format: nas.IETV3, Value: drx}}})

	if !bytes.Equal(ue.DRXParameter, drx) {
		t.Fatalf("DRXParameter = %x, want %x", ue.DRXParameter, drx)
	}

	// Omitted DRX parameter leaves it nil.
	ue2 := &mme.UeContext{}
	ingestAttachRequest(context.Background(), ue2, &eps.AttachRequest{})

	if ue2.DRXParameter != nil {
		t.Fatalf("DRXParameter = %x, want nil when omitted", ue2.DRXParameter)
	}
}

func TestIngestAttachRequestExtractsAPN(t *testing.T) {
	esm, err := (&eps.PDNConnectivityRequest{
		PTI: 1, RequestType: 1, PDNType: eps.PDNTypeIPv4, AccessPointName: new(eps.APN("ims")),
	}).MarshalBinary()
	if err != nil {
		t.Fatalf("marshal PDN Connectivity Request: %v", err)
	}

	ue := &mme.UeContext{}
	ingestAttachRequest(context.Background(), ue, &eps.AttachRequest{ESMMessageContainer: esm})

	if ue.RequestedAPN != "ims" {
		t.Errorf("requestedAPN = %q, want %q", ue.RequestedAPN, "ims")
	}

	// No APN IE → empty (use the default policy).
	esm2, err := (&eps.PDNConnectivityRequest{PTI: 1, RequestType: 1, PDNType: eps.PDNTypeIPv4}).MarshalBinary()
	if err != nil {
		t.Fatalf("marshal PDN Connectivity Request (no APN): %v", err)
	}

	ue2 := &mme.UeContext{}
	ingestAttachRequest(context.Background(), ue2, &eps.AttachRequest{ESMMessageContainer: esm2})

	if ue2.RequestedAPN != "" {
		t.Errorf("requestedAPN = %q, want empty for an attach without an APN", ue2.RequestedAPN)
	}
}

// TestIngestAttachRequest_SoftIEErrorKeepsRequest pins TS 24.301 §7.7.1 on the
// ESM container: a syntactically incorrect optional element leaves the rest of
// the PDN CONNECTIVITY REQUEST usable. Dropping the whole message on a soft
// error lost the requested APN, PDN type and PTI, so the UE got the default APN,
// IPv4 where it asked for IPv6, and a PTI of 0 echoed back against §6.4.1.
func TestIngestAttachRequest_SoftIEErrorKeepsRequest(t *testing.T) {
	apn := eps.APN("internet")

	esm, err := (&eps.PDNConnectivityRequest{
		EPSBearerIdentity: 0,
		PTI:               5,
		RequestType:       eps.RequestTypeInitialRequest,
		PDNType:           eps.PDNTypeIPv6,
		AccessPointName:   &apn,
		// An access point name the codec cannot use, under an IEI the message
		// models: recognised, soft-failed, and preserved (§7.7.1).
		Unrecognized: []nas.RawIE{{IEI: 0x5D, Format: nas.IETLV, Value: []byte{0x00}}},
	}).MarshalBinary()
	if err != nil {
		t.Fatalf("build PDN CONNECTIVITY REQUEST: %v", err)
	}

	// Append a second, unusable APN so the parse reports a soft error.
	esm = append(esm, 0x28, 0x01, 0x00)

	ue := mme.NewUeContext()
	ingestAttachRequest(context.Background(), ue, &eps.AttachRequest{ESMMessageContainer: esm})

	if ue.RequestedAPN != "internet" {
		t.Errorf("RequestedAPN = %q, want %q", ue.RequestedAPN, "internet")
	}

	if ue.RequestedPDNType != uint8(eps.PDNTypeIPv6) {
		t.Errorf("RequestedPDNType = %d, want %d", ue.RequestedPDNType, uint8(eps.PDNTypeIPv6))
	}

	if ue.RequestedPTI != 5 {
		t.Errorf("RequestedPTI = %d, want 5", ue.RequestedPTI)
	}
}
