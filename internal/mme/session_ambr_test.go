// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

func TestSubscribedUEAMBRIsTheProfiles(t *testing.T) {
	m := newTestMME(t)

	ambr, err := SubscribedUEAMBR(context.Background(), m, testSubscriber.IMSI)
	if err != nil {
		t.Fatal(err)
	}

	want := models.MustParseBitRate("1 Gbps")
	if !ambr.Uplink.Equal(want) || !ambr.Downlink.Equal(want) {
		t.Errorf("UE-AMBR = %s/%s, want the profile's 1 Gbps", ambr.Uplink, ambr.Downlink)
	}
}
