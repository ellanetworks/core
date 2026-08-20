// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, beforeEach } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
import {
  flowReport,
  flowReportPage,
  flowStats,
  usageBySubscriber,
} from "@/test/fixtures";
import type { FlowReport } from "@/queries/flow_reports";
import type { UsageResult } from "@/queries/usage";
import Traffic from "./Traffic";

const api = setupApiServer();

const IMSI_A = "001010000000001";
const IMSI_B = "001010000000002";

const FLOWS_PATH = "/api/v1/flow-reports";
const USAGE_PATH = "/api/v1/subscriber-usage";

type Seed = {
  flows?: FlowReport[];
  totalCount?: number;
  usageBySub?: UsageResult;
  usagePerDay?: UsageResult;
  imsis?: string[];
};

const seedApi = ({
  flows = [flowReport(1)],
  totalCount,
  usageBySub = usageBySubscriber({ [IMSI_A]: 4_000 }),
  usagePerDay = usageBySubscriber({ "2026-08-01": 4_000 }),
  imsis = [IMSI_A, IMSI_B],
}: Seed = {}) => {
  api.get(USAGE_PATH, ({ params }) =>
    params.get("group_by") === "day" ? usagePerDay : usageBySub,
  );
  api.get("/api/v1/subscriber-usage/retention", () => ({ days: 30 }));
  api.get(FLOWS_PATH, () =>
    flowReportPage(flows, { total_count: totalCount ?? flows.length }),
  );
  api.get("/api/v1/flow-reports/retention", () => ({ days: 30 }));
  api.get("/api/v1/flow-reports/stats", () => flowStats());
  api.get("/api/v1/networking/flow-accounting", () => ({ enabled: true }));
  api.get("/api/v1/subscribers", () => ({
    items: imsis.map((imsi) => ({
      imsi,
      profile_name: "default",
      status: {},
    })),
    page: 1,
    per_page: 100,
    total_count: imsis.length,
  }));
};

const renderTraffic = async (path = "/traffic/flows") => {
  const result = renderWithProviders(<Traffic />, {
    initialEntries: [path],
    auth: {},
  });
  await screen.findByRole("heading", { name: "Traffic" });
  return result;
};

const flowRequests = () => api.requests(FLOWS_PATH);

const lastFlowParams = () => {
  const request = api.lastRequest(FLOWS_PATH);
  if (!request) throw new Error("no flow-reports request was made");
  return Object.fromEntries(request.params);
};

const waitForFlowRequests = (count: number) =>
  waitFor(() => expect(flowRequests().length).toBeGreaterThanOrEqual(count));

const selectOption = async (
  user: ReturnType<typeof userEvent.setup>,
  label: string,
  option: string,
) => {
  await user.click(screen.getByRole("combobox", { name: label }));
  await user.click(
    within(await screen.findByRole("listbox")).getByRole("option", {
      name: option,
    }),
  );
};

const chooseSubscriber = async (
  user: ReturnType<typeof userEvent.setup>,
  imsi: string,
) => {
  await user.click(screen.getByRole("combobox", { name: /Subscriber/ }));
  await user.click(await screen.findByRole("option", { name: imsi }));
};

beforeEach(() => {
  seedApi();
});

describe("Traffic flow filters", () => {
  it("omits every filter the operator has not set", async () => {
    await renderTraffic();
    await waitForFlowRequests(1);

    const params = lastFlowParams();

    expect(params).not.toHaveProperty("direction");
    expect(params).not.toHaveProperty("protocol");
    expect(params).not.toHaveProperty("action");
    expect(params).not.toHaveProperty("source");
    expect(params).not.toHaveProperty("destination");
    expect(params).not.toHaveProperty("subscriber_id");
  });

  it("sends the selected direction", async () => {
    const user = userEvent.setup();
    await renderTraffic();
    await waitForFlowRequests(1);

    await selectOption(user, "Direction", "Uplink");

    await waitFor(() => expect(lastFlowParams().direction).toBe("uplink"));
  });

  it("sends the selected action", async () => {
    const user = userEvent.setup();
    await renderTraffic();
    await waitForFlowRequests(1);

    await selectOption(user, "Action", "Dropped");

    await waitFor(() => expect(lastFlowParams().action).toBe("drop"));
  });

  it("sends the selected subscriber", async () => {
    const user = userEvent.setup();
    await renderTraffic();
    await waitForFlowRequests(1);

    await chooseSubscriber(user, IMSI_B);

    await waitFor(() => expect(lastFlowParams().subscriber_id).toBe(IMSI_B));
  });

  it("carries the subscriber from the URL into the first request", async () => {
    await renderTraffic(`/traffic/flows?subscriber_id=${IMSI_B}`);
    await waitForFlowRequests(1);

    expect(lastFlowParams().subscriber_id).toBe(IMSI_B);
  });
});

describe("Traffic flow pagination", () => {
  const manyFlows = Array.from({ length: 25 }, (_, i) => flowReport(i + 1));

  const goToNextPage = async (user: ReturnType<typeof userEvent.setup>) => {
    await user.click(screen.getByRole("button", { name: /next page/i }));
    await waitFor(() => expect(lastFlowParams().page).toBe("2"));
  };

  it("requests the page the operator navigated to", async () => {
    const user = userEvent.setup();
    seedApi({ flows: manyFlows, totalCount: 500 });
    await renderTraffic();
    await waitForFlowRequests(1);

    await goToNextPage(user);
  });

  it("returns to the first page when the direction changes", async () => {
    const user = userEvent.setup();
    seedApi({ flows: manyFlows, totalCount: 500 });
    await renderTraffic();
    await waitForFlowRequests(1);
    await goToNextPage(user);

    await selectOption(user, "Direction", "Uplink");

    await waitFor(() => {
      const params = lastFlowParams();
      expect(params.direction).toBe("uplink");
      expect(params.page).toBe("1");
    });
  });

  it("returns to the first page when the subscriber changes", async () => {
    const user = userEvent.setup();
    seedApi({ flows: manyFlows, totalCount: 500 });
    await renderTraffic();
    await waitForFlowRequests(1);
    await goToNextPage(user);

    await chooseSubscriber(user, IMSI_B);

    await waitFor(() => {
      const params = lastFlowParams();
      expect(params.subscriber_id).toBe(IMSI_B);
      expect(params.page).toBe("1");
    });
  });

  it("never requests a stale page against the new subscriber", async () => {
    const user = userEvent.setup();
    seedApi({ flows: manyFlows, totalCount: 500 });
    await renderTraffic();
    await waitForFlowRequests(1);
    await goToNextPage(user);

    const before = flowRequests().length;
    await chooseSubscriber(user, IMSI_B);
    await waitFor(() => expect(lastFlowParams().subscriber_id).toBe(IMSI_B));

    const afterFilterChange = flowRequests().slice(before);
    expect(
      afterFilterChange.filter((r) => r.params.get("page") !== "1"),
    ).toEqual([]);
  });

  it("returns to the first page when the date range changes", async () => {
    const user = userEvent.setup();
    seedApi({ flows: manyFlows, totalCount: 500 });
    await renderTraffic();
    await waitForFlowRequests(1);
    await goToNextPage(user);

    const start = screen.getByLabelText("Start date");
    await user.clear(start);
    await user.type(start, "2026-07-01");

    await waitFor(() => {
      const params = lastFlowParams();
      expect(params.start).toBe("2026-07-01");
      expect(params.page).toBe("1");
    });
  });
});

describe("Traffic flow text filters", () => {
  it("applies the source filter once typing settles", async () => {
    const user = userEvent.setup();
    await renderTraffic();
    await waitForFlowRequests(1);

    await user.type(screen.getByLabelText("Source"), "10.45.0.1");

    await waitFor(() => expect(lastFlowParams().source).toBe("10.45.0.1"), {
      timeout: 2000,
    });
  });

  it("applies the destination filter once typing settles", async () => {
    const user = userEvent.setup();
    await renderTraffic();
    await waitForFlowRequests(1);

    await user.type(screen.getByLabelText("Destination"), "93.184.216.34");

    await waitFor(
      () => expect(lastFlowParams().destination).toBe("93.184.216.34"),
      { timeout: 2000 },
    );
  });

  it("keeps both text filters when they are typed one after the other", async () => {
    const user = userEvent.setup();
    await renderTraffic();
    await waitForFlowRequests(1);

    await user.type(screen.getByLabelText("Source"), "10.45.0.1");
    await user.type(screen.getByLabelText("Destination"), "93.184.216.34");

    await waitFor(
      () => {
        const params = lastFlowParams();
        expect(params.source).toBe("10.45.0.1");
        expect(params.destination).toBe("93.184.216.34");
      },
      { timeout: 2000 },
    );
  });

  it("drops the source filter from the request when it is cleared", async () => {
    const user = userEvent.setup();
    await renderTraffic();
    await waitForFlowRequests(1);

    const source = screen.getByLabelText("Source");
    await user.type(source, "10.45.0.1");
    await waitFor(() => expect(lastFlowParams().source).toBe("10.45.0.1"), {
      timeout: 2000,
    });

    await user.clear(source);

    await waitFor(() => expect(lastFlowParams()).not.toHaveProperty("source"), {
      timeout: 2000,
    });
  });
});

describe("Traffic date range", () => {
  it("rejects an end date that precedes the start date", async () => {
    const user = userEvent.setup();
    await renderTraffic();
    await waitForFlowRequests(1);

    const start = screen.getByLabelText("Start date");
    const end = screen.getByLabelText("End date");
    await user.clear(start);
    await user.type(start, "2026-08-10");
    await user.clear(end);
    await user.type(end, "2026-08-01");

    expect(await screen.findByRole("alert")).toHaveTextContent(
      /end date must be on or after the start date/i,
    );
  });

  it("does not query an inverted range", async () => {
    const user = userEvent.setup();
    await renderTraffic();
    await waitForFlowRequests(1);

    const start = screen.getByLabelText("Start date");
    const end = screen.getByLabelText("End date");
    await user.clear(start);
    await user.type(start, "2026-08-10");
    await user.clear(end);
    await user.type(end, "2026-08-01");
    await screen.findByRole("alert");

    const ranges = flowRequests().map((r) => [
      r.params.get("start"),
      r.params.get("end"),
    ]);
    expect(ranges.filter(([from, to]) => !from || !to)).toEqual([]);
    expect(ranges.filter(([from, to]) => from! > to!)).toEqual([]);
  });
});

describe("Traffic flow table", () => {
  it("renders a flow row", async () => {
    seedApi({ flows: [flowReport(1, { destination_ip: "93.184.216.34" })] });
    await renderTraffic();

    expect(await screen.findByText("93.184.216.34")).toBeVisible();
  });

  it("announces dropped flows to screen readers", async () => {
    seedApi({ flows: [flowReport(1, { action: "drop" })] });
    await renderTraffic();

    expect(await screen.findByText("Dropped flow")).toBeInTheDocument();
  });

  it("shows the empty state when nothing matches", async () => {
    seedApi({ flows: [] });
    await renderTraffic();

    expect(await screen.findByText("No flow reports found")).toBeVisible();
    expect(screen.queryByRole("grid")).not.toBeInTheDocument();
  });
});

describe("Traffic usage chart", () => {
  it("scales the axis to the largest value across both datasets", async () => {
    seedApi({
      usageBySub: usageBySubscriber({ [IMSI_A]: 5_000_000_000 }),
      usagePerDay: usageBySubscriber({ "2026-08-01": 5_000_000_000 }),
    });
    await renderTraffic("/traffic/usage");

    expect(
      await screen.findByText(/Daily data usage \(all subscribers\) in GB/),
    ).toBeVisible();
  });

  it("uses a smaller unit for smaller volumes", async () => {
    seedApi({
      usageBySub: usageBySubscriber({ [IMSI_A]: 2_000 }),
      usagePerDay: usageBySubscriber({ "2026-08-01": 2_000 }),
    });
    await renderTraffic("/traffic/usage");

    expect(
      await screen.findByText(/Daily data usage \(all subscribers\) in KB/),
    ).toBeVisible();
  });
});
