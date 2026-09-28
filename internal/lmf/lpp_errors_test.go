// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/lmf/lpp"
	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
	"github.com/ellanetworks/core/internal/lmf/models"
)

func startCapturedSession(t *testing.T) (*LMF, *captureLPPHandler, *lpp.Session, etsi.SUPI) {
	t.Helper()

	handler := &captureLPPHandler{}
	lmfInstance := New(amf.New(nil, nil, nil), nil, nil)
	lmfInstance.SetLPPHandler(handler)

	supi, err := etsi.NewSUPIFromIMSI("123456789012345")
	if err != nil {
		t.Fatalf("NewSUPIFromIMSI: %v", err)
	}

	session := lpp.NewSession(supi.String(), "session-1", string(RequestedGNSS))
	session.SetTransport(
		func(lppMsg []byte) error {
			return handler.ForwardLPPToUE(context.Background(), CoreAMF, supi.String(), nil, lppMsg)
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

	return lmfInstance, handler, session, supi
}

func uplink(t *testing.T, seq byte, body *lpptype.LPPMessageBodyC1) []byte {
	t.Helper()

	s := int64(seq)

	raw, err := lpp.Encoder(&lpptype.LPPMessage{
		TransactionID:   &lpptype.LPPTransactionID{Initiator: lpptype.InitiatorTargetDevice, TransactionNumber: 2},
		EndTransaction:  true,
		SequenceNumber:  &s,
		Acknowledgement: &lpptype.Acknowledgement{AckRequested: true},
		LPPMessageBody:  &lpptype.LPPMessageBody{C1: body},
	})
	if err != nil {
		t.Fatalf("Encoder: %v", err)
	}

	return raw
}

func downlinks(t *testing.T, h *captureLPPHandler) []*lpp.DecodedMessage {
	t.Helper()

	h.mu.Lock()
	defer h.mu.Unlock()

	out := make([]*lpp.DecodedMessage, 0, len(h.messages))

	for i, raw := range h.messages {
		msg, err := lpp.DecodeLPPMessage(raw)
		if err != nil {
			t.Fatalf("downlink %d: %v", i, err)
		}

		out = append(out, msg)
	}

	return out
}

func TestUndecodableMessageIsAcknowledgedAndRejected(t *testing.T) {
	lmfInstance, handler, session, supi := startCapturedSession(t)

	inbound := provideCapabilitiesRequestingAck(t, 7)
	inbound = inbound[:len(inbound)-2]

	if err := ForwardLPPToLMF(lmfInstance, context.Background(), CoreAMF, supi, nil, inbound); err == nil {
		t.Fatal("expected a decode error")
	}

	msgs := downlinks(t, handler)
	if len(msgs) != 3 {
		t.Fatalf("got %d downlinks, want request, acknowledgement and error", len(msgs))
	}

	raw, err := lpp.Decoder(handler.messages[1])
	if err != nil {
		t.Fatalf("Decoder: %v", err)
	}

	if raw.Acknowledgement == nil || raw.Acknowledgement.AckIndicator == nil || *raw.Acknowledgement.AckIndicator != 7 {
		t.Errorf("second downlink is not an acknowledgement of sequence number 7: %+v", raw.Acknowledgement)
	}

	errMsg := msgs[2]
	if errMsg.BodyKind != lpptype.LPPMessageBodyC1PresentError {
		t.Fatalf("third downlink body kind = %d, want Error", errMsg.BodyKind)
	}

	if errMsg.Error.Cause == nil || *errMsg.Error.Cause != int64(lpptype.ErrorCauseLPPMessageBodyError) {
		t.Errorf("error cause = %v, want lppMessageBodyError", errMsg.Error.Cause)
	}

	if !errMsg.TransactionIDPresent || errMsg.Initiator != lpptype.InitiatorTargetDevice {
		t.Errorf("error does not echo the UE transaction ID: %+v", errMsg)
	}

	if session.State() != lpp.SessionFailed || !errors.Is(session.Failure(), lpp.ErrUndecodableMessage) {
		t.Errorf("session state = %s, failure = %v", session.State(), session.Failure())
	}
}

func TestUEAbortFailsSession(t *testing.T) {
	lmfInstance, handler, session, supi := startCapturedSession(t)

	abort := uplink(t, 3, &lpptype.LPPMessageBodyC1{Abort: &lpptype.Abort{CriticalExtensions: lpptype.AbortCriticalExtensions{
		C1: &lpptype.AbortCriticalExtensionsC1{AbortR9: &lpptype.AbortR9IEs{
			CommonIEsAbort: &lpptype.CommonIEsAbort{AbortCause: lpptype.AbortCauseTargetDeviceAbort},
		}},
	}}})

	if err := ForwardLPPToLMF(lmfInstance, context.Background(), CoreAMF, supi, nil, abort); !errors.Is(err, lpp.ErrUEAborted) {
		t.Fatalf("ForwardLPPToLMF = %v, want ErrUEAborted", err)
	}

	if session.State() != lpp.SessionFailed {
		t.Errorf("session state = %s, want failed", session.State())
	}

	for i, msg := range downlinks(t, handler) {
		if msg.BodyKind == lpptype.LPPMessageBodyC1PresentError {
			t.Errorf("downlink %d answers an Abort with an Error", i)
		}
	}
}

func TestUERequestAssistanceDataIsAnswered(t *testing.T) {
	lmfInstance, handler, session, supi := startCapturedSession(t)

	request := uplink(t, 4, &lpptype.LPPMessageBodyC1{RequestAssistanceData: &lpptype.RequestAssistanceData{
		CriticalExtensions: lpptype.RequestAssistanceDataCriticalExtensions{
			C1: &lpptype.RequestAssistanceDataCriticalExtensionsC1{RequestAssistanceDataR9: &lpptype.RequestAssistanceDataR9IEs{
				AGNSSRequestAssistanceData: &lpptype.AGNSSRequestAssistanceData{},
			}},
		},
	}})

	if err := ForwardLPPToLMF(lmfInstance, context.Background(), CoreAMF, supi, nil, request); err != nil {
		t.Fatalf("ForwardLPPToLMF: %v", err)
	}

	msgs := downlinks(t, handler)
	last := msgs[len(msgs)-1]

	if last.BodyKind != lpptype.LPPMessageBodyC1PresentProvideAssistanceData || last.TransactionID != 2 || last.Initiator != lpptype.InitiatorTargetDevice {
		t.Fatalf("last downlink = %+v, want ProvideAssistanceData in the UE's transaction", last)
	}

	if session.State() == lpp.SessionFailed {
		t.Errorf("an assistance data request must not fail the session: %v", session.Failure())
	}
}
