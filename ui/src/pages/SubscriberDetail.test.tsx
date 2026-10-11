// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { screen } from "@testing-library/react";
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

describe("Subscriber IMS registration", () => {
  it("shows the IMS registration state", async () => {
    seedSubscriber("+15551230001", {
      ims: {
        private_identity: `${IMSI}@ims.mnc001.mcc001.3gppnetwork.org`,
        public_identities: [
          {
            identity: "tel:+15551230001",
            barred: false,
            user_state: "registered_unreg_services",
          },
        ],
      },
    });
    renderDetail();

    expect(await screen.findByText("IMS Registration")).toBeInTheDocument();
    expect(
      screen.getByText("Unregistered (S-CSCF assigned)"),
    ).toBeInTheDocument();
  });

  const imsSubscription = {
    private_identity: `${IMSI}@ims.mnc001.mcc001.3gppnetwork.org`,
    public_identities: [
      {
        identity: `sip:${IMSI}@ims.mnc001.mcc001.3gppnetwork.org`,
        barred: true,
        user_state: "not_registered",
      },
    ],
  };

  it("shows No MSISDN instead of the IMS state without an MSISDN", async () => {
    seedSubscriber("", { ims: imsSubscription });
    renderDetail();

    expect(await screen.findByText("No MSISDN")).toBeInTheDocument();
    expect(screen.queryByText("Not registered")).not.toBeInTheDocument();
  });

  it("shows whether the registered system advertised IMS voice over PS", async () => {
    seedSubscriber("+15551230001", {
      ims: imsSubscription,
      registrations: [
        {
          system: "5G",
          registered: true,
          connection_state: "connected",
          last_seen_at: "2026-03-16T12:34:56Z",
          ims_voice_over_ps: false,
          connection: null,
        },
      ],
    });
    renderDetail();

    expect(await screen.findByText("Voice over PS")).toBeInTheDocument();
    expect(screen.getByText("No")).toBeInTheDocument();
  });

  it("omits Voice over PS without a registration", async () => {
    seedSubscriber("+15551230001", { ims: imsSubscription });
    renderDetail();

    expect(await screen.findByText("IMS Registration")).toBeInTheDocument();
    expect(screen.queryByText("Voice over PS")).not.toBeInTheDocument();
  });

  it("omits IMS for a subscriber without an IMS subscription", async () => {
    seedSubscriber();
    renderDetail();

    expect(await screen.findByText("+15551230001")).toBeInTheDocument();
    expect(screen.queryByText("IMS Registration")).not.toBeInTheDocument();
  });
});
