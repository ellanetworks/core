// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandlePDUSessionResourceReleaseResponse_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	amf := newTestAMF()
	msg := &ngapType.PDUSessionResourceReleaseResponse{}

	assertNoPanic(t, "HandlePDUSessionResourceReleaseResponse(empty IEs)", func() {
		ngap.HandlePDUSessionResourceReleaseResponse(context.Background(), amf, ran, msg)
	})
}
