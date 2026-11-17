// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"maps"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
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

func TestHandleNAS_CountsTheDecipheredEMMMessageOnce(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.SetULCountForTest(0)

	msg := protectedUplink(t, ue, nas.MakeCount(0, 0).Value())

	before := nasCounts(t)

	HandleNAS(context.Background(), m, ue.Conn(), msg)
	HandleNAS(context.Background(), m, ue.Conn(), msg)

	want := map[nasKey]float64{
		{metrics.RAT4G, metrics.DirectionInbound, "EMMStatus"}:            1,
		{metrics.RAT4G, metrics.DirectionInbound, metrics.NASTypeUnknown}: 1,
	}

	if got := nasDelta(t, before); !maps.Equal(got, want) {
		t.Fatalf("app_nas_messages_total delta = %v, want %v", got, want)
	}
}

func TestHandleNAS_CountsADecipheredESMMessage(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.SetULCountForTest(0)

	plain := []byte{0x62, 0x03, byte(eps.MsgModifyEPSBearerContextReject), byte(eps.ESMCauseProtocolErrorUnspecified)}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nas.MakeCount(0, 0), nas.DirectionUplink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest()))
	if err != nil {
		t.Fatal(err)
	}

	before := nasCounts(t)

	HandleNAS(context.Background(), m, ue.Conn(), wire)

	inbound := map[nasKey]float64{}

	for k, v := range nasDelta(t, before) {
		if k.direction == metrics.DirectionInbound {
			inbound[k] = v
		}
	}

	want := map[nasKey]float64{{metrics.RAT4G, metrics.DirectionInbound, "ModifyEPSBearerContextReject"}: 1}

	if !maps.Equal(inbound, want) {
		t.Fatalf("inbound app_nas_messages_total delta = %v, want %v", inbound, want)
	}
}

func TestDownlinkNAS_CountsPlainProtectedAndResentMessages(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	accept := []byte{0x07, byte(eps.MsgAttachAccept), 0x01, 0x21, 0x03, 0x00, 0x00, 0x01, 0x00, 0x03, 0x52, 0x00, byte(eps.MsgActivateDefaultEPSBearerContextRequest)}

	before := nasCounts(t)
	sentBefore := cc.count()

	ue.Conn().SendDownlinkMessage(context.Background(), &eps.AuthenticationReject{})
	ue.Conn().SendDownlinkProtected(context.Background(), &eps.ServiceReject{Cause: eps.EMMCauseCongestion})

	ue.Conn().AttachAcceptPlain = accept
	ue.Conn().ResendAttachAccept(context.Background())
	ue.Conn().ResendAttachAccept(context.Background())
	ue.Conn().StopNASGuard(context.Background())

	want := map[nasKey]float64{
		{metrics.RAT4G, metrics.DirectionOutbound, "AuthenticationReject"}:                   1,
		{metrics.RAT4G, metrics.DirectionOutbound, "ServiceReject"}:                          1,
		{metrics.RAT4G, metrics.DirectionOutbound, "AttachAccept"}:                           2,
		{metrics.RAT4G, metrics.DirectionOutbound, "ActivateDefaultEPSBearerContextRequest"}: 2,
	}

	if got := nasDelta(t, before); !maps.Equal(got, want) {
		t.Fatalf("app_nas_messages_total delta = %v, want %v", got, want)
	}

	if got := cc.count() - sentBefore; got != 4 {
		t.Fatalf("S1AP messages sent = %d, want 4", got)
	}
}
