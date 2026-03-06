// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleUplinkRanConfigurationTransfer_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	amf := newTestAMF()
	msg := &ngapType.UplinkRANConfigurationTransfer{}

	assertNoPanic(t, "HandleUplinkRanConfigurationTransfer(empty IEs)", func() {
		ngap.HandleUplinkRanConfigurationTransfer(context.Background(), amf, ran, msg)
	})
}
