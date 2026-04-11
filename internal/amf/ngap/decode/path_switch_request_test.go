// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func validUESecurityCapabilities() *ngapType.UESecurityCapabilities {
	caps := &ngapType.UESecurityCapabilities{}
	caps.NRencryptionAlgorithms.Value = aper.BitString{Bytes: []byte{0x80, 0x00}, BitLength: 16}
	caps.NRintegrityProtectionAlgorithms.Value = aper.BitString{Bytes: []byte{0x40, 0x00}, BitLength: 16}
	caps.EUTRAencryptionAlgorithms.Value = aper.BitString{Bytes: []byte{0x00, 0x00}, BitLength: 16}
	caps.EUTRAintegrityProtectionAlgorithms.Value = aper.BitString{Bytes: []byte{0x00, 0x00}, BitLength: 16}

	return caps
}

func validPDUSessionDLList() *ngapType.PDUSessionResourceToBeSwitchedDLList {
	return &ngapType.PDUSessionResourceToBeSwitchedDLList{
		List: []ngapType.PDUSessionResourceToBeSwitchedDLItem{
			{
				PDUSessionID:              ngapType.PDUSessionID{Value: 1},
				PathSwitchRequestTransfer: []byte{0xDE, 0xAD, 0xBE, 0xEF},
			},
		},
	}
}

func validPathSwitchRequest() *ngapType.PathSwitchRequest {
	msg := &ngapType.PathSwitchRequest{}

	msg.ProtocolIEs.List = []ngapType.PathSwitchRequestIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.PathSwitchRequestIEsValue{
				Present:     ngapType.PathSwitchRequestIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 7},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDSourceAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.PathSwitchRequestIEsValue{
				Present:           ngapType.PathSwitchRequestIEsPresentSourceAMFUENGAPID,
				SourceAMFUENGAPID: &ngapType.AMFUENGAPID{Value: 11},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDUserLocationInformation},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.PathSwitchRequestIEsValue{
				Present: ngapType.PathSwitchRequestIEsPresentUserLocationInformation,
				UserLocationInformation: &ngapType.UserLocationInformation{
					Present:                   ngapType.UserLocationInformationPresentUserLocationInformationNR,
					UserLocationInformationNR: &ngapType.UserLocationInformationNR{},
				},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDUESecurityCapabilities},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.PathSwitchRequestIEsValue{
				Present:                ngapType.PathSwitchRequestIEsPresentUESecurityCapabilities,
				UESecurityCapabilities: validUESecurityCapabilities(),
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDPDUSessionResourceToBeSwitchedDLList},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.PathSwitchRequestIEsValue{
				Present:                              ngapType.PathSwitchRequestIEsPresentPDUSessionResourceToBeSwitchedDLList,
				PDUSessionResourceToBeSwitchedDLList: validPDUSessionDLList(),
			},
		},
	}

	return msg
}

func TestDecodePathSwitchRequest_Happy(t *testing.T) {
	msg := validPathSwitchRequest()

	out, report := decode.DecodePathSwitchRequest(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.RANUENGAPID != 7 {
		t.Errorf("RANUENGAPID = %d, want 7", out.RANUENGAPID)
	}

	if out.SourceAMFUENGAPID != 11 {
		t.Errorf("SourceAMFUENGAPID = %d, want 11", out.SourceAMFUENGAPID)
	}

	if out.UserLocationInformation.Kind() != decode.UserLocationKindNR {
		t.Errorf("ULI kind = %d, want NR", out.UserLocationInformation.Kind())
	}

	if out.UESecurityCapabilities == nil {
		t.Error("UESecurityCapabilities should be non-nil after happy-path decode")
	}

	if len(out.PDUSessionResourceItems) != 1 {
		t.Fatalf("PDUSessionResourceItems len = %d, want 1", len(out.PDUSessionResourceItems))
	}

	if out.PDUSessionResourceItems[0].PDUSessionID.Value != 1 {
		t.Errorf("PDUSessionID = %d, want 1", out.PDUSessionResourceItems[0].PDUSessionID.Value)
	}

	if out.FailedToSetupItems != nil {
		t.Errorf("FailedToSetupItems should be nil when IE absent, got %+v", out.FailedToSetupItems)
	}
}

func TestDecodePathSwitchRequest_NilBody(t *testing.T) {
	out, report := decode.DecodePathSwitchRequest(nil)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if !report.Fatal() {
		t.Error("expected fatal report for nil body")
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}

	if len(report.Items) != 0 {
		t.Errorf("expected no per-IE items for nil body, got %+v", report.Items)
	}

	if out.RANUENGAPID != 0 {
		t.Errorf("expected zero value, got %d", out.RANUENGAPID)
	}
}

func TestDecodePathSwitchRequest_EmptyIEs(t *testing.T) {
	_, report := decode.DecodePathSwitchRequest(&ngapType.PathSwitchRequest{})
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}

	wantIEs := map[int64]bool{
		ngapType.ProtocolIEIDRANUENGAPID:                          true,
		ngapType.ProtocolIEIDSourceAMFUENGAPID:                    true,
		ngapType.ProtocolIEIDPDUSessionResourceToBeSwitchedDLList: true,
		ngapType.ProtocolIEIDUserLocationInformation:              true,
		ngapType.ProtocolIEIDUESecurityCapabilities:               true,
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

func TestDecodePathSwitchRequest_MissingRANUENGAPID(t *testing.T) {
	msg := validPathSwitchRequest()

	filtered := msg.ProtocolIEs.List[:0]
	for _, ie := range msg.ProtocolIEs.List {
		if ie.Id.Value == ngapType.ProtocolIEIDRANUENGAPID {
			continue
		}

		filtered = append(filtered, ie)
	}

	msg.ProtocolIEs.List = filtered

	_, report := decode.DecodePathSwitchRequest(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}
}

func TestDecodePathSwitchRequest_NilRANUENGAPIDValue(t *testing.T) {
	msg := validPathSwitchRequest()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDRANUENGAPID {
			msg.ProtocolIEs.List[i].Value.RANUENGAPID = nil
		}
	}

	_, report := decode.DecodePathSwitchRequest(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}

	// Exactly one item should be present: a Malformed for RANUENGAPID,
	// not also a MissingMandatory.
	if len(report.Items) != 1 {
		t.Fatalf("expected 1 report item, got %d (%+v)", len(report.Items), report.Items)
	}

	if report.Items[0].IEID != ngapType.ProtocolIEIDRANUENGAPID {
		t.Errorf("expected RANUENGAPID item, got IE %d", report.Items[0].IEID)
	}
}

func TestDecodePathSwitchRequest_NilPDUListValue(t *testing.T) {
	msg := validPathSwitchRequest()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDPDUSessionResourceToBeSwitchedDLList {
			msg.ProtocolIEs.List[i].Value.PDUSessionResourceToBeSwitchedDLList = nil
		}
	}

	_, report := decode.DecodePathSwitchRequest(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}
}

func TestDecodePathSwitchRequest_MalformedULIIsNonFatal(t *testing.T) {
	msg := validPathSwitchRequest()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDUserLocationInformation {
			msg.ProtocolIEs.List[i].Value.UserLocationInformation = &ngapType.UserLocationInformation{
				Present: ngapType.UserLocationInformationPresentUserLocationInformationNR,
				// inner pointer nil — malformed
			}
		}
	}

	out, report := decode.DecodePathSwitchRequest(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	// UserLocationInformation is mandatory-ignore: malformed must not
	// be fatal so the handler still gets invoked.
	if report.Fatal() {
		t.Errorf("expected non-fatal report for malformed ULI, got %+v", report)
	}

	if out.UserLocationInformation.Kind() != decode.UserLocationKindUnknown {
		t.Errorf("expected zero ULI kind on malformed input, got %d", out.UserLocationInformation.Kind())
	}
}

func TestDecodePathSwitchRequest_NilSecCapsValueNonFatal(t *testing.T) {
	msg := validPathSwitchRequest()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDUESecurityCapabilities {
			msg.ProtocolIEs.List[i].Value.UESecurityCapabilities = nil
		}
	}

	out, report := decode.DecodePathSwitchRequest(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report for nil security caps, got %+v", report)
	}

	if out.UESecurityCapabilities != nil {
		t.Errorf("expected nil UESecurityCapabilities on malformed input, got %+v", out.UESecurityCapabilities)
	}
}

func TestDecodePathSwitchRequest_OptionalFailedToSetupList(t *testing.T) {
	msg := validPathSwitchRequest()

	failed := &ngapType.PDUSessionResourceFailedToSetupListPSReq{
		List: []ngapType.PDUSessionResourceFailedToSetupItemPSReq{
			{
				PDUSessionID:                         ngapType.PDUSessionID{Value: 2},
				PathSwitchRequestSetupFailedTransfer: []byte{0xCA, 0xFE},
			},
		},
	}

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.PathSwitchRequestIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListPSReq},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.PathSwitchRequestIEsValue{
			Present:                                  ngapType.PathSwitchRequestIEsPresentPDUSessionResourceFailedToSetupListPSReq,
			PDUSessionResourceFailedToSetupListPSReq: failed,
		},
	})

	out, report := decode.DecodePathSwitchRequest(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if len(out.FailedToSetupItems) != 1 {
		t.Fatalf("FailedToSetupItems len = %d, want 1", len(out.FailedToSetupItems))
	}

	if out.FailedToSetupItems[0].PDUSessionID.Value != 2 {
		t.Errorf("expected PDUSessionID 2, got %d", out.FailedToSetupItems[0].PDUSessionID.Value)
	}
}

func TestDecodePathSwitchRequest_DuplicateIELastWins(t *testing.T) {
	msg := validPathSwitchRequest()

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.PathSwitchRequestIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.PathSwitchRequestIEsValue{
			Present:     ngapType.PathSwitchRequestIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 999},
		},
	})

	out, report := decode.DecodePathSwitchRequest(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.RANUENGAPID != 999 {
		t.Errorf("expected last-wins RANUENGAPID=999, got %d", out.RANUENGAPID)
	}
}
