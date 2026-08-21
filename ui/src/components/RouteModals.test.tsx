// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import CreateRouteModal from "./CreateRouteModal";
import CreateStaticIpModal from "./CreateStaticIpModal";
import CreateFramedRouteModal from "./CreateFramedRouteModal";

const api = setupApiServer();

const dialog = () => screen.getByRole("dialog");
const button = (name: RegExp) => within(dialog()).getByRole("button", { name });
const field = (label: RegExp) => screen.getByLabelText(label);

const ELIGIBLE = "/api/v1/subscribers";

describe("CreateRouteModal", () => {
  const PATH = "/api/v1/networking/routes";

  const render = () => {
    const onClose = vi.fn();
    renderWithProviders(
      <CreateRouteModal open onClose={onClose} onSuccess={vi.fn()} />,
      { auth: {} },
    );
    return { onClose };
  };

  it("rejects a destination that is not CIDR", async () => {
    const user = userEvent.setup();
    render();

    await user.type(field(/Destination/), "10.0.0.1");
    await user.tab();

    await screen.findByText("Destination must be a valid CIDR (IPv4 or IPv6)");
    expect(button(/^Create$/)).toBeDisabled();
  });

  it("checking Default Route fills and locks the destination", async () => {
    const user = userEvent.setup();
    render();

    await user.click(screen.getByRole("checkbox"));

    await waitFor(() => expect(field(/Destination/)).toHaveValue("0.0.0.0/0"));
    expect(field(/Destination/)).toBeDisabled();
  });

  it("derives an IPv6 default destination from an IPv6 gateway", async () => {
    const user = userEvent.setup();
    render();

    await user.type(field(/Gateway/), "fd00::1");
    await user.click(screen.getByRole("checkbox"));

    await waitFor(() => expect(field(/Destination/)).toHaveValue("::/0"));
  });

  it("unchecking Default Route clears the destination again", async () => {
    const user = userEvent.setup();
    render();

    await user.click(screen.getByRole("checkbox"));
    await waitFor(() => expect(field(/Destination/)).toHaveValue("0.0.0.0/0"));

    await user.click(screen.getByRole("checkbox"));
    await waitFor(() => expect(field(/Destination/)).toHaveValue(""));
    expect(field(/Destination/)).not.toBeDisabled();
  });

  it("submits the route with a numeric metric", async () => {
    const user = userEvent.setup();
    api.post(PATH, () => ({}));
    const { onClose } = render();

    await user.type(field(/Destination/), "10.0.0.0/8");
    await user.type(field(/Gateway/), "10.0.0.1");
    fireEvent.change(field(/Metric/), { target: { value: "100" } });

    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(PATH)?.body).toEqual({
      destination: "10.0.0.0/8",
      gateway: "10.0.0.1",
      interface: "n6",
      metric: 100,
    });
  });
});

describe("CreateStaticIpModal", () => {
  const CREATE_PATH = "/api/v1/networking/data-networks/internet/static-ips";

  const seedSubscribers = () =>
    api.get(ELIGIBLE, () => ({
      items: [{ imsi: "001010123456789" }],
      page: 1,
      per_page: 100,
      total_count: 1,
    }));

  it("picks a subscriber and posts the address", async () => {
    const user = userEvent.setup();
    seedSubscribers();
    api.post(CREATE_PATH, () => ({}));
    const onClose = vi.fn();

    renderWithProviders(
      <CreateStaticIpModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        dataNetwork="internet"
        ipv4Pool="10.45.0.0/22"
      />,
      { auth: {} },
    );

    expect(screen.getByText(/IPv4 pool: 10.45.0.0\/22/)).toBeInTheDocument();
    expect(button(/^Save$/)).toBeDisabled();

    await user.click(
      await screen.findByRole("combobox", { name: /Subscriber/ }),
    );
    await user.click(
      await screen.findByRole("option", { name: "001010123456789" }),
    );
    await user.type(field(/Address/), "10.45.0.10");

    await waitFor(() => expect(button(/^Save$/)).toBeEnabled());
    await user.click(button(/^Save$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(CREATE_PATH)?.body).toMatchObject({
      imsi: "001010123456789",
      address: "10.45.0.10",
    });
  });

  it("locks the subscriber and warns about an active session when editing", async () => {
    renderWithProviders(
      <CreateStaticIpModal
        open
        onClose={vi.fn()}
        onSuccess={vi.fn()}
        dataNetwork="internet"
        ipv6Pool="2001:db8::/56"
        edit={{
          imsi: "001010123456789",
          ipVersion: "ipv6",
          address: "2001:db8::5",
          active: true,
        }}
      />,
      { auth: {} },
    );

    expect(field(/Subscriber/)).toBeDisabled();
    expect(field(/Address/)).toHaveValue("2001:db8::5");
    expect(screen.getByText(/has an active session/)).toBeInTheDocument();
    expect(screen.getByText("IPv6 pool: 2001:db8::/56")).toBeInTheDocument();
  });

  it("shows the raw backend error without a prefix", async () => {
    const user = userEvent.setup();
    api.put(
      "/api/v1/networking/data-networks/:name/static-ips/:imsi/:version",
      () => httpError(409, "address already assigned"),
    );

    renderWithProviders(
      <CreateStaticIpModal
        open
        onClose={vi.fn()}
        onSuccess={vi.fn()}
        dataNetwork="internet"
        edit={{
          imsi: "001010123456789",
          ipVersion: "ipv4",
          address: "1.1.1.1",
        }}
      />,
      { auth: {} },
    );

    await user.click(button(/^Save$/));

    const alert = await screen.findByText(/address already assigned/);
    expect(alert.textContent).not.toMatch(/Failed to/);
  });
});

describe("CreateFramedRouteModal", () => {
  const renderEdit = (ipv4: string[] = [], ipv6: string[] = []) => {
    const onClose = vi.fn();
    renderWithProviders(
      <CreateFramedRouteModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        dataNetwork="internet"
        edit={{ imsi: "001010123456789", ipv4, ipv6 }}
      />,
      { auth: {} },
    );
    return { onClose };
  };

  it("requires at least one prefix", () => {
    renderEdit();
    expect(button(/^Save$/)).toBeDisabled();
  });

  it("rejects an invalid CIDR prefix", async () => {
    const user = userEvent.setup();
    renderEdit();

    await user.click(screen.getByRole("button", { name: /Add IPv4 prefix/ }));
    const input = within(dialog()).getAllByPlaceholderText(
      "e.g., 192.168.60.0/24",
    )[0];
    await user.type(input, "not-a-cidr");

    await screen.findByText("Enter a valid CIDR prefix.");
    expect(button(/^Save$/)).toBeDisabled();
  });

  it("caps each family at eight prefixes", async () => {
    renderEdit(Array.from({ length: 8 }, (_, i) => `10.0.${i}.0/24`));

    expect(
      screen.getByRole("button", { name: /Add IPv4 prefix/ }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: /Add IPv6 prefix/ }),
    ).toBeEnabled();
    await waitFor(() => expect(button(/^Save$/)).toBeEnabled());
  });

  it("drops blank rows and submits cleaned prefixes", async () => {
    const user = userEvent.setup();
    api.put(
      "/api/v1/networking/data-networks/:name/framed-routes/:imsi",
      () => ({}),
    );
    const { onClose } = renderEdit(["10.0.0.0/24"]);

    await user.click(screen.getByRole("button", { name: /Add IPv4 prefix/ }));
    await waitFor(() => expect(button(/^Save$/)).toBeEnabled());
    await user.click(button(/^Save$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(
      api.lastRequest(
        "/api/v1/networking/data-networks/internet/framed-routes/001010123456789",
      )?.body,
    ).toMatchObject({ ipv4: ["10.0.0.0/24"], ipv6: [] });
  });
});
