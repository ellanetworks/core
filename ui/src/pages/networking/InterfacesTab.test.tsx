// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { beforeEach, describe, it, expect } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Outlet, Route, Routes } from "react-router-dom";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
import { stubDiameterStatus } from "@/test/diameter";
import InterfacesTab from "./InterfacesTab";
import type { NetworkingTabProps } from "./types";

const api = setupApiServer();

beforeEach(() => {
  stubDiameterStatus(api);
});

const INTERFACES = "/api/v1/networking/interfaces";

beforeEach(() => {
  api.get("/api/v1/slices", () => ({
    items: [],
    page: 1,
    per_page: 100,
    total_count: 0,
  }));
});

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

    const user = userEvent.setup();
    const n32 = within(await card("N32 (SEPP)"));
    await user.hover(n32.getByText("Not listening"));
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "Set interfaces.n32 in the configuration file.",
    );
    expect(n32.queryByText(/Port:/)).not.toBeInTheDocument();
  });
});

describe("InterfacesTab Diameter", () => {
  it("links to the Integrations tabs for identity and peers", async () => {
    api.get(INTERFACES, () => ({
      ...baseInterfaces,
      diameter: { addresses: ["10.0.0.2"], port: 3868 },
    }));

    renderTab();

    const diameter = within(await card("Diameter"));
    expect(
      ["IMS", "SMS", "Roaming"].map((name) =>
        diameter.getByRole("link", { name }).getAttribute("href"),
      ),
    ).toEqual([
      "/integrations/ims",
      "/integrations/sms",
      "/integrations/roaming",
    ]);
  });

  it("explains in a tooltip why Diameter is not listening", async () => {
    api.get(INTERFACES, () => baseInterfaces);

    const user = userEvent.setup();
    renderTab();

    await user.hover(within(await card("Diameter")).getByText("Not listening"));
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "Set interfaces.diameter in the configuration file.",
    );
  });
});

describe("InterfacesTab N32 link", () => {
  it("links to Integrations › Roaming", async () => {
    api.get(INTERFACES, () => baseInterfaces);

    renderTab();

    expect(
      within(await card("N32 (SEPP)")).getByRole("link", {
        name: "Integrations › Roaming",
      }),
    ).toHaveAttribute("href", "/integrations/roaming");
  });
});
