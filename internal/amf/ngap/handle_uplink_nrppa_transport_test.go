// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
)

// TS 38.413 §8.10.4
func TestHandleUplinkNRPPaTransport_RoutingID(t *testing.T) {
	tests := []struct {
		name      string
		addressed []int64
		routingID ngap.RoutingID
		want      int
	}{
		{"a Routing ID this AMF addressed", []int64{7}, ngap.RoutingID{0, 0, 0, 7}, 1},
		{"one of several addressed", []int64{1, 7, 9}, ngap.RoutingID{0, 0, 0, 9}, 1},
		{"a Routing ID never addressed", []int64{7}, ngap.RoutingID{0xde, 0xad, 0xbe, 0xef}, 0},
		{"no LMF addressed at all", nil, ngap.RoutingID{0, 0, 0, 0}, 0},
		{"not the four-octet form this AMF emits", []int64{0}, ngap.RoutingID{0, 0}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amfInstance := newTestAMF()
			ran := newTestRadio(amfInstance)

			ue := amf.NewUeContext()
			ueConn := amf.NewUeConnForTest(ran, 2, 1, logger.AmfLog)
			ueConn.AMFForTest().AttachUeConn(ue, ueConn)

			for _, id := range tt.addressed {
				ue.RecordNRPPaRoutingID(id)
			}

			HandleUplinkUEAssociatedNRPPaTransport(context.Background(), amfInstance, ran, &ngap.UplinkUEAssociatedNRPPaTransport{
				AMFUENGAPID: 1,
				RANUENGAPID: 2,
				RoutingID:   tt.routingID,
				NRPPaPDU:    ngap.NRPPaPDU{0x01, 0x02},
			})

			if got := len(ue.GetNRPPaMessages()); got != tt.want {
				t.Fatalf("stored %d NRPPa PDUs, want %d", got, tt.want)
			}
		})
	}
}
