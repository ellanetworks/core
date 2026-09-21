// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import DeleteConfirmationModal from "./DeleteConfirmationModal";
import DrainNodeModal from "./DrainNodeModal";
import ResumeNodeModal from "./ResumeNodeModal";
import RemoveNodeModal from "./RemoveNodeModal";

const api = setupApiServer();

const NODE_UUID = "0199c4f1-2ab3-7c1d-9f2a-6fe8422da1ec";

const dialog = () => screen.getByRole("dialog");
const button = (name: RegExp) => within(dialog()).getByRole("button", { name });

describe("DeleteConfirmationModal", () => {
  it("keeps its existing labels and resolves its description", () => {
    renderWithProviders(
      <DeleteConfirmationModal
        open
        onClose={vi.fn()}
        onConfirm={vi.fn()}
        title="Confirm Deletion"
        description="Are you sure?"
      />,
      { auth: {} },
    );

    expect(dialog()).toHaveAccessibleName("Confirm Deletion");
    const describedBy = dialog().getAttribute("aria-describedby");
    expect(document.getElementById(describedBy!)).toHaveTextContent(
      "Are you sure?",
    );
    expect(button(/^Confirm$/)).toBeInTheDocument();
  });

  it("disables both buttons while confirming", async () => {
    const user = userEvent.setup();
    let release: () => void = () => {};
    const onConfirm = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          release = resolve;
        }),
    );

    renderWithProviders(
      <DeleteConfirmationModal
        open
        onClose={vi.fn()}
        onConfirm={onConfirm}
        title="Confirm Deletion"
        description="Are you sure?"
      />,
      { auth: {} },
    );

    await user.click(button(/^Confirm$/));
    await screen.findByText("Deleting…");
    expect(button(/^Cancel$/)).toBeDisabled();

    release();
    await waitFor(() => expect(button(/^Confirm$/)).toBeEnabled());
  });
});

describe("DrainNodeModal", () => {
  const PATH = "/api/v1/cluster/members/:id/drain";

  const render = () => {
    const onClose = vi.fn();
    const onSuccess = vi.fn();
    renderWithProviders(
      <DrainNodeModal
        open
        nodeId={2}
        nodeLabel="2"
        onClose={onClose}
        onSuccess={onSuccess}
      />,
      { auth: {} },
    );
    return { onClose, onSuccess };
  };

  it("names the node and says what drain does", () => {
    render();

    expect(dialog()).toHaveAccessibleName("Drain node 2?");
    expect(
      screen.getByText(/moves its subscribers to the rest of the cluster/),
    ).toBeVisible();
  });

  it("passes the drain result to onSuccess and closes", async () => {
    const user = userEvent.setup();
    api.post(PATH, () => ({ drainState: "draining" }));
    const { onClose, onSuccess } = render();

    await user.click(button(/^Drain$/));

    await waitFor(() => expect(onSuccess).toHaveBeenCalled());
    expect(onSuccess.mock.calls[0][0]).toMatchObject({
      drainState: "draining",
    });
    expect(onClose).toHaveBeenCalled();
  });

  it("shows the failure inline and stays open", async () => {
    const user = userEvent.setup();
    api.post(PATH, () => httpError(409, "node is already drained"));
    const { onClose } = render();

    await user.click(button(/^Drain$/));

    await screen.findByText(/node is already drained/);
    expect(onClose).not.toHaveBeenCalled();
    expect(button(/^Drain$/)).toBeEnabled();
  });

  it("shows the full node identity alongside the display name", () => {
    renderWithProviders(
      <DrainNodeModal
        open
        nodeId={NODE_UUID}
        nodeLabel="core-1"
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />,
      { auth: {} },
    );

    expect(dialog()).toHaveAccessibleName("Drain node core-1?");
    expect(within(dialog()).getByText(NODE_UUID)).toBeVisible();
  });
});

describe("ResumeNodeModal", () => {
  const PATH = "/api/v1/cluster/members/:id/resume";

  it("names the node and confirms", async () => {
    const user = userEvent.setup();
    api.post(PATH, () => ({}));
    const onClose = vi.fn();
    const onSuccess = vi.fn();

    renderWithProviders(
      <ResumeNodeModal
        open
        nodeId={3}
        nodeLabel="3"
        onClose={onClose}
        onSuccess={onSuccess}
      />,
      { auth: {} },
    );

    expect(dialog()).toHaveAccessibleName("Resume node 3?");
    expect(screen.getByText(/Clears drain state/)).toBeVisible();

    await user.click(button(/^Resume$/));
    await waitFor(() => expect(onSuccess).toHaveBeenCalled());
    expect(onClose).toHaveBeenCalled();
  });

  it("shows the full node identity alongside the display name", () => {
    renderWithProviders(
      <ResumeNodeModal
        open
        nodeId={NODE_UUID}
        nodeLabel="core-1"
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />,
      { auth: {} },
    );

    expect(dialog()).toHaveAccessibleName("Resume node core-1?");
    expect(within(dialog()).getByText(NODE_UUID)).toBeVisible();
  });
});

describe("RemoveNodeModal", () => {
  it("shows the full node identity alongside the display name", () => {
    renderWithProviders(
      <RemoveNodeModal
        open
        nodeId={NODE_UUID}
        nodeLabel="core-1"
        drainState="drained"
        health="healthy"
        isSelf={false}
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />,
      { auth: {} },
    );

    expect(dialog()).toHaveAccessibleName("Remove node core-1?");
    expect(within(dialog()).getByText(NODE_UUID)).toBeVisible();
  });
});
