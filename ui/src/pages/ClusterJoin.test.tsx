// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import { Route, Routes } from "react-router-dom";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
import ClusterJoin from "./ClusterJoin";
import Initialize from "./Initialize";

const api = setupApiServer();

const STATUS = "/api/v1/status";
const JOIN = "/api/v1/cluster/join";

const seedStatus = (enabled: boolean) =>
  api.get(STATUS, () => ({
    initialized: true,
    ready: true,
    schemaVersion: 7,
    cluster: enabled
      ? { enabled: true, role: "Follower", nodeId: 1 }
      : { enabled: false },
  }));

describe("Cluster join page", () => {
  it("shows the join form on a node that has a bind address", async () => {
    seedStatus(true);
    api.get(JOIN, () => ({ state: "waiting" }));

    renderWithProviders(<ClusterJoin />, { auth: {} });

    expect(await screen.findByLabelText(/Join token/)).toBeTruthy();
    expect(screen.getByLabelText(/Cluster address/)).toBeTruthy();
    expect(screen.getByRole("button", { name: /^Join$/ })).toBeDisabled();
  });

  it("redirects away from a node that already belongs to a cluster", async () => {
    seedStatus(true);
    api.get(JOIN, () => ({ state: "unavailable" }));

    renderWithProviders(
      <Routes>
        <Route path="/cluster/join" element={<ClusterJoin />} />
        <Route path="/cluster" element={<div>cluster page</div>} />
      </Routes>,
      { auth: {}, initialEntries: ["/cluster/join"] },
    );

    expect(await screen.findByText("cluster page")).toBeTruthy();
    expect(screen.queryByLabelText(/Join token/)).toBeNull();
  });

  it("redirects back to initialize when the bind address is not configured", async () => {
    seedStatus(false);
    api.get(JOIN, () => ({ state: "unavailable" }));

    renderWithProviders(
      <Routes>
        <Route path="/initialize/join" element={<ClusterJoin />} />
        <Route path="/initialize" element={<div>initialize page</div>} />
      </Routes>,
      { auth: {}, initialEntries: ["/initialize/join"] },
    );

    expect(await screen.findByText("initialize page")).toBeTruthy();
  });
});

describe("Initialize page join affordance", () => {
  it("disables the join link and says why when no bind address is set", async () => {
    api.get(STATUS, () => ({
      initialized: false,
      ready: true,
      schemaVersion: 7,
      cluster: { enabled: false },
    }));
    api.get(JOIN, () => ({ state: "unavailable" }));

    renderWithProviders(<Initialize />, { auth: {} });

    const button = await screen.findByRole("button", {
      name: /Join an existing cluster instead/,
    });
    expect(button).toBeDisabled();
    expect(
      screen.getByText(/cluster bind address is not set in the config file/),
    ).toBeTruthy();
  });

  it("enables the join link once a bind address is configured", async () => {
    api.get(STATUS, () => ({
      initialized: false,
      ready: true,
      schemaVersion: 7,
      cluster: { enabled: true, role: "Follower", nodeId: 1 },
    }));
    api.get(JOIN, () => ({ state: "waiting" }));

    renderWithProviders(<Initialize />, { auth: {} });

    const button = await screen.findByRole("button", {
      name: /Join an existing cluster instead/,
    });
    await waitFor(() => expect(button).toBeEnabled());
  });
});
