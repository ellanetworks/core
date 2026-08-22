// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import CreateBGPPeerModal from "./CreateBGPPeerModal";
import EditBGPPeerModal from "./EditBGPPeerModal";
import EditBGPSettingsModal from "./EditBGPSettingsModal";
import type { BGPPeer } from "@/queries/bgp";

const api = setupApiServer();

const PEERS_PATH = "/api/v1/networking/bgp/peers";

const dialog = () => screen.getByRole("dialog");
const lastPeerBody = (path: string) => {
  const request = api.lastRequest(path);
  if (!request) throw new Error(`no request was made to ${path}`);
  return request.body as Record<string, unknown>;
};
const button = (name: RegExp) => within(dialog()).getByRole("button", { name });
const field = (label: RegExp) => screen.getByLabelText(label);
const setNumber = (label: RegExp, value: string) => {
  fireEvent.change(field(label), { target: { value } });
  fireEvent.blur(field(label));
};

const renderCreate = (rejectedPrefixes = []) => {
  const onClose = vi.fn();
  const onSuccess = vi.fn();
  renderWithProviders(
    <CreateBGPPeerModal
      open
      onClose={onClose}
      onSuccess={onSuccess}
      rejectedPrefixes={rejectedPrefixes}
    />,
    { auth: {} },
  );
  return { onClose, onSuccess };
};

describe("CreateBGPPeerModal", () => {
  it("prefills AS and hold time and starts on Deny All", () => {
    renderCreate();
    expect(field(/Remote AS/)).toHaveValue(64512);
    expect(field(/Hold Time/)).toHaveValue(90);
    expect(
      screen.getByText("All routes from this peer will be rejected."),
    ).toBeInTheDocument();
    expect(dialog()).toHaveAccessibleName("Create BGP Peer");
  });

  it("rejects a neighbor address that is not an IP", async () => {
    const user = userEvent.setup();
    renderCreate();

    await user.type(field(/Neighbor Address/), "not-an-ip");
    await user.tab();

    await screen.findByText("Must be a valid IP address");
    expect(button(/^Create$/)).toBeDisabled();
  });

  it("enforces the hold time range", async () => {
    renderCreate();

    setNumber(/Hold Time/, "1");
    await screen.findByText("Must be at least 3");
    expect(button(/^Create$/)).toBeDisabled();
  });

  it("switching presets rewrites the prefix list", async () => {
    const user = userEvent.setup();
    api.post(PEERS_PATH, () => ({}));
    const { onClose } = renderCreate();

    await user.type(field(/Neighbor Address/), "10.0.0.1");
    await user.click(
      screen.getByRole("button", { name: "Default Route Only" }),
    );
    expect(
      screen.getByText("Only the default route (0.0.0.0/0) will be accepted."),
    ).toBeInTheDocument();

    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(PEERS_PATH)?.body).toMatchObject({
      address: "10.0.0.1",
      importPrefixes: [{ prefix: "0.0.0.0/0", maxLength: 0 }],
    });
  });

  it("blocks submit while a custom prefix is invalid CIDR", async () => {
    const user = userEvent.setup();
    renderCreate();

    await user.type(field(/Neighbor Address/), "10.0.0.1");
    await user.click(screen.getByRole("button", { name: "Custom" }));
    await user.type(field(/^Prefix$/), "not-a-cidr");

    await screen.findByText(/Must be valid CIDR/);
    await waitFor(() => expect(button(/^Create$/)).toBeDisabled());
  });

  it("accepts a valid custom prefix and omits empty optional fields", async () => {
    const user = userEvent.setup();
    api.post(PEERS_PATH, () => ({}));
    renderCreate();

    await user.type(field(/Neighbor Address/), "10.0.0.1");
    await user.click(screen.getByRole("button", { name: "Custom" }));
    await user.type(field(/^Prefix$/), "10.0.0.0/8");

    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await waitFor(() => {
      const body = lastPeerBody(PEERS_PATH);
      expect(body.importPrefixes).toEqual([
        { prefix: "10.0.0.0/8", maxLength: 32 },
      ]);
      expect(body.password).toBeUndefined();
      expect(body.description).toBeUndefined();
    });
  });

  it("hides system rejected prefixes behind a toggle", async () => {
    const user = userEvent.setup();
    renderCreate([{ prefix: "127.0.0.0/8", description: "loopback" }] as never);

    expect(screen.queryByText("loopback")).not.toBeVisible();
    await user.click(screen.getByRole("button", { name: /1 rejected prefix/ }));
    await waitFor(() => expect(screen.getByText("loopback")).toBeVisible());
  });

  it("keeps the dialog open on a failed create", async () => {
    const user = userEvent.setup();
    api.post(PEERS_PATH, () => httpError(409, "peer already exists"));
    const { onClose } = renderCreate();

    await user.type(field(/Neighbor Address/), "10.0.0.1");
    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await screen.findByText(/Failed to create BGP peer: .*peer already exists/);
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe("EditBGPPeerModal", () => {
  const peer = {
    id: 1,
    address: "10.0.0.1",
    remoteAS: 65001,
    holdTime: 90,
    description: "upstream",
    hasPassword: true,
    importPrefixes: [{ prefix: "10.0.0.0/8", maxLength: 24 }],
  } satisfies BGPPeer;

  const renderEdit = (override: Partial<BGPPeer> = {}) => {
    const onClose = vi.fn();
    renderWithProviders(
      <EditBGPPeerModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        peer={{ ...peer, ...override }}
      />,
      { auth: {} },
    );
    return { onClose };
  };

  it("seeds from the peer and detects its custom preset", () => {
    renderEdit();
    expect(field(/Neighbor Address/)).toHaveValue("10.0.0.1");
    expect(field(/Remote AS/)).toHaveValue(65001);
    expect(field(/^Prefix$/)).toHaveValue("10.0.0.0/8");
    expect(field(/Max Length/)).toHaveValue(24);
  });

  it("leaves the password out of the request when untouched", async () => {
    const user = userEvent.setup();
    api.put(`${PEERS_PATH}/:id`, () => ({}));
    const { onClose } = renderEdit();

    await user.click(button(/^Update$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(lastPeerBody(`${PEERS_PATH}/1`).password).toBeUndefined();
  });

  it("sends an empty password when Clear is used", async () => {
    const user = userEvent.setup();
    api.put(`${PEERS_PATH}/:id`, () => ({}));
    renderEdit();

    await user.click(screen.getByRole("button", { name: "Clear" }));
    expect(
      screen.getByText("Password will be removed on save"),
    ).toBeInTheDocument();
    expect(field(/Password/)).toBeDisabled();

    await user.click(button(/^Update$/));
    await waitFor(() =>
      expect(lastPeerBody(`${PEERS_PATH}/1`).password).toBe(""),
    );
  });

  it("Undo restores the password field", async () => {
    const user = userEvent.setup();
    renderEdit();

    await user.click(screen.getByRole("button", { name: "Clear" }));
    await user.click(screen.getByRole("button", { name: "Undo" }));

    expect(field(/Password/)).not.toBeDisabled();
    expect(
      screen.getByText("TCP MD5 authentication password"),
    ).toBeInTheDocument();
  });

  it("offers no Clear button when the peer has no password", () => {
    renderEdit({ hasPassword: false });
    expect(
      screen.queryByRole("button", { name: "Clear" }),
    ).not.toBeInTheDocument();
    expect(field(/Password/)).toHaveAttribute("placeholder", "Optional");
  });

  it("sends a newly typed password", async () => {
    const user = userEvent.setup();
    api.put(`${PEERS_PATH}/:id`, () => ({}));
    renderEdit();

    await user.type(field(/Password/), "s3cret");
    await user.click(button(/^Update$/));

    await waitFor(() =>
      expect(lastPeerBody(`${PEERS_PATH}/1`).password).toBe("s3cret"),
    );
  });
});

describe("EditBGPSettingsModal", () => {
  const SETTINGS_PATH = "/api/v1/networking/bgp";
  const initialData = {
    enabled: true,
    localAS: "65000",
    routerID: "10.0.0.1",
    listenAddress: ":179",
  };

  const renderSettings = () => {
    const onClose = vi.fn();
    renderWithProviders(
      <EditBGPSettingsModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        initialData={initialData}
      />,
      { auth: {} },
    );
    return { onClose };
  };

  it("rejects a malformed listen address", async () => {
    const user = userEvent.setup();
    renderSettings();

    await user.clear(field(/Listen Address/));
    await user.type(field(/Listen Address/), "179");
    await user.tab();

    await screen.findByText(
      "Listen address must be in host:port or :port format",
    );
    expect(button(/^Update$/)).toBeDisabled();
  });

  it("allows an empty router ID", async () => {
    const user = userEvent.setup();
    api.put(SETTINGS_PATH, () => ({}));
    const { onClose } = renderSettings();

    await user.clear(field(/Router ID/));
    await user.tab();
    await waitFor(() => expect(button(/^Update$/)).toBeEnabled());
    await user.click(button(/^Update$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest(SETTINGS_PATH)?.body).toEqual({
      enabled: true,
      localAS: 65000,
      routerID: "",
      listenAddress: ":179",
    });
  });

  it("sends localAS as a number and preserves the enabled flag", async () => {
    const user = userEvent.setup();
    api.put(SETTINGS_PATH, () => ({}));
    renderSettings();

    await user.clear(field(/Local AS/));
    await user.type(field(/Local AS/), "65010");
    await waitFor(() => expect(button(/^Update$/)).toBeEnabled());
    await user.click(button(/^Update$/));

    await waitFor(() =>
      expect(api.lastRequest(SETTINGS_PATH)?.body).toMatchObject({
        localAS: 65010,
        enabled: true,
      }),
    );
  });
});
