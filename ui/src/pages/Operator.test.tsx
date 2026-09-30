// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import Operator from "./Operator";

const api = setupApiServer();

const operator = (sms: Record<string, unknown>) => ({
  id: { mcc: "001", mnc: "01" },
  tracking: { supportedTacs: ["000001"] },
  homeNetworkKeys: [],
  nasSecurity: { ciphering: ["AES"], integrity: ["AES"] },
  spn: { fullName: "Ella Networks", shortName: "Ella" },
  sms,
});

const enabledSMS = {
  enabled: true,
  smscAddress: "2001:db8::10",
  smscPort: 3868,
  smsNumber: "+15550001111",
};

const diameter = (peer: Record<string, unknown>) => ({
  host: "mmec01.mmegi8204.mme.epc.mnc001.mcc001.3gppnetwork.org",
  realm: "epc.mnc001.mcc001.3gppnetwork.org",
  peers: [
    {
      role: "smsc",
      address: "2001:db8::10",
      port: 3868,
      since: "2026-09-29T16:00:00Z",
      ...peer,
    },
  ],
});

const smsRow = async (label: string) => {
  const cell = await screen.findByText(label);
  return within(cell.closest("tr")!);
};

const renderOperator = (role = "Admin") =>
  renderWithProviders(<Operator />, {
    initialEntries: ["/operator"],
    auth: { role },
  });

describe("Operator SMS section", () => {
  it("cannot turn SMS on without an SMSC and an SMS number", async () => {
    api.get("/api/v1/operator", () =>
      operator({
        enabled: false,
        smscAddress: "",
        smscPort: 3868,
        smsNumber: "",
      }),
    );
    renderOperator();

    expect(await (await smsRow("SMSC")).findByText("N/A")).toBeInTheDocument();
    expect(await screen.findByText("SMS is OFF")).toBeInTheDocument();
    expect(screen.getByRole("switch", { name: "SMS is OFF" })).toBeDisabled();
    expect(
      await (await smsRow("SMSC Link")).findByText("Disabled"),
    ).toBeInTheDocument();
    expect(api.requests("/api/v1/networking/diameter")).toHaveLength(0);
  });

  it("keeps the settings while SMS is off and turns it on", async () => {
    const user = userEvent.setup();
    api.get("/api/v1/operator", () =>
      operator({ ...enabledSMS, enabled: false }),
    );
    api.put("/api/v1/operator/sms", () => ({}));
    renderOperator();

    expect(
      await (await smsRow("SMSC")).findByText("[2001:db8::10]:3868"),
    ).toBeInTheDocument();
    expect(
      await (await smsRow("SMSC Link")).findByText("Disabled"),
    ).toBeInTheDocument();

    const toggle = await screen.findByRole("switch", { name: "SMS is OFF" });
    await waitFor(() => expect(toggle).toBeEnabled());
    await user.click(toggle);

    await waitFor(() =>
      expect(api.lastRequest("/api/v1/operator/sms")?.body).toEqual(enabledSMS),
    );
  });

  it("asks for confirmation before turning SMS off", async () => {
    const user = userEvent.setup();
    api.get("/api/v1/operator", () => operator(enabledSMS));
    api.get("/api/v1/networking/diameter", () => diameter({ state: "open" }));
    api.put("/api/v1/operator/sms", () => ({}));
    renderOperator();

    const toggle = await screen.findByRole("switch", { name: "SMS is ON" });
    await waitFor(() => expect(toggle).toBeEnabled());
    await user.click(toggle);

    expect(await screen.findByText("Turn SMS off?")).toBeInTheDocument();
    expect(api.requests("/api/v1/operator/sms")).toHaveLength(0);

    await user.click(screen.getByRole("button", { name: "Turn Off" }));

    await waitFor(() =>
      expect(api.lastRequest("/api/v1/operator/sms")?.body).toEqual({
        ...enabledSMS,
        enabled: false,
      }),
    );
  });

  it("shows the SMSC, the SMS number and a connected link", async () => {
    api.get("/api/v1/operator", () => operator(enabledSMS));
    api.get("/api/v1/networking/diameter", () =>
      diameter({
        state: "open",
        host: "smsc.example.org",
        realm: "example.org",
      }),
    );
    renderOperator();

    expect(
      await (await smsRow("SMSC")).findByText("[2001:db8::10]:3868"),
    ).toBeInTheDocument();
    expect(
      await (await smsRow("SMS Number")).findByText("+15550001111"),
    ).toBeInTheDocument();

    const link = await smsRow("SMSC Link");
    expect(await link.findByText("Connected")).toBeInTheDocument();
    expect(
      await link.findByText("smsc.example.org · example.org"),
    ).toBeInTheDocument();
  });

  it("shows a disconnected link", async () => {
    api.get("/api/v1/operator", () => operator(enabledSMS));
    api.get("/api/v1/networking/diameter", () => diameter({ state: "down" }));
    renderOperator();

    const link = await smsRow("SMSC Link");
    expect(await link.findByText("Disconnected")).toBeInTheDocument();
  });

  it("shows the link as connecting until the peer matches the SMSC", async () => {
    api.get("/api/v1/operator", () => operator(enabledSMS));
    api.get("/api/v1/networking/diameter", () =>
      diameter({ state: "open", address: "192.0.2.99" }),
    );
    renderOperator();

    const link = await smsRow("SMSC Link");
    expect(await link.findByText("Connecting")).toBeInTheDocument();
    expect(link.queryByText("Connected")).not.toBeInTheDocument();
  });

  it("shows the link as connecting before the node has a peer", async () => {
    api.get("/api/v1/operator", () => operator(enabledSMS));
    api.get("/api/v1/networking/diameter", () => ({
      host: "mmec01.mmegi8204.mme.epc.mnc001.mcc001.3gppnetwork.org",
      realm: "epc.mnc001.mcc001.3gppnetwork.org",
      peers: [],
    }));
    renderOperator();

    const link = await smsRow("SMSC Link");
    expect(await link.findByText("Connecting")).toBeInTheDocument();
  });

  it("says so when the link state cannot be loaded", async () => {
    api.get("/api/v1/operator", () => operator(enabledSMS));
    api.get("/api/v1/networking/diameter", () =>
      httpError(500, "diameter unavailable"),
    );
    renderOperator();

    const link = await smsRow("SMSC Link");
    expect(
      await link.findByText("Could not load the link state"),
    ).toBeInTheDocument();
  });

  it("refreshes the link state after an edit", async () => {
    const user = userEvent.setup();
    let address = "2001:db8::10";
    api.get("/api/v1/operator", () =>
      operator({ ...enabledSMS, smscAddress: address }),
    );
    api.get("/api/v1/networking/diameter", () =>
      diameter({ state: "open", address }),
    );
    api.put("/api/v1/operator/sms", () => {
      address = "2001:db8::11";
      return {};
    });
    renderOperator();

    expect(
      await (await smsRow("SMSC Link")).findByText("Connected"),
    ).toBeInTheDocument();
    const before = api.requests("/api/v1/networking/diameter").length;

    await user.click(
      await screen.findByRole("button", { name: "Edit SMS settings" }),
    );
    const input = await screen.findByLabelText(/SMSC Address/);
    await user.clear(input);
    await user.type(input, "2001:db8::11");
    await user.click(screen.getByRole("button", { name: "Update" }));

    expect(
      await (await smsRow("SMSC")).findByText("[2001:db8::11]:3868"),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(
        api.requests("/api/v1/networking/diameter").length,
      ).toBeGreaterThan(before),
    );
  });

  it("lets a read-only user see but not change SMS", async () => {
    api.get("/api/v1/operator", () => operator(enabledSMS));
    api.get("/api/v1/networking/diameter", () => diameter({ state: "open" }));
    renderOperator("Read Only");

    expect(
      await (await smsRow("SMSC")).findByText("[2001:db8::10]:3868"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Edit SMS settings" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("switch", { name: "SMS is ON" })).toBeDisabled();
  });
});
