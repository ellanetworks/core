// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
import DeploymentIdentity from "./DeploymentIdentity";

const api = setupApiServer();

const STATUS = "/api/v1/status";

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

describe("DeploymentIdentity", () => {
  it("reports standalone mode next to the version", async () => {
    api.get(STATUS, () => standalone);

    renderWithProviders(<DeploymentIdentity />, { auth: {} });

    const mode = await screen.findByRole("link", { name: "Standalone" });
    expect(mode).toHaveAttribute("href", "/cluster");
    expect(mode.parentElement).toHaveTextContent("v1.18.0 · Standalone");
  });

  it("reports the node identity in a cluster", async () => {
    api.get(STATUS, () => clustered);

    renderWithProviders(<DeploymentIdentity />, { auth: {} });

    const mode = await screen.findByRole("link", { name: "Cluster node 2" });
    expect(mode.parentElement).toHaveTextContent("v1.18.0 · Cluster node 2");
    expect(screen.queryByText("Standalone")).toBeNull();
  });

  it("does not link non-admins to the admin-only cluster page", async () => {
    api.get(STATUS, () => clustered);

    renderWithProviders(<DeploymentIdentity />, {
      auth: { role: "Read Only" },
    });

    await screen.findByText(/Cluster node 2/);
    expect(screen.queryByRole("link", { name: "Cluster node 2" })).toBeNull();
  });

  it("shows nothing until the status response arrives", async () => {
    let release: (() => void) | undefined;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });

    api.get(STATUS, async () => {
      await gate;
      return clustered;
    });

    renderWithProviders(<DeploymentIdentity />, { auth: {} });

    expect(screen.queryByText("Standalone")).toBeNull();
    expect(screen.queryByText(/Cluster node/)).toBeNull();

    release?.();

    await waitFor(() =>
      expect(screen.getByRole("link", { name: "Cluster node 2" })).toBeTruthy(),
    );
  });

  it("shows only the version when compact", async () => {
    api.get(STATUS, () => clustered);

    renderWithProviders(<DeploymentIdentity compact />, { auth: {} });

    await screen.findByText("v1.18.0");
    expect(screen.queryByText(/Cluster node/)).toBeNull();
  });
});
