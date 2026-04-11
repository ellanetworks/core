// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func validInitialContextSetupFailure() *ngapType.InitialContextSetupFailure {
	msg := &ngapType.InitialContextSetupFailure{}

	msg.ProtocolIEs.List = []ngapType.InitialContextSetupFailureIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.InitialContextSetupFailureIEsValue{
				Present:     ngapType.InitialContextSetupFailureIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 5},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.InitialContextSetupFailureIEsValue{
				Present:     ngapType.InitialContextSetupFailureIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 9},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDCause},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.InitialContextSetupFailureIEsValue{
				Present: ngapType.InitialContextSetupFailureIEsPresentCause,
				Cause: &ngapType.Cause{
					Present:      ngapType.CausePresentRadioNetwork,
					RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnspecified},
				},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListCxtFail},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.InitialContextSetupFailureIEsValue{
				Present: ngapType.InitialContextSetupFailureIEsPresentPDUSessionResourceFailedToSetupListCxtFail,
				PDUSessionResourceFailedToSetupListCxtFail: &ngapType.PDUSessionResourceFailedToSetupListCxtFail{
					List: []ngapType.PDUSessionResourceFailedToSetupItemCxtFail{
						{
							PDUSessionID: ngapType.PDUSessionID{Value: 1},
							PDUSessionResourceSetupUnsuccessfulTransfer: aper.OctetString{0xAA, 0xBB},
						},
					},
				},
			},
		},
	}

	return msg
}

func TestDecodeInitialContextSetupFailure_Happy(t *testing.T) {
	out, report := decode.DecodeInitialContextSetupFailure(validInitialContextSetupFailure())
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID != 5 || out.RANUENGAPID != 9 {
		t.Errorf("UE NGAP IDs = (%d, %d), want (5, 9)", out.AMFUENGAPID, out.RANUENGAPID)
	}

	if out.Cause.Present != ngapType.CausePresentRadioNetwork {
		t.Errorf("Cause.Present = %d, want RadioNetwork", out.Cause.Present)
	}

	if len(out.PDUSessionResourceFailedToSetupItems) != 1 {
		t.Fatalf("expected 1 failed PDU session item, got %d", len(out.PDUSessionResourceFailedToSetupItems))
	}

	if out.PDUSessionResourceFailedToSetupItems[0].PDUSessionID.Value != 1 {
		t.Errorf("PDUSessionID = %d, want 1", out.PDUSessionResourceFailedToSetupItems[0].PDUSessionID.Value)
	}
}

func TestDecodeInitialContextSetupFailure_NilBody(t *testing.T) {
	_, report := decode.DecodeInitialContextSetupFailure(nil)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}
}

func TestDecodeInitialContextSetupFailure_EmptyIEs(t *testing.T) {
	_, report := decode.DecodeInitialContextSetupFailure(&ngapType.InitialContextSetupFailure{})
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report (missing UE NGAP IDs), got %+v", report)
	}

	wantIEs := map[int64]bool{
		ngapType.ProtocolIEIDAMFUENGAPID: true,
		ngapType.ProtocolIEIDRANUENGAPID: true,
		ngapType.ProtocolIEIDCause:       true,
	}

	gotIEs := make(map[int64]bool)
	for _, item := range report.Items {
		gotIEs[item.IEID] = true
	}

	for id := range wantIEs {
		if !gotIEs[id] {
			t.Errorf("missing report item for IE %d", id)
		}
	}
}

func TestDecodeInitialContextSetupFailure_OptionalAbsent(t *testing.T) {
	msg := validInitialContextSetupFailure()

	filtered := msg.ProtocolIEs.List[:0]
	for _, ie := range msg.ProtocolIEs.List {
		if ie.Id.Value != ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListCxtFail {
			filtered = append(filtered, ie)
		}
	}

	msg.ProtocolIEs.List = filtered

	out, report := decode.DecodeInitialContextSetupFailure(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.PDUSessionResourceFailedToSetupItems != nil {
		t.Error("expected nil PDUSessionResourceFailedToSetupItems when absent")
	}
}

func TestDecodeInitialContextSetupFailure_NilCauseValueNonFatal(t *testing.T) {
	msg := validInitialContextSetupFailure()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDCause {
			msg.ProtocolIEs.List[i].Value.Cause = nil
		}
	}

	out, report := decode.DecodeInitialContextSetupFailure(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report (Cause criticality ignore), got %+v", report)
	}

	if out.Cause.Present != 0 {
		t.Error("expected zero-value Cause when malformed")
	}
}

func TestDecodeInitialContextSetupFailure_NilAMFUENGAPIDValueIsFatal(t *testing.T) {
	msg := validInitialContextSetupFailure()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
			msg.ProtocolIEs.List[i].Value.AMFUENGAPID = nil
		}
	}

	_, report := decode.DecodeInitialContextSetupFailure(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	if len(report.Items) != 1 {
		t.Fatalf("expected 1 item (no double-report), got %d", len(report.Items))
	}
}

func TestDecodeInitialContextSetupFailure_DuplicateIELastWins(t *testing.T) {
	msg := validInitialContextSetupFailure()

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.InitialContextSetupFailureIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.InitialContextSetupFailureIEsValue{
			Present:     ngapType.InitialContextSetupFailureIEsPresentAMFUENGAPID,
			AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 42},
		},
	})

	out, report := decode.DecodeInitialContextSetupFailure(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID != 42 {
		t.Errorf("AMFUENGAPID = %d, want 42 (last-wins)", out.AMFUENGAPID)
	}
}
