// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { beforeEach, describe, it, expect, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  LOCATION_TEST_ID,
  renderWithProviders,
} from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
import AppRouter from "@/router";

vi.mock("@/layouts/CoreLayout", async () => {
  const { Outlet } = await import("react-router-dom");
  return { default: () => <Outlet /> };
});

vi.mock("@/layouts/AuthLayout", async () => {
  const { Outlet } = await import("react-router-dom");
  return { default: () => <Outlet /> };
});

const api = setupApiServer();

beforeEach(() => {
  api.get("/api/v1/integrations/diameter", () => ({
    realm: "epc.mnc001.mcc001.3gppnetwork.org",
    servedRealms: ["epc.mnc001.mcc001.3gppnetwork.org"],
    nodes: [],
    listener: { enabled: false, addresses: [] },
    peers: [],
  }));
  api.get("/api/v1/integrations/ims", () => ({
    pcscfAddresses: [],
    voiceStatus: "no_pcscf",
    homeDomain: "",
    derivedHomeDomain: "ims.mnc001.mcc001.3gppnetwork.org",
    restrictPeers: false,
  }));
  api.get("/api/v1/integrations/ims/allowed-peers", () => ({ items: [] }));
  api.get("/api/v1/integrations/ims/application-servers", () => ({
    items: [],
  }));
  api.get("/api/v1/integrations/ims/service-profiles", () => ({ items: [] }));
  api.get("/api/v1/integrations/sms", () => ({ servingNodeNumber: "" }));
  api.get("/api/v1/integrations/sms/smscs", () => ({ items: [] }));
  api.get("/api/v1/integrations/roaming/partners", () => ({ items: [] }));
  api.get("/api/v1/integrations/roaming/diameter-peers", () => ({
    items: [],
  }));
  api.get("/api/v1/integrations/roaming/sepp", () => ({
    listener: { configured: false, listening: false },
    trustAnchors: [],
    certificates: [],
  }));
  api.get("/api/v1/networking/interfaces", () => ({}));
  api.get("/api/v1/operator", () => ({
    id: { mcc: "001", mnc: "01" },
    tracking: { supportedTacs: ["000001"] },
    homeNetworkKeys: [],
    nasSecurity: { ciphering: ["AES"], integrity: ["AES"] },
    spn: { fullName: "Ella Networks", shortName: "Ella" },
  }));
});

const renderAt = (path: string) =>
  renderWithProviders(<AppRouter />, {
    initialEntries: [path],
    auth: { role: "Admin" },
  });

const location = () => screen.getByTestId(LOCATION_TEST_ID).textContent;

const selectedTab = () =>
  screen
    .getAllByRole("tab")
    .find((tab) => tab.getAttribute("aria-selected") === "true")?.textContent;

describe("Integrations routes", () => {
  it("opens IMS by default", async () => {
    renderAt("/integrations");

    expect(await screen.findByText("Allowed IMS Peers")).toBeInTheDocument();
    expect(location()).toBe("/integrations/ims");
    expect(selectedTab()).toBe("IMS");
  });

  it("redirects the former overview to IMS", async () => {
    renderAt("/integrations/overview");

    expect(await screen.findByText("Allowed IMS Peers")).toBeInTheDocument();
    expect(location()).toBe("/integrations/ims");
  });

  it("has no overview tab", async () => {
    renderAt("/integrations/ims");

    await screen.findByText("Allowed IMS Peers");
    expect(screen.getAllByRole("tab").map((tab) => tab.textContent)).toEqual([
      "IMS",
      "SMS",
      "Roaming",
    ]);
  });

  it.each([
    ["/integrations/ims", "IMS", "Allowed IMS Peers"],
    ["/integrations/sms", "SMS", "Serving Node Number"],
    ["/integrations/roaming", "Roaming", "No roaming partners yet."],
  ])("renders %s", async (path, tab, content) => {
    renderAt(path);

    expect(await screen.findByText(content)).toBeInTheDocument();
    expect(selectedTab()).toBe(tab);
  });

  it("switches tabs", async () => {
    const user = userEvent.setup();
    renderAt("/integrations/ims");

    await user.click(await screen.findByRole("tab", { name: "SMS" }));

    expect(await screen.findByText("SMSCs")).toBeInTheDocument();
    expect(location()).toBe("/integrations/sms");
  });
});
