// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleHandoverCancel_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	msg := &ngapType.HandoverCancel{}

	assertNoPanic(t, "HandleHandoverCancel(empty IEs)", func() {
		ngap.HandleHandoverCancel(context.Background(), ran, msg)
	})
}
