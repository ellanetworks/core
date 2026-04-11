// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
)

func TestHandleInitialContextSetupFailure_MissingCause(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()
	msg := decode.InitialContextSetupFailure{
		AMFUENGAPID: 1,
		RANUENGAPID: 1,
	}

	assertNoPanic(t, "HandleInitialContextSetupFailure(missing cause)", func() {
		ngap.HandleInitialContextSetupFailure(context.Background(), amfInstance, ran, msg)
	})
}
