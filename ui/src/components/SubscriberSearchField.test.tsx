// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
import SubscriberSearchField from "./SubscriberSearchField";

const api = setupApiServer();

const PATH = "/api/v1/subscribers";

const ROSTER = Array.from(
  { length: 40 },
  (_, i) => `00101${String(i + 1).padStart(10, "0")}`,
);

const seed = () =>
  api.get(PATH, ({ params }) => {
    const search = params.get("search") ?? "";
    const perPage = Number(params.get("per_page"));
    const matching = ROSTER.filter((imsi) => imsi.includes(search));

    return {
      items: matching.slice(0, perPage).map((imsi) => ({ imsi })),
      page: 1,
      per_page: perPage,
      total_count: matching.length,
    };
  });

describe("SubscriberSearchField", () => {
  it("lists one bounded page and searches the server as the user types", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    seed();

    renderWithProviders(
      <SubscriberSearchField value="" onChange={onChange} />,
      { auth: {} },
    );

    await user.click(screen.getByRole("combobox", { name: /Subscriber/ }));
    await screen.findByRole("option", { name: ROSTER[0] });
    expect(screen.getAllByRole("option").length).toBeLessThan(ROSTER.length);

    await user.type(screen.getByRole("combobox"), "0000000037");

    await waitFor(() =>
      expect(
        api.requests(PATH).some((r) => r.params.get("search") === "0000000037"),
      ).toBe(true),
    );

    await user.click(await screen.findByRole("option", { name: ROSTER[36] }));
    expect(onChange).toHaveBeenCalledWith(ROSTER[36]);
  });

  it("scopes the search to a data network", async () => {
    seed();

    renderWithProviders(
      <SubscriberSearchField
        value=""
        onChange={vi.fn()}
        dataNetwork="internet"
      />,
      { auth: {} },
    );

    await waitFor(() =>
      expect(api.lastRequest(PATH)?.params.get("data_network")).toBe(
        "internet",
      ),
    );
  });
});
