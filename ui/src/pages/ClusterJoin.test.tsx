// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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
      screen.queryByText(/cluster bind address is not set in the config file/),
    ).toBeNull();

    await userEvent.hover(button.parentElement!);

    expect(
      await screen.findByText(
        /cluster bind address is not set in the config file/,
      ),
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

describe("Cluster join form", () => {
  it("surfaces a join failure in the snackbar, not inside the card", async () => {
    seedStatus(true);
    api.get(JOIN, () => ({
      state: "waiting",
      error:
        'peer 192.168.40.58:7000 rejected the join: server returned 500: {"error":"Failed to register cluster member"}',
    }));

    renderWithProviders(<ClusterJoin />, { auth: {} });

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("rejected the join");

    const card = screen.getByLabelText(/Join token/).closest("form, div");
    expect(card?.contains(alert)).toBe(false);
  });

  it("collapses the token field once a token is pasted", async () => {
    seedStatus(true);
    api.get(JOIN, () => ({ state: "waiting" }));

    const user = userEvent.setup();
    renderWithProviders(<ClusterJoin />, { auth: {} });

    const field = await screen.findByLabelText(/Join token/);
    await user.click(field);
    await user.paste("a".repeat(64) + "a4f2");

    await waitFor(() =>
      expect(screen.queryByLabelText(/Join token/)).toBeNull(),
    );
    expect(screen.getByText(/Token pasted · ends in a4f2/)).toBeTruthy();

    await user.click(screen.getByRole("button", { name: /Clear join token/ }));
    expect(await screen.findByLabelText(/Join token/)).toBeTruthy();
  });

  it("rejects a cluster address that is not host:port", async () => {
    seedStatus(true);
    api.get(JOIN, () => ({ state: "waiting" }));

    const user = userEvent.setup();
    renderWithProviders(<ClusterJoin />, { auth: {} });

    const token = await screen.findByLabelText(/Join token/);
    await user.click(token);
    await user.paste("a".repeat(68));

    const address = screen.getByLabelText(/Cluster address/);
    await user.type(address, "https://192.168.40.58:7000");
    await user.tab();

    expect(
      await screen.findByText(/Enter a cluster address, not a URL/),
    ).toBeTruthy();
    expect(screen.getByRole("button", { name: /^Join$/ })).toBeDisabled();

    await user.clear(address);
    await user.type(address, "192.168.40.58");
    await user.tab();
    expect(await screen.findByText(/cluster port is missing/)).toBeTruthy();

    await user.clear(address);
    await user.type(address, "192.168.40.58:7000");
    await user.tab();
    await waitFor(() =>
      expect(screen.getByRole("button", { name: /^Join$/ })).toBeEnabled(),
    );
  });
});
