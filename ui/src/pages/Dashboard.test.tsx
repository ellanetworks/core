// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { screen, waitFor } from "@testing-library/react";
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

describe("Dashboard deployment identity", () => {
  it("reports standalone mode next to the version", async () => {
    seedDashboard();
    api.get(STATUS, () => standalone);

    renderWithProviders(<Dashboard />, { auth: {} });

    const mode = await screen.findByRole("link", { name: "Standalone" });
    expect(mode).toHaveAttribute("href", "/cluster");
    expect(mode.parentElement).toHaveTextContent("v1.18.0 · Standalone");
  });

  it("reports the node identity in a cluster", async () => {
    seedDashboard();
    api.get(STATUS, () => clustered);

    renderWithProviders(<Dashboard />, { auth: {} });

    const mode = await screen.findByRole("link", { name: "Cluster node 2" });
    expect(mode.parentElement).toHaveTextContent("v1.18.0 · Cluster node 2");
    expect(screen.queryByText("Standalone")).toBeNull();
  });

  it("does not link non-admins to the admin-only cluster page", async () => {
    seedDashboard();
    api.get(STATUS, () => clustered);

    renderWithProviders(<Dashboard />, { auth: { role: "Read Only" } });

    await screen.findByText(/Cluster node 2/);
    expect(screen.queryByRole("link", { name: "Cluster node 2" })).toBeNull();
  });

  it("shows no mode until the status response arrives", async () => {
    let release: (() => void) | undefined;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });

    seedDashboard();
    api.get(STATUS, async () => {
      await gate;
      return clustered;
    });

    renderWithProviders(<Dashboard />, { auth: {} });

    expect(screen.queryByText("Standalone")).toBeNull();
    expect(screen.queryByText(/Cluster node/)).toBeNull();

    release?.();

    await waitFor(() =>
      expect(screen.getByRole("link", { name: "Cluster node 2" })).toBeTruthy(),
    );
  });
});
