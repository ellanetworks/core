// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

func TestPDNBearerWriteVsStatusNoRace(t *testing.T) {
	m := newTestMME(t)
	ue := m.NewUe(t.Context(), &captureConn{}, 7)

	ueAmbr := models.Ambr{Uplink: models.MustParseBitRate("1 Gbps"), Downlink: models.MustParseBitRate("1 Gbps")}
	bearer := models.EPSBearer{PDNType: 1, IPv4: netip.MustParseAddr("10.0.0.1"), QoS: models.EPSBearerQoS{
		QCI: 9, ARP: 1, APNAMBR: models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("200 Mbps")},
	}}

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()

		for range 500 {
			m.InstallDefaultBearer(ue, ueAmbr, "internet", bearer, false)
		}
	}()

	go func() {
		defer wg.Done()

		for range 500 {
			_ = m.pdnSessionViews(ue)

			_, _ = ue.AmbrRates()
		}
	}()

	wg.Wait()
}
