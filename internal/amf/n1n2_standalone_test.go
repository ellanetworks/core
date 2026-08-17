// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

// connectedUE returns a registered UE with a NAS connection and the sender behind it.
func connectedUE(t *testing.T, imsi string) (*amf.UeContext, *amf.UeConn, *fakeNGAPSender) {
	t.Helper()

	sender := &fakeNGAPSender{}
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, imsi, func(u *amf.UeContext) {
		u.ForceStateForTest(amf.Registered)
	})

	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	return ue, ueConn, sender
}

func TestDeliverStandaloneN1N2_LPP_SendsDLNASTransport(t *testing.T) {
	ue, conn, sender := connectedUE(t, "001010000000060")

	req := &models.N1N2MessageTransferRequest{
		N1Class:             models.N1ClassLPP,
		BinaryDataN1Message: []byte{0x01, 0x02, 0x03},
		LCSCorrelationID:    []byte{0xde, 0xad, 0xbe, 0xef},
	}

	if err := amf.DeliverStandaloneN1N2(context.Background(), ue, conn, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.downlinkNasTransportCalls != 1 {
		t.Errorf("DL NAS Transport calls = %d, want 1", sender.downlinkNasTransportCalls)
	}

	if sender.nrppaTransportCalls != 0 {
		t.Errorf("NRPPa transport calls = %d, want 0", sender.nrppaTransportCalls)
	}
}

func TestDeliverStandaloneN1N2_NRPPa_SendsNGAPTransport(t *testing.T) {
	ue, conn, sender := connectedUE(t, "001010000000061")

	req := &models.N1N2MessageTransferRequest{
		N2Class:                 models.N2ClassNRPPa,
		BinaryDataN2Information: []byte{0x0a, 0x0b},
		RoutingID:               3,
	}

	if err := amf.DeliverStandaloneN1N2(context.Background(), ue, conn, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.nrppaTransportCalls != 1 {
		t.Errorf("NRPPa transport calls = %d, want 1", sender.nrppaTransportCalls)
	}

	if sender.downlinkNasTransportCalls != 0 {
		t.Errorf("DL NAS Transport calls = %d, want 0", sender.downlinkNasTransportCalls)
	}
}

// A request carrying both an N1 and an N2 part delivers both.
func TestDeliverStandaloneN1N2_BothParts(t *testing.T) {
	ue, conn, sender := connectedUE(t, "001010000000062")

	req := &models.N1N2MessageTransferRequest{
		N1Class:                 models.N1ClassLPP,
		N2Class:                 models.N2ClassNRPPa,
		BinaryDataN1Message:     []byte{0x01},
		BinaryDataN2Information: []byte{0x02},
	}

	if err := amf.DeliverStandaloneN1N2(context.Background(), ue, conn, req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.downlinkNasTransportCalls != 1 || sender.nrppaTransportCalls != 1 {
		t.Errorf("sends = (%d DL NAS, %d NRPPa), want (1, 1)",
			sender.downlinkNasTransportCalls, sender.nrppaTransportCalls)
	}
}

// An unmapped class is an error rather than a silent mis-delivery, so adding a class
// without wiring its transport cannot go unnoticed.
func TestDeliverStandaloneN1N2_UnknownClass(t *testing.T) {
	for _, tc := range []struct {
		name    string
		req     *models.N1N2MessageTransferRequest
		wantErr string
	}{
		{
			name:    "unmapped N1 class",
			req:     &models.N1N2MessageTransferRequest{N1Class: "SMS", BinaryDataN1Message: []byte{0x01}},
			wantErr: "no NAS payload container",
		},
		{
			name:    "unmapped N2 class",
			req:     &models.N1N2MessageTransferRequest{N2Class: "PWS", BinaryDataN2Information: []byte{0x01}},
			wantErr: "no NGAP transport",
		},
		{
			name:    "SM N1 is not standalone-deliverable",
			req:     &models.N1N2MessageTransferRequest{N1Class: models.N1ClassSM, BinaryDataN1Message: []byte{0x01}},
			wantErr: "no NAS payload container",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ue, conn, sender := connectedUE(t, "001010000000063")

			err := amf.DeliverStandaloneN1N2(context.Background(), ue, conn, tc.req)
			if err == nil {
				t.Fatal("expected an error")
			}

			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantErr)
			}

			if sender.downlinkNasTransportCalls != 0 || sender.nrppaTransportCalls != 0 {
				t.Error("nothing must be sent for an unmapped class")
			}
		})
	}
}

func TestDeliverStandaloneN1N2_NilArguments(t *testing.T) {
	ue, conn, _ := connectedUE(t, "001010000000064")
	req := &models.N1N2MessageTransferRequest{N1Class: models.N1ClassLPP}

	if err := amf.DeliverStandaloneN1N2(context.Background(), nil, conn, req); err == nil {
		t.Error("expected an error for a nil UE")
	}

	if err := amf.DeliverStandaloneN1N2(context.Background(), ue, nil, req); err == nil {
		t.Error("expected an error for a nil connection")
	}

	if err := amf.DeliverStandaloneN1N2(context.Background(), ue, conn, nil); err == nil {
		t.Error("expected an error for a nil request")
	}
}
