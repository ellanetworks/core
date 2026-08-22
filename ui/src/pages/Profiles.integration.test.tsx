// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
import Profiles from "./Profiles";

const api = setupApiServer();

const PROFILES = "/api/v1/profiles";

describe("Profiles page create flow", () => {
  it("shows the new profile in the table after creating it", async () => {
    const user = userEvent.setup();

    const stored = [
      {
        name: "existing",
        ue_ambr_uplink: "100 Mbps",
        ue_ambr_downlink: "100 Mbps",
        allow_4g: true,
        allow_5g: true,
      },
    ];

    api.get(PROFILES, () => ({
      items: stored,
      page: 1,
      per_page: 25,
      total_count: stored.length,
    }));
    api.post(PROFILES, (req) => {
      const body = req.body as { name: string };
      stored.push({
        name: body.name,
        ue_ambr_uplink: "100 Mbps",
        ue_ambr_downlink: "100 Mbps",
        allow_4g: true,
        allow_5g: true,
      });
      return {};
    });

    renderWithProviders(<Profiles />, { auth: {} });

    await screen.findByText("existing");

    await user.click(screen.getByRole("button", { name: /Create/i }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText(/Name/), "brand-new");
    await waitFor(() =>
      expect(
        within(dialog).getByRole("button", { name: /^Create$/ }),
      ).toBeEnabled(),
    );
    await user.click(within(dialog).getByRole("button", { name: /^Create$/ }));

    await screen.findByText("Profile created successfully.");
    expect(
      api.requests(PROFILES).filter((r) => r.method === "POST"),
    ).toHaveLength(1);

    await waitFor(
      () => expect(screen.getByText("brand-new")).toBeInTheDocument(),
      { timeout: 3000 },
    );
  });

  it("replaces the empty state with the new row when the table starts empty", async () => {
    const user = userEvent.setup();
    const stored: { name: string }[] = [];

    api.get(PROFILES, () => ({
      items: stored.map((p) => ({
        name: p.name,
        ue_ambr_uplink: "100 Mbps",
        ue_ambr_downlink: "100 Mbps",
        allow_4g: true,
        allow_5g: true,
      })),
      page: 1,
      per_page: 25,
      total_count: stored.length,
    }));
    api.post(PROFILES, (req) => {
      stored.push({ name: (req.body as { name: string }).name });
      return {};
    });

    renderWithProviders(<Profiles />, { auth: {} });

    await screen.findByText("No profiles yet");

    await user.click(screen.getByRole("button", { name: /Create/i }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText(/Name/), "first-one");
    await waitFor(() =>
      expect(
        within(dialog).getByRole("button", { name: /^Create$/ }),
      ).toBeEnabled(),
    );
    await user.click(within(dialog).getByRole("button", { name: /^Create$/ }));

    await screen.findByText("Profile created successfully.");
    await waitFor(
      () => expect(screen.getByText("first-one")).toBeInTheDocument(),
      { timeout: 3000 },
    );
    expect(screen.queryByText("No profiles yet")).not.toBeInTheDocument();
  });
});
