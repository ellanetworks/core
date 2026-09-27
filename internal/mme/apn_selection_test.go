// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/udm"
)

type countingBearerStore struct {
	fakeBearerStore
	dataNetworkReads atomic.Int64
}

func (s *countingBearerStore) GetDataNetworkByID(ctx context.Context, id string) (*db.DataNetwork, error) {
	s.dataNetworkReads.Add(1)

	return s.fakeBearerStore.GetDataNetworkByID(ctx, id)
}

func TestSubscribedAPNReadsDataNetworkOnce(t *testing.T) {
	for _, tc := range []struct {
		apn      string
		examined int64
	}{
		{apn: "internet", examined: 1},
		{apn: "ims", examined: 2},
	} {
		t.Run(tc.apn, func(t *testing.T) {
			store := &countingBearerStore{}
			m := New(udm.New(newFakeCredStore(), noopKeyResolver), store, &fakeSessionManager{})

			apn, err := SubscribedAPN(context.Background(), m, testSubscriber.IMSI, tc.apn)
			if err != nil {
				t.Fatalf("SubscribedAPN: %v", err)
			}

			if apn != tc.apn {
				t.Fatalf("APN = %q, want %q", apn, tc.apn)
			}

			if got := store.dataNetworkReads.Load(); got != tc.examined {
				t.Fatalf("read the data network %d times, want %d", got, tc.examined)
			}
		})
	}
}

// TS 23.401 §5.3.2.1
func TestSubscribedAPNDefaultWhenNoAPN(t *testing.T) {
	m := newTestMME(t)

	apn, err := SubscribedAPN(context.Background(), m, testSubscriber.IMSI, "")
	if err != nil {
		t.Fatalf("SubscribedAPN: %v", err)
	}

	if apn != "internet" {
		t.Errorf("APN = %q, want the default %q", apn, "internet")
	}
}

func TestSubscribedAPNRejectsUnknownAPN(t *testing.T) {
	m := newTestMME(t)

	if _, err := SubscribedAPN(context.Background(), m, testSubscriber.IMSI, "nonexistent"); !errors.Is(err, ErrUnknownAPN) {
		t.Fatalf("SubscribedAPN error = %v, want ErrUnknownAPN", err)
	}
}
