// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

func TestUERadioCapabilityInfoIndication_UnknownRanUeNgapID(t *testing.T) {
	ran := newTestRadio()

	ngap.HandleUERadioCapabilityInfoIndication(context.Background(), ran, decode.UERadioCapabilityInfoIndication{
		RANUENGAPID: 99,
	})
}

func TestUERadioCapabilityInfoIndication_NilAmfUe(t *testing.T) {
	ran := newTestRadio()
	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	ran.RanUEs[1] = ranUe

	ngap.HandleUERadioCapabilityInfoIndication(context.Background(), ran, decode.UERadioCapabilityInfoIndication{
		RANUENGAPID: 1,
	})
}

func TestUERadioCapabilityInfoIndication_SetsRadioCapability(t *testing.T) {
	ran := newTestRadio()
	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(ranUe)
	ran.RanUEs[1] = ranUe

	ngap.HandleUERadioCapabilityInfoIndication(context.Background(), ran, decode.UERadioCapabilityInfoIndication{
		RANUENGAPID:       1,
		UERadioCapability: []byte{0xDE, 0xAD, 0xBE, 0xEF},
	})

	if amfUe.UeRadioCapability != "deadbeef" {
		t.Errorf("UeRadioCapability = %q, want %q", amfUe.UeRadioCapability, "deadbeef")
	}
}

func TestUERadioCapabilityInfoIndication_SetsRadioCapabilityForPaging(t *testing.T) {
	ran := newTestRadio()
	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(ranUe)
	ran.RanUEs[1] = ranUe

	ngap.HandleUERadioCapabilityInfoIndication(context.Background(), ran, decode.UERadioCapabilityInfoIndication{
		RANUENGAPID: 1,
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
	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(ranUe)
	ran.RanUEs[1] = ranUe

	ngap.HandleUERadioCapabilityInfoIndication(context.Background(), ran, decode.UERadioCapabilityInfoIndication{
		RANUENGAPID: 1,
	})

	if amfUe.UeRadioCapability != "" {
		t.Errorf("UeRadioCapability = %q, want empty", amfUe.UeRadioCapability)
	}

	if amfUe.UeRadioCapabilityForPaging != nil {
		t.Errorf("UeRadioCapabilityForPaging = %+v, want nil", amfUe.UeRadioCapabilityForPaging)
	}
}
