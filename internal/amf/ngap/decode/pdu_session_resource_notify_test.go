// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/ngap/ngapType"
)

func validPDUSessionResourceNotify() *ngapType.PDUSessionResourceNotify {
	msg := &ngapType.PDUSessionResourceNotify{}

	msg.ProtocolIEs.List = []ngapType.PDUSessionResourceNotifyIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.PDUSessionResourceNotifyIEsValue{
				Present:     ngapType.PDUSessionResourceNotifyIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 1},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.PDUSessionResourceNotifyIEsValue{
				Present:     ngapType.PDUSessionResourceNotifyIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 2},
			},
		},
	}

	return msg
}

func TestDecodePDUSessionResourceNotify_Happy(t *testing.T) {
	out, report := decode.DecodePDUSessionResourceNotify(validPDUSessionResourceNotify())
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID != 1 {
		t.Errorf("AMFUENGAPID = %d, want 1", out.AMFUENGAPID)
	}

	if out.RANUENGAPID != 2 {
		t.Errorf("RANUENGAPID = %d, want 2", out.RANUENGAPID)
	}

	if out.HasNotifyList {
		t.Error("expected HasNotifyList=false when IE absent")
	}

	if out.PDUSessionResourceReleasedItems != nil {
		t.Error("expected nil ReleasedItems when IE absent")
	}
}

func TestDecodePDUSessionResourceNotify_WithReleasedList(t *testing.T) {
	msg := validPDUSessionResourceNotify()
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.PDUSessionResourceNotifyIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDPDUSessionResourceReleasedListNot},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.PDUSessionResourceNotifyIEsValue{
			Present: ngapType.PDUSessionResourceNotifyIEsPresentPDUSessionResourceReleasedListNot,
			PDUSessionResourceReleasedListNot: &ngapType.PDUSessionResourceReleasedListNot{
				List: []ngapType.PDUSessionResourceReleasedItemNot{
					{PDUSessionID: ngapType.PDUSessionID{Value: 5}},
				},
			},
		},
	})

	out, report := decode.DecodePDUSessionResourceNotify(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if len(out.PDUSessionResourceReleasedItems) != 1 {
		t.Fatalf("ReleasedItems len = %d, want 1", len(out.PDUSessionResourceReleasedItems))
	}
}

func TestDecodePDUSessionResourceNotify_NilBody(t *testing.T) {
	_, report := decode.DecodePDUSessionResourceNotify(nil)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}
}

func TestDecodePDUSessionResourceNotify_EmptyIEs(t *testing.T) {
	_, report := decode.DecodePDUSessionResourceNotify(&ngapType.PDUSessionResourceNotify{})
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}
}

func TestDecodePDUSessionResourceNotify_NilAMFUENGAPIDValue(t *testing.T) {
	msg := validPDUSessionResourceNotify()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
			msg.ProtocolIEs.List[i].Value.AMFUENGAPID = nil
		}
	}

	_, report := decode.DecodePDUSessionResourceNotify(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}

	if len(report.Items) != 1 {
		t.Fatalf("expected 1 report item, got %d", len(report.Items))
	}
}

func TestDecodePDUSessionResourceNotify_DuplicateIELastWins(t *testing.T) {
	msg := validPDUSessionResourceNotify()
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.PDUSessionResourceNotifyIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.PDUSessionResourceNotifyIEsValue{
			Present:     ngapType.PDUSessionResourceNotifyIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 999},
		},
	})

	out, report := decode.DecodePDUSessionResourceNotify(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.RANUENGAPID != 999 {
		t.Errorf("expected last-wins RANUENGAPID=999, got %d", out.RANUENGAPID)
	}
}
