// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"maps"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/prometheus/client_golang/prometheus"
)

var registerMetricsOnce sync.Once

type nasKey struct{ rat, direction, msgType string }

func nasCounts(t *testing.T) map[nasKey]float64 {
	t.Helper()

	registerMetricsOnce.Do(metrics.RegisterMetrics)

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	out := map[nasKey]float64{}

	for _, f := range families {
		if f.GetName() != "app_nas_messages_total" {
			continue
		}

		for _, m := range f.GetMetric() {
			var k nasKey

			for _, l := range m.GetLabel() {
				switch l.GetName() {
				case "rat":
					k.rat = l.GetValue()
				case "direction":
					k.direction = l.GetValue()
				case "type":
					k.msgType = l.GetValue()
				}
			}

			out[k] = m.GetCounter().GetValue()
		}
	}

	return out
}

func nasDelta(t *testing.T, before map[nasKey]float64) map[nasKey]float64 {
	t.Helper()

	delta := map[nasKey]float64{}

	for k, v := range nasCounts(t) {
		if d := v - before[k]; d != 0 {
			delta[k] = d
		}
	}

	return delta
}

func bareUeConn(t *testing.T) (*amf.AMF, *amf.UeConn, *fakeNGAPSender) {
	t.Helper()

	ngapSender := &fakeNGAPSender{}
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: `["000001"]`},
	}, nil, nil)
	radio := amf.Radio{Conn: ngapSender}
	radio.BindAMFForTest(amfInstance)

	ueConn, err := amfInstance.NewUeConn(&radio, 0)
	if err != nil {
		t.Fatalf("could not create ueConn: %v", err)
	}

	return amfInstance, ueConn, ngapSender
}

func TestHandleNAS_CountsNoContextServiceRequestAndItsReject(t *testing.T) {
	amfInstance, ueConn, ngapSender := bareUeConn(t)

	before := nasCounts(t)

	HandleNAS(context.Background(), amfInstance, ueConn, encodePlainServiceRequest(t))

	want := map[nasKey]float64{
		{metrics.RAT5G, metrics.DirectionInbound, "ServiceRequest"}: 1,
		{metrics.RAT5G, metrics.DirectionOutbound, "ServiceReject"}: 1,
	}

	if got := nasDelta(t, before); !maps.Equal(got, want) {
		t.Fatalf("app_nas_messages_total delta = %v, want %v", got, want)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("downlink NAS transports = %d, want 1", len(ngapSender.SentDownlinkNASTransport))
	}
}

func TestHandleNAS_CountsAnUnreadableCipheredMessageAsUnknown(t *testing.T) {
	amfInstance, ueConn, ngapSender := bareUeConn(t)

	ciphered := []byte{0x7E, byte(fgs.SHTIntegrityProtectedCiphered), 0x01, 0x02, 0x03, 0x04, 0x00, 0x55, 0x66, 0x77}

	before := nasCounts(t)

	HandleNAS(context.Background(), amfInstance, ueConn, ciphered)

	want := map[nasKey]float64{
		{metrics.RAT5G, metrics.DirectionInbound, metrics.NASTypeUnknown}: 1,
		{metrics.RAT5G, metrics.DirectionOutbound, "5GMMStatus"}:          1,
	}

	if got := nasDelta(t, before); !maps.Equal(got, want) {
		t.Fatalf("app_nas_messages_total delta = %v, want %v", got, want)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("downlink NAS transports = %d, want 1", len(ngapSender.SentDownlinkNASTransport))
	}
}

func TestHandleNAS_CountsTheDecipheredMessageAndItsSessionPayloadOnce(t *testing.T) {
	f := connectedModeUe(t, &fakeSmf{})

	inner, err := (&fgs.ULNASTransport{
		PayloadContainerType: fgs.PayloadContainerTypeN1SMInfo,
		PayloadContainer:     []byte{0x2E, 0x0C, 0x01, byte(fgs.MsgPDUSessionReleaseRequest)},
		PDUSessionID:         new(fgs.PDUSessionID(12)),
	}).MarshalBinary()
	if err != nil {
		t.Fatalf("marshal UL NAS transport: %v", err)
	}

	cnt, err := f.ue.ULCountForTest().Estimate(0)
	if err != nil {
		t.Fatalf("estimate uplink count: %v", err)
	}

	sc := mustSecurityContext(t, f.ue.IntegrityAlgForTest(), f.ue.CipheringAlgForTest(), f.ue.KnasIntForTest(), f.ue.KnasEncForTest())

	wire, err := fgs.Protect(inner, fgs.SHTIntegrityProtectedCiphered, cnt, nas.DirectionUplink, sc)
	if err != nil {
		t.Fatalf("protect: %v", err)
	}

	before := nasCounts(t)

	HandleNAS(t.Context(), f.amf, f.conn(), wire)

	inbound := map[nasKey]float64{}

	for k, v := range nasDelta(t, before) {
		if k.direction == metrics.DirectionInbound {
			inbound[k] = v
		}
	}

	want := map[nasKey]float64{
		{metrics.RAT5G, metrics.DirectionInbound, "ULNASTransport"}:           1,
		{metrics.RAT5G, metrics.DirectionInbound, "PDUSessionReleaseRequest"}: 1,
	}

	if !maps.Equal(inbound, want) {
		t.Fatalf("inbound app_nas_messages_total delta = %v, want %v", inbound, want)
	}
}

func TestSendDownlinkNAS_CountsEachTransmission(t *testing.T) {
	f := connectedModeUe(t, &fakeSmf{})

	plain, err := amf.BuildStatus5GMM(fgs.GMMCauseProtocolErrorUnspecified)
	if err != nil {
		t.Fatalf("build 5GMM STATUS: %v", err)
	}

	before := nasCounts(t)

	for range 2 {
		if err := f.ue.SendDownlinkNAS(plain, uint8(fgs.SHTIntegrityProtectedCiphered), func(wire []byte) error {
			return f.conn().SendDownlinkNASTransport(t.Context(), wire)
		}); err != nil {
			t.Fatalf("send: %v", err)
		}
	}

	want := map[nasKey]float64{{metrics.RAT5G, metrics.DirectionOutbound, "5GMMStatus"}: 2}

	if got := nasDelta(t, before); !maps.Equal(got, want) {
		t.Fatalf("app_nas_messages_total delta = %v, want %v", got, want)
	}

	if len(f.ngapSender.SentDownlinkNASTransport) != 2 {
		t.Fatalf("downlink NAS transports = %d, want 2", len(f.ngapSender.SentDownlinkNASTransport))
	}
}
