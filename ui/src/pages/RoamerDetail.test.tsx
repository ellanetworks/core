// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router-dom";
import {
  LOCATION_TEST_ID,
  renderWithProviders,
} from "@/test/renderWithProviders";
import { httpError, setupApiServer } from "@/test/apiServer";
import RoamerDetail from "./RoamerDetail";

const api = setupApiServer();

const IMSI = "208930000000001";

const ROAMER_PATH = `/api/v1/roamers/${IMSI}`;

const seedRoamer = (extra: Record<string, unknown> = {}) =>
  api.get("/api/v1/roamers/:imsi", () => ({
    imsi: IMSI,
    partner: "Partner A",
    home_plmn: { mcc: "208", mnc: "93" },
    msisdn: "+33612345678",
    since: "2026-11-01T10:00:00Z",
    registrations: [
      {
        system: "5G",
        registered: true,
        connection_state: "connected",
        radio: "gnb-01",
        connection: null,
      },
    ],
    sessions: [],
    ...extra,
  }));

const renderDetail = (role = "Admin") =>
  renderWithProviders(
    <Routes>
      <Route path="/subscribers/roamers/:imsi" element={<RoamerDetail />} />
      <Route path="/subscribers/roamers" element={<div>Roamers list</div>} />
    </Routes>,
    { initialEntries: [`/subscribers/roamers/${IMSI}`], auth: { role } },
  );

const fieldValue = async (label: string) =>
  (await screen.findByText(label)).parentElement!;

describe("Roamer detail", () => {
  it("shows the roamer read-only", async () => {
    seedRoamer();
    renderDetail();

    expect(await fieldValue("Partner")).toHaveTextContent("Partner A");
    expect(await fieldValue("Home PLMN")).toHaveTextContent("208 / 93");
    expect(await fieldValue("MSISDN")).toHaveTextContent("+33612345678");
    expect(screen.getByText("Connected")).toBeInTheDocument();
    for (const name of [
      "Delete",
      "Show credentials",
      "Edit profile",
      "Edit MSISDN",
    ]) {
      expect(screen.queryByRole("button", { name })).not.toBeInTheDocument();
    }
    expect(screen.queryByText("Voice")).not.toBeInTheDocument();
    expect(api.requests("/api/v1/subscribers/" + IMSI)).toHaveLength(0);
  });

  it("links back to the roamers", async () => {
    seedRoamer();
    renderDetail();

    expect(
      await screen.findByRole("link", { name: "Roamers" }),
    ).toHaveAttribute("href", "/subscribers/roamers");
  });

  it("deregisters the roamer after confirmation", async () => {
    const user = userEvent.setup();
    seedRoamer();
    api.delete("/api/v1/roamers/:imsi", () => ({
      message: "Roamer deregistered successfully",
    }));
    renderDetail();

    await user.click(await screen.findByRole("button", { name: "Deregister" }));
    await user.click(
      within(screen.getByRole("dialog")).getByRole("button", {
        name: "Deregister",
      }),
    );

    await waitFor(() =>
      expect(api.lastRequest(ROAMER_PATH)?.method).toBe("DELETE"),
    );
    expect(
      await screen.findByText(`Roamer "${IMSI}" deregistered successfully.`),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByTestId(LOCATION_TEST_ID)).toHaveTextContent(
        /^\/subscribers\/roamers$/,
      ),
    );
  });

  it("shows why a deregistration was refused", async () => {
    const user = userEvent.setup();
    seedRoamer();
    api.delete("/api/v1/roamers/:imsi", () =>
      httpError(409, "The roamer is still registered after one minute"),
    );
    renderDetail();

    await user.click(await screen.findByRole("button", { name: "Deregister" }));
    const dialog = screen.getByRole("dialog");
    await user.click(
      within(dialog).getByRole("button", { name: "Deregister" }),
    );

    expect(
      await within(dialog).findByText(
        "The roamer is still registered after one minute",
      ),
    ).toBeInTheDocument();
  });

  it("hides the deregistration from read-only users", async () => {
    seedRoamer();
    renderDetail("Read Only");

    expect(await fieldValue("Partner")).toHaveTextContent("Partner A");
    expect(
      screen.queryByRole("button", { name: "Deregister" }),
    ).not.toBeInTheDocument();
  });

  it("reports an unknown roamer", async () => {
    api.get("/api/v1/roamers/:imsi", () => httpError(404, "Roamer not found"));
    renderDetail();

    expect(await screen.findByText(/Roamer not found/)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Deregister" }),
    ).not.toBeInTheDocument();
  });
});
