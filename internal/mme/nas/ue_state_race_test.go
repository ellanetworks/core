// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"net/netip"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
)

func TestUEStateConcurrentAccess(t *testing.T) {
	m := newTestMME(t)
	ue, _ := connectedBearerUE(t, m)

	ctx := context.Background()

	const iters = 1500

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		mod := models.EPSBearerModification{DNS: netip.MustParseAddr("9.9.9.9")}

		for i := 0; i < iters; i++ {
			_ = m.ModifyEPSBearer(ctx, ue.IMSI(), mme.DefaultERABID, mod)
		}
	}()

	wg.Add(1)

	go func() {
		defer wg.Done()

		for i := 0; i < iters; i++ {
			_ = m.ConnectedSubscribers()
		}
	}()

	wg.Add(1)

	go func() {
		defer wg.Done()

		rej := &eps.ModifyEPSBearerContextReject{}

		for i := 0; i < iters; i++ {
			handleModifyBearerReject(context.Background(), m, ue, ue.Conn(), rej)
		}
	}()

	wg.Add(1)

	go func() {
		defer wg.Done()

		for i := 0; i < iters; i++ {
			ue.ForceStateForTest(mme.EMMRegistered)
		}
	}()

	wg.Wait()
}

func TestS1IdentityConcurrentSendVsResume(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	cc2 := &captureConn{}

	const iters = 2000

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		for i := 0; i < iters; i++ {
			establishResumeForTest(m, ue, cc2, 9)
		}
	}()

	wg.Add(1)

	go func() {
		defer wg.Done()

		for i := 0; i < iters; i++ {
			ue.Conn().SendDownlinkNASTransport(context.Background(), []byte{0x07, 0x42})
		}
	}()

	wg.Wait()
}
