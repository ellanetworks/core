// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
)

func TestUERadioCapabilityInfoIndication_UnknownAmfUeNgapID(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	HandleUERadioCapabilityInfoIndication(context.Background(), amfInstance, ran, &ngap.UERadioCapabilityInfoIndication{
		RANUENGAPID: ngap.RANUENGAPID(99),
		AMFUENGAPID: ngap.AMFUENGAPID(999),
	})

	errInd := assertSingleErrorIndication(t, sender, ngap.CauseRadioNetworkUnknownLocalUENGAPID)
	assertErrorIndicationEchoesIDs(t, errInd, 999, 99)
}

func TestUERadioCapabilityInfoIndication_NilUeContext(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)
	amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)

	HandleUERadioCapabilityInfoIndication(context.Background(), amfInstance, ran, &ngap.UERadioCapabilityInfoIndication{
		RANUENGAPID: ngap.RANUENGAPID(1),
		AMFUENGAPID: ngap.AMFUENGAPID(10),
	})

	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("a resolvable connection with no UE context must be dropped silently, got %d error indications", len(sender.SentErrorIndications))
	}
}

func TestUERadioCapabilityInfoIndication_SetsRadioCapability(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	amfUe := amf.NewUeContext()

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	HandleUERadioCapabilityInfoIndication(context.Background(), amfInstance, ran, &ngap.UERadioCapabilityInfoIndication{
		RANUENGAPID:       ngap.RANUENGAPID(1),
		AMFUENGAPID:       ngap.AMFUENGAPID(10),
		UERadioCapability: ngap.UERadioCapability{0xDE, 0xAD, 0xBE, 0xEF},
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

	HandleUERadioCapabilityInfoIndication(context.Background(), amfInstance, ran, &ngap.UERadioCapabilityInfoIndication{
		RANUENGAPID: ngap.RANUENGAPID(1),
		AMFUENGAPID: ngap.AMFUENGAPID(10),
		UERadioCapabilityForPaging: &ngap.UERadioCapabilityForPaging{
			NR:    &ngap.UERadioCapabilityForPagingOfNR{0xCA, 0xFE},
			EUTRA: &ngap.UERadioCapabilityForPagingOfEUTRA{0xBA, 0xBE},
		},
	})

	if amfUe.RadioCapabilityForPaging == nil {
		t.Fatal("RadioCapabilityForPaging is nil")
	}

	if !bytes.Equal(amfUe.RadioCapabilityForPaging.NR, []byte{0xCA, 0xFE}) {
		t.Errorf("NR = %x, want cafe", amfUe.RadioCapabilityForPaging.NR)
	}

	if !bytes.Equal(amfUe.RadioCapabilityForPaging.EUTRA, []byte{0xBA, 0xBE}) {
		t.Errorf("EUTRA = %x, want babe", amfUe.RadioCapabilityForPaging.EUTRA)
	}
}

// TS 38.413 §10.3.5
func TestUERadioCapabilityInfoIndication_AbsentCapabilityKeepsStored(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	amfUe := amf.NewUeContext()

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	stored := []byte{0x01, 0x02, 0x03, 0x04}
	amfUe.RadioCapability = stored
	amfUe.RadioCapabilityForPaging = &models.UERadioCapabilityForPaging{NR: []byte{0x0a}}

	HandleUERadioCapabilityInfoIndication(context.Background(), amfInstance, ran, &ngap.UERadioCapabilityInfoIndication{
		RANUENGAPID: ngap.RANUENGAPID(1),
		AMFUENGAPID: ngap.AMFUENGAPID(10),
	})

	if !bytes.Equal(amfUe.RadioCapability, stored) {
		t.Errorf("RadioCapability = %x, want the stored %x", amfUe.RadioCapability, stored)
	}

	if amfUe.RadioCapabilityForPaging == nil || !bytes.Equal(amfUe.RadioCapabilityForPaging.NR, []byte{0x0a}) {
		t.Errorf("RadioCapabilityForPaging = %+v, want the stored one", amfUe.RadioCapabilityForPaging)
	}
}
