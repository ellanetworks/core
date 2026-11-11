// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router-dom";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
import { flowStats, usageBySubscriber } from "@/test/fixtures";
import SubscriberDetail from "./SubscriberDetail";

const api = setupApiServer();

const IMSI = "001010100007487";

const seedSubscriber = (
  msisdn = "+15551230001",
  extra: Record<string, unknown> = {},
  ims: Record<string, unknown> = {
    pcscfAddresses: [],
    voiceStatus: "no_pcscf",
  },
) => {
  api.get("/api/v1/integrations/ims", () => ({
    homeDomain: "",
    derivedHomeDomain: "ims.mnc001.mcc001.3gppnetwork.org",
    restrictPeers: true,
    ...ims,
  }));
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

const voiceOn = { pcscfAddresses: ["10.6.0.5"], voiceStatus: "enabled" };

const identities = (
  userState: string,
  msisdn = true,
  mcpttState: string | null = null,
) => ({
  private_identity: `${IMSI}@ims.mnc001.mcc001.3gppnetwork.org`,
  public_identities: [
    ...(msisdn
      ? [
          {
            identity: "tel:+15551230001",
            barred: false,
            registration_set: "mmtel",
            user_state: userState,
          },
        ]
      : []),
    {
      identity: `sip:${IMSI}@ims.mnc001.mcc001.3gppnetwork.org`,
      barred: true,
      registration_set: "mmtel",
      user_state: userState,
    },
    ...(msisdn && mcpttState
      ? [
          {
            identity: "sip:mcptt-15551230001@ims.mnc001.mcc001.3gppnetwork.org",
            barred: false,
            registration_set: "mcptt",
            user_state: mcpttState,
          },
        ]
      : []),
  ],
});

const registered = (voiceOverPS: boolean) => [
  {
    system: "5G",
    registered: true,
    connection_state: "connected",
    last_seen_at: "2026-03-16T12:34:56Z",
    ims_voice_over_ps: voiceOverPS,
    connection: null,
  },
];

const voiceRow = async () => {
  const label = await screen.findByText("Voice");
  return label.parentElement!;
};

describe("Subscriber voice", () => {
  it("shows the IMS registration once voice is offered", async () => {
    seedSubscriber(
      "+15551230001",
      { ims: identities("registered"), registrations: registered(true) },
      voiceOn,
    );
    renderDetail();

    expect(await voiceRow()).toHaveTextContent("Registered");
  });

  it("shows the MCPTT registration apart from voice", async () => {
    seedSubscriber(
      "+15551230001",
      {
        ims: identities("not_registered", true, "registered"),
        registrations: registered(true),
      },
      voiceOn,
    );
    renderDetail();

    expect(await voiceRow()).toHaveTextContent("Not registered");
    expect((await screen.findByText("MCPTT")).parentElement!).toHaveTextContent(
      "Registered",
    );
  });

  it("shows MCPTT as not registered while its client is not registered", async () => {
    seedSubscriber(
      "+15551230001",
      {
        ims: identities("registered", true, "not_registered"),
        registrations: registered(true),
      },
      voiceOn,
    );
    renderDetail();

    expect(await voiceRow()).toHaveTextContent("Registered");
    expect((await screen.findByText("MCPTT")).parentElement!).toHaveTextContent(
      "Not registered",
    );
  });

  it("hides MCPTT without an MCPTT identity", async () => {
    seedSubscriber(
      "+15551230001",
      { ims: identities("registered"), registrations: registered(true) },
      voiceOn,
    );
    renderDetail();

    expect(await voiceRow()).toHaveTextContent("Registered");
    expect(screen.queryByText("MCPTT")).not.toBeInTheDocument();
  });

  it("says the phone has no number when every identity is barred", async () => {
    seedSubscriber("", { ims: identities("not_registered", false) }, voiceOn);
    renderDetail();

    expect(await voiceRow()).toHaveTextContent("No phone number");
  });

  it("explains a network-side cause", async () => {
    seedSubscriber(
      "+15551230001",
      { ims: identities("not_registered") },
      { pcscfAddresses: ["2001:db8::5"], voiceStatus: "ip_family_mismatch" },
    );
    renderDetail();

    const row = await voiceRow();
    expect(row).toHaveTextContent("Off");
    expect(row).toHaveTextContent(
      "No P-CSCF address matches the ims data network's IP family",
    );
  });

  it("says when the phone was not offered voice at its last registration", async () => {
    seedSubscriber(
      "+15551230001",
      { ims: identities("not_registered"), registrations: registered(false) },
      voiceOn,
    );
    renderDetail();

    const row = await voiceRow();
    expect(row).toHaveTextContent("Not offered to the phone");
    expect(row).toHaveTextContent("next registration");
  });

  it("says voice is not offered while its advertisement is off", async () => {
    seedSubscriber(
      "+15551230001",
      { ims: identities("not_registered"), registrations: registered(true) },
      { ...voiceOn, advertiseVoice: false },
    );
    renderDetail();

    const row = await voiceRow();
    expect(row).toHaveTextContent("Not offered to the phone");
    expect(row).toHaveTextContent(
      "Advertise IMS Voice is off in Integrations › IMS",
    );
  });

  it("says a subscriber is not subscribed when voice is set up", async () => {
    seedSubscriber("+15551230001", {}, voiceOn);
    renderDetail();

    expect(await voiceRow()).toHaveTextContent("Not subscribed");
  });

  it("shows no voice line on a network without voice", async () => {
    seedSubscriber();
    renderDetail();

    expect(await screen.findByText("+15551230001")).toBeInTheDocument();
    expect(screen.queryByText("Voice")).not.toBeInTheDocument();
  });
});

const ipSMGW = {
  number: "+15550000100",
  diameter_host: "ims.ims.mnc001.mcc001.3gppnetwork.org",
  diameter_realm: "ims.mnc001.mcc001.3gppnetwork.org",
};

const deregisterButton = () =>
  screen.queryByRole("button", { name: "Deregister from IMS" });

describe("Subscriber IMS deregistration", () => {
  it("offers it while the subscriber is registered with the IMS", async () => {
    seedSubscriber(
      "+15551230001",
      { ims: identities("registered"), registrations: registered(true) },
      voiceOn,
    );
    renderDetail();

    expect(
      await screen.findByRole("button", { name: "Deregister from IMS" }),
    ).toBeInTheDocument();
  });

  it("offers it while only an IP-SM-GW is registered", async () => {
    seedSubscriber(
      "+15551230001",
      { ims: identities("not_registered"), ip_sm_gw: ipSMGW },
      voiceOn,
    );
    renderDetail();

    expect(
      await screen.findByRole("button", { name: "Deregister from IMS" }),
    ).toBeInTheDocument();
  });

  it("hides it when there is nothing to clear", async () => {
    seedSubscriber(
      "+15551230001",
      { ims: identities("not_registered") },
      voiceOn,
    );
    renderDetail();

    expect(await voiceRow()).toBeInTheDocument();
    expect(deregisterButton()).not.toBeInTheDocument();
  });

  it("hides it from a read-only user", async () => {
    seedSubscriber(
      "+15551230001",
      { ims: identities("registered"), ip_sm_gw: ipSMGW },
      voiceOn,
    );
    renderDetail("Read Only");

    expect(await voiceRow()).toHaveTextContent("Registered");
    expect(deregisterButton()).not.toBeInTheDocument();
  });

  it.each([
    [true, "Subscriber deregistered from the IMS."],
    [false, "The IMS did not answer"],
  ])(
    "deregisters after confirmation (acknowledged: %s)",
    async (acknowledged, message) => {
      seedSubscriber(
        "+15551230001",
        { ims: identities("registered"), ip_sm_gw: ipSMGW },
        voiceOn,
      );
      api.post("/api/v1/subscribers/:imsi/ims/deregister", () => ({
        message: "IMS registration cleared",
        acknowledged,
      }));
      renderDetail();

      await userEvent.click(
        await screen.findByRole("button", { name: "Deregister from IMS" }),
      );
      const dialog = await screen.findByRole("dialog");
      await userEvent.click(
        within(dialog).getByRole("button", { name: "Deregister" }),
      );

      expect(await screen.findByText(new RegExp(message))).toBeInTheDocument();
      await waitFor(() =>
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
      );
      expect(
        api.requests(`/api/v1/subscribers/${IMSI}/ims/deregister`),
      ).toHaveLength(1);
    },
  );
});
