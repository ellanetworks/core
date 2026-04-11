// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
)

func TestHandlePDUSessionResourceReleaseResponse_MissingIDs(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	msg := decode.PDUSessionResourceReleaseResponse{}

	assertNoPanic(t, "HandlePDUSessionResourceReleaseResponse(missing IDs)", func() {
		ngap.HandlePDUSessionResourceReleaseResponse(context.Background(), amfInstance, ran, msg)
	})
}
