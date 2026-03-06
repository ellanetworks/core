// Copyright 2026 Ella Networks

package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleUERadioCapabilityInfoIndication_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	msg := &ngapType.UERadioCapabilityInfoIndication{}

	assertNoPanic(t, "HandleUERadioCapabilityInfoIndication(empty IEs)", func() {
		ngap.HandleUERadioCapabilityInfoIndication(ran, msg)
	})
}
