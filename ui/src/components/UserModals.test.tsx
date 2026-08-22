// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import {
  screen,
  waitFor,
  waitForElementToBeRemoved,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import { allowConsole } from "../../vitest.setup";
import CreateUserModal from "./CreateUserModal";
import EditUserModal from "./EditUserModal";
import EditUserPasswordModal from "./EditUserPasswordModal";
import EditMyUserPasswordModal from "./EditMyUserPasswordModal";
import { RoleID } from "@/queries/users";

const api = setupApiServer();

const dialog = () => screen.getByRole("dialog");
const submitButton = (name: RegExp) =>
  within(dialog()).getByRole("button", { name });

describe("CreateUserModal", () => {
  const renderModal = (props = {}) => {
    const onClose = vi.fn();
    const onSuccess = vi.fn();
    const result = renderWithProviders(
      <CreateUserModal
        open
        onClose={onClose}
        onSuccess={onSuccess}
        {...props}
      />,
      { auth: {} },
    );
    return { ...result, onClose, onSuccess };
  };

  it("keeps submit disabled until the form is valid", async () => {
    const user = userEvent.setup();
    renderModal();

    expect(submitButton(/^Create$/)).toBeDisabled();

    await user.type(screen.getByLabelText(/Email/), "new@ellanetworks.com");
    await user.type(screen.getByLabelText(/Password/), "hunter2");

    await waitFor(() => expect(submitButton(/^Create$/)).toBeEnabled());
  });

  it("shows a field error only after the field is touched", async () => {
    const user = userEvent.setup();
    renderModal();

    const email = screen.getByLabelText(/Email/);
    await user.type(email, "not-an-email");
    expect(
      screen.queryByText(/must be a valid email/i),
    ).not.toBeInTheDocument();

    await user.tab();
    await screen.findByText(/must be a valid email/i);
  });

  it("posts the entered values and closes on success", async () => {
    const user = userEvent.setup();
    api.post("/api/v1/users", () => ({}));
    const { onClose, onSuccess } = renderModal();

    await user.type(screen.getByLabelText(/Email/), "new@ellanetworks.com");
    await user.type(screen.getByLabelText(/Password/), "hunter2");
    await user.click(await screen.findByRole("combobox", { name: /Role/ }));
    await user.click(await screen.findByRole("option", { name: "Read Only" }));
    await waitFor(() => expect(submitButton(/^Create$/)).toBeEnabled());
    await user.click(submitButton(/^Create$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(onSuccess).toHaveBeenCalled();
    expect(api.lastRequest("/api/v1/users")?.body).toEqual({
      email: "new@ellanetworks.com",
      password: "hunter2",
      role_id: RoleID.ReadOnly,
    });
  });

  it("surfaces a submit failure and stays open", async () => {
    const user = userEvent.setup();
    api.post("/api/v1/users", () => httpError(409, "user already exists"));
    const { onClose } = renderModal();

    await user.type(screen.getByLabelText(/Email/), "new@ellanetworks.com");
    await user.type(screen.getByLabelText(/Password/), "hunter2");
    await waitFor(() => expect(submitButton(/^Create$/)).toBeEnabled());
    await user.click(submitButton(/^Create$/));

    await screen.findByText(/Failed to create user: .*user already exists/);
    expect(onClose).not.toHaveBeenCalled();
  });

  it("clears entered values when reopened (MUI dialog transitions log act warnings)", async () => {
    const user = userEvent.setup();
    const { rerender } = renderModal();

    await user.type(screen.getByLabelText(/Password/), "hunter2");

    rerender(
      <CreateUserModal open={false} onClose={vi.fn()} onSuccess={vi.fn()} />,
    );
    await waitForElementToBeRemoved(() => screen.queryByRole("dialog"));

    rerender(<CreateUserModal open onClose={vi.fn()} onSuccess={vi.fn()} />);
    await screen.findByRole("dialog");

    await waitFor(() =>
      expect(screen.getByLabelText(/Password/)).toHaveValue(""),
    );
    allowConsole();
  });

  it("labels the dialog and leaves no dangling description reference", () => {
    renderModal();
    expect(dialog()).toHaveAccessibleName("Create User");
    expect(dialog()).not.toHaveAttribute("aria-describedby");
  });
});

describe("EditUserModal", () => {
  const initialData = {
    email: "ops@ellanetworks.com",
    role_id: RoleID.ReadOnly,
  };

  it("seeds from initialData and submits the changed role", async () => {
    const user = userEvent.setup();
    api.put("/api/v1/users/:email", () => ({}));
    const onClose = vi.fn();
    const onSuccess = vi.fn();

    renderWithProviders(
      <EditUserModal
        open
        onClose={onClose}
        onSuccess={onSuccess}
        initialData={initialData}
      />,
      { auth: {} },
    );

    expect(screen.getByLabelText(/Email/)).toHaveValue("ops@ellanetworks.com");
    expect(screen.getByLabelText(/Email/)).toBeDisabled();

    await user.click(screen.getByRole("combobox", { name: /Role/ }));
    await user.click(await screen.findByRole("option", { name: "Admin" }));
    await user.click(submitButton(/^Update$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(onSuccess).toHaveBeenCalled();
    const request = api.lastRequest("/api/v1/users/ops@ellanetworks.com");
    expect(request?.body).toEqual({ role_id: RoleID.Admin });
  });
});

describe("EditUserPasswordModal", () => {
  it("strips the status prefix from the backend error", async () => {
    const user = userEvent.setup();
    api.put("/api/v1/users/:email/password", () =>
      httpError(400, "password is too short"),
    );

    renderWithProviders(
      <EditUserPasswordModal
        open
        onClose={vi.fn()}
        onSuccess={vi.fn()}
        initialData={{ email: "ops@ellanetworks.com" }}
      />,
      { auth: {} },
    );

    await user.type(screen.getByLabelText(/New Password/), "x");
    await waitFor(() => expect(submitButton(/^Update$/)).toBeEnabled());
    await user.click(submitButton(/^Update$/));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(
      "Failed to update password: password is too short",
    );
    expect(alert.textContent).not.toMatch(
      /^\s*Failed to update password: \d{3}:/,
    );
  });
});

describe("EditMyUserPasswordModal", () => {
  it("submits both passwords", async () => {
    const user = userEvent.setup();
    api.put("/api/v1/users/me/password", () => ({}));
    const onClose = vi.fn();

    renderWithProviders(
      <EditMyUserPasswordModal open onClose={onClose} onSuccess={vi.fn()} />,
      { auth: {} },
    );

    expect(submitButton(/^Update$/)).toBeDisabled();
    await user.type(screen.getByLabelText(/Current Password/), "old");
    await user.type(screen.getByLabelText(/New Password/), "new");
    await waitFor(() => expect(submitButton(/^Update$/)).toBeEnabled());
    await user.click(submitButton(/^Update$/));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest("/api/v1/users/me/password")?.body).toEqual({
      current_password: "old",
      password: "new",
    });
  });
});
