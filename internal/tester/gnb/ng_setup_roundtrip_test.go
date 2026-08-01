// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	amfngap "github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

// TestNGSetupGNBToAMF drives the whole procedure across both sides of the
// in-house codec: the simulated gNB encodes an NG SETUP REQUEST, the AMF's
// handler parses it and answers, and the answer is parsed back as a gNB would.
// Everything below the SCTP write is exercised.
func TestNGSetupGNBToAMF(t *testing.T) {
	req, err := gnb.BuildNGSetupRequest(&gnb.NGSetupRequestOpts{
		Name:  "ella-gnb",
		GnbID: "000102",
		Mcc:   "001",
		Mnc:   "01",
		Tac:   "000001",
		Sst:   1,
		Sd:    "010203",
	})
	if err != nil {
		t.Fatalf("gNB could not build NG Setup Request: %v", err)
	}

	w := &captureWriter{}

	amfInstance := amf.New(&stubDB{
		operator: &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: `["000001"]`},
		slices:   []db.NetworkSlice{{ID: "slice-1", Name: "default", Sst: 1, Sd: ngap.Ptr("010203")}},
	}, nil, nil)

	radio := &amf.Radio{Conn: w, Log: zap.NewNop()}
	radio.BindAMFForTest(amfInstance)

	// The dispatcher's envelope check needs a live SCTP connection, so the
	// message is unwrapped here exactly as it unwraps it.
	pduIn, err := ngap.Unmarshal(req)
	if err != nil {
		t.Fatalf("AMF could not decode the gNB's PDU envelope: %v", err)
	}

	im, ok := pduIn.(*ngap.InitiatingMessage)
	if !ok || im.ProcedureCode != ngap.ProcNGSetup {
		t.Fatalf("gNB sent %T for procedure %v, want an NG Setup Request", pduIn, pduIn)
	}

	parsed, err := ngap.ParseNGSetupRequest(im.Value)
	if err != nil {
		t.Fatalf("AMF could not parse the gNB's NG Setup Request: %v", err)
	}

	if parsed.RANNodeName == nil || *parsed.RANNodeName != "ella-gnb" {
		t.Errorf("AMF read RAN node name %v, want ella-gnb", parsed.RANNodeName)
	}

	amfngap.HandleNGSetupRequest(context.Background(), amfInstance, radio, parsed)

	if len(w.msgs) != 1 {
		t.Fatalf("AMF sent %d messages, want 1 (NG Setup Response)", len(w.msgs))
	}

	pdu, err := ngap.Unmarshal(w.msgs[0])
	if err != nil {
		t.Fatalf("gNB could not decode the AMF's answer: %v", err)
	}

	so, ok := pdu.(*ngap.SuccessfulOutcome)
	if !ok {
		t.Fatalf("AMF answered with %T, want an NG Setup Response", pdu)
	}

	resp, err := ngap.ParseNGSetupResponse(so.Value)
	if err != nil {
		t.Fatalf("gNB could not parse the NG Setup Response: %v", err)
	}

	if resp.AMFName == "" || len(resp.ServedGUAMIList) == 0 || len(resp.PLMNSupportList) == 0 {
		t.Fatalf("NG Setup Response is missing mandatory content: %+v", resp)
	}
}

type captureWriter struct{ msgs [][]byte }

func (w *captureWriter) WriteMsg(b []byte, _ *sctp.SndRcvInfo) (int, error) {
	w.msgs = append(w.msgs, append([]byte(nil), b...))

	return len(b), nil
}

// stubDB serves only what NG Setup reads: the operator and its slices.
type stubDB struct {
	amf.DBer

	operator *db.Operator
	slices   []db.NetworkSlice
}

func (s *stubDB) GetOperator(context.Context) (*db.Operator, error) { return s.operator, nil }

func (s *stubDB) ListAllNetworkSlices(context.Context) ([]db.NetworkSlice, error) {
	return s.slices, nil
}

func (s *stubDB) NodeID() int { return 0 }
