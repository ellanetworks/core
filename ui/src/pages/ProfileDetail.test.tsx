// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { beforeEach, describe, it, expect } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router-dom";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
import ProfileDetail from "./ProfileDetail";

const api = setupApiServer();

const IMS_PATH = "/api/v1/profiles/default/ims-services";

const service = (name: string, kind = "mcptt") => ({
  name,
  kind,
  application_server_name: "as",
  initial_filter_criteria: [],
});

beforeEach(() => {
  api.get("/api/v1/profiles/default", () => ({
    name: "default",
    ue_ambr_uplink: "1 Gbps",
    ue_ambr_downlink: "1 Gbps",
    allow_4g: true,
    allow_5g: true,
  }));
  api.get("/api/v1/policies", () => ({
    items: [],
    page: 1,
    per_page: 100,
    total_count: 0,
  }));
  api.get("/api/v1/operator/ims/services", () => ({
    items: [service("mcptt"), service("voicemail", "custom"), service("conf")],
  }));
});

const renderProfile = (role = "Admin") =>
  renderWithProviders(
    <Routes>
      <Route path="/profiles/:name" element={<ProfileDetail />} />
    </Routes>,
    { initialEntries: ["/profiles/default"], auth: { role } },
  );

const card = async () =>
  within(
    (await screen.findByText("IMS Services")).closest(
      ".MuiCard-root",
    ) as HTMLElement,
  );

describe("ProfileDetail IMS services", () => {
  it("lists the profile's services in order", async () => {
    api.get(IMS_PATH, () => ({ services: ["voicemail", "mcptt"] }));
    renderProfile();

    const c = await card();
    const rows = await c.findAllByRole("row");
    expect(rows.map((r) => r.textContent)).toEqual(["1voicemail", "2mcptt"]);
    expect(c.getByText(/Order decides iFC priority/)).toBeInTheDocument();
  });

  it("shows when the profile has no services", async () => {
    api.get(IMS_PATH, () => ({ services: [] }));
    renderProfile();

    expect(
      await (await card()).findByText("No IMS services."),
    ).toBeInTheDocument();
  });

  it("adds, removes and reorders services", async () => {
    const user = userEvent.setup();
    let services = ["mcptt", "voicemail"];
    api.get(IMS_PATH, () => ({ services }));
    api.put(IMS_PATH, ({ body }) => {
      services = (body as { services: string[] }).services;
      return {};
    });
    renderProfile();

    await (await card()).findByText("mcptt");
    await user.click(screen.getByRole("button", { name: "Edit IMS services" }));
    const dialog = within(await screen.findByRole("dialog"));

    expect(
      dialog.getByRole("button", { name: "Move mcptt up" }),
    ).toBeDisabled();
    await user.click(dialog.getByRole("button", { name: "Remove mcptt" }));
    await user.click(dialog.getByRole("combobox", { name: /Add Service/ }));
    await user.click(await screen.findByRole("option", { name: "conf" }));
    await user.click(dialog.getByRole("button", { name: "Move conf up" }));
    await user.click(dialog.getByRole("button", { name: "Save" }));

    await screen.findByText("IMS services updated successfully.");
    expect(
      api.requests(IMS_PATH).find((r) => r.method === "PUT")?.body,
    ).toEqual({ services: ["conf", "voicemail"] });
  });

  it("only offers services not already assigned", async () => {
    const user = userEvent.setup();
    api.get(IMS_PATH, () => ({ services: ["mcptt"] }));
    renderProfile();

    await (await card()).findByText("mcptt");
    await user.click(screen.getByRole("button", { name: "Edit IMS services" }));
    const dialog = within(await screen.findByRole("dialog"));
    await user.click(dialog.getByRole("combobox", { name: /Add Service/ }));

    const options = await screen.findAllByRole("option");
    expect(options.map((o) => o.textContent)).toEqual(["voicemail", "conf"]);
  });

  it("hides the edit button from a read-only user", async () => {
    api.get(IMS_PATH, () => ({ services: ["mcptt"] }));
    renderProfile("Read Only");

    await (await card()).findByText("mcptt");
    expect(
      screen.queryByRole("button", { name: "Edit IMS services" }),
    ).not.toBeInTheDocument();
  });
});
