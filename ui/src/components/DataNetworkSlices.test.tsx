// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer } from "@/test/apiServer";
import DataNetworkSlices from "./DataNetworkSlices";

const api = setupApiServer();

const pairs = [
  {
    slice: "default",
    data_network: "internet",
    nat: true,
    local_switch: false,
    network_instance: "default",
  },
];

const render = (ims = false) =>
  renderWithProviders(
    <DataNetworkSlices
      accessToken="token"
      dataNetwork="internet"
      ims={ims}
      slices={["default", "iot"]}
      pairs={pairs}
      networkInstances={["default", "ims"]}
      canEdit
      onError={vi.fn()}
    />,
    { auth: {} },
  );

describe("DataNetworkSlices", () => {
  it("updates NAT and local switching of one slice", async () => {
    const user = userEvent.setup();
    api.put("/api/v1/slices/:name/data-networks/:dn", () => ({}));
    render();

    await user.click(screen.getByRole("switch", { name: "NAT on default" }));
    await waitFor(() =>
      expect(
        api.lastRequest("/api/v1/slices/default/data-networks/internet")?.body,
      ).toEqual({ nat: false }),
    );

    await user.click(
      screen.getByRole("switch", { name: "Local switch on default" }),
    );
    await waitFor(() =>
      expect(
        api.lastRequest("/api/v1/slices/default/data-networks/internet")?.body,
      ).toEqual({ local_switch: true }),
    );
  });

  it("serves the data network on another slice", async () => {
    const user = userEvent.setup();
    api.post("/api/v1/slices/:name/data-networks", () => ({}));
    render();

    await user.click(screen.getByRole("combobox", { name: /Slice/ }));
    await user.click(await screen.findByRole("option", { name: "iot" }));
    await user.click(screen.getByRole("button", { name: /Serve on slice/ }));

    await waitFor(() =>
      expect(api.lastRequest("/api/v1/slices/iot/data-networks")?.body).toEqual(
        { data_network: "internet", nat: true, local_switch: false },
      ),
    );
  });

  it("moves a slice to another network instance", async () => {
    const user = userEvent.setup();
    api.put("/api/v1/slices/:name/data-networks/:dn", () => ({}));
    render();

    await user.click(
      screen.getByRole("combobox", { name: /Network instance on default/ }),
    );
    await user.click(await screen.findByRole("option", { name: "ims" }));

    await waitFor(() =>
      expect(
        api.lastRequest("/api/v1/slices/default/data-networks/internet")?.body,
      ).toEqual({ network_instance: "ims" }),
    );
  });

  it("locks forwarding on an IMS data network", () => {
    render(true);

    expect(
      screen.getByRole("switch", { name: "NAT on default" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("switch", { name: "Local switch on default" }),
    ).toBeDisabled();
  });
});
