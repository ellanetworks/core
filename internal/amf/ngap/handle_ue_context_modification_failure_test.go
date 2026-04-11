// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
)

func TestHandleUEContextModificationFailure_MissingAMFUENGAPID(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()
	ranUeNgapID := int64(1)
	msg := decode.UEContextModificationFailure{
		RANUENGAPID: &ranUeNgapID,
	}

	assertNoPanic(t, "HandleUEContextModificationFailure(missing AMFUENGAPID)", func() {
		ngap.HandleUEContextModificationFailure(context.Background(), amfInstance, ran, msg)
	})
}
