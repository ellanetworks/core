// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, beforeEach } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
import AuditLogs from "./AuditLogs";

const api = setupApiServer();

const LOGS_PATH = "/api/v1/logs/audit";

const auditLog = (seed: number) => ({
  id: seed,
  timestamp: "2026-08-01T10:00:00Z",
  level: "info",
  actor: "admin@ellanetworks.com",
  action: "create_subscriber",
  ip: "10.0.0.1",
  details: "created",
});

const seedApi = ({ logs = [auditLog(1)], totalCount = 1 } = {}) => {
  api.get(LOGS_PATH, () => ({
    items: logs,
    page: 1,
    per_page: 25,
    total_count: totalCount,
  }));
  api.get("/api/v1/logs/audit/retention", () => ({ days: 30 }));
  api.get("/api/v1/users", () => ({
    items: [
      { email: "admin@ellanetworks.com", role_id: 1 },
      { email: "ops@ellanetworks.com", role_id: 3 },
    ],
    page: 1,
    per_page: 100,
    total_count: 2,
  }));
};

const renderAuditLogs = async (path = "/audit-logs") => {
  const result = renderWithProviders(<AuditLogs />, {
    initialEntries: [path],
    auth: {},
  });
  await screen.findByRole("heading", { name: "Audit Logs" });
  return result;
};

const logRequests = () => api.requests(LOGS_PATH);

const lastLogParams = () => {
  const request = api.lastRequest(LOGS_PATH);
  if (!request) throw new Error("no audit log request was made");
  return Object.fromEntries(request.params);
};

const waitForLogRequests = (count: number) =>
  waitFor(() => expect(logRequests().length).toBeGreaterThanOrEqual(count));

beforeEach(() => {
  seedApi();
});

describe("AuditLogs filters authorization", () => {
  it("sends the access token on every request", async () => {
    await renderAuditLogs();
    await waitForLogRequests(1);

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

describe("AuditLogs filters", () => {
  it("seeds the user filter from the URL", async () => {
    await renderAuditLogs("/audit-logs?user=ops@ellanetworks.com");
    await waitForLogRequests(1);

    expect(lastLogParams().user).toBe("ops@ellanetworks.com");
  });

  it("sends the selected user", async () => {
    const user = userEvent.setup();
    await renderAuditLogs();
    await waitForLogRequests(1);

    await user.click(screen.getByRole("combobox", { name: "User" }));
    await user.click(
      within(await screen.findByRole("listbox")).getByRole("option", {
        name: "ops@ellanetworks.com",
      }),
    );

    await waitFor(() =>
      expect(lastLogParams().user).toBe("ops@ellanetworks.com"),
    );
  });
});

describe("AuditLogs date range", () => {
  it("reports an end date that precedes the start date", async () => {
    const user = userEvent.setup();
    await renderAuditLogs();
    await waitForLogRequests(1);

    const start = screen.getByLabelText("Start date");
    const end = screen.getByLabelText("End date");
    await user.clear(start);
    await user.type(start, "2026-08-10");
    await user.clear(end);
    await user.type(end, "2026-08-01");

    expect(
      await screen.findByText("End date must be on or after the start date."),
    ).toBeVisible();
  });

  it("does not query an inverted range", async () => {
    const user = userEvent.setup();
    await renderAuditLogs();
    await waitForLogRequests(1);

    const start = screen.getByLabelText("Start date");
    const end = screen.getByLabelText("End date");
    await user.clear(start);
    await user.type(start, "2026-08-10");
    await user.clear(end);
    await user.type(end, "2026-08-01");
    await screen.findByText("End date must be on or after the start date.");

    const inverted = logRequests().filter((r) => {
      const from = r.params.get("start");
      const to = r.params.get("end");
      return !!from && !!to && from > to;
    });
    expect(inverted).toEqual([]);
  });
});
