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

const seedSubscriber = (msisdn = "+15551230001") => {
  api.get("/api/v1/subscribers/:imsi", () => ({
    imsi: IMSI,
    profile_name: "default",
    msisdn,
    registrations: [],
    sessions: [],
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
