// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, beforeEach, vi } from "vitest";
import { screen, waitFor, within, fireEvent } from "@testing-library/react";
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
  user: "admin@ellanetworks.com",
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

const timeRangeButton = () =>
  screen.getByRole("button", { name: /^Time range:/ });

const openTimeRange = async (user: ReturnType<typeof userEvent.setup>) => {
  if (screen.queryByLabelText("From")) return;
  await user.click(timeRangeButton());
  await screen.findByLabelText("From");
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

describe("AuditLogs time range presets", () => {
  it("offers a range that covers the retention window", async () => {
    const user = userEvent.setup();
    await renderAuditLogs();
    await waitForLogRequests(1);
    await openTimeRange(user);

    for (const name of ["Last 15 minutes", "Last 30 days", "Last 90 days"]) {
      expect(screen.getByRole("menuitem", { name })).toBeInTheDocument();
    }
  });

  it("queries the full window a long preset resolves to", async () => {
    const user = userEvent.setup();
    await renderAuditLogs();
    await waitForLogRequests(1);
    await openTimeRange(user);

    await user.click(screen.getByRole("menuitem", { name: "Last 90 days" }));

    await waitFor(() => {
      const elapsed = Date.now() - Date.parse(lastLogParams().start);
      expect(elapsed).toBeGreaterThan(89 * 24 * 60 * 60_000);
      expect(elapsed).toBeLessThan(91 * 24 * 60 * 60_000);
    });
  });
});

describe("AuditLogs auto refresh", () => {
  it("keeps polling for new entries", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      await renderAuditLogs();
      await waitForLogRequests(1);
      const before = logRequests().length;

      await vi.advanceTimersByTimeAsync(11_000);

      await waitFor(() => expect(logRequests().length).toBeGreaterThan(before));
    } finally {
      vi.useRealTimers();
    }
  });
});

describe("AuditLogs date range from the URL", () => {
  it("ignores a start param it cannot parse", async () => {
    await renderAuditLogs("/audit-logs?range=custom&start=not-a-date");
    await waitForLogRequests(1);

    expect(lastLogParams()).not.toHaveProperty("start");
    expect(timeRangeButton()).not.toHaveTextContent("not-a-date");
  });
});

describe("AuditLogs date range accessibility", () => {
  const invert = async (user: ReturnType<typeof userEvent.setup>) => {
    await openTimeRange(user);
    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-08-10T10:00" },
    });
    fireEvent.change(screen.getByLabelText("To"), {
      target: { value: "2026-08-01T10:00" },
    });
    await screen.findByRole("alert");
  };

  it.each(["From", "To"])(
    "describes the %s field with the reason it is invalid",
    async (label) => {
      const user = userEvent.setup();
      await renderAuditLogs();
      await waitForLogRequests(1);
      await openTimeRange(user);

      await invert(user);

      expect(screen.getByLabelText(label)).toHaveAccessibleDescription(
        /must be on or after/i,
      );
    },
  );

  it("stops the picker offering an end date before the start", async () => {
    const user = userEvent.setup();
    await renderAuditLogs();
    await waitForLogRequests(1);
    await openTimeRange(user);

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-08-10T10:00" },
    });

    await waitFor(() =>
      expect(screen.getByLabelText("To")).toHaveAttribute(
        "min",
        "2026-08-10T10:00",
      ),
    );
  });
});

describe("AuditLogs stale results", () => {
  it("keeps showing the rows of the last applied range", async () => {
    const user = userEvent.setup();
    await renderAuditLogs();
    await screen.findAllByText("create_subscriber");
    await openTimeRange(user);

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-08-10T10:00" },
    });
    fireEvent.change(screen.getByLabelText("To"), {
      target: { value: "2026-08-01T10:00" },
    });
    await screen.findByRole("alert");

    expect(
      (await screen.findAllByText("create_subscriber")).length,
    ).toBeGreaterThan(0);
  });
});

describe("AuditLogs date range", () => {
  it("reports an end date that precedes the start date", async () => {
    const user = userEvent.setup();
    await renderAuditLogs();
    await waitForLogRequests(1);
    await openTimeRange(user);

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-08-10T10:00" },
    });
    fireEvent.change(screen.getByLabelText("To"), {
      target: { value: "2026-08-01T10:00" },
    });

    expect(
      (
        await screen.findAllByText(
          "The To timestamp must be on or after the From timestamp.",
        )
      ).length,
    ).toBeGreaterThan(0);
  });

  it("does not query an inverted range", async () => {
    const user = userEvent.setup();
    await renderAuditLogs();
    await waitForLogRequests(1);
    await openTimeRange(user);

    fireEvent.change(screen.getByLabelText("From"), {
      target: { value: "2026-08-10T10:00" },
    });
    fireEvent.change(screen.getByLabelText("To"), {
      target: { value: "2026-08-01T10:00" },
    });
    await screen.findAllByText(
      "The To timestamp must be on or after the From timestamp.",
    );

    const inverted = logRequests().filter((r) => {
      const from = r.params.get("start");
      const to = r.params.get("end");
      return !!from && !!to && from > to;
    });
    expect(inverted).toEqual([]);
  });
});

describe("AuditLogs actor column", () => {
  it("links an actor that is a user account", async () => {
    seedApi({ logs: [{ ...auditLog(1), user: "admin@ellanetworks.com" }] });
    await renderAuditLogs();

    const cell = await screen.findByText("admin@ellanetworks.com");
    expect(cell.closest("a")).toHaveAttribute(
      "href",
      "/users/admin%40ellanetworks.com",
    );
  });

  it.each(["system", "ella-node-1"])(
    "renders the %s actor as plain text",
    async (actor) => {
      seedApi({ logs: [{ ...auditLog(1), user: actor }] });
      await renderAuditLogs();

      const cell = await screen.findByText(actor);
      expect(cell.closest("a")).toBeNull();
    },
  );
});
