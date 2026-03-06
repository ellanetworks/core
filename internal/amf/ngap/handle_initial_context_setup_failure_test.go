// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleInitialContextSetupFailure_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	msg := &ngapType.InitialContextSetupFailure{}

	assertNoPanic(t, "HandleInitialContextSetupFailure(empty IEs)", func() {
		ngap.HandleInitialContextSetupFailure(context.Background(), ran, msg)
	})
}

func TestHandleInitialContextSetupFailure_MissingCause(t *testing.T) {
	ran := newTestRadio()
	msg := &ngapType.InitialContextSetupFailure{}
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.InitialContextSetupFailureIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.InitialContextSetupFailureIEsValue{
			Present:     ngapType.InitialContextSetupFailureIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 1},
		},
	})

	assertNoPanic(t, "HandleInitialContextSetupFailure(missing cause)", func() {
		ngap.HandleInitialContextSetupFailure(context.Background(), ran, msg)
	})
}
