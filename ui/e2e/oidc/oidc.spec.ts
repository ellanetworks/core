// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import {
  expect,
  test,
  type APIRequestContext,
  type BrowserContext,
  type Page,
} from "@playwright/test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import {
  ADMIN_EMAIL,
  ADMIN_PASSWORD,
  RoleID,
  ensureInitialized,
  json,
  send,
} from "../api";

const CORE_PORT = process.env.ELLA_E2E_OIDC_PORT ?? "15444";
const KEYCLOAK_PORT = process.env.KEYCLOAK_E2E_PORT ?? "18444";
const OIDC_DIR =
  process.env.ELLA_E2E_OIDC_DIR ??
  fileURLToPath(new URL("../.build/oidc", import.meta.url));
const EXTERNAL_URL = `https://localhost:${CORE_PORT}`;
const ISSUER = `https://keycloak:${KEYCLOAK_PORT}/realms/ella`;
const CLIENT_ID = "ella-core";
const CLIENT_SECRET = "e2e-client-secret";
const CA_CERTIFICATE = readFileSync(join(OIDC_DIR, "ca.pem"), "utf8");
const SESSION_COOKIE = "session_token";
const LIFETIME_SECONDS = 300;
const OPERATORS = "core-operators";
const VIEWERS = "core-viewers";
const LOGIN_LIMIT = 20;
const LOGIN_WINDOW_MS = 61_000;

const ALICE = { username: "alice", email: "alice@example.com" };
const ALICE_PASSWORD = "AliceE2ePassw0rd!";
const BOB = { username: "bob", email: "bob@example.com" };
const CAROL = { username: "carol", email: "carol@example.com" };

type User = {
  email: string;
  role_id: number;
  password_set: boolean;
  oidc_linked: boolean;
};
type GroupRole = { group: string; role_id: number };

let adminToken = "";
let loginWindow = { start: 0, count: 0 };

async function spendLogins(count: number) {
  const now = Date.now();
  if (now - loginWindow.start > LOGIN_WINDOW_MS) {
    loginWindow = { start: now, count: 0 };
  }
  if (loginWindow.count + count > LOGIN_LIMIT) {
    const wait = loginWindow.start + LOGIN_WINDOW_MS - now;
    test.info().setTimeout(test.info().timeout + wait);
    await new Promise((resolve) => setTimeout(resolve, wait));
    loginWindow = { start: Date.now(), count: 0 };
  }
  loginWindow.count += count;
}

async function passwordLogin(
  request: APIRequestContext,
  email = ADMIN_EMAIL,
  password = ADMIN_PASSWORD,
) {
  await spendLogins(1);
  return request.post("/api/v1/auth/login", { data: { email, password } });
}

const userPath = (email: string) =>
  `/api/v1/users/${encodeURIComponent(email)}`;

async function configureOIDC(
  request: APIRequestContext,
  opts: { enabled?: boolean; groupRoles?: GroupRole[] } = {},
) {
  const groupRoles = opts.groupRoles ?? [];
  await json(request, "put", "/api/v1/oidc", {
    token: adminToken,
    data: {
      enabled: opts.enabled ?? true,
      external_url: EXTERNAL_URL,
      issuer: ISSUER,
      client_id: CLIENT_ID,
      client_secret: CLIENT_SECRET,
      display_name: "Keycloak",
      scopes: ["openid", "email", "profile"],
      session_lifetime_seconds: LIFETIME_SECONDS,
      trust_unverified_email: false,
      ca_certificate: CA_CERTIFICATE,
      groups_claim: groupRoles.length > 0 ? "groups" : "",
      group_roles: groupRoles,
    },
  });
}

async function keycloakLogin(page: Page, username: string, start = "/login") {
  await spendLogins(1);
  await page.goto(start);
  await page.getByRole("link", { name: "Log in with Keycloak" }).click();
  await page.waitForURL((url) => url.host === `keycloak:${KEYCLOAK_PORT}`);
  await page.locator("#username").fill(username);
  await page.locator("#password").fill(`${username}-password`);
  await page.locator("#kc-login").click();
  await page.waitForURL((url) => url.host === `localhost:${CORE_PORT}`);
}

async function expectSignedIn(page: Page, path?: string) {
  await expect(
    page.getByRole("button", { name: "account menu" }),
  ).toBeVisible();
  expect(new URL(page.url()).pathname).not.toBe("/login");
  if (path) await expect(page).toHaveURL(new RegExp(`${path}$`));
}

async function signOutOfKeycloak(context: BrowserContext) {
  await context.clearCookies();
}

async function uiPasswordLogin(page: Page, email: string, password: string) {
  await spendLogins(1);
  await page.getByLabel("Email").fill(email);
  await page.getByLabel(/^Password/).fill(password);
  await page.getByRole("button", { name: "Login" }).click();
  await expectSignedIn(page);
}

async function getUser(request: APIRequestContext, email: string) {
  return json<User>(request, "get", userPath(email), { token: adminToken });
}

async function deleteUserIfPresent(request: APIRequestContext, email: string) {
  const response = await send(request, "delete", userPath(email), {
    token: adminToken,
  });
  expect([200, 204, 404]).toContain(response.status());
}

async function sessionCookie(context: BrowserContext) {
  const cookies = await context.cookies(EXTERNAL_URL);
  return cookies.find((cookie) => cookie.name === SESSION_COOKIE);
}

async function refresh(page: Page) {
  return page.request.post("/api/v1/auth/refresh");
}

test.describe.configure({ mode: "serial" });

test.beforeAll(async ({ request }) => {
  await spendLogins(1);
  await ensureInitialized(request);
  const login = await passwordLogin(request);
  expect(login.ok()).toBe(true);
  adminToken = ((await login.json()) as { result: { token: string } }).result
    .token;
  await deleteUserIfPresent(request, BOB.email);
  await deleteUserIfPresent(request, CAROL.email);
});

test("Keycloak is discovered through the CA certificate", async ({
  request,
}) => {
  const discovery = await json<{
    issuer: string;
    pkce: boolean;
    token_endpoint_auth_method: string;
    userinfo_endpoint: string;
  }>(request, "post", "/api/v1/oidc/discovery", {
    token: adminToken,
    data: { issuer: ISSUER, ca_certificate: CA_CERTIFICATE },
  });

  expect(discovery.issuer).toBe(ISSUER);
  expect(discovery.pkce).toBe(true);
  expect(discovery.token_endpoint_auth_method).toBe("client_secret_basic");
  expect(discovery.userinfo_endpoint).toBe(
    `${ISSUER}/protocol/openid-connect/userinfo`,
  );
});

test("an admin configures single sign-on in the UI", async ({
  page,
  request,
}) => {
  await json(request, "put", "/api/v1/oidc", {
    token: adminToken,
    data: {
      enabled: false,
      external_url: "",
      issuer: "",
      client_id: "",
      client_secret: "former-secret",
      display_name: "",
      scopes: ["openid", "email"],
      session_lifetime_seconds: 86400,
      trust_unverified_email: false,
      ca_certificate: "",
      groups_claim: "",
      group_roles: [],
    },
  });

  expect((await passwordLogin(page.request)).ok()).toBe(true);
  await page.goto("/single-sign-on");
  await expect(page.getByRole("switch", { name: "Enabled" })).toBeDisabled();
  await page.getByRole("button", { name: "Edit provider" }).click();
  const dialog = page.getByRole("dialog", { name: "Edit Provider" });

  await expect(dialog.getByLabel("External URL")).toHaveValue(EXTERNAL_URL);
  await expect(
    dialog.getByRole("textbox", { name: "Redirect URL" }),
  ).toHaveValue(`${EXTERNAL_URL}/api/v1/auth/oidc/callback`);
  await expect(
    dialog.getByRole("textbox", { name: "Post-logout redirect URL" }),
  ).toHaveValue(`${EXTERNAL_URL}/login`);
  await dialog.getByLabel("Display name").fill("Keycloak");
  await dialog.getByLabel("Issuer URL").fill(ISSUER);
  await dialog.getByLabel("Client ID").fill(CLIENT_ID);
  await dialog.getByLabel("Client secret").fill(CLIENT_SECRET);
  await dialog.getByLabel("Scopes").fill("openid email profile");
  await dialog.getByRole("combobox", { name: "Unit" }).click();
  await page.getByRole("option", { name: "minutes" }).click();
  await dialog.getByLabel("Session lifetime").fill("5");
  await dialog
    .getByLabel("CA certificate", { exact: true })
    .fill(CA_CERTIFICATE);

  await dialog.getByRole("button", { name: "Test" }).click();
  await expect(dialog.getByRole("status")).toContainText("Works");

  await dialog.getByRole("button", { name: "Update" }).click();
  await expect(
    page.getByText("Single sign-on updated successfully."),
  ).toBeVisible();
  await expect(dialog).toBeHidden();

  const table = page.getByRole("table", { name: "Provider" });
  await expect(table.getByText("5 minutes")).toBeVisible();
  await expect(
    table.getByText(`${EXTERNAL_URL}/api/v1/auth/oidc/callback`),
  ).toBeVisible();
  await expect(table.getByText(`${EXTERNAL_URL}/login`)).toBeVisible();
  await expect(page.getByRole("switch", { name: "Enabled" })).not.toBeChecked();

  await page.getByRole("button", { name: "Edit roles from groups" }).click();
  const groups = page.getByRole("dialog", { name: "Edit Roles from Groups" });
  await groups.getByLabel("Groups claim").fill("groups");
  await groups.getByRole("button", { name: "Add group" }).click();
  await groups.getByRole("textbox", { name: /^Group 1/ }).fill(OPERATORS);
  await groups.getByRole("button", { name: "Update" }).click();
  await expect(groups).toBeHidden();
  await expect(
    page
      .getByRole("table", { name: "Roles from groups" })
      .getByText(`${OPERATORS} → Read Only`),
  ).toBeVisible();

  await page.getByRole("switch", { name: "Enabled" }).click();
  await expect(
    page.getByText("Single sign-on enabled successfully."),
  ).toBeVisible();
  await expect(page.getByRole("switch", { name: "Enabled" })).toBeChecked();

  const saved = await json<{
    enabled: boolean;
    issuer: string;
    client_secret_set: boolean;
    session_lifetime_seconds: number;
    groups_claim: string;
    group_roles: GroupRole[];
  }>(request, "get", "/api/v1/oidc", { token: adminToken });
  expect(saved).toMatchObject({
    enabled: true,
    issuer: ISSUER,
    client_secret_set: true,
    session_lifetime_seconds: LIFETIME_SECONDS,
    groups_claim: "groups",
    group_roles: [{ group: OPERATORS, role_id: RoleID.ReadOnly }],
  });
});

test("a user with a password links from their profile, then signs in with single sign-on", async ({
  page,
  request,
}) => {
  await configureOIDC(request);

  const existing = await send(request, "get", userPath(ALICE.email), {
    token: adminToken,
  });
  if (existing.status() === 404) {
    await json(request, "post", "/api/v1/users", {
      token: adminToken,
      data: {
        email: ALICE.email,
        password: ALICE_PASSWORD,
        role_id: RoleID.ReadOnly,
      },
    });
  } else if ((await getUser(request, ALICE.email)).oidc_linked) {
    await json(request, "delete", `${userPath(ALICE.email)}/oidc`, {
      token: adminToken,
    });
  }

  await keycloakLogin(page, ALICE.username);
  await expect(
    page.getByText(
      "This account has a password. Log in with it, then link your Keycloak account from your profile.",
    ),
  ).toBeVisible();
  await expect(page).toHaveURL(/\/login$/);
  expect(await sessionCookie(page.context())).toBeUndefined();
  expect((await getUser(request, ALICE.email)).oidc_linked).toBe(false);

  await uiPasswordLogin(page, ALICE.email, ALICE_PASSWORD);
  await page.goto("/profile");
  await spendLogins(1);
  await page.getByRole("link", { name: "Link Keycloak account" }).click();
  await page.waitForURL(/\/profile$/);
  await expect(
    page.getByText("Your account is linked. You can now log in with Keycloak."),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Link Keycloak account" }),
  ).toHaveCount(0);
  expect(await getUser(request, ALICE.email)).toMatchObject({
    oidc_linked: true,
    password_set: true,
    role_id: RoleID.ReadOnly,
  });

  await signOutOfKeycloak(page.context());
  await keycloakLogin(page, ALICE.username, "/subscribers");
  await expectSignedIn(page, "/subscribers");
  await expect(
    page.getByRole("heading", { level: 1, name: /^Subscribers/ }),
  ).toBeVisible();
  expect(await sessionCookie(page.context())).toBeDefined();
  expect((await refresh(page)).status()).toBe(200);
});

test("logging out of a single sign-on session also logs out of the identity provider", async ({
  page,
}) => {
  await keycloakLogin(page, ALICE.username);
  await expectSignedIn(page);

  await page.getByRole("button", { name: "account menu" }).click();
  await page.getByRole("menuitem", { name: "Log Out" }).click();
  await page.waitForURL(
    (url) =>
      url.host === `keycloak:${KEYCLOAK_PORT}` || url.pathname === "/login",
  );
  if (new URL(page.url()).host === `keycloak:${KEYCLOAK_PORT}`) {
    await page.locator("#kc-logout").click();
  }
  await page.waitForURL(
    (url) => url.host === `localhost:${CORE_PORT}` && url.pathname === "/login",
  );
  await expect(
    page.getByRole("link", { name: "Log in with Keycloak" }),
  ).toBeVisible();
  expect(await sessionCookie(page.context())).toBeUndefined();

  await spendLogins(1);
  await page.getByRole("link", { name: "Log in with Keycloak" }).click();
  await page.waitForURL((url) => url.host === `keycloak:${KEYCLOAK_PORT}`);
  await expect(page.locator("#username")).toBeVisible();
});

test("logging out of a password session stays in Ella Core", async ({
  page,
}) => {
  await page.goto("/login");
  await uiPasswordLogin(page, ALICE.email, ALICE_PASSWORD);

  const logout = page.waitForResponse("**/api/v1/auth/logout");
  await page.getByRole("button", { name: "account menu" }).click();
  await page.getByRole("menuitem", { name: "Log Out" }).click();
  expect((await logout).status()).toBe(204);
  await expect(page).toHaveURL(/\/login$/);
  expect(await sessionCookie(page.context())).toBeUndefined();
});

test("an unknown user is refused without a group mapping", async ({
  page,
  request,
}) => {
  await configureOIDC(request);

  await keycloakLogin(page, BOB.username);

  await expect(
    page.getByText(
      "Your account has not been set up in Ella Core. Contact an administrator.",
    ),
  ).toBeVisible();
  await expect(page).toHaveURL(/\/login$/);
  expect(await sessionCookie(page.context())).toBeUndefined();
  expect(
    (
      await send(request, "get", userPath(BOB.email), { token: adminToken })
    ).status(),
  ).toBe(404);
});

test("a user in a mapped group is created with the group's role", async ({
  page,
  request,
}) => {
  await configureOIDC(request, {
    groupRoles: [{ group: OPERATORS, role_id: RoleID.NetworkManager }],
  });

  await keycloakLogin(page, CAROL.username);
  await expectSignedIn(page);
  expect((await refresh(page)).status()).toBe(200);

  const user = await getUser(request, CAROL.email);
  expect(user.oidc_linked).toBe(true);
  expect(user.password_set).toBe(false);
  expect(user.role_id).toBe(RoleID.NetworkManager);
});

test("the role follows the groups from one login to the next", async ({
  page,
  request,
}) => {
  await configureOIDC(request, {
    groupRoles: [{ group: OPERATORS, role_id: RoleID.ReadOnly }],
  });

  await keycloakLogin(page, CAROL.username);
  await expectSignedIn(page);

  expect((await getUser(request, CAROL.email)).role_id).toBe(RoleID.ReadOnly);
});

test("the highest role among the user's mapped groups wins", async ({
  page,
  request,
}) => {
  await configureOIDC(request, {
    groupRoles: [
      { group: VIEWERS, role_id: RoleID.ReadOnly },
      { group: OPERATORS, role_id: RoleID.NetworkManager },
    ],
  });
  await keycloakLogin(page, CAROL.username);
  await expectSignedIn(page);
  expect((await getUser(request, CAROL.email)).role_id).toBe(
    RoleID.NetworkManager,
  );

  await configureOIDC(request, {
    groupRoles: [
      { group: OPERATORS, role_id: RoleID.ReadOnly },
      { group: VIEWERS, role_id: RoleID.Admin },
    ],
  });
  await signOutOfKeycloak(page.context());
  await keycloakLogin(page, CAROL.username);
  await expectSignedIn(page);
  expect((await getUser(request, CAROL.email)).role_id).toBe(RoleID.Admin);
});

test("the role of a user whose role comes from groups cannot be edited", async ({
  page,
  request,
}) => {
  await configureOIDC(request, {
    groupRoles: [{ group: OPERATORS, role_id: RoleID.NetworkManager }],
  });
  const before = (await getUser(request, CAROL.email)).role_id;

  const refused = await send(request, "put", userPath(CAROL.email), {
    token: adminToken,
    data: {
      role_id:
        before === RoleID.ReadOnly ? RoleID.NetworkManager : RoleID.ReadOnly,
    },
  });
  expect(refused.status()).toBe(409);
  expect((await getUser(request, CAROL.email)).role_id).toBe(before);

  expect((await passwordLogin(page.request)).ok()).toBe(true);
  await page.goto(`/users/${encodeURIComponent(CAROL.email)}`);
  await expect(page.getByText("From groups")).toBeVisible();
  await expect(page.getByRole("button", { name: "Edit role" })).toBeDisabled();
});

test("a user in no mapped group is refused", async ({ page, request }) => {
  await configureOIDC(request, {
    groupRoles: [{ group: OPERATORS, role_id: RoleID.NetworkManager }],
  });

  await keycloakLogin(page, BOB.username);

  await expect(
    page.getByText(
      "None of your groups gives access to Ella Core. Contact an administrator.",
    ),
  ).toBeVisible();
  await expect(page).toHaveURL(/\/login$/);
  expect(
    (
      await send(request, "get", userPath(BOB.email), { token: adminToken })
    ).status(),
  ).toBe(404);
});

test("a user created in the UI with no password is linked at first login and signs in only with single sign-on", async ({
  page,
  request,
}) => {
  await configureOIDC(request);

  expect((await passwordLogin(page.request)).ok()).toBe(true);
  await page.goto("/users");
  await page.getByRole("button", { name: "Create" }).click();
  const dialog = page.getByRole("dialog", { name: "Create User" });
  await dialog.getByLabel("Email").fill(BOB.email);
  await dialog.getByRole("switch", { name: "Password" }).uncheck();
  await dialog.getByRole("combobox", { name: /Role/ }).click();
  await page.getByRole("option", { name: "Read Only" }).click();
  await dialog.getByRole("button", { name: "Create" }).click();
  await expect(page.getByText("User created successfully.")).toBeVisible();
  await page.context().clearCookies();

  expect(await getUser(request, BOB.email)).toMatchObject({
    role_id: RoleID.ReadOnly,
    password_set: false,
    oidc_linked: false,
  });

  await keycloakLogin(page, BOB.username);
  await expectSignedIn(page, "/dashboard");

  expect(await getUser(request, BOB.email)).toMatchObject({
    role_id: RoleID.ReadOnly,
    oidc_linked: true,
  });
  expect(
    (
      await passwordLogin(request, BOB.email, `${BOB.username}-password`)
    ).status(),
  ).toBe(401);
  expect(
    (
      await send(request, "delete", `${userPath(BOB.email)}/password`, {
        token: adminToken,
      })
    ).status(),
  ).toBe(409);
});

test("an unlinked user without a password is linked again at their next login", async ({
  page,
  request,
}) => {
  await configureOIDC(request);

  await json(request, "delete", `${userPath(BOB.email)}/oidc`, {
    token: adminToken,
  });
  expect((await getUser(request, BOB.email)).oidc_linked).toBe(false);
  expect(
    (
      await send(request, "delete", `${userPath(BOB.email)}/oidc`, {
        token: adminToken,
      })
    ).status(),
  ).toBe(404);

  await keycloakLogin(page, BOB.username);
  await expectSignedIn(page);

  expect((await getUser(request, BOB.email)).oidc_linked).toBe(true);
});

test("the OIDC session lasts the session lifetime and ends when OIDC is disabled", async ({
  page,
  request,
}) => {
  await configureOIDC(request);

  const before = Date.now() / 1000;
  await keycloakLogin(page, ALICE.username);
  const after = Date.now() / 1000;
  await expectSignedIn(page);

  const cookie = await sessionCookie(page.context());
  expect(cookie).toBeDefined();
  expect(cookie!.expires).toBeGreaterThanOrEqual(
    Math.floor(before) + LIFETIME_SECONDS - 5,
  );
  expect(cookie!.expires).toBeLessThanOrEqual(
    Math.ceil(after) + LIFETIME_SECONDS + 5,
  );

  const refreshed = await refresh(page);
  expect(refreshed.status()).toBe(200);
  const { result } = (await refreshed.json()) as { result: { token: string } };
  const me = await json<User>(request, "get", "/api/v1/users/me", {
    token: result.token,
  });
  expect(me.email).toBe(ALICE.email);

  await configureOIDC(request, { enabled: false });

  expect((await refresh(page)).status()).toBe(401);
  expect(
    (
      await send(request, "get", "/api/v1/users/me", { token: result.token })
    ).status(),
  ).toBe(401);
  expect(
    (await send(request, "get", "/api/v1/users", { token: adminToken })).ok(),
  ).toBe(true);
});

test("an admin disables single sign-on after confirmation", async ({
  page,
  request,
}) => {
  await configureOIDC(request);

  expect((await passwordLogin(page.request)).ok()).toBe(true);
  await page.goto("/single-sign-on");
  await page.getByRole("switch", { name: "Enabled" }).click();
  const dialog = page.getByRole("dialog", { name: "Disable Single Sign-On" });
  await expect(dialog).toContainText(
    "Everyone who logged in with single sign-on is signed out.",
  );
  await dialog.getByRole("button", { name: "Disable" }).click();
  await expect(
    page.getByText("Single sign-on disabled successfully."),
  ).toBeVisible();
  await expect(page.getByRole("switch", { name: "Enabled" })).not.toBeChecked();

  const saved = await json<{ enabled: boolean; client_secret_set: boolean }>(
    request,
    "get",
    "/api/v1/oidc",
    { token: adminToken },
  );
  expect(saved).toMatchObject({ enabled: false, client_secret_set: true });
});
