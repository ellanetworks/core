// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router-dom";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import type { OIDCSettings } from "@/queries/oidc";
import { RoleID, type APIUser } from "@/queries/users";
import CreateUserModal from "@/components/CreateUserModal";
import EditUserModal from "@/components/EditUserModal";
import Users from "./Users";
import UserDetail from "./UserDetail";
import Profile from "./Profile";

const api = setupApiServer();

const oidc = (overrides: Partial<OIDCSettings> = {}): OIDCSettings => ({
  enabled: true,
  external_url: "https://core.example.com",
  redirect_url: "https://core.example.com/api/v1/auth/oidc/callback",
  issuer: "https://idp.example.com/realms/core",
  client_id: "ella-core",
  client_secret_set: true,
  display_name: "Keycloak",
  scopes: ["openid", "email"],
  session_lifetime_seconds: 86400,
  trust_unverified_email: false,
  ca_certificate: "",
  groups_claim: "groups",
  group_roles: [{ group: "operators", role_id: RoleID.NetworkManager }],
  ...overrides,
});

const user = (overrides: Partial<APIUser> = {}): APIUser => ({
  email: "ops@ellanetworks.com",
  role_id: RoleID.NetworkManager,
  role_pinned: false,
  password_set: true,
  oidc_linked: false,
  ...overrides,
});

const serveDetail = (target: APIUser, settings = oidc()) => {
  api.get("/api/v1/oidc", () => settings);
  api.get("/api/v1/users/:email", () => target);
  api.get("/api/v1/users/:email/api-tokens", () => ({
    items: [],
    page: 1,
    per_page: 12,
    total_count: 0,
  }));
  api.get("/api/v1/logs/audit", () => ({
    items: [],
    page: 1,
    per_page: 10,
    total_count: 0,
  }));
};

const renderDetail = (email = "ops@ellanetworks.com") =>
  renderWithProviders(
    <Routes>
      <Route path="/users/:email" element={<UserDetail />} />
    </Routes>,
    { initialEntries: [`/users/${email}`], auth: { role: "Admin" } },
  );

const dialog = () => screen.getByRole("dialog");

describe("Users and single sign-on", () => {
  it("lists how each user signs in, and pinned roles", async () => {
    api.get("/api/v1/oidc", () => oidc());
    api.get("/api/v1/users", () => ({
      items: [
        user({ email: "a@example.com", oidc_linked: true }),
        user({
          email: "b@example.com",
          role_pinned: true,
          password_set: false,
          oidc_linked: true,
        }),
        user({ email: "c@example.com", password_set: false }),
      ],
      page: 1,
      per_page: 25,
      total_count: 3,
    }));
    renderWithProviders(<Users />, {
      initialEntries: ["/users"],
      auth: { role: "Admin" },
    });

    const a = (await screen.findByText("a@example.com")).closest(
      '[role="row"]',
    ) as HTMLElement;
    await waitFor(() =>
      expect(a).toHaveTextContent("Network Manager (from groups)"),
    );
    expect(a).toHaveTextContent("Password, Single sign-on");
    const b = screen.getByText("b@example.com").closest('[role="row"]');
    expect(b).toHaveTextContent("Network Manager (pinned)");
    expect(b).toHaveTextContent("Single sign-on");
    const c = screen.getByText("c@example.com").closest('[role="row"]');
    expect(c).toHaveTextContent("Single sign-on, not linked yet");
  });

  it("shows on the account that a role comes from groups", async () => {
    serveDetail(user({ oidc_linked: true }));
    renderDetail();

    expect(await screen.findByText("From groups")).toBeVisible();
    expect(screen.getByText("Single sign-on")).toBeVisible();
    expect(screen.getByRole("button", { name: "Unlink" })).toBeVisible();
  });

  it("removes a user's password", async () => {
    const u = userEvent.setup();
    serveDetail(user({ oidc_linked: true }));
    api.delete("/api/v1/users/:email/password", () => undefined);
    renderDetail();

    await u.click(
      await screen.findByRole("button", { name: "Remove Password" }),
    );
    expect(dialog()).toHaveTextContent(
      "Remove the password of ops@ellanetworks.com? They can then sign in only with single sign-on. Their sessions end.",
    );
    await u.click(within(dialog()).getByRole("button", { name: "Remove" }));

    await screen.findByText("Password removed successfully.");
    expect(
      api.lastRequest("/api/v1/users/ops@ellanetworks.com/password")?.method,
    ).toBe("DELETE");
  });

  it("says why a password could not be removed", async () => {
    const u = userEvent.setup();
    serveDetail(user({ role_id: RoleID.Admin }));
    api.delete("/api/v1/users/:email/password", () =>
      httpError(409, "At least one admin must keep a password"),
    );
    renderDetail();

    await u.click(
      await screen.findByRole("button", { name: "Remove Password" }),
    );
    await u.click(within(dialog()).getByRole("button", { name: "Remove" }));

    await screen.findByText(
      "Failed to remove the password: At least one admin must keep a password",
    );
  });

  it("unlinks a user from the identity provider", async () => {
    const u = userEvent.setup();
    serveDetail(user({ oidc_linked: true }));
    api.delete("/api/v1/users/:email/oidc", () => undefined);
    renderDetail();

    await u.click(await screen.findByRole("button", { name: "Unlink" }));
    expect(dialog()).toHaveAccessibleName("Unlink from Identity Provider");
    await u.click(within(dialog()).getByRole("button", { name: "Unlink" }));

    await screen.findByText("User unlinked successfully.");
    expect(
      api.lastRequest("/api/v1/users/ops@ellanetworks.com/oidc")?.method,
    ).toBe("DELETE");
  });

  it("offers to set a password to a user without one", async () => {
    serveDetail(user({ password_set: false, oidc_linked: true }));
    renderDetail();

    expect(
      await screen.findByRole("button", { name: "Set Password" }),
    ).toBeVisible();
    expect(
      screen.getByText(
        "This user has no password and signs in only with single sign-on.",
      ),
    ).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "Remove Password" }),
    ).not.toBeInTheDocument();
  });

  it("does not offer a password change on the profile of a user without a password", async () => {
    api.get("/api/v1/users/me", () =>
      user({ password_set: false, oidc_linked: true }),
    );
    api.get("/api/v1/users/me/api-tokens", () => ({
      items: [],
      page: 1,
      per_page: 100,
      total_count: 0,
    }));
    renderWithProviders(<Profile />, {
      initialEntries: ["/profile"],
      auth: { role: "Admin" },
    });

    expect(
      await screen.findByText(
        "You sign in with single sign-on, and have no password.",
      ),
    ).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "Change Password" }),
    ).not.toBeInTheDocument();
  });

  it("creates a user without a password", async () => {
    const u = userEvent.setup();
    api.get("/api/v1/oidc", () => oidc());
    api.post("/api/v1/users", () => ({}));
    const onClose = vi.fn();
    renderWithProviders(
      <CreateUserModal open onClose={onClose} onSuccess={vi.fn()} />,
      { auth: {} },
    );

    await u.type(
      screen.getByRole("textbox", { name: /Email/ }),
      "new@example.com",
    );
    await u.click(await screen.findByRole("switch", { name: "Password" }));
    await u.click(screen.getByRole("checkbox", { name: "Pin role" }));
    await waitFor(() =>
      expect(
        within(dialog()).getByRole("button", { name: "Create" }),
      ).toBeEnabled(),
    );
    await u.click(within(dialog()).getByRole("button", { name: "Create" }));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest("/api/v1/users")?.body).toEqual({
      email: "new@example.com",
      password: "",
      role_id: RoleID.Admin,
      role_pinned: true,
    });
  });

  it("needs a password when single sign-on is not configured", async () => {
    api.get("/api/v1/oidc", () =>
      oidc({ enabled: false, issuer: "", client_id: "", group_roles: [] }),
    );
    renderWithProviders(
      <CreateUserModal open onClose={vi.fn()} onSuccess={vi.fn()} />,
      { auth: {} },
    );

    await waitFor(() => expect(api.requests("/api/v1/oidc")).toHaveLength(1));
    expect(
      screen.queryByRole("switch", { name: "Password" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("checkbox", { name: "Pin role" }),
    ).not.toBeInTheDocument();
    expect(screen.getByLabelText(/Password/)).toBeRequired();
  });

  it("pins a role from groups when it is changed", async () => {
    const u = userEvent.setup();
    api.get("/api/v1/oidc", () => oidc());
    api.put("/api/v1/users/:email", () => ({}));
    const onClose = vi.fn();
    renderWithProviders(
      <EditUserModal
        open
        onClose={onClose}
        onSuccess={vi.fn()}
        initialData={user({ oidc_linked: true })}
      />,
      { auth: {} },
    );

    const pin = await screen.findByRole("checkbox", { name: "Pin role" });
    expect(pin).not.toBeChecked();
    await u.click(screen.getByRole("combobox", { name: /Role/ }));
    await u.click(await screen.findByRole("option", { name: "Read Only" }));
    expect(pin).toBeChecked();
    expect(pin).toBeDisabled();
    await u.click(within(dialog()).getByRole("button", { name: "Update" }));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(api.lastRequest("/api/v1/users/ops@ellanetworks.com")?.body).toEqual(
      { role_id: RoleID.ReadOnly, role_pinned: true },
    );
  });

  it("does not offer pinning without group mapping", async () => {
    api.get("/api/v1/oidc", () => oidc({ group_roles: [] }));
    renderWithProviders(
      <EditUserModal
        open
        onClose={vi.fn()}
        onSuccess={vi.fn()}
        initialData={user({ oidc_linked: true })}
      />,
      { auth: {} },
    );

    await waitFor(() => expect(api.requests("/api/v1/oidc")).toHaveLength(1));
    expect(
      screen.queryByRole("checkbox", { name: "Pin role" }),
    ).not.toBeInTheDocument();
  });
});
