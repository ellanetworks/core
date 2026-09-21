// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import AddNodeModal from "./AddNodeModal";
import ViewBGPPeerModal from "./ViewBGPPeerModal";
import type { BGPPeer } from "@/queries/bgp";

const api = setupApiServer();

const JOIN_TOKENS = "/api/v1/cluster/pki/join-tokens";

const dialog = () => screen.getByRole("dialog");
const button = (name: RegExp) => within(dialog()).getByRole("button", { name });

const renderAddNode = () => {
  const onClose = vi.fn();
  renderWithProviders(<AddNodeModal open onClose={onClose} />, { auth: {} });
  return { onClose };
};

describe("AddNodeModal", () => {
  it("names its dialog and asks only for a token lifetime", async () => {
    renderAddNode();

    expect(dialog()).toHaveAccessibleName("Add a Node to the Cluster");
    await screen.findByRole("combobox", { name: /Token lifetime/ });
    expect(screen.queryByLabelText(/Node ID/)).not.toBeInTheDocument();
    expect(button(/Mint Token/)).toBeEnabled();
  });

  it("mints without naming a node", async () => {
    const user = userEvent.setup();
    api.post(JOIN_TOKENS, () => ({ token: "t", expiresAt: 4102444800 }));
    renderAddNode();

    await user.click(button(/Mint Token/));

    await waitFor(() => expect(api.lastRequest(JOIN_TOKENS)).toBeTruthy());
    expect(api.lastRequest(JOIN_TOKENS)?.body).not.toHaveProperty("nodeID");
  });

  it("shows the minted token and a config snippet carrying only the token", async () => {
    const user = userEvent.setup();
    api.post(JOIN_TOKENS, () => ({
      token: "join-abc123",
      expiresAt: 4102444800,
    }));
    const { onClose } = renderAddNode();

    await user.click(button(/Mint Token/));

    await screen.findByText(/Token minted/);
    expect(screen.getByText("join-abc123")).toBeInTheDocument();
    expect(screen.getByText(/paste this token/i)).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
    expect(
      within(dialog()).queryByRole("button", { name: /Mint Token/ }),
    ).not.toBeInTheDocument();
    expect(button(/^Close$/)).toBeInTheDocument();
  });

  it("sends the selected token lifetime", async () => {
    const user = userEvent.setup();
    api.post(JOIN_TOKENS, () => ({ token: "t", expiresAt: 4102444800 }));
    renderAddNode();

    await user.click(screen.getByRole("combobox", { name: /Token lifetime/ }));
    await user.click(await screen.findByRole("option", { name: "1 hour" }));
    await user.click(button(/Mint Token/));

    await waitFor(() =>
      expect(api.lastRequest(JOIN_TOKENS)?.body).toMatchObject({
        ttlSeconds: 3600,
      }),
    );
  });

  it("reports a mint failure and stays on the form", async () => {
    const user = userEvent.setup();
    api.post(JOIN_TOKENS, () => httpError(500, "pki unavailable"));
    const { onClose } = renderAddNode();

    await user.click(button(/Mint Token/));

    await screen.findByText(/pki unavailable/);
    expect(onClose).not.toHaveBeenCalled();
    expect(button(/Mint Token/)).toBeEnabled();
  });
});

describe("ViewBGPPeerModal", () => {
  const peer = {
    id: 1,
    address: "10.0.0.1",
    remoteAS: 65001,
    holdTime: 90,
    description: "upstream",
    hasPassword: true,
    state: "established",
    uptime: "3h",
    importPrefixes: [{ prefix: "0.0.0.0/0", maxLength: 32 }],
  } as BGPPeer;

  it("names its dialog and renders peer state read-only", () => {
    renderWithProviders(
      <ViewBGPPeerModal open onClose={vi.fn()} peer={peer} />,
      { auth: {} },
    );

    expect(dialog()).toHaveAccessibleName(/BGP Peer/);
    expect(screen.getByText("Established (3h)")).toBeInTheDocument();
    expect(
      screen.getByText(/Import Prefix List: Accept All/),
    ).toBeInTheDocument();
    expect(within(dialog()).queryAllByRole("textbox")).toHaveLength(0);
  });
});
