// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import CreateDataNetworkModal from "./CreateDataNetworkModal";
import EditDataNetworkModal from "./EditDataNetworkModal";

const api = setupApiServer();

const PATH = "/api/v1/networking/data-networks";

const dialog = () => screen.getByRole("dialog");
const button = (name: RegExp) => within(dialog()).getByRole("button", { name });

const renderCreate = () => {
  const onClose = vi.fn();
  const onSuccess = vi.fn();
  renderWithProviders(
    <CreateDataNetworkModal open onClose={onClose} onSuccess={onSuccess} />,
    { auth: {} },
  );
  return { onClose, onSuccess };
};

const field = (label: RegExp) => screen.getByLabelText(label);

const replace = async (
  user: ReturnType<typeof userEvent.setup>,
  label: RegExp,
  value: string,
) => {
  const input = field(label);
  if ((input as HTMLInputElement).type === "number") {
    fireEvent.change(input, { target: { value } });
  } else {
    await user.clear(input);
    if (value) await user.type(input, value);
  }
  await user.tab();
};

describe("CreateDataNetworkModal", () => {
  it("prefills the documented defaults", () => {
    renderCreate();
    expect(field(/IPv4 Pool/)).toHaveValue("10.45.0.0/22");
    expect(field(/DNS/)).toHaveValue("8.8.8.8");
    expect(field(/MTU/)).toHaveValue(1456);
    expect(dialog()).toHaveAccessibleName("Create Data Network");
  });

  it("shows the IPv6 pool hint until the field is invalid", async () => {
    const user = userEvent.setup();
    renderCreate();

    expect(
      screen.getByText(/Ella Core delegates \/64s from within the pool/),
    ).toBeInTheDocument();

    await replace(user, /IPv6 Pool/, "2001:db8::/64");
    await screen.findByText(/prefix length between \/48 and \/60/i);
  });

  it("requires at least one pool across the two pool fields", async () => {
    const user = userEvent.setup();
    renderCreate();

    await user.type(field(/Name/), "internet");
    await replace(user, /IPv4 Pool/, "");

    await screen.findByText("At least one IP pool (IPv4 or IPv6) is required");
    expect(button(/^Create$/)).toBeDisabled();

    await replace(user, /IPv6 Pool/, "2001:db8::/56");
    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
  });

  it("rejects an invalid name", async () => {
    const user = userEvent.setup();
    renderCreate();

    await user.type(field(/Name/), "Not A Valid Name");
    await user.tab();

    await screen.findByText(/Must be a valid name/);
    expect(button(/^Create$/)).toBeDisabled();
  });

  it("omits an empty ipv6 pool from the request", async () => {
    const user = userEvent.setup();
    api.post(PATH, () => ({}));
    const { onClose, onSuccess } = renderCreate();

    await user.type(field(/Name/), "internet");
    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(onSuccess).toHaveBeenCalled();
    expect(api.lastRequest(PATH)?.body).toEqual({
      name: "internet",
      ipv4_pool: "10.45.0.0/22",
      dns: "8.8.8.8",
      mtu: 1456,
    });
  });

  it("sends mtu as a number", async () => {
    const user = userEvent.setup();
    api.post(PATH, () => ({}));
    renderCreate();

    await user.type(field(/Name/), "ims");
    await replace(user, /MTU/, "1400");
    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await waitFor(() =>
      expect(api.lastRequest(PATH)?.body).toMatchObject({ mtu: 1400 }),
    );
  });

  it("keeps the dialog open when the API rejects the create", async () => {
    const user = userEvent.setup();
    api.post(PATH, () => httpError(409, "data network already exists"));
    const { onClose } = renderCreate();

    await user.type(field(/Name/), "internet");
    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await screen.findByText(
      /Failed to create data network: .*data network already exists/,
    );
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe("EditDataNetworkModal", () => {
  const initialData = {
    name: "internet",
    ipv4_pool: "10.45.0.0/22",
    ipv6_pool: "",
    dns: "8.8.8.8",
    mtu: 1456,
  };

  it("seeds from initialData and locks the name", () => {
    renderWithProviders(
      <EditDataNetworkModal
        open
        onClose={vi.fn()}
        onSuccess={vi.fn()}
        initialData={initialData}
      />,
      { auth: {} },
    );

    expect(field(/Name/)).toHaveValue("internet");
    expect(field(/Name/)).toBeDisabled();
    expect(dialog()).toHaveAccessibleName("Edit Data Network");
  });

  it("puts the edited pool to the named resource", async () => {
    const user = userEvent.setup();
    api.put(`${PATH}/:name`, () => ({}));
    const onClose = vi.fn();

    renderWithProviders(
      <EditDataNetworkModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        initialData={initialData}
      />,
      { auth: {} },
    );

    await replace(user, /IPv4 Pool/, "10.60.0.0/16");
    await waitFor(() => expect(button(/^Update$/)).toBeEnabled());
    await user.click(button(/^Update$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(`${PATH}/internet`)?.body).toEqual({
      name: "internet",
      ipv4_pool: "10.60.0.0/16",
      dns: "8.8.8.8",
      mtu: 1456,
    });
  });
});
