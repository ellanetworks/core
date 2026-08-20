// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import type {
  FlowReport,
  FlowReportStatsResponse,
  ListFlowReportsResponse,
} from "@/queries/flow_reports";
import type {
  APIRadioEvent,
  ListRadioEventsResponse,
} from "@/queries/radio_events";
import type { UsageResult } from "@/queries/usage";

export const flowReport = (
  seed: number,
  overrides: Partial<FlowReport> = {},
): FlowReport => ({
  id: seed,
  subscriber_id: `00101${String(seed).padStart(10, "0")}`,
  source_ip: `10.45.0.${seed % 256}`,
  destination_ip: `93.184.216.${seed % 256}`,
  source_port: 40000 + seed,
  destination_port: 443,
  protocol: 6,
  packets: seed * 10,
  bytes: seed * 1000,
  start_time: "2026-08-01T10:00:00Z",
  end_time: "2026-08-01T10:01:00Z",
  direction: "uplink",
  action: "allow",
  ...overrides,
});

export const flowReportPage = (
  items: FlowReport[],
  overrides: Partial<Omit<ListFlowReportsResponse, "items">> = {},
): ListFlowReportsResponse => ({
  items,
  page: 1,
  per_page: 25,
  total_count: items.length,
  ...overrides,
});

export const flowStats = (
  overrides: Partial<FlowReportStatsResponse> = {},
): FlowReportStatsResponse => ({
  protocols: [
    { protocol: 6, count: 120 },
    { protocol: 17, count: 30 },
  ],
  top_destinations_uplink: [{ ip: "93.184.216.34", count: 90 }],
  ...overrides,
});

export const radioEvent = (
  seed: number,
  overrides: Partial<APIRadioEvent> = {},
): APIRadioEvent => ({
  id: seed,
  timestamp: "2026-08-01T10:00:00Z",
  protocol: "NGAP",
  message_type: "InitialUEMessage",
  direction: "incoming",
  radio: "radio-1",
  address: "10.0.0.1:38412",
  ...overrides,
});

export const radioEventPage = (
  items: APIRadioEvent[],
  overrides: Partial<Omit<ListRadioEventsResponse, "items">> = {},
): ListRadioEventsResponse => ({
  items,
  page: 1,
  per_page: 25,
  total_count: items.length,
  ...overrides,
});

export const usageBySubscriber = (
  entries: Record<string, number>,
): UsageResult =>
  Object.entries(entries).map(([imsi, total]) => ({
    [imsi]: {
      uplink_bytes: Math.floor(total / 4),
      downlink_bytes: total - Math.floor(total / 4),
      total_bytes: total,
    },
  }));

export const usageByDay = (entries: Record<string, number>): UsageResult =>
  usageBySubscriber(entries);
