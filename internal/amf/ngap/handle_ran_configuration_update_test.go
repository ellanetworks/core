// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleRanConfigurationUpdate_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	amf := newTestAMF()
	msg := &ngapType.RANConfigurationUpdate{}

	assertNoPanic(t, "HandleRanConfigurationUpdate(empty IEs)", func() {
		ngap.HandleRanConfigurationUpdate(context.Background(), amf, ran, msg)
	})
}
