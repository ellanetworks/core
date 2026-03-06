// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleUEContextReleaseRequest_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	amf := newTestAMF()
	msg := &ngapType.UEContextReleaseRequest{}

	assertNoPanic(t, "HandleUEContextReleaseRequest(empty IEs)", func() {
		ngap.HandleUEContextReleaseRequest(context.Background(), amf, ran, msg)
	})
}
