// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError, type ApiRequest } from "@/test/apiServer";
import type { OIDCSettings } from "@/queries/oidc";
import { RoleID } from "@/queries/users";
import SingleSignOn from "./SingleSignOn";

const api = setupApiServer();

const configured: OIDCSettings = {
  enabled: false,
  external_url: "https://core.example.com",
  redirect_url: "https://core.example.com/api/v1/auth/oidc/callback",
  post_logout_redirect_url: "https://core.example.com/login",
  issuer: "https://idp.example.com/realms/core",
  client_id: "ella-core",
  client_secret_set: true,
  display_name: "Keycloak",
  scopes: ["openid", "email", "profile"],
  session_lifetime_seconds: 86400,
  trust_unverified_email: false,
  ca_certificate:
    "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----",
  ca_certificates: [
    { subject: "CN=Example CA", not_after: "2030-01-02T00:00:00Z" },
  ],
  groups_claim: "groups",
  group_roles: [
    { group: "admins", role_id: RoleID.Admin },
    { group: "operators", role_id: RoleID.NetworkManager },
  ],
};

const blank: OIDCSettings = {
  ...configured,
  external_url: "",
  redirect_url: "",
  post_logout_redirect_url: "",
  issuer: "",
  client_id: "",
  client_secret_set: false,
  display_name: "",
  scopes: ["openid", "email"],
  ca_certificate: "",
  ca_certificates: [],
  groups_claim: "",
  group_roles: [],
};

const serve = (settings: OIDCSettings) => {
  let current = settings;
  api.get("/api/v1/oidc", () => current);
  api.put("/api/v1/oidc", (req: ApiRequest) => {
    const body = req.body as Record<string, unknown>;
    current = {
      ...current,
      ...(body as Partial<OIDCSettings>),
      client_secret_set: current.client_secret_set || body.client_secret !== "",
    };
    return current;
  });
};

const renderPage = () =>
  renderWithProviders(<SingleSignOn />, {
    initialEntries: ["/single-sign-on"],
    auth: { role: "Admin" },
  });

const dialog = (name: string) => screen.getByRole("dialog", { name });

describe("Single sign-on", () => {
  it("shows the settings and the redirect URLs, never the secret", async () => {
    serve(configured);
    renderPage();

    const provider = await screen.findByRole("table", { name: "Provider" });
    expect(await within(provider).findByText("Keycloak")).toBeInTheDocument();
    const redirects = screen.getByRole("table", { name: "URLs" });
    expect(within(redirects).getByText(configured.redirect_url)).toBeVisible();
    expect(
      within(redirects).getByText(configured.post_logout_redirect_url),
    ).toBeVisible();
    expect(
      within(redirects).getByRole("button", {
        name: "Copy the post-logout redirect URL",
      }),
    ).toBeVisible();
    expect(within(provider).getByText("••••••••")).toBeVisible();
    expect(
      within(provider).getByText("CN=Example CA · expires 2030-01-02"),
    ).toBeVisible();
    expect(within(provider).getByText("1 day")).toBeVisible();
    const groups = screen.getByRole("table", { name: "Roles from groups" });
    expect(within(groups).getByText("admins → Admin")).toBeVisible();
    expect(
      within(groups).getByText("operators → Network Manager"),
    ).toBeVisible();
    expect(
      screen.getByRole("link", { name: /Configure Single Sign-On/ }),
    ).toHaveAttribute(
      "href",
      "https://docs.ellanetworks.com/core/how_to/configure_single_sign_on/",
    );
  });

  it("says what is missing before single sign-on can be enabled", async () => {
    serve(blank);
    renderPage();

    const toggle = await screen.findByRole("switch", { name: "Enabled" });
    expect(toggle).toBeDisabled();
    fireEvent.mouseOver(toggle.closest("label")!);
    expect(
      await screen.findByRole("tooltip", {
        name: "Missing: external URL, issuer URL, client ID, client secret, display name.",
      }),
    ).toBeInTheDocument();
  });

  it("enables single sign-on from the page", async () => {
    const user = userEvent.setup();
    serve(configured);
    renderPage();

    await user.click(await screen.findByRole("switch", { name: "Enabled" }));

    await screen.findByText("Single sign-on enabled successfully.");
    expect(api.lastRequest("/api/v1/oidc")?.body).toMatchObject({
      enabled: true,
      client_secret: "",
      group_roles: configured.group_roles,
    });
    expect(screen.getByRole("switch", { name: "Enabled" })).toBeChecked();
  });

  it("shows why single sign-on could not be enabled", async () => {
    const user = userEvent.setup();
    api.get("/api/v1/oidc", () => configured);
    api.put("/api/v1/oidc", () =>
      httpError(400, "the identity provider is unreachable"),
    );
    renderPage();

    await user.click(await screen.findByRole("switch", { name: "Enabled" }));

    await screen.findByText(
      "Failed to enable single sign-on: the identity provider is unreachable",
    );
  });

  it("disables single sign-on after confirmation", async () => {
    const user = userEvent.setup();
    serve({ ...configured, enabled: true });
    renderPage();

    await user.click(await screen.findByRole("switch", { name: "Enabled" }));
    const confirm = dialog("Disable Single Sign-On");
    expect(confirm).toHaveTextContent(
      "Everyone who logged in with single sign-on is signed out.",
    );
    await user.click(within(confirm).getByRole("button", { name: "Disable" }));

    await screen.findByText("Single sign-on disabled successfully.");
    expect(api.lastRequest("/api/v1/oidc")?.body).toMatchObject({
      enabled: false,
    });
  });

  it("saves the provider, keeping the secret and the groups", async () => {
    const user = userEvent.setup();
    serve(configured);
    renderPage();

    await user.click(
      await screen.findByRole("button", { name: "Edit provider" }),
    );
    const edit = dialog("Edit Provider");
    const name = within(edit).getByRole("textbox", { name: /Display name/ });
    await user.clear(name);
    await user.type(name, "Corporate SSO");
    await user.click(within(edit).getByRole("button", { name: "Update" }));

    await screen.findByText("Single sign-on updated successfully.");
    expect(api.lastRequest("/api/v1/oidc")?.body).toEqual({
      enabled: false,
      external_url: configured.external_url,
      issuer: configured.issuer,
      client_id: configured.client_id,
      client_secret: "",
      display_name: "Corporate SSO",
      scopes: configured.scopes,
      session_lifetime_seconds: 86400,
      trust_unverified_email: false,
      ca_certificate: configured.ca_certificate,
      groups_claim: "groups",
      group_roles: configured.group_roles,
    });
  });

  it("saves the external URL, keeping the provider", async () => {
    const user = userEvent.setup();
    serve(configured);
    renderPage();

    await user.click(
      await screen.findByRole("button", { name: "Edit external URL" }),
    );
    const edit = dialog("Edit External URL");
    const url = within(edit).getByRole("textbox", { name: /External URL/ });
    await user.clear(url);
    await user.type(url, "https://sso.example.com/");
    await user.click(within(edit).getByRole("button", { name: "Update" }));

    await screen.findByText("Single sign-on updated successfully.");
    expect(api.lastRequest("/api/v1/oidc")?.body).toMatchObject({
      external_url: "https://sso.example.com",
      issuer: configured.issuer,
      client_id: configured.client_id,
      client_secret: "",
      display_name: configured.display_name,
    });
  });

  it("needs the client secret again when the issuer or client ID changes", async () => {
    const user = userEvent.setup();
    serve(configured);
    renderPage();

    await user.click(
      await screen.findByRole("button", { name: "Edit provider" }),
    );
    const edit = dialog("Edit Provider");
    const clientID = within(edit).getByRole("textbox", { name: /Client ID/ });
    await user.type(clientID, "-2");

    expect(
      await within(edit).findByText(
        "Enter the client secret again: the issuer URL or client ID changed.",
      ),
    ).toBeVisible();
    expect(within(edit).getByRole("button", { name: "Update" })).toBeDisabled();

    await user.type(within(edit).getByLabelText("Client secret"), "s3cret");
    await waitFor(() =>
      expect(
        within(edit).getByRole("button", { name: "Update" }),
      ).toBeEnabled(),
    );
  });

  it("warns that changing the issuer signs everyone out", async () => {
    const user = userEvent.setup();
    serve({ ...configured, enabled: true });
    renderPage();

    await user.click(
      await screen.findByRole("button", { name: "Edit provider" }),
    );
    const edit = dialog("Edit Provider");
    await user.type(
      within(edit).getByRole("textbox", { name: /Issuer URL/ }),
      "-new",
    );

    expect(
      within(edit).getByText(
        "Saving signs out everyone who logged in with single sign-on.",
      ),
    ).toBeVisible();
  });

  it("refuses settings the API would refuse", async () => {
    const user = userEvent.setup();
    serve(configured);
    renderPage();

    await user.click(
      await screen.findByRole("button", { name: "Edit provider" }),
    );
    const edit = dialog("Edit Provider");
    const issuer = within(edit).getByRole("textbox", { name: /Issuer URL/ });
    await user.clear(issuer);
    await user.type(issuer, "http://idp.example.com");
    await user.tab();
    const scopes = within(edit).getByRole("textbox", { name: /Scopes/ });
    await user.clear(scopes);
    await user.type(scopes, "openid profile");
    const lifetime = within(edit).getByRole("spinbutton", {
      name: /Session lifetime/,
    });
    await user.clear(lifetime);
    await user.type(lifetime, "31");

    expect(await within(edit).findByText("An https URL.")).toBeVisible();
    expect(within(edit).getByText("Include openid and email.")).toBeVisible();
    expect(within(edit).getByText("From 5 minutes to 30 days.")).toBeVisible();
    expect(within(edit).getByRole("button", { name: "Update" })).toBeDisabled();
  });

  it("saves the roles from groups, keeping the provider", async () => {
    const user = userEvent.setup();
    serve(configured);
    renderPage();

    await user.click(
      await screen.findByRole("button", { name: "Edit roles from groups" }),
    );
    const edit = dialog("Edit Roles from Groups");
    expect(edit).toHaveTextContent(
      "A user in several mapped groups gets the highest of their roles.",
    );
    expect(
      within(edit).queryByRole("button", { name: /Move group/ }),
    ).not.toBeInTheDocument();
    await user.click(within(edit).getByRole("button", { name: "Add group" }));
    await user.type(
      within(edit).getByRole("textbox", { name: /Group 3/ }),
      "viewers",
    );
    await user.click(within(edit).getByRole("button", { name: "Update" }));

    await screen.findByText("Single sign-on updated successfully.");
    expect(api.lastRequest("/api/v1/oidc")?.body).toMatchObject({
      issuer: configured.issuer,
      client_secret: "",
      groups_claim: "groups",
      group_roles: [
        { group: "admins", role_id: RoleID.Admin },
        { group: "operators", role_id: RoleID.NetworkManager },
        { group: "viewers", role_id: RoleID.ReadOnly },
      ],
    });
  });

  it("shows each group's error on its field", async () => {
    const user = userEvent.setup();
    serve(configured);
    renderPage();

    await user.click(
      await screen.findByRole("button", { name: "Edit roles from groups" }),
    );
    const edit = dialog("Edit Roles from Groups");
    const second = within(edit).getByRole("textbox", { name: /Group 2/ });
    await user.clear(second);
    await user.type(second, "admins");
    await user.tab();

    expect(
      await within(edit).findByText("Appears more than once."),
    ).toBeVisible();
    expect(within(edit).getByRole("button", { name: "Update" })).toBeDisabled();
  });
});
