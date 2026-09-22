// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, rawBody } from "@/test/apiServer";
import Dashboard from "./Dashboard";

const api = setupApiServer();

const STATUS = "/api/v1/status";

const seedDashboard = () => {
  api.get("/api/v1/subscribers", () => ({ items: [], total_count: 0 }));
  api.get("/api/v1/ran/radios", () => ({ items: [], total_count: 0 }));
  api.get("/api/v1/ran/events", () => ({ items: [], total_count: 0 }));
  api.get("/api/v1/flow-reports/stats", () => ({ protocols: [] }));
  api.get("/api/v1/subscriber-usage", () => []);
  api.get("/api/v1/metrics", () =>
    rawBody("", { "Content-Type": "text/plain" }),
  );
};

const standalone = {
  initialized: true,
  ready: true,
  version: "v1.18.0",
  schemaVersion: 19,
  cluster: { enabled: false },
};

const clustered = {
  initialized: true,
  ready: true,
  version: "v1.18.0",
  schemaVersion: 19,
  cluster: {
    enabled: true,
    role: "Follower",
    nodeId: 2,
    isLeader: false,
    leaderNodeId: 1,
    appliedIndex: 42,
    appliedSchemaVersion: 19,
  },
};

const HEALTH = "/api/v1/cluster/health";

const card = (title: string) =>
  screen.getByText(title).closest(".MuiCard-root") as HTMLElement;

describe("Dashboard high availability card", () => {
  it("reports a standalone node", async () => {
    seedDashboard();
    api.get(STATUS, () => standalone);

    renderWithProviders(<Dashboard />, { auth: {} });

    await waitFor(() =>
      expect(
        within(card("High Availability")).getByText("Standalone"),
      ).toBeTruthy(),
    );
  });

  it("reports the healthy voter ratio", async () => {
    seedDashboard();
    api.get(STATUS, () => clustered);
    api.get(HEALTH, () => ({
      enabled: true,
      hasLeader: true,
      healthy: true,
      healthyVoters: 3,
      totalVoters: 3,
      failureTolerance: 1,
    }));

    renderWithProviders(<Dashboard />, { auth: {} });

    const ha = await waitFor(() => card("High Availability"));

    await waitFor(() => expect(within(ha).getByText("3/3")).toBeTruthy());
  });

  it("reports a degraded voter ratio", async () => {
    seedDashboard();
    api.get(STATUS, () => clustered);
    api.get(HEALTH, () => ({
      enabled: true,
      hasLeader: true,
      healthy: false,
      healthyVoters: 2,
      totalVoters: 3,
      failureTolerance: 0,
    }));

    renderWithProviders(<Dashboard />, { auth: {} });

    const ha = await waitFor(() => card("High Availability"));

    await waitFor(() => expect(within(ha).getByText("2/3")).toBeTruthy());
  });

  it("says so when no leader is known, rather than showing a ratio", async () => {
    seedDashboard();
    api.get(STATUS, () => clustered);
    api.get(HEALTH, () => ({
      enabled: true,
      hasLeader: false,
      healthy: false,
      healthyVoters: 0,
      totalVoters: 0,
      failureTolerance: 0,
    }));

    renderWithProviders(<Dashboard />, { auth: {} });

    const ha = await waitFor(() => card("High Availability"));

    await waitFor(() => expect(within(ha).getByText("No leader")).toBeTruthy());
    expect(within(ha).queryByText("0/0")).toBeNull();
  });
});

describe("Dashboard system cards", () => {
  it("shows Up Since and no longer shows Routines", async () => {
    seedDashboard();
    api.get(STATUS, () => standalone);
    api.get("/api/v1/metrics", () =>
      rawBody("process_start_time_seconds 1700000000\ngo_goroutines 42\n", {
        "Content-Type": "text/plain",
      }),
    );

    renderWithProviders(<Dashboard />, { auth: {} });

    await screen.findByText("Up Since");

    expect(screen.queryByText("Routines")).toBeNull();
    expect(screen.queryByText("42")).toBeNull();
    expect(within(card("Up Since")).queryByText("N/A")).toBeNull();
  });
});
