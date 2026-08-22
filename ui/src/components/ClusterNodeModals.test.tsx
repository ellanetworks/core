// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import AddNodeModal from "./AddNodeModal";
import ViewBGPPeerModal from "./ViewBGPPeerModal";
import type { BGPPeer } from "@/queries/bgp";

const api = setupApiServer();

const MEMBERS = "/api/v1/cluster/members";
const JOIN_TOKENS = "/api/v1/cluster/pki/join-tokens";

const dialog = () => screen.getByRole("dialog");
const button = (name: RegExp) => within(dialog()).getByRole("button", { name });

const seedMembers = (ids: number[]) =>
  api.get(MEMBERS, () =>
    ids.map((nodeId) => ({ nodeId, address: "10.0.0.1" })),
  );

const renderAddNode = () => {
  const onClose = vi.fn();
  renderWithProviders(<AddNodeModal open onClose={onClose} />, { auth: {} });
  return { onClose };
};

describe("AddNodeModal", () => {
  it("names its dialog and suggests the lowest free node id", async () => {
    seedMembers([1, 2]);
    renderAddNode();

    expect(dialog()).toHaveAccessibleName("Add a Node to the Cluster");
    await waitFor(() =>
      expect(screen.getByLabelText(/Node ID/)).toHaveValue(3),
    );
  });

  it("blocks a node id that is already taken", async () => {
    seedMembers([1, 2]);
    renderAddNode();
    await waitFor(() =>
      expect(screen.getByLabelText(/Node ID/)).toHaveValue(3),
    );

    fireEvent.change(screen.getByLabelText(/Node ID/), {
      target: { value: "2" },
    });

    await screen.findByText("This ID is already in use by another node.");
    expect(button(/Mint Token/)).toBeDisabled();
  });

  it("blocks a node id outside the allowed range", async () => {
    seedMembers([]);
    renderAddNode();

    fireEvent.change(screen.getByLabelText(/Node ID/), {
      target: { value: "64" },
    });

    await screen.findByText("Must be between 1 and 63.");
    expect(button(/Mint Token/)).toBeDisabled();
  });

  it("shows the minted token and config snippet, and keeps the dialog open", async () => {
    const user = userEvent.setup();
    seedMembers([1]);
    api.post(JOIN_TOKENS, () => ({
      token: "join-abc123",
      expiresAt: 4102444800,
    }));
    const { onClose } = renderAddNode();

    await waitFor(() =>
      expect(screen.getByLabelText(/Node ID/)).toHaveValue(2),
    );
    await user.click(button(/Mint Token/));

    await screen.findByText(/Token minted/);
    expect(screen.getByText(/join-token: join-abc123/)).toBeInTheDocument();
    expect(screen.getByText(/node-id: 2/)).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
    expect(
      within(dialog()).queryByRole("button", { name: /Mint Token/ }),
    ).not.toBeInTheDocument();
    expect(button(/^Close$/)).toBeInTheDocument();
  });

  it("sends the selected token lifetime", async () => {
    const user = userEvent.setup();
    seedMembers([]);
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
    seedMembers([]);
    api.post(JOIN_TOKENS, () => httpError(500, "pki unavailable"));
    const { onClose } = renderAddNode();

    await user.click(button(/Mint Token/));

    await screen.findByText(/pki unavailable/);
    expect(onClose).not.toHaveBeenCalled();
    expect(button(/Mint Token/)).toBeEnabled();
  });

  it("surfaces a members load failure without blocking minting", async () => {
    api.get(MEMBERS, () => httpError(500, "members unavailable"));
    renderAddNode();

    await screen.findByText(/cluster members/i);
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
