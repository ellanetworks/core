// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import { Route, Routes } from "react-router-dom";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
import { radioEvent, radioEventPage } from "@/test/fixtures";
import RadioDetail from "./RadioDetail";

const api = setupApiServer();

const REF = "eNB:001-01:MacroeNB-00001";

const seedRadio = (name: string) => {
  api.get("/api/v1/ran/radios/:ref", () => ({
    name,
    ref: REF,
    id: "MacroeNB-00001",
    address: "10.0.0.9:36412",
    status: "online",
    connected_at: "2026-11-13T10:00:00Z",
    last_seen_at: "2026-11-13T10:00:00Z",
    disconnected_at: "",
    type: "eNB",
    supported_tais: [],
  }));
  api.get("/api/v1/subscribers", () => ({
    items: [],
    page: 1,
    per_page: 10,
    total_count: 0,
  }));
  api.get("/api/v1/roamers", () => ({
    items: [],
    page: 1,
    per_page: 10,
    total_count: 0,
  }));
  api.get("/api/v1/ran/events", () =>
    radioEventPage([radioEvent(1, { radio: name, radio_ref: REF })]),
  );
};

const renderDetail = () =>
  renderWithProviders(
    <Routes>
      <Route path="/radios/:ref" element={<RadioDetail />} />
    </Routes>,
    {
      initialEntries: [`/radios/${encodeURIComponent(REF)}`],
      auth: {},
    },
  );

describe("Radio detail", () => {
  it.each(["", "MacroeNB"])(
    "loads subscribers, roamers and events by ref for a radio named %j",
    async (name) => {
      seedRadio(name);
      renderDetail();

      await waitFor(() => {
        expect(
          api.lastRequest("/api/v1/subscribers")?.params.get("radio"),
        ).toBe(REF);
        expect(api.lastRequest("/api/v1/roamers")?.params.get("radio")).toBe(
          REF,
        );
        expect(api.lastRequest("/api/v1/ran/events")?.params.get("radio")).toBe(
          REF,
        );
      });

      const link = await screen.findByRole("link", {
        name: /View all events for/,
      });
      expect(link).toHaveAttribute(
        "href",
        `/radios/events?radio=${encodeURIComponent(REF)}`,
      );
      expect(link).toHaveTextContent(
        name
          ? `View all events for ${name}`
          : "View all events for eNB MacroeNB-00001",
      );
    },
  );
});
