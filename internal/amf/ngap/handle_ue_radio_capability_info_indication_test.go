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
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)

	ngap.HandleUERadioCapabilityInfoIndication(context.Background(), ran, decode.UERadioCapabilityInfoIndication{
		RANUENGAPID: 99,
		AMFUENGAPID: 999,
	})

	errInd := assertSingleErrorIndication(t, sender, ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID)
	assertErrorIndicationEchoesIDs(t, errInd, 999, 99)
}

func TestUERadioCapabilityInfoIndication_NilUeContext(t *testing.T) {
	ran := newTestRadio()
	amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)

	ngap.HandleUERadioCapabilityInfoIndication(context.Background(), ran, decode.UERadioCapabilityInfoIndication{
		RANUENGAPID: 1,
		AMFUENGAPID: 10,
	})
}

func TestUERadioCapabilityInfoIndication_SetsRadioCapability(t *testing.T) {
	ran := newTestRadio()
	amfUe := amf.NewUeContext()
	amfUe.Log = logger.AmfLog

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

	ngap.HandleUERadioCapabilityInfoIndication(context.Background(), ran, decode.UERadioCapabilityInfoIndication{
		RANUENGAPID:       1,
		AMFUENGAPID:       10,
		UERadioCapability: []byte{0xDE, 0xAD, 0xBE, 0xEF},
	})

	if !bytes.Equal(amfUe.UeRadioCapability, []byte{0xDE, 0xAD, 0xBE, 0xEF}) {
		t.Errorf("UeRadioCapability = %x, want %x", amfUe.UeRadioCapability, []byte{0xDE, 0xAD, 0xBE, 0xEF})
	}
}

func TestUERadioCapabilityInfoIndication_SetsRadioCapabilityForPaging(t *testing.T) {
	ran := newTestRadio()
	amfUe := amf.NewUeContext()
	amfUe.Log = logger.AmfLog

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

	ngap.HandleUERadioCapabilityInfoIndication(context.Background(), ran, decode.UERadioCapabilityInfoIndication{
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

	if amfUe.UeRadioCapabilityForPaging == nil {
		t.Fatal("UeRadioCapabilityForPaging is nil")
	}

	if amfUe.UeRadioCapabilityForPaging.NR != "cafe" {
		t.Errorf("NR = %q, want %q", amfUe.UeRadioCapabilityForPaging.NR, "cafe")
	}

	if amfUe.UeRadioCapabilityForPaging.EUTRA != "babe" {
		t.Errorf("EUTRA = %q, want %q", amfUe.UeRadioCapabilityForPaging.EUTRA, "babe")
	}
}

func TestUERadioCapabilityInfoIndication_NilCapabilityFieldsNoOp(t *testing.T) {
	ran := newTestRadio()
	amfUe := amf.NewUeContext()
	amfUe.Log = logger.AmfLog

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

	ngap.HandleUERadioCapabilityInfoIndication(context.Background(), ran, decode.UERadioCapabilityInfoIndication{
		RANUENGAPID: 1,
		AMFUENGAPID: 10,
	})

	if len(amfUe.UeRadioCapability) != 0 {
		t.Errorf("UeRadioCapability = %x, want empty", amfUe.UeRadioCapability)
	}

	if amfUe.UeRadioCapabilityForPaging != nil {
		t.Errorf("UeRadioCapabilityForPaging = %+v, want nil", amfUe.UeRadioCapabilityForPaging)
	}
}
