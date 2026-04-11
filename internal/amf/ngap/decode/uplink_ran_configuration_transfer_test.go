// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/ngap/ngapType"
)

func validUplinkRANConfigurationTransfer() *ngapType.UplinkRANConfigurationTransfer {
	msg := &ngapType.UplinkRANConfigurationTransfer{}

	msg.ProtocolIEs.List = []ngapType.UplinkRANConfigurationTransferIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDSONConfigurationTransferUL},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.UplinkRANConfigurationTransferIEsValue{
				Present:                    ngapType.UplinkRANConfigurationTransferIEsPresentSONConfigurationTransferUL,
				SONConfigurationTransferUL: &ngapType.SONConfigurationTransfer{},
			},
		},
	}

	return msg
}

func TestDecodeUplinkRANConfigurationTransfer_Happy(t *testing.T) {
	out, report := decode.DecodeUplinkRANConfigurationTransfer(validUplinkRANConfigurationTransfer())
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.SONConfigurationTransferUL == nil {
		t.Error("expected non-nil SONConfigurationTransferUL")
	}
}

func TestDecodeUplinkRANConfigurationTransfer_NilBody(t *testing.T) {
	_, report := decode.DecodeUplinkRANConfigurationTransfer(nil)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}
}

func TestDecodeUplinkRANConfigurationTransfer_EmptyIEsNonFatal(t *testing.T) {
	out, report := decode.DecodeUplinkRANConfigurationTransfer(&ngapType.UplinkRANConfigurationTransfer{})
	if report != nil {
		t.Fatalf("expected nil report for empty optional-only IEs, got %+v", report)
	}

	if out.SONConfigurationTransferUL != nil {
		t.Error("expected nil SONConfigurationTransferUL")
	}
}

func TestDecodeUplinkRANConfigurationTransfer_NilSONValueNonFatal(t *testing.T) {
	msg := validUplinkRANConfigurationTransfer()
	msg.ProtocolIEs.List[0].Value.SONConfigurationTransferUL = nil

	out, report := decode.DecodeUplinkRANConfigurationTransfer(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report; got %+v", report)
	}

	if out.SONConfigurationTransferUL != nil {
		t.Error("expected nil SONConfigurationTransferUL")
	}
}
