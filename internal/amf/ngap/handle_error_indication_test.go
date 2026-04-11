// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
)

func TestHandleErrorIndication_EmptyIEs(t *testing.T) {
	ran := newTestRadio()

	assertNoPanic(t, "HandleErrorIndication(empty IEs)", func() {
		ngap.HandleErrorIndication(context.Background(), ran, decode.ErrorIndication{})
	})
}
