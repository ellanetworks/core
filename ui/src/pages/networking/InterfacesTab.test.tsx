// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { screen, within } from "@testing-library/react";
import { Outlet, Route, Routes } from "react-router-dom";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
import InterfacesTab from "./InterfacesTab";
import type { NetworkingTabProps } from "./types";

const api = setupApiServer();

const INTERFACES = "/api/v1/networking/interfaces";

const baseInterfaces = {
  n2: { addresses: ["10.0.0.2"], port: 38412 },
  n3: { name: "eth1", addresses: ["10.0.0.3"], external_address: "" },
  n6: { name: "eth2", addresses: ["10.0.0.6"] },
  api: { addresses: ["10.0.0.1"], port: 5002 },
};

const context: NetworkingTabProps = {
  accessToken: "test-token",
  canEdit: true,
  showSnackbar: () => {},
};

const renderTab = () =>
  renderWithProviders(
    <Routes>
      <Route element={<Outlet context={context} />}>
        <Route path="/" element={<InterfacesTab />} />
      </Route>
    </Routes>,
    { auth: {} },
  );

const card = async (title: string) => {
  const heading = await screen.findByText(title);
  return heading.parentElement!.parentElement!;
};

describe("InterfacesTab N32", () => {
  it("shows the N32 interface when configured", async () => {
    api.get(INTERFACES, () => ({
      ...baseInterfaces,
      n32: {
        addresses: ["10.0.0.32", "2001:db8::32"],
        port: 443,
        interface: "ens5",
      },
    }));

    renderTab();

    const n32 = within(await card("N32 (SEPP)"));
    expect(n32.getByText("Roaming")).toBeInTheDocument();
    expect(n32.getByText("ens5")).toBeInTheDocument();
    expect(n32.getByText("10.0.0.32")).toBeInTheDocument();
    expect(n32.getByText("2001:db8::32")).toBeInTheDocument();
    expect(n32.getByText("443")).toBeInTheDocument();
  });

  it("omits the interface name when N32 is bound to an address", async () => {
    api.get(INTERFACES, () => ({
      ...baseInterfaces,
      n32: { addresses: ["10.0.0.32"], port: 8443 },
    }));

    renderTab();

    const n32 = within(await card("N32 (SEPP)"));
    expect(n32.getByText("10.0.0.32")).toBeInTheDocument();
    expect(n32.getByText("8443")).toBeInTheDocument();
    expect(n32.queryByText(/Interface name/)).not.toBeInTheDocument();
  });

  it("shows a not-configured hint when N32 is absent", async () => {
    api.get(INTERFACES, () => baseInterfaces);

    renderTab();

    const n32 = within(await card("N32 (SEPP)"));
    expect(n32.getByText(/Not listening/)).toBeInTheDocument();
    expect(n32.getByText("interfaces.n32")).toBeInTheDocument();
    expect(n32.queryByText(/Port:/)).not.toBeInTheDocument();
  });
});
