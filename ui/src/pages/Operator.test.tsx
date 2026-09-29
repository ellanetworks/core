// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { screen, within } from "@testing-library/react";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
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

const renderOperator = () =>
  renderWithProviders(<Operator />, {
    initialEntries: ["/operator"],
    auth: {},
  });

describe("Operator SMS section", () => {
  it("shows SMS as disabled without an SMSC", async () => {
    api.get("/api/v1/operator", () =>
      operator({
        enabled: false,
        smscAddress: "",
        smscPort: 3868,
        smsNumber: "",
      }),
    );
    renderOperator();

    expect(
      await (await smsRow("SMSC")).findByText("Disabled"),
    ).toBeInTheDocument();
    expect(
      await (await smsRow("SMSC Link")).findByText("Disabled"),
    ).toBeInTheDocument();
    expect(api.requests("/api/v1/networking/diameter")).toHaveLength(0);
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
});
