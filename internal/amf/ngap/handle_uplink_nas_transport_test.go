// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
)

func TestHandleUplinkNasTransport_UnknownRanUe(t *testing.T) {
	ran := newTestRadio()
	amf := newTestAMF()

	assertNoPanic(t, "HandleUplinkNasTransport(unknown RAN UE)", func() {
		ngap.HandleUplinkNasTransport(context.Background(), amf, ran, decode.UplinkNASTransport{
			AMFUENGAPID: 1,
			RANUENGAPID: 1,
			NASPDU:      []byte{0x7E, 0x00, 0x55},
		})
	})
}
