// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/dbwriter"
)

func TestFlowReportStatsCountHomeSubscribersOnly(t *testing.T) {
	f := newRoamingFixture(t)
	f.createPartner(t)
	f.createHomeSubscriber(t, "001019000000001")
	f.registerVisitor(t, "208930000000001")

	ctx := context.Background()
	now := time.Now().UTC()

	flow := func(imsi, dst string, protocol uint8) *dbwriter.FlowReport {
		return &dbwriter.FlowReport{
			SubscriberID: imsi, SourceIP: "10.45.0.2", DestinationIP: dst, SourcePort: 1000, DestinationPort: 443, Protocol: protocol,
			Packets: 1, Bytes: 100, StartTime: dbwriter.EpochMillis(now.Add(-time.Minute)), EndTime: dbwriter.EpochMillis(now), Direction: "uplink",
		}
	}

	if err := f.db.InsertFlowReports(ctx, []*dbwriter.FlowReport{flow("001019000000001", "192.0.2.1", 6), flow("208930000000001", "192.0.2.2", 17)}); err != nil {
		t.Fatalf("InsertFlowReports: %s", err)
	}

	protocols, destinations, err := f.db.GetFlowReportStats(ctx, nil)
	if err != nil {
		t.Fatalf("GetFlowReportStats: %s", err)
	}

	if len(protocols) != 1 || protocols[0].Protocol != 6 || len(destinations) != 1 || destinations[0].IP != "192.0.2.1" {
		t.Fatalf("stats = %+v %+v, want the home subscriber's flow only", protocols, destinations)
	}

	_, total, err := f.db.ListFlowReports(ctx, 1, 10, nil)
	if err != nil || total != 2 {
		t.Fatalf("ListFlowReports total = %d, %v, want the roamer's flow kept in the log", total, err)
	}
}

func TestUsageAggregatesCountHomeSubscribersOnly(t *testing.T) {
	f := newRoamingFixture(t)
	f.createPartner(t)
	f.createHomeSubscriber(t, "001019000000001")
	f.registerVisitor(t, "208930000000001")

	ctx := context.Background()
	now := time.Now().UTC()
	day := db.DaysSinceEpoch(now)

	for _, usage := range []db.DailyUsage{
		{EpochDay: day, IMSI: "001019000000001", BytesUplink: 100, BytesDownlink: 200},
		{EpochDay: day, IMSI: "208930000000001", BytesUplink: 1000, BytesDownlink: 2000},
	} {
		if err := f.db.IncrementDailyUsage(ctx, usage); err != nil {
			t.Fatalf("IncrementDailyUsage: %s", err)
		}
	}

	days := db.NewDayRange(now.AddDate(0, 0, -1), now)

	perDay, err := f.db.GetUsagePerDay(ctx, "", days)
	if err != nil {
		t.Fatalf("GetUsagePerDay: %s", err)
	}

	if len(perDay) != 1 || perDay[0].BytesUplink != 100 || perDay[0].BytesDownlink != 200 {
		t.Fatalf("per day = %+v, want the home subscriber's usage only", perDay)
	}

	roamerDay, err := f.db.GetUsagePerDay(ctx, "208930000000001", days)
	if err != nil || len(roamerDay) != 0 {
		t.Fatalf("per day of roamer = %+v, %v, want none", roamerDay, err)
	}

	perSubscriber, err := f.db.GetUsagePerSubscriber(ctx, "", days, db.NoUsageLimit)
	if err != nil {
		t.Fatalf("GetUsagePerSubscriber: %s", err)
	}

	if len(perSubscriber) != 1 || perSubscriber[0].IMSI != "001019000000001" || perSubscriber[0].BytesUplink != 100 {
		t.Fatalf("per subscriber = %+v, want the home subscriber only", perSubscriber)
	}
}
