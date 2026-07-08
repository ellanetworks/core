// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

func TestUERadioCapabilityInfoIndication_UnknownAmfUeNgapID(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	ngap.HandleUERadioCapabilityInfoIndication(context.Background(), amfInstance, ran, decode.UERadioCapabilityInfoIndication{
		RANUENGAPID: 99,
		AMFUENGAPID: 999,
	})

	errInd := assertSingleErrorIndication(t, sender, ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID)
	assertErrorIndicationEchoesIDs(t, errInd, 999, 99)
}

func TestUERadioCapabilityInfoIndication_NilUeContext(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)

	ngap.HandleUERadioCapabilityInfoIndication(context.Background(), amfInstance, ran, decode.UERadioCapabilityInfoIndication{
		RANUENGAPID: 1,
		AMFUENGAPID: 10,
	})
}

func TestUERadioCapabilityInfoIndication_SetsRadioCapability(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	amfUe := amf.NewUeContext()

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	ngap.HandleUERadioCapabilityInfoIndication(context.Background(), amfInstance, ran, decode.UERadioCapabilityInfoIndication{
		RANUENGAPID:       1,
		AMFUENGAPID:       10,
		UERadioCapability: []byte{0xDE, 0xAD, 0xBE, 0xEF},
	})

	if !bytes.Equal(amfUe.RadioCapability, []byte{0xDE, 0xAD, 0xBE, 0xEF}) {
		t.Errorf("RadioCapability = %x, want %x", amfUe.RadioCapability, []byte{0xDE, 0xAD, 0xBE, 0xEF})
	}
}

func TestUERadioCapabilityInfoIndication_SetsRadioCapabilityForPaging(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	amfUe := amf.NewUeContext()

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	ngap.HandleUERadioCapabilityInfoIndication(context.Background(), amfInstance, ran, decode.UERadioCapabilityInfoIndication{
		RANUENGAPID: 1,
		AMFUENGAPID: 10,
		UERadioCapabilityForPaging: &ngapType.UERadioCapabilityForPaging{
			UERadioCapabilityForPagingOfNR: &ngapType.UERadioCapabilityForPagingOfNR{
				Value: []byte{0xCA, 0xFE},
			},
			UERadioCapabilityForPagingOfEUTRA: &ngapType.UERadioCapabilityForPagingOfEUTRA{
				Value: []byte{0xBA, 0xBE},
			},
		},
	})

	if amfUe.RadioCapabilityForPaging == nil {
		t.Fatal("RadioCapabilityForPaging is nil")
	}

	if amfUe.RadioCapabilityForPaging.NR != "cafe" {
		t.Errorf("NR = %q, want %q", amfUe.RadioCapabilityForPaging.NR, "cafe")
	}

	if amfUe.RadioCapabilityForPaging.EUTRA != "babe" {
		t.Errorf("EUTRA = %q, want %q", amfUe.RadioCapabilityForPaging.EUTRA, "babe")
	}
}

func TestUERadioCapabilityInfoIndication_NilCapabilityFieldsNoOp(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	amfUe := amf.NewUeContext()

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	ngap.HandleUERadioCapabilityInfoIndication(context.Background(), amfInstance, ran, decode.UERadioCapabilityInfoIndication{
		RANUENGAPID: 1,
		AMFUENGAPID: 10,
	})

	if len(amfUe.RadioCapability) != 0 {
		t.Errorf("RadioCapability = %x, want empty", amfUe.RadioCapability)
	}

	if amfUe.RadioCapabilityForPaging != nil {
		t.Errorf("RadioCapabilityForPaging = %+v, want nil", amfUe.RadioCapabilityForPaging)
	}
}
