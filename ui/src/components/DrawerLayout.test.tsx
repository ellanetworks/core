// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import DrawerLayout from "./DrawerLayout";

vi.mock("@/contexts/AuthContext", () => ({
  useAuth: () => ({ role: "Admin", setAuthData: vi.fn() }),
}));

vi.mock("@/queries/auth", () => ({ logout: vi.fn() }));

vi.mock("./SupportModal", () => ({ default: () => null }));

const renderAt = (path: string) =>
  render(
    <MemoryRouter initialEntries={[path]}>
      <DrawerLayout>
        <div />
      </DrawerLayout>
    </MemoryRouter>,
  );

const nav = () => screen.getByRole("navigation", { name: "Main" });

const current = () =>
  within(nav())
    .getAllByRole("link")
    .filter((el) => el.getAttribute("aria-current") === "page")
    .map((el) => el.textContent);

describe("DrawerLayout marks the current route for assistive tech", () => {
  it("sets aria-current on the exact-match route only", () => {
    renderAt("/dashboard");

    expect(current()).toEqual(["Dashboard"]);
  });

  it("keeps the section marked on a nested detail route", () => {
    renderAt("/radios/abc-123");

    expect(current()).toEqual(["Radios"]);
  });

  it("marks Traffic when its landing child is active", () => {
    renderAt("/traffic/usage");

    expect(current()).toEqual(["Traffic"]);
  });

  it("marks nothing when no nav route matches", () => {
    renderAt("/profile");

    expect(current()).toEqual([]);
  });

  it("does not mark an exact-only route from a deeper path", () => {
    renderAt("/audit-logs/42");

    expect(current()).toEqual([]);
  });

  it("leaves the external Support links unmarked", () => {
    renderAt("/dashboard");

    const doc = screen.getByRole("link", { name: /Documentation/ });
    expect(doc).not.toHaveAttribute("aria-current");
  });
});
