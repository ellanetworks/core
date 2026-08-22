// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError, rawBody } from "@/test/apiServer";
import CreateAPITokenModal from "./CreateAPITokenModal";
import SupportModal from "./SupportModal";

const api = setupApiServer();

const MY_TOKENS = "/api/v1/users/me/api-tokens";
const USER_TOKENS = "/api/v1/users/:email/api-tokens";

const dialog = () => screen.getByRole("dialog");
const button = (name: RegExp) => within(dialog()).getByRole("button", { name });
const field = (label: RegExp) => screen.getByLabelText(label);

describe("CreateAPITokenModal", () => {
  const render = (targetEmail?: string) => {
    const onClose = vi.fn();
    const onSuccess = vi.fn();
    renderWithProviders(
      <CreateAPITokenModal
        open
        onClose={onClose}
        onSuccess={onSuccess}
        targetEmail={targetEmail}
      />,
      { auth: {} },
    );
    return { onClose, onSuccess };
  };

  it("requires an expiry date until No expiry is ticked", async () => {
    const user = userEvent.setup();
    render();

    await user.type(field(/Name/), "ci-pipeline");
    expect(button(/^Create$/)).toBeDisabled();

    await user.click(screen.getByRole("checkbox", { name: "No expiry" }));
    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    expect(screen.getByRole("group", { name: /Expiry date/ })).toHaveClass(
      "Mui-disabled",
    );
  });

  it("rejects a name shorter than three characters", async () => {
    const user = userEvent.setup();
    render();

    await user.type(field(/Name/), "ab");
    await user.tab();

    await screen.findByText("Name must be at least 3 characters");
    expect(button(/^Create$/)).toBeDisabled();
  });

  it("sends an empty expiry and hands the token back", async () => {
    const user = userEvent.setup();
    api.post(MY_TOKENS, () => ({ token: "ella_secret" }));
    const { onClose, onSuccess } = render();

    await user.type(field(/Name/), "ci-pipeline");
    await user.click(screen.getByRole("checkbox", { name: "No expiry" }));
    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await waitFor(() => expect(onSuccess).toHaveBeenCalledWith("ella_secret"));
    expect(onClose).toHaveBeenCalled();
    expect(api.lastRequest(MY_TOKENS)?.body).toMatchObject({
      name: "ci-pipeline",
      expires_at: "",
    });
  });

  it("targets another user's tokens when given an email", async () => {
    const user = userEvent.setup();
    api.post(USER_TOKENS, () => ({ token: "other_secret" }));
    render("ops@ellanetworks.com");

    await user.type(field(/Name/), "their-token");
    await user.click(screen.getByRole("checkbox", { name: "No expiry" }));
    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await waitFor(() =>
      expect(
        api.lastRequest("/api/v1/users/ops@ellanetworks.com/api-tokens"),
      ).toBeDefined(),
    );
  });

  it("keeps the dialog open on a failed create", async () => {
    const user = userEvent.setup();
    api.post(MY_TOKENS, () => httpError(409, "token name taken"));
    const { onClose, onSuccess } = render();

    await user.type(field(/Name/), "ci-pipeline");
    await user.click(screen.getByRole("checkbox", { name: "No expiry" }));
    await waitFor(() => expect(button(/^Create$/)).toBeEnabled());
    await user.click(button(/^Create$/));

    await screen.findByText(/Failed to create API token: .*token name taken/);
    expect(onClose).not.toHaveBeenCalled();
    expect(onSuccess).not.toHaveBeenCalled();
  });
});

describe("SupportModal", () => {
  const PATH = "/api/v1/support-bundle";

  it("resolves its description and offers a Generate action", () => {
    renderWithProviders(<SupportModal open onClose={vi.fn()} />, { auth: {} });

    expect(dialog()).toHaveAccessibleName("Generate Support Bundle");
    const describedBy = dialog().getAttribute("aria-describedby");
    expect(document.getElementById(describedBy!)).toHaveTextContent(
      /system diagnostics/,
    );
    expect(button(/^Generate$/)).toBeInTheDocument();
  });

  it("downloads the bundle and closes on success", async () => {
    const user = userEvent.setup();
    api.post(PATH, () =>
      rawBody("bundle", { "Content-Type": "application/gzip" }),
    );
    const onClose = vi.fn();
    const click = vi.fn();

    window.URL.createObjectURL = vi.fn(() => "blob:bundle");
    window.URL.revokeObjectURL = vi.fn();
    const realCreate = document.createElement.bind(document);
    vi.spyOn(document, "createElement").mockImplementation((tag: string) => {
      const el = realCreate(tag);
      if (tag === "a") el.click = click;
      return el;
    });

    renderWithProviders(<SupportModal open onClose={onClose} />, { auth: {} });
    await user.click(button(/^Generate$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(click).toHaveBeenCalled();
    expect(window.URL.createObjectURL).toHaveBeenCalled();
    expect(window.URL.revokeObjectURL).toHaveBeenCalledWith("blob:bundle");
    vi.restoreAllMocks();
  });

  it("stays open and reports a failure through the snackbar", async () => {
    const user = userEvent.setup();
    api.post(PATH, () => httpError(500, "bundle failed"));
    const onClose = vi.fn();

    renderWithProviders(<SupportModal open onClose={onClose} />, { auth: {} });
    await user.click(button(/^Generate$/));

    await screen.findByText(/Failed to generate support bundle/);
    expect(onClose).not.toHaveBeenCalled();
  });
});
