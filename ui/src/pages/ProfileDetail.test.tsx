// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { beforeEach, describe, it, expect } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router-dom";
import { renderWithProviders } from "@/test/renderWithProviders";
import { httpError, setupApiServer } from "@/test/apiServer";
import ProfileDetail from "./ProfileDetail";

const api = setupApiServer();

const SP_PATH = "/api/v1/profiles/default/ims-service-profiles";

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
  api.get("/api/v1/operator/ims/service-profiles", () => ({
    items: [
      { name: "mcptt", initial_filter_criteria: [] },
      { name: "volte", initial_filter_criteria: [] },
    ],
  }));
  api.get("/api/v1/networking/data-networks", () => ({
    items: [
      { name: "internet", ims: false },
      { name: "ims", ims: true },
    ],
    page: 1,
    per_page: 100,
    total_count: 2,
  }));
});

const NO_IMS_POLICY =
  "This profile has no policy on an IMS data network, so its subscribers can't register with the IMS.";

const policiesOn = (...dataNetworks: string[]) =>
  api.get("/api/v1/policies", () => ({
    items: dataNetworks.map((dn) => ({
      name: dn,
      data_network_name: dn,
      slice_name: "default",
      session_ambr_uplink: "100 Mbps",
      session_ambr_downlink: "100 Mbps",
      var5qi: 9,
      arp: 15,
    })),
    page: 1,
    per_page: 100,
    total_count: dataNetworks.length,
  }));

const renderProfile = (role = "Admin") =>
  renderWithProviders(
    <Routes>
      <Route path="/profiles/:name" element={<ProfileDetail />} />
    </Routes>,
    { initialEntries: ["/profiles/default"], auth: { role } },
  );

const card = async () =>
  within(
    (await screen.findByText("IMS Service Profiles")).closest(
      ".MuiCard-root",
    ) as HTMLElement,
  );

const choose = async (
  user: ReturnType<typeof userEvent.setup>,
  scope: ReturnType<typeof within>,
  name: string | RegExp,
  option: string | RegExp,
) => {
  await user.click(scope.getByRole("combobox", { name }));
  await user.click(await screen.findByRole("option", { name: option }));
};

describe("ProfileDetail IMS service profiles", () => {
  it("shows the service profile of each registration set", async () => {
    api.get(SP_PATH, () => ({ mmtel: "volte", mcptt: null }));
    renderProfile();

    const c = await card();
    await c.findByText("volte");
    expect(c.getAllByRole("row").map((r) => r.textContent)).toEqual([
      "Registration SetService Profile",
      "MMTel (voice, SMS)sip:+MSISDN, tel:+MSISDN, barred IMSIvolte",
      "MCPTTsip:mcptt-MSISDNOff",
    ]);
  });

  it("assigns service profiles", async () => {
    const user = userEvent.setup();
    api.get(SP_PATH, () => ({ mmtel: "volte", mcptt: null }));
    api.put(SP_PATH, () => ({}));
    renderProfile();

    await (await card()).findByText("volte");
    await user.click(
      screen.getByRole("button", { name: "Edit IMS service profiles" }),
    );
    const dialog = within(
      await screen.findByRole("dialog", { name: "Edit IMS Service Profiles" }),
    );
    await choose(user, dialog, /MCPTT/, "mcptt");
    await choose(user, dialog, /MMTel/, "None (basic calls only)");
    await user.click(dialog.getByRole("button", { name: "Save" }));

    await screen.findByText("IMS service profiles updated successfully.");
    expect(api.requests(SP_PATH).find((r) => r.method === "PUT")?.body).toEqual(
      { mmtel: null, mcptt: "mcptt" },
    );
  });

  it("points to the Operator page when there is no service profile", async () => {
    const user = userEvent.setup();
    api.get(SP_PATH, () => ({}));
    api.get("/api/v1/operator/ims/service-profiles", () => ({ items: [] }));
    renderProfile();

    await (await card()).findByText("MMTel (voice, SMS)");
    await user.click(
      screen.getByRole("button", { name: "Edit IMS service profiles" }),
    );
    const dialog = within(
      await screen.findByRole("dialog", { name: "Edit IMS Service Profiles" }),
    );
    expect(
      await dialog.findByRole("link", { name: "Operator page" }),
    ).toBeInTheDocument();
  });

  it("shows the error when the server rejects the assignment", async () => {
    const user = userEvent.setup();
    api.get(SP_PATH, () => ({ mmtel: "volte" }));
    api.put(SP_PATH, () => httpError(400, "Service profile not found: volte"));
    renderProfile();

    await (await card()).findByText("volte");
    await user.click(
      screen.getByRole("button", { name: "Edit IMS service profiles" }),
    );
    const dialog = within(
      await screen.findByRole("dialog", { name: "Edit IMS Service Profiles" }),
    );
    await user.click(dialog.getByRole("button", { name: "Save" }));

    expect(
      await dialog.findByText(
        "Failed to update IMS service profiles: Service profile not found: volte",
      ),
    ).toBeInTheDocument();
  });

  it("warns when the profile has no policy on an IMS data network", async () => {
    api.get(SP_PATH, () => ({ mmtel: "volte" }));
    policiesOn("internet");
    renderProfile();

    expect(await (await card()).findByText(NO_IMS_POLICY)).toBeInTheDocument();
  });

  it("does not warn when the profile has a policy on an IMS data network", async () => {
    api.get(SP_PATH, () => ({ mmtel: "volte" }));
    policiesOn("internet", "ims");
    renderProfile();

    const c = await card();
    await c.findByText("volte");
    await screen.findByText("Policies (2)");
    expect(c.queryByText(NO_IMS_POLICY)).not.toBeInTheDocument();
  });

  it("hides the edit button from a read-only user", async () => {
    api.get(SP_PATH, () => ({ mmtel: "volte" }));
    renderProfile("Read Only");

    await (await card()).findByText("volte");
    expect(
      screen.queryByRole("button", { name: "Edit IMS service profiles" }),
    ).not.toBeInTheDocument();
  });
});
