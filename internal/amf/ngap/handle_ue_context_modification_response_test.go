// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
)

func TestHandleUEContextModificationResponse_MissingAMFUENGAPID(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	ranID := int64(1)
	msg := decode.UEContextModificationResponse{
		RANUENGAPID: &ranID,
	}

	assertNoPanic(t, "HandleUEContextModificationResponse(missing AMFUENGAPID)", func() {
		ngap.HandleUEContextModificationResponse(context.Background(), amfInstance, ran, msg)
	})
}
