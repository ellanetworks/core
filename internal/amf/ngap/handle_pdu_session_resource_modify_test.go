// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandlePDUSessionResourceNotify_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	amf := newTestAMF()
	msg := &ngapType.PDUSessionResourceNotify{}

	assertNoPanic(t, "HandlePDUSessionResourceNotify(empty IEs)", func() {
		ngap.HandlePDUSessionResourceNotify(context.Background(), amf, ran, msg)
	})
}
