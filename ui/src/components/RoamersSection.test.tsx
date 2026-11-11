// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
import { listRoamersByRadio } from "@/queries/roamers";
import RoamersSection from "./RoamersSection";

const api = setupApiServer();

const renderSection = () =>
  renderWithProviders(
    <RoamersSection
      title="Connected Roamers"
      queryKey={["roamers-by-radio", "gnb-01"]}
      fetchPage={(page, perPage) =>
        listRoamersByRadio("test-token", "gnb-01", page, perPage)
      }
      enabled
    />,
  );

describe("RoamersSection", () => {
  it("lists the roamers with a link to each", async () => {
    api.get("/api/v1/roamers", () => ({
      items: [
        {
          imsi: "208930000000001",
          partner: "Partner A",
          status: { num_sessions: 1 },
        },
      ],
      total_count: 1,
    }));

    renderSection();

    expect(
      await screen.findByText("Connected Roamers (1)"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "208930000000001" }),
    ).toHaveAttribute("href", "/subscribers/roamers/208930000000001");
    expect(screen.getByText("Partner A")).toBeInTheDocument();
    expect(api.lastRequest("/api/v1/roamers")?.params.get("radio")).toBe(
      "gnb-01",
    );
  });

  it("stays hidden when there are no roamers", async () => {
    api.get("/api/v1/roamers", () => ({ items: [], total_count: 0 }));

    renderSection();

    await waitFor(() =>
      expect(api.requests("/api/v1/roamers")).toHaveLength(1),
    );
    expect(screen.queryByText(/Connected Roamers/)).not.toBeInTheDocument();
  });
});
