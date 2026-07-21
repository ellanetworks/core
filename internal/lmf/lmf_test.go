// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/lmf/lpp"
	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
	"github.com/ellanetworks/core/internal/lmf/models"
)

type captureLPPHandler struct {
	mu             sync.Mutex
	correlationIDs [][]byte
}

func (h *captureLPPHandler) ForwardLPPToUE(_ context.Context, _ string, correlationID, _ []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.correlationIDs = append(h.correlationIDs, append([]byte(nil), correlationID...))

	return nil
}

// provideCapabilitiesRequestingAck encodes a UE ProvideCapabilities that carries
// a sequence number and requests an acknowledgement (TS 37.355 §4.3.3).
func provideCapabilitiesRequestingAck(t *testing.T, seq byte) []byte {
	t.Helper()

	raw, err := lpp.EncodeProvideCapabilities(0x00, []int64{lpptype.GnssIDGps})
	if err != nil {
		t.Fatalf("EncodeProvideCapabilities: %v", err)
	}

	msg, err := lpp.Decoder(raw)
	if err != nil {
		t.Fatalf("Decoder: %v", err)
	}

	s := int64(seq)
	msg.SequenceNumber = &s
	msg.Acknowledgement = &lpptype.Acknowledgement{AckRequested: true}

	encoded, err := lpp.Encoder(msg)
	if err != nil {
		t.Fatalf("Encoder: %v", err)
	}

	return encoded
}

// TestAcknowledgementUsesSessionCorrelationID checks that every downlink of a
// positioning session carries the session's LCS correlation identifier, not the
// identifier the UE echoed on the uplink and not a per-message one
// (TS 23.273 §6.11.1 NOTE 11).
func TestAcknowledgementUsesSessionCorrelationID(t *testing.T) {
	handler := &captureLPPHandler{}
	lmfInstance := New(amf.New(nil, nil, nil), nil, nil)
	lmfInstance.SetLPPHandler(handler)

	supi, err := etsi.NewSUPIFromIMSI("123456789012345")
	if err != nil {
		t.Fatalf("NewSUPIFromIMSI: %v", err)
	}

	sessionCorrelationID := []byte{0xAA, 0xBB, 0xCC, 0xDD}
	ueCorrelationID := []byte{0x11, 0x22, 0x33, 0x44}

	session := lpp.NewSession(supi.String(), "session-1", string(MethodAGNSSAssisted))
	session.SetCorrelationID(sessionCorrelationID)
	session.SetTransport(
		func(lppMsg []byte) error {
			return handler.ForwardLPPToUE(context.Background(), supi.String(), session.CorrelationID(), lppMsg)
		},
		func(*models.LocationResult) error { return nil },
		func() error { return nil },
		func() error { return nil },
		func() {},
	)

	lmfInstance.RegisterLPPSession(session.SessionID(), session)

	if err := session.StartSession(); err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	inbound := provideCapabilitiesRequestingAck(t, 7)
	if err := ForwardLPPToLMF(lmfInstance, context.Background(), supi, ueCorrelationID, inbound); err != nil {
		t.Fatalf("ForwardLPPToLMF: %v", err)
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()

	if len(handler.correlationIDs) < 2 {
		t.Fatalf("expected at least the capabilities request and the acknowledgement, got %d downlinks", len(handler.correlationIDs))
	}

	for i, got := range handler.correlationIDs {
		if !bytes.Equal(got, sessionCorrelationID) {
			t.Errorf("downlink %d: correlation ID = %x, want session ID %x", i, got, sessionCorrelationID)
		}
	}
}
