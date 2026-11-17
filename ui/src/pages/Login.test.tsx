// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { afterEach, describe, it, expect } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes, useLocation } from "react-router-dom";
import { renderWithProviders } from "@/test/renderWithProviders";
import { setupApiServer, httpError } from "@/test/apiServer";
import { rememberReturnPath } from "@/utils/returnPath";
import Login from "./Login";

const api = setupApiServer();

afterEach(() => sessionStorage.clear());

const serve = ({
  session = false,
  sso = { enabled: false } as { enabled: boolean; display_name?: string },
} = {}) => {
  api.get("/api/v1/status", () => ({ initialized: true }));
  api.post("/api/v1/auth/refresh", () =>
    session ? { token: "token" } : httpError(401, "no session"),
  );
  api.get("/api/v1/auth/oidc", () => sso);
};

const Elsewhere = () => (
  <span data-testid="elsewhere">{useLocation().pathname}</span>
);

const renderLogin = (entry: string | { pathname: string; state: unknown }) =>
  renderWithProviders(
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="*" element={<Elsewhere />} />
    </Routes>,
    { initialEntries: [entry] },
  );

const location = () => screen.queryByTestId("elsewhere")?.textContent;

describe("Login", () => {
  it("offers single sign-on when it is on", async () => {
    serve({ sso: { enabled: true, display_name: "Keycloak" } });
    renderLogin("/login");

    const sso = await screen.findByRole("link", {
      name: "Log in with Keycloak",
    });
    expect(sso).toHaveAttribute("href", "/api/v1/auth/oidc/login");
    await waitFor(() => expect(sso).toHaveFocus());
    expect(screen.getByRole("button", { name: "Login" })).toBeVisible();
  });

  it("offers only the password without single sign-on", async () => {
    serve();
    renderLogin("/login");

    expect(
      await screen.findByRole("button", { name: "Login" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: /Log in with/ }),
    ).not.toBeInTheDocument();
    expect(screen.getByLabelText(/Email/)).toHaveFocus();
  });

  it("says why single sign-on failed", async () => {
    serve({ sso: { enabled: true, display_name: "Keycloak" } });
    renderLogin("/login?error=no_group");

    expect(
      await screen.findByText(
        "None of your groups gives access to Ella Core. Contact an administrator.",
      ),
    ).toBeVisible();
  });

  it("asks a user with a password to link from the profile", async () => {
    serve({ sso: { enabled: true, display_name: "Keycloak" } });
    renderLogin("/login?error=link_required");

    expect(
      await screen.findByText(
        "This account has a password. Log in with it, then link your Keycloak account from your profile.",
      ),
    ).toBeVisible();
  });

  it("gives a general message for an unknown single sign-on error", async () => {
    serve();
    renderLogin("/login?error=whatever");

    expect(
      await screen.findByText(
        "Single sign-on failed. Try again, or contact an administrator.",
      ),
    ).toBeVisible();
  });

  it("remembers the page asked for across single sign-on", async () => {
    const user = userEvent.setup();
    serve({ sso: { enabled: true, display_name: "Keycloak" } });
    renderLogin({ pathname: "/login", state: { from: "/subscribers" } });

    const sso = await screen.findByRole("link", {
      name: "Log in with Keycloak",
    });
    sso.addEventListener("click", (e) => e.preventDefault());
    await user.click(sso);

    expect(
      JSON.parse(sessionStorage.getItem("core.returnPath") ?? "null"),
    ).toMatchObject({ path: "/subscribers" });
  });

  it("returns to the remembered page once single sign-on succeeds", async () => {
    rememberReturnPath("/subscribers");
    serve({ session: true });
    renderLogin("/login");

    await waitFor(() => expect(location()).toBe("/subscribers"));
    expect(sessionStorage.getItem("core.returnPath")).toBeNull();
  });

  it("goes to the dashboard after single sign-on with nothing remembered", async () => {
    serve({ session: true });
    renderLogin("/login");

    await waitFor(() => expect(location()).toBe("/dashboard"));
  });
});
