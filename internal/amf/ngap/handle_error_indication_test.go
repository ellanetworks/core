// Copyright 2026 Ella Networks

package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleErrorIndication_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	msg := &ngapType.ErrorIndication{}

	assertNoPanic(t, "HandleErrorIndication(empty IEs)", func() {
		ngap.HandleErrorIndication(ran, msg)
	})
}
