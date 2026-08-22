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

const api = setupApiServer();

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

  const render = (isLeader = false) => {
    const onClose = vi.fn();
    const onSuccess = vi.fn();
    renderWithProviders(
      <DrainNodeModal
        open
        nodeId={2}
        isLeader={isLeader}
        onClose={onClose}
        onSuccess={onSuccess}
      />,
      { auth: {} },
    );
    return { onClose, onSuccess };
  };

  it("names the node and hides the explanation until asked", async () => {
    const user = userEvent.setup();
    render();

    expect(dialog()).toHaveAccessibleName("Drain node 2?");
    expect(screen.queryByText(/AMF Status Indication/)).not.toBeVisible();

    await user.click(screen.getByRole("button", { name: /What this does/ }));
    await waitFor(() =>
      expect(screen.getByText(/AMF Status Indication/)).toBeVisible(),
    );
  });

  it("mentions leadership transfer only for a leader", async () => {
    const user = userEvent.setup();
    render(true);

    await user.click(screen.getByRole("button", { name: /What this does/ }));
    await screen.findByText(/Transfers Raft leadership/);
  });

  it("passes the drain result to onSuccess and closes", async () => {
    const user = userEvent.setup();
    api.post(PATH, () => ({ drainState: "drained" }));
    const { onClose, onSuccess } = render();

    await user.click(button(/^Drain$/));

    await waitFor(() => expect(onSuccess).toHaveBeenCalled());
    expect(onSuccess.mock.calls[0][0]).toMatchObject({
      drainState: "drained",
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
});

describe("ResumeNodeModal", () => {
  const PATH = "/api/v1/cluster/members/:id/resume";

  it("lists what resume does not reverse and confirms", async () => {
    const user = userEvent.setup();
    api.post(PATH, () => ({}));
    const onClose = vi.fn();
    const onSuccess = vi.fn();

    renderWithProviders(
      <ResumeNodeModal
        open
        nodeId={3}
        onClose={onClose}
        onSuccess={onSuccess}
      />,
      { auth: {} },
    );

    expect(dialog()).toHaveAccessibleName("Resume node 3?");
    await user.click(
      screen.getByRole("button", { name: /What Resume does not reverse/ }),
    );
    await waitFor(() =>
      expect(screen.getByText(/stays a follower/)).toBeVisible(),
    );

    await user.click(button(/^Resume$/));
    await waitFor(() => expect(onSuccess).toHaveBeenCalled());
    expect(onClose).toHaveBeenCalled();
  });
});
