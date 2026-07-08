// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/ngap/ngapType"
)

func validUplinkRANStatusTransfer() *ngapType.UplinkRANStatusTransfer {
	msg := &ngapType.UplinkRANStatusTransfer{}

	msg.ProtocolIEs.List = []ngapType.UplinkRANStatusTransferIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.UplinkRANStatusTransferIEsValue{
				Present:     ngapType.UplinkRANStatusTransferIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 42},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.UplinkRANStatusTransferIEsValue{
				Present:     ngapType.UplinkRANStatusTransferIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 7},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANStatusTransferTransparentContainer},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.UplinkRANStatusTransferIEsValue{
				Present:                               ngapType.UplinkRANStatusTransferIEsPresentRANStatusTransferTransparentContainer,
				RANStatusTransferTransparentContainer: &ngapType.RANStatusTransferTransparentContainer{},
			},
		},
	}

	return msg
}

func TestDecodeUplinkRANStatusTransfer_Valid(t *testing.T) {
	out, report := decode.DecodeUplinkRANStatusTransfer(validUplinkRANStatusTransfer())
	if report != nil {
		t.Fatalf("expected no report, got %+v", report)
	}

	if out.AMFUENGAPID != 42 || out.RANUENGAPID != 7 {
		t.Fatalf("IDs mismatch: %+v", out)
	}

	if out.Container == nil {
		t.Fatal("expected the transparent container to be captured for relay")
	}
}

func TestDecodeUplinkRANStatusTransfer_MissingContainer(t *testing.T) {
	msg := validUplinkRANStatusTransfer()
	msg.ProtocolIEs.List = msg.ProtocolIEs.List[:2] // drop the mandatory container

	_, report := decode.DecodeUplinkRANStatusTransfer(msg)
	if report == nil || !report.HasItems() {
		t.Fatal("expected a report for the missing mandatory container")
	}
}

func TestDecodeUplinkRANStatusTransfer_Nil(t *testing.T) {
	_, report := decode.DecodeUplinkRANStatusTransfer(nil)
	if report == nil || !report.ProcedureRejected {
		t.Fatal("expected a procedure-rejected report for nil input")
	}
}
