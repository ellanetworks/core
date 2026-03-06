// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandlePDUSessionResourceModifyIndication_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	msg := &ngapType.PDUSessionResourceModifyIndication{}

	assertNoPanic(t, "HandlePDUSessionResourceModifyIndication(empty IEs)", func() {
		ngap.HandlePDUSessionResourceModifyIndication(context.Background(), ran, msg)
	})
}
