// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import EditInterfaceN3Modal from "./EditInterfaceN3Modal";
import CreateHomeNetworkKeyModal from "./CreateHomeNetworkKeyModal";

const api = setupApiServer();

const dialog = () => screen.getByRole("dialog");
const button = (name: RegExp) => within(dialog()).getByRole("button", { name });
const field = (label: RegExp) => screen.getByLabelText(label);

describe("EditInterfaceN3Modal", () => {
  const PATH = "/api/v1/networking/interfaces/n3";

  const render = (externalAddress = "") => {
    const onClose = vi.fn();
    renderWithProviders(
      <EditInterfaceN3Modal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        initialData={{ externalAddress }}
      />,
      { auth: {} },
    );
    return { onClose };
  };

  it("accepts an empty address and resolves its description", async () => {
    const user = userEvent.setup();
    api.put(PATH, () => ({}));
    const { onClose } = render();

    const describedBy = dialog().getAttribute("aria-describedby");
    expect(document.getElementById(describedBy!)).toHaveTextContent(
      /Configure an external address/,
    );

    await waitFor(() => expect(button(/^Update$/)).toBeEnabled());
    await user.click(button(/^Update$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(PATH)?.body).toEqual({ external_address: "" });
  });

  it("rejects a non-IP address", async () => {
    const user = userEvent.setup();
    render();

    await user.type(field(/External Address/), "not-an-ip");

    await screen.findByText(
      "External address must be a valid IPv4 or IPv6 address",
    );
    expect(button(/^Update$/)).toBeDisabled();
  });

  it("accepts an IPv6 address", async () => {
    const user = userEvent.setup();
    api.put(PATH, () => ({}));
    render("1.2.3.4");

    await user.clear(field(/External Address/));
    await user.type(field(/External Address/), "fd00::1");
    await waitFor(() => expect(button(/^Update$/)).toBeEnabled());
    await user.click(button(/^Update$/));

    await waitFor(() =>
      expect(api.lastRequest(PATH)?.body).toEqual({
        external_address: "fd00::1",
      }),
    );
  });
});

describe("CreateHomeNetworkKeyModal", () => {
  const PATH = "/api/v1/operator/home-network-keys";

  const render = () => {
    const onClose = vi.fn();
    renderWithProviders(
      <CreateHomeNetworkKeyModal open onClose={onClose} onSuccess={vi.fn()} />,
      { auth: {} },
    );
    return { onClose };
  };

  it("starts invalid with the default scheme selected", () => {
    render();
    expect(field(/Key Identifier/)).toHaveValue(0);
    expect(screen.getByText("Profile A (X25519)")).toBeInTheDocument();
    expect(button(/^Create$/)).toBeDisabled();
  });

  it("rejects a key identifier above 255", async () => {
    render();

    fireEvent.change(field(/Key Identifier/), { target: { value: "300" } });
    fireEvent.blur(field(/Key Identifier/));

    await screen.findByText("Key Identifier must be between 0 and 255.");
    expect(button(/^Create$/)).toBeDisabled();
  });

  it("rejects a private key that is not 64 hex characters", async () => {
    const user = userEvent.setup();
    render();

    await user.type(field(/Private Key/), "abc");
    await user.tab();

    await screen.findByText(
      "Private Key must be a 64-character hexadecimal string.",
    );
  });

  it("Generate fills a valid 64 character key and enables submit", async () => {
    const user = userEvent.setup();
    render();

    await user.click(screen.getByRole("button", { name: "Generate" }));

    await waitFor(() =>
      expect((field(/Private Key/) as HTMLInputElement).value).toMatch(
        /^[0-9a-f]{64}$/,
      ),
    );
    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
  });

  it("submits the identifier as a number", async () => {
    const user = userEvent.setup();
    api.post(PATH, () => ({}));
    const { onClose } = render();

    fireEvent.change(field(/Key Identifier/), { target: { value: "7" } });
    await user.click(screen.getByRole("button", { name: "Generate" }));
    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    const body = api.lastRequest(PATH)?.body as Record<string, unknown>;
    expect(body.keyIdentifier).toBe(7);
    expect(body.scheme).toBe("A");
  });

  it("keeps the dialog open on a failed create", async () => {
    const user = userEvent.setup();
    api.post(PATH, () => httpError(409, "key identifier already used"));
    const { onClose } = render();

    await user.click(screen.getByRole("button", { name: "Generate" }));
    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await screen.findByText(
      /Failed to create home network key: .*key identifier already used/,
    );
    expect(onClose).not.toHaveBeenCalled();
  });
});
