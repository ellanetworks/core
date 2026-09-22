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
  cluster: {
    enabled: false,
    nodeId: "0199c4f1-2a7e-7b31-9c5d-1f2e3a4b5c6d",
  },
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
    displayName: "core-mtl-a",
    isLeader: false,
    leaderNodeId: 1,
    appliedIndex: 42,
    appliedSchemaVersion: 19,
  },
};

describe("DeploymentIdentity", () => {
  it("names the node on a standalone deployment", async () => {
    api.get(STATUS, () => standalone);

    renderWithProviders(<DeploymentIdentity />, { auth: {} });

    await screen.findByText("v1.18.0 · Node 1f2e3a4b5c6d");
    expect(screen.queryByText(/Standalone/)).toBeNull();
  });

  it("prefers the display name in a cluster", async () => {
    api.get(STATUS, () => clustered);

    renderWithProviders(<DeploymentIdentity />, { auth: {} });

    await screen.findByText("v1.18.0 · Node core-mtl-a");
    expect(screen.queryByText(/Cluster node/)).toBeNull();
  });

  it("renders the identity as plain text, not a link", async () => {
    api.get(STATUS, () => clustered);

    renderWithProviders(<DeploymentIdentity />, { auth: {} });

    await screen.findByText("v1.18.0 · Node core-mtl-a");
    expect(screen.queryByRole("link")).toBeNull();
  });

  it("falls back to the version alone when no identity is reported", async () => {
    api.get(STATUS, () => ({
      initialized: true,
      ready: true,
      version: "v1.18.0",
      schemaVersion: 19,
      cluster: { enabled: false },
    }));

    renderWithProviders(<DeploymentIdentity />, { auth: {} });

    await screen.findByText("v1.18.0");
    expect(screen.queryByText(/Node/)).toBeNull();
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

    expect(screen.queryByText(/Node/)).toBeNull();

    release?.();

    await waitFor(() =>
      expect(screen.getByText("v1.18.0 · Node core-mtl-a")).toBeTruthy(),
    );
  });

  it("shows only the version when compact", async () => {
    api.get(STATUS, () => clustered);

    renderWithProviders(<DeploymentIdentity compact />, { auth: {} });

    await screen.findByText("v1.18.0");
    expect(screen.queryByText(/Node/)).toBeNull();
  });
});
