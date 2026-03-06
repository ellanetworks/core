// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleUEContextReleaseComplete_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	amf := newTestAMF()
	msg := &ngapType.UEContextReleaseComplete{}

	assertNoPanic(t, "HandleUEContextReleaseComplete(empty IEs)", func() {
		ngap.HandleUEContextReleaseComplete(context.Background(), amf, ran, msg)
	})
}
