// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

func scaleSubscriberCount(t *testing.T) int {
	t.Helper()

	raw := os.Getenv("ELLA_SCALE_SUBSCRIBERS")
	if raw == "" {
		t.Skip("set ELLA_SCALE_SUBSCRIBERS (e.g. 100000) to run the subscriber scale test")
	}

	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		t.Fatalf("invalid ELLA_SCALE_SUBSCRIBERS %q", raw)
	}

	return n
}

func setupScaleDB(t *testing.T) (*db.Database, *db.DataNetwork, *db.Profile) {
	t.Helper()

	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabase: %s", err)
	}

	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Close: %s", err)
		}
	})

	if err := database.CreateDataNetwork(ctx, &db.DataNetwork{Name: "scale-dn", IPv4Pool: "10.0.0.0/14", DNS: "8.8.8.8", MTU: 1400}); err != nil {
		t.Fatalf("CreateDataNetwork: %s", err)
	}

	dn, err := database.GetDataNetwork(ctx, "scale-dn")
	if err != nil {
		t.Fatalf("GetDataNetwork: %s", err)
	}

	if err := database.CreateProfile(ctx, &db.Profile{Name: "scale-profile", UeAmbrUplink: "200 Mbps", UeAmbrDownlink: "200 Mbps"}); err != nil {
		t.Fatalf("CreateProfile: %s", err)
	}

	profile, err := database.GetProfile(ctx, "scale-profile")
	if err != nil {
		t.Fatalf("GetProfile: %s", err)
	}

	if err := database.CreateNetworkSlice(ctx, &db.NetworkSlice{Name: "scale-slice", Sst: 1}); err != nil {
		t.Fatalf("CreateNetworkSlice: %s", err)
	}

	slice, err := database.GetNetworkSlice(ctx, "scale-slice")
	if err != nil {
		t.Fatalf("GetNetworkSlice: %s", err)
	}

	if err := database.CreatePolicy(ctx, &db.Policy{Name: "scale-policy", DataNetworkID: dn.ID, ProfileID: profile.ID, SliceID: slice.ID}); err != nil {
		t.Fatalf("CreatePolicy: %s", err)
	}

	return database, dn, profile
}

func scaleIMSI(i int) string {
	return fmt.Sprintf("00101%010d", i)
}

func timeIt(t *testing.T, label string, fn func()) {
	t.Helper()

	start := time.Now()

	fn()

	t.Logf("%-45s %v", label, time.Since(start))
}

func TestScaleSubscribers(t *testing.T) {
	n := scaleSubscriberCount(t)
	ctx := context.Background()
	database, dn, profile := setupScaleDB(t)

	timeIt(t, fmt.Sprintf("create %d subscribers", n), func() {
		for i := range n {
			sub := &db.Subscriber{
				Imsi:           scaleIMSI(i),
				SequenceNumber: "000000000001",
				PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
				Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
				ProfileID:      profile.ID,
			}

			if err := database.CreateSubscriber(ctx, sub); err != nil {
				t.Fatalf("CreateSubscriber %d: %s", i, err)
			}
		}
	})

	timeIt(t, "count subscribers", func() {
		count, err := database.CountSubscribers(ctx)
		if err != nil || count != n {
			t.Fatalf("CountSubscribers = %d, %v", count, err)
		}
	})

	timeIt(t, "list page 1 (per_page 25)", func() {
		subs, total, err := database.ListSubscribersPage(ctx, nil, 1, 25)
		if err != nil || total != n || len(subs) != min(25, n) {
			t.Fatalf("ListSubscribersPage = %d items, total %d, %v", len(subs), total, err)
		}
	})

	timeIt(t, "list last page (per_page 100)", func() {
		page := (n + 99) / 100

		subs, total, err := database.ListSubscribersPage(ctx, nil, page, 100)
		if err != nil || total != n || len(subs) == 0 {
			t.Fatalf("ListSubscribersPage = %d items, total %d, %v", len(subs), total, err)
		}
	})

	timeIt(t, "list page 1 filtered by data network", func() {
		_, total, err := database.ListSubscribersPage(ctx, &db.SubscriberFilters{DataNetworkID: &dn.ID}, 1, 25)
		if err != nil {
			t.Fatalf("ListSubscribersPage: %s", err)
		}

		if total != 0 {
			t.Logf("data network filter total = %d", total)
		}
	})

	search := scaleIMSI(n - 1)[8:]

	timeIt(t, "list page 1 with search", func() {
		_, total, err := database.ListSubscribersPage(ctx, &db.SubscriberFilters{Search: &search}, 1, 25)
		if err != nil || total < 1 {
			t.Fatalf("ListSubscribersPage search = %d, %v", total, err)
		}
	})

	connected := make([]string, 0, n/2)
	for i := 0; i < n; i += 2 {
		connected = append(connected, scaleIMSI(i))
	}

	timeIt(t, fmt.Sprintf("list page 1 filtered by %d IMSIs", len(connected)), func() {
		_, total, err := database.ListSubscribersPage(ctx, &db.SubscriberFilters{IMSIs: connected}, 1, 25)
		if err != nil || total != len(connected) {
			t.Fatalf("ListSubscribersPage IMSIs = %d, %v", total, err)
		}
	})

	timeIt(t, "get one subscriber", func() {
		if _, err := database.GetSubscriber(ctx, scaleIMSI(n/2)); err != nil {
			t.Fatalf("GetSubscriber: %s", err)
		}
	})

	today := db.DaysSinceEpoch(time.Now())

	timeIt(t, fmt.Sprintf("record usage for %d subscribers x 7 days", n), func() {
		const batch = 1000

		for day := today - 6; day <= today; day++ {
			rows := make([]db.DailyUsage, 0, batch)

			for i := range n {
				rows = append(rows, db.DailyUsage{EpochDay: day, IMSI: scaleIMSI(i), BytesUplink: int64(i), BytesDownlink: int64(2 * i)})
				if len(rows) == batch || i == n-1 {
					if err := database.IncrementDailyUsageBatch(ctx, rows); err != nil {
						t.Fatalf("IncrementDailyUsageBatch: %s", err)
					}

					rows = rows[:0]
				}
			}
		}
	})

	days := db.DayRange{First: today - 6, Last: today}

	timeIt(t, "usage per subscriber, top 10", func() {
		usage, err := database.GetUsagePerSubscriber(ctx, "", days, 10)
		if err != nil || len(usage) != min(10, n) {
			t.Fatalf("GetUsagePerSubscriber = %d, %v", len(usage), err)
		}
	})

	timeIt(t, "usage per subscriber, unbounded", func() {
		usage, err := database.GetUsagePerSubscriber(ctx, "", days, db.NoUsageLimit)
		if err != nil || len(usage) != n {
			t.Fatalf("GetUsagePerSubscriber = %d, %v", len(usage), err)
		}
	})

	timeIt(t, "usage per subscriber, one IMSI", func() {
		if _, err := database.GetUsagePerSubscriber(ctx, scaleIMSI(n/2), days, db.NoUsageLimit); err != nil {
			t.Fatalf("GetUsagePerSubscriber: %s", err)
		}
	})

	timeIt(t, "usage per day, all subscribers", func() {
		if _, err := database.GetUsagePerDay(ctx, "", days); err != nil {
			t.Fatalf("GetUsagePerDay: %s", err)
		}
	})

	timeIt(t, "usage per day, one IMSI", func() {
		if _, err := database.GetUsagePerDay(ctx, scaleIMSI(n/2), days); err != nil {
			t.Fatalf("GetUsagePerDay: %s", err)
		}
	})

	step := max(n/10, 1)
	allocStart := time.Now()

	for i := range n {
		if _, err := database.AllocateIPLease(ctx, dn.ID, "ipv4", scaleIMSI(i), 1, "1"); err != nil {
			t.Fatalf("AllocateIPLease %d: %s", i, err)
		}

		if (i+1)%step == 0 {
			t.Logf("%-45s %v", fmt.Sprintf("allocate IP leases %d..%d", i+1-step, i+1), time.Since(allocStart))
			allocStart = time.Now()
		}
	}

	timeIt(t, "release and re-allocate 1000 leases", func() {
		for i := 0; i < n; i += max(n/1000, 1) {
			if _, err := database.ReleaseIPLease(ctx, dn.ID, "ipv4", scaleIMSI(i), 1, "1"); err != nil {
				t.Fatalf("ReleaseIPLease: %s", err)
			}

			if _, err := database.AllocateIPLease(ctx, dn.ID, "ipv4", scaleIMSI(i), 2, "1"); err != nil {
				t.Fatalf("AllocateIPLease: %s", err)
			}
		}
	})

	timeIt(t, "count leases by pool", func() {
		count, err := database.CountLeasesByPool(ctx, dn.ID, "ipv4")
		if err != nil || count != n {
			t.Fatalf("CountLeasesByPool = %d, %v", count, err)
		}
	})

	timeIt(t, "list leases by pool, page 1", func() {
		if _, _, err := database.ListLeasesByPoolPage(ctx, dn.ID, "ipv4", 1, 25); err != nil {
			t.Fatalf("ListLeasesByPoolPage: %s", err)
		}
	})

	timeIt(t, "delete one subscriber", func() {
		if err := database.DeleteSubscriber(ctx, scaleIMSI(n/3)); err != nil {
			t.Fatalf("DeleteSubscriber: %s", err)
		}
	})
}
