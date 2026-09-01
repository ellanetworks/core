// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, beforeEach } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  renderWithProviders,
  LOCATION_TEST_ID,
} from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import {
  flowReport,
  flowReportPage,
  flowStats,
  usageBySubscriber,
} from "@/test/fixtures";
import type {
  FlowReport,
  FlowReportStatsResponse,
} from "@/queries/flow_reports";
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
  stats?: (params: URLSearchParams) => FlowReportStatsResponse;
};

const seedApi = ({
  flows = [flowReport(1)],
  totalCount,
  usageBySub = usageBySubscriber({ [IMSI_A]: 4_000 }),
  usagePerDay = usageBySubscriber({ "2026-08-01": 4_000 }),
  imsis = [IMSI_A, IMSI_B],
  stats,
}: Seed = {}) => {
  api.get(USAGE_PATH, ({ params }) =>
    params.get("group_by") === "day" ? usagePerDay : usageBySub,
  );
  api.get("/api/v1/subscriber-usage/retention", () => ({ days: 30 }));
  api.get(FLOWS_PATH, () =>
    flowReportPage(flows, { total_count: totalCount ?? flows.length }),
  );
  api.get("/api/v1/flow-reports/retention", () => ({ days: 30 }));
  api.get("/api/v1/flow-reports/stats", ({ params }) =>
    stats ? stats(params) : flowStats(),
  );
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

describe("Traffic flow filters authorization", () => {
  it("sends the access token on every request", async () => {
    await renderTraffic();
    await waitForFlowRequests(1);

    const requests = api.requests();
    expect(requests.length).toBeGreaterThan(0);
    for (const request of requests) {
      expect(
        request.headers.get("authorization"),
        `${request.method} ${request.url.pathname} was sent unauthenticated`,
      ).toBe("Bearer test-token");
    }
  });
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

describe("Traffic date range accessibility", () => {
  const invert = async (user: ReturnType<typeof userEvent.setup>) => {
    const start = screen.getByLabelText("Start date");
    const end = screen.getByLabelText("End date");
    await user.clear(start);
    await user.type(start, "2026-08-10");
    await user.clear(end);
    await user.type(end, "2026-08-01");
    await screen.findByRole("alert");
  };

  it("describes the start date field with the reason it is invalid", async () => {
    const user = userEvent.setup();
    await renderTraffic();
    await waitForFlowRequests(1);

    await invert(user);

    expect(screen.getByLabelText("Start date")).toHaveAccessibleDescription(
      /end date must be on or after the start date/i,
    );
  });

  it("describes the end date field with the reason it is invalid", async () => {
    const user = userEvent.setup();
    await renderTraffic();
    await waitForFlowRequests(1);

    await invert(user);

    expect(screen.getByLabelText("End date")).toHaveAccessibleDescription(
      /end date must be on or after the start date/i,
    );
  });

  it("carries no stale description once the range is valid", async () => {
    const user = userEvent.setup();
    await renderTraffic();
    await waitForFlowRequests(1);
    await invert(user);

    const end = screen.getByLabelText("End date");
    await user.clear(end);
    await user.type(end, "2026-08-20");

    await waitFor(() =>
      expect(screen.queryByRole("alert")).not.toBeInTheDocument(),
    );
    expect(end).toHaveAccessibleDescription("");
  });

  it("stops the picker offering an end date before the start", async () => {
    const user = userEvent.setup();
    await renderTraffic();
    await waitForFlowRequests(1);

    const start = screen.getByLabelText("Start date");
    await user.clear(start);
    await user.type(start, "2026-08-10");

    await waitFor(() =>
      expect(screen.getByLabelText("End date")).toHaveAttribute(
        "min",
        "2026-08-10",
      ),
    );
  });
});

describe("Traffic incomplete date range", () => {
  it("explains why results stopped updating when a date is cleared", async () => {
    const user = userEvent.setup();
    await renderTraffic();
    await waitForFlowRequests(1);

    await user.clear(screen.getByLabelText("Start date"));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      /select both a start and an end date/i,
    );
  });

  it("stops querying while the range is incomplete", async () => {
    const user = userEvent.setup();
    await renderTraffic();
    await waitForFlowRequests(1);

    const before = flowRequests().length;
    await user.clear(screen.getByLabelText("Start date"));
    await screen.findByRole("alert");

    expect(flowRequests().length).toBe(before);
  });

  it("never disables the flow query without saying why", async () => {
    const user = userEvent.setup();
    await renderTraffic();
    await waitForFlowRequests(1);

    for (const label of ["Start date", "End date"]) {
      await user.clear(screen.getByLabelText(label));
      expect(
        screen.queryAllByRole("alert").length,
        `clearing ${label} froze the page with no explanation`,
      ).toBeGreaterThan(0);
    }
  });

  it("resumes querying once the range is complete again", async () => {
    const user = userEvent.setup();
    await renderTraffic();
    await waitForFlowRequests(1);

    const start = screen.getByLabelText("Start date");
    await user.clear(start);
    await screen.findByRole("alert");

    await user.type(start, "2026-07-01");

    await waitFor(() => expect(lastFlowParams().start).toBe("2026-07-01"));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});

describe("Traffic stale results", () => {
  it("stops showing flow rows once the range is incomplete", async () => {
    const user = userEvent.setup();
    seedApi({ flows: [flowReport(1, { destination_ip: "93.184.216.34" })] });
    await renderTraffic();
    await screen.findAllByText("93.184.216.34");

    await user.clear(screen.getByLabelText("Start date"));
    await screen.findByRole("alert");

    expect(screen.queryByText("93.184.216.34")).not.toBeInTheDocument();
  });

  it("does not claim there are no matching flows when it never asked", async () => {
    const user = userEvent.setup();
    seedApi({ flows: [] });
    await renderTraffic();
    await screen.findByText("No flow reports found");

    await user.clear(screen.getByLabelText("Start date"));
    await screen.findByRole("alert");

    expect(screen.queryByText("No flow reports found")).not.toBeInTheDocument();
  });

  it("shows no loading indicator for a request it will never send", async () => {
    const user = userEvent.setup();
    await renderTraffic();
    await waitForFlowRequests(1);

    await user.clear(screen.getByLabelText("Start date"));
    await screen.findByRole("alert");

    expect(screen.queryAllByRole("progressbar")).toEqual([]);
  });

  it("hides the usage chart while the range is incomplete", async () => {
    const user = userEvent.setup();
    await renderTraffic("/traffic/usage");
    await screen.findByText(/Daily data usage/);

    await user.clear(screen.getByLabelText("Start date"));
    await screen.findByRole("alert");

    expect(screen.queryByText(/Daily data usage/)).not.toBeInTheDocument();
  });
});

describe("Traffic session expiry", () => {
  it("signs the operator out when the flow query is rejected as unauthorized", async () => {
    seedApi();
    api.get(FLOWS_PATH, () => httpError(401, "token expired"));
    renderWithProviders(<Traffic />, {
      initialEntries: ["/traffic/flows"],
      auth: {},
    });

    await waitFor(() =>
      expect(screen.getByTestId(LOCATION_TEST_ID)).toHaveTextContent("/login"),
    );
  });

  it("stops rendering the page once signed out", async () => {
    seedApi();
    api.get(FLOWS_PATH, () => httpError(401, "token expired"));
    renderWithProviders(<Traffic />, {
      initialEntries: ["/traffic/flows"],
      auth: {},
    });

    await waitFor(() =>
      expect(screen.getByTestId(LOCATION_TEST_ID)).toHaveTextContent("/login"),
    );
    expect(
      screen.queryByRole("heading", { name: "Traffic" }),
    ).not.toBeInTheDocument();
  });

  it("keeps the operator on the page for a forbidden response", async () => {
    seedApi();
    api.get(FLOWS_PATH, () => httpError(403, "not allowed"));
    await renderTraffic();

    await waitForFlowRequests(1);
    expect(screen.getByTestId(LOCATION_TEST_ID)).toHaveTextContent(
      "/traffic/flows",
    );
  });

  it("keeps the operator on the page for a server error", async () => {
    seedApi();
    api.get(FLOWS_PATH, () => httpError(500, "boom"));
    await renderTraffic();

    await waitForFlowRequests(1);
    expect(screen.getByTestId(LOCATION_TEST_ID)).toHaveTextContent(
      "/traffic/flows",
    );
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

describe("Traffic usage query", () => {
  const usageRequests = () => api.requests(USAGE_PATH);

  it("never requests an incomplete range", async () => {
    const user = userEvent.setup();
    await renderTraffic("/traffic/usage");
    await waitFor(() => expect(usageRequests().length).toBeGreaterThan(0));

    await user.clear(screen.getByLabelText("Start date"));
    await screen.findByRole("alert");

    const ranges = usageRequests().map((r) => [
      r.params.get("start"),
      r.params.get("end"),
    ]);
    expect(ranges.filter(([from, to]) => !from || !to)).toEqual([]);
  });

  it("never requests an inverted range", async () => {
    const user = userEvent.setup();
    await renderTraffic("/traffic/usage");
    await waitFor(() => expect(usageRequests().length).toBeGreaterThan(0));

    const start = screen.getByLabelText("Start date");
    const end = screen.getByLabelText("End date");
    await user.clear(start);
    await user.type(start, "2026-08-10");
    await user.clear(end);
    await user.type(end, "2026-08-01");
    await screen.findByRole("alert");

    const inverted = usageRequests().filter((r) => {
      const from = r.params.get("start");
      const to = r.params.get("end");
      return !!from && !!to && from > to;
    });
    expect(inverted).toEqual([]);
  });

  it("groups by both subscriber and day", async () => {
    await renderTraffic("/traffic/usage");
    await waitFor(() => expect(usageRequests().length).toBeGreaterThan(1));

    const groupings = new Set(
      usageRequests().map((r) => r.params.get("group_by")),
    );
    expect(groupings).toEqual(new Set(["subscriber", "day"]));
  });
});

describe("Traffic usage pagination", () => {
  const manyUsage = usageBySubscriber(
    Object.fromEntries(
      Array.from({ length: 30 }, (_, i) => [
        `00101${String(i + 1).padStart(10, "0")}`,
        (i + 1) * 1000,
      ]),
    ),
  );

  const footer = () =>
    document.querySelector(".MuiTablePagination-displayedRows")?.textContent ??
    "";

  it("returns to the first page when the subscriber changes", async () => {
    const user = userEvent.setup();
    seedApi({ usageBySub: manyUsage });
    await renderTraffic("/traffic/usage");
    await waitFor(() => expect(footer()).toMatch(/1–25/));

    await user.click(screen.getByRole("button", { name: /next page/i }));
    await waitFor(() => expect(footer()).toMatch(/26–30/));

    await chooseSubscriber(user, IMSI_B);

    await waitFor(() => expect(footer()).toMatch(/1–25/));
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

const pieArcs = (title: string) => {
  const heading = screen.getByRole("heading", { name: title });
  const container = heading.parentElement as HTMLElement;
  return Array.from(
    container.querySelectorAll<SVGPathElement>("path.MuiPieChart-arc"),
  );
};

const protocolArcs = () => pieArcs("Protocols (by flow count)");
const destinationArcs = () =>
  pieArcs("Top 10 Destinations (uplink, by flow count)");

const fillsOf = (arcs: SVGPathElement[]) =>
  arcs.map((arc) => arc.getAttribute("fill"));

const TCP_BLUE = "#2196F3";
const UDP_GREEN = "#4CAF50";

describe("Traffic pie chart selection", () => {
  const statsResolver = (params: URLSearchParams): FlowReportStatsResponse => {
    const protocol = params.get("protocol");
    const destination = params.get("destination");
    if (protocol) {
      return flowStats({
        protocols: [{ protocol: Number(protocol), count: 30 }],
        top_destinations_uplink: [{ ip: "1.1.1.1", count: 30 }],
      });
    }
    if (destination) {
      return flowStats({
        protocols: [{ protocol: 6, count: 5 }],
        top_destinations_uplink: [{ ip: destination, count: 5 }],
      });
    }
    return flowStats({
      top_destinations_uplink: [
        { ip: "1.1.1.1", count: 90 },
        { ip: "8.8.8.8", count: 50 },
        { ip: "93.184.216.34", count: 10 },
      ],
    });
  };

  beforeEach(() => {
    seedApi({ stats: statsResolver });
  });

  it("keeps the selected protocol slice at its own color", async () => {
    const user = userEvent.setup();
    await renderTraffic();

    await waitFor(() => expect(protocolArcs()).toHaveLength(2));
    expect(fillsOf(protocolArcs())).toEqual([TCP_BLUE, UDP_GREEN]);

    await user.click(protocolArcs()[1]);

    await waitFor(() => expect(protocolArcs()).toHaveLength(1));
    expect(protocolArcs()[0]).toHaveAttribute("fill", UDP_GREEN);
  });

  it("does not transition the fill of a pie slice", async () => {
    await renderTraffic();

    await waitFor(() => expect(protocolArcs()).toHaveLength(2));

    for (const arc of [...protocolArcs(), ...destinationArcs()]) {
      expect(window.getComputedStyle(arc).transitionProperty).not.toContain(
        "fill",
      );
    }
  });

  it("keeps the selected destination slice at its own color", async () => {
    const user = userEvent.setup();
    await renderTraffic();

    await waitFor(() => expect(destinationArcs()).toHaveLength(3));
    const selected = fillsOf(destinationArcs())[1];

    await user.click(destinationArcs()[1]);

    await waitFor(() => expect(destinationArcs()).toHaveLength(1));
    expect(destinationArcs()[0]).toHaveAttribute("fill", selected);
  });
});
