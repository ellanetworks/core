// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router-dom";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
import { flowStats, usageBySubscriber } from "@/test/fixtures";
import SubscriberDetail from "./SubscriberDetail";

const api = setupApiServer();

const IMSI = "001010100007487";

const operator = (pcscfAddresses: string[]) => ({
  id: { mcc: "001", mnc: "01" },
  tracking: { supportedTacs: ["000001"] },
  homeNetworkKeys: [],
  nasSecurity: { ciphering: [], integrity: [] },
  spn: { fullName: "", shortName: "" },
  sms: { smsNumber: "" },
  ims: { pcscfAddresses },
});

const seedSubscriber = (
  msisdn = "+15551230001",
  extra: Record<string, unknown> = {},
  voice: { policies?: string[]; pcscf?: string[] } = {},
) => {
  api.get("/api/v1/subscribers/:imsi", () => ({
    imsi: IMSI,
    profile_name: "default",
    msisdn,
    registrations: [],
    sessions: [],
    ...extra,
  }));
  api.get("/api/v1/subscriber-usage", () => usageBySubscriber({}));
  api.get("/api/v1/flow-reports/stats", () => flowStats());
  api.get("/api/v1/policies", () => {
    const items = (voice.policies ?? []).map((dn) => ({
      name: dn,
      data_network_name: dn,
    }));
    return { items, total_count: items.length };
  });
  api.get("/api/v1/operator", () => operator(voice.pcscf ?? []));
};

const renderDetail = (role = "Admin") =>
  renderWithProviders(
    <Routes>
      <Route path="/subscribers/:imsi" element={<SubscriberDetail />} />
    </Routes>,
    { initialEntries: [`/subscribers/${IMSI}`], auth: { role } },
  );

describe("Subscriber MSISDN", () => {
  it("lets an admin edit the MSISDN", async () => {
    seedSubscriber();
    renderDetail("Admin");

    expect(
      await screen.findByRole("button", { name: "Edit MSISDN" }),
    ).toBeInTheDocument();
  });

  it("hides the MSISDN edit from a read-only user", async () => {
    seedSubscriber();
    renderDetail("Read Only");

    expect(await screen.findByText("+15551230001")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Edit MSISDN" }),
    ).not.toBeInTheDocument();
  });
});

describe("Subscriber voice", () => {
  const card = async () =>
    within(
      (await screen.findByRole("heading", { name: "Voice" })).closest(
        ".MuiCard-root",
      ) as HTMLElement,
    );

  it("shows a ready subscriber's IMS subscription", async () => {
    const impi = `${IMSI}@ims.mnc001.mcc001.3gppnetwork.org`;
    seedSubscriber(
      "+15551230001",
      {
        registrations: [
          {
            system: "4G",
            registered: true,
            connection_state: "connected",
            ims_voice_over_ps: true,
            connection: null,
          },
        ],
        ims: {
          private_identity: impi,
          scscf_name: "sip:scscf.ims.mnc001.mcc001.3gppnetwork.org:5080",
          public_identities: [
            {
              identity: "tel:+15551230001",
              barred: false,
              user_state: "registered",
            },
            { identity: `sip:${impi}`, barred: true, user_state: "registered" },
          ],
        },
      },
      { policies: ["ims"], pcscf: ["10.6.0.5"] },
    );
    renderDetail();

    const voice = await card();
    expect(await voice.findByText("Ready")).toBeInTheDocument();
    expect(voice.getByText("IMS Voice over PS (4G)")).toBeInTheDocument();
    expect(voice.getByText("Supported")).toBeInTheDocument();
    expect(voice.getByText(impi)).toBeInTheDocument();
    expect(voice.getByText("tel:+15551230001")).toBeInTheDocument();
    expect(voice.getAllByText("Registered")).toHaveLength(2);
    expect(voice.getByText("Barred")).toBeInTheDocument();
    expect(
      voice.getByText("sip:scscf.ims.mnc001.mcc001.3gppnetwork.org:5080"),
    ).toBeInTheDocument();
  });

  it("names what a subscriber still needs for voice", async () => {
    const user = userEvent.setup();
    seedSubscriber("", {}, { policies: ["internet"] });
    renderDetail();

    const voice = await card();
    expect(
      await voice.findByText("a policy on ims in its profile"),
    ).toHaveAttribute("href", "/profiles/default");
    expect(voice.getByText("P-CSCF addresses")).toHaveAttribute(
      "href",
      "/operator",
    );
    expect(voice.queryByText("Private Identity")).not.toBeInTheDocument();

    await user.click(voice.getByRole("button", { name: "a phone number" }));
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
  });
});
