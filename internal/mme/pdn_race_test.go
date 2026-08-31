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
	ue := m.NewUe(&captureConn{}, 7)

	qos := &EpsQoS{APN: "internet", SessAmbrUL: models.MustParseBitRate("100 Mbps"), SessAmbrDL: models.MustParseBitRate("200 Mbps"), QCI: 9, ARP: 1}
	bearer := models.EPSBearer{PDNType: 1, IPv4: netip.MustParseAddr("10.0.0.1")}

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()

		for range 500 {
			m.InstallDefaultBearer(ue, qos, bearer)
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
