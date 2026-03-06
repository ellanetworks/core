// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleHandoverFailure_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	amf := newTestAMF()
	msg := &ngapType.HandoverFailure{}

	assertNoPanic(t, "HandleHandoverFailure(empty IEs)", func() {
		ngap.HandleHandoverFailure(context.Background(), amf, ran, msg)
	})
}

func TestHandleHandoverFailure_MissingCause(t *testing.T) {
	ran := newTestRadio()
	amf := newTestAMF()
	msg := &ngapType.HandoverFailure{}
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.HandoverFailureIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.HandoverFailureIEsValue{
			Present:     ngapType.HandoverFailureIEsPresentAMFUENGAPID,
			AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 1},
		},
	})

	assertNoPanic(t, "HandleHandoverFailure(missing cause)", func() {
		ngap.HandleHandoverFailure(context.Background(), amf, ran, msg)
	})
}
