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

func TestResolveQoSByAPNReadsDataNetworkOnce(t *testing.T) {
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

			qos, err := ResolveQoSByAPN(context.Background(), m, testSubscriber.IMSI, tc.apn)
			if err != nil {
				t.Fatalf("ResolveQoSByAPN: %v", err)
			}

			if qos.APN != tc.apn {
				t.Fatalf("APN = %q, want %q", qos.APN, tc.apn)
			}

			if got := store.dataNetworkReads.Load(); got != tc.examined {
				t.Fatalf("read the data network %d times, want %d", got, tc.examined)
			}
		})
	}
}

// TS 24.301 §6.5.1.3
func TestResolveAttachQoSDefaultWhenNoAPN(t *testing.T) {
	m := newTestMME(t)
	ue := &UeContext{supi: mustSUPI(testSubscriber.IMSI)}

	qos, err := ResolveAttachQoS(context.Background(), m, ue)
	if err != nil {
		t.Fatalf("ResolveAttachQoS: %v", err)
	}

	if qos.APN != "internet" {
		t.Errorf("APN = %q, want the default %q", qos.APN, "internet")
	}
}

func TestResolveAttachQoSSelectsRequestedAPN(t *testing.T) {
	m := newTestMME(t)
	ue := &UeContext{supi: mustSUPI(testSubscriber.IMSI), RequestedAPN: "ims"}

	qos, err := ResolveAttachQoS(context.Background(), m, ue)
	if err != nil {
		t.Fatalf("ResolveAttachQoS: %v", err)
	}

	if qos.APN != "ims" {
		t.Errorf("APN = %q, want the requested %q", qos.APN, "ims")
	}

	if qos.IPv4Pool != "10.46.0.0/16" {
		t.Errorf("IPv4Pool = %q, want the ims pool 10.46.0.0/16", qos.IPv4Pool)
	}
}

func TestResolveAttachQoSRejectsUnknownAPN(t *testing.T) {
	m := newTestMME(t)
	ue := &UeContext{supi: mustSUPI(testSubscriber.IMSI), RequestedAPN: "nonexistent"}

	if _, err := ResolveAttachQoS(context.Background(), m, ue); !errors.Is(err, ErrUnknownAPN) {
		t.Fatalf("ResolveAttachQoS error = %v, want ErrUnknownAPN", err)
	}
}
